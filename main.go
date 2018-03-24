package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"time"

	core "fakco.in/FAKitCore"
	"fakco.in/fakd/chaincfg"
	"fakco.in/fakutil"

	qrcode "github.com/skip2/go-qrcode"
	"github.com/zserge/webview"
)

const (
	MIN_FEE = 100000
	COIN    = 100000000
)

type app struct {
	view    webview.WebView
	wallet  *core.BRWallet
	peerMgr *core.BRPeerManager
	key     core.PrivateKey
}

func (a *app) run(keyPhrase string) {
	a.key = core.BRBIP39DeriveKey(keyPhrase)

	a.wallet = a.InitWallet(a.key)
	a.wallet.SetFeePerKB(MIN_FEE)

	a.peerMgr = InitPM(a.wallet)
	go a.peerMgr.Connect()

	a.view.Dispatch(func() {
		a.view.Eval(fmt.Sprintf("app.address = %q", a.wallet.ReceiveAddress()))
	})

	for range time.NewTicker(500 * time.Millisecond).C {
		p := a.peerMgr.Progress()
		a.updateProgress(p)
		if p == 1 {
			break
		}
	}
}

func (a *app) InitWallet(privKey core.PrivateKey) *core.BRWallet {
	mpk := core.BRBIP32MasterPubKey(privKey)
	wallet := core.BRWalletNew(nil, mpk)
	wallet.BalanceChanged = func(balance uint64) {
		a.updateBalance(balance)
	}
	wallet.TXAdded = func(brTx *core.BRTransaction) {
		tx, err := fakutil.NewTxFromBytes(brTx.Serialize())
		if err != nil {
			log.Println(err)
			return
		}
		a.addTX(tx)
	}
	return wallet
}

func InitPM(w *core.BRWallet) *core.BRPeerManager {
	return core.BRPeerManagerNewMainNet(w, 0, nil, nil)
}

func (a *app) updateProgress(p float64) {
	a.view.Dispatch(func() {
		a.view.Eval(fmt.Sprintf("app.progress = %.2f", p*100))
	})
}

func (a *app) updateBalance(b uint64) {
	a.view.Dispatch(func() {
		a.view.Eval(fmt.Sprintf("app.balance = %d", b))
		a.view.Eval(fmt.Sprintf("app.address = %q", a.wallet.ReceiveAddress()))
	})
}

func (a *app) sendTo(addr string, amount uint64) error {
	if !core.BRAddressIsValid(addr) {
		return errors.New("invalid send address")
	}

	balance := a.wallet.Balance()
	if balance < amount+MIN_FEE {
		return errors.New("not enough funds")
	}

	tx := a.wallet.CreateTransaction(amount, addr)
	if tx == nil {
		return errors.New("unable to create transaction")
	}

	if !a.wallet.SignTransaction(tx, a.key) {
		return errors.New("unable to sign transaction")
	}

	a.peerMgr.PublishTx(tx)

	return nil
}

type apiMsg struct {
	Fn   string          `json:"fn"`
	Data json.RawMessage `json:"data"`
}

type sendReq struct {
	Address string `json:"address"`
	Amount  uint64 `json:"amount"`
}

func (a *app) event(w webview.WebView, data string) {
	var msg apiMsg
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		log.Fatal(err)
	}

	switch msg.Fn {
	case "connect":
		var phrase string
		if err := json.Unmarshal(msg.Data, &phrase); err != nil {
			log.Fatal(err)
		}

		log.Printf("[CONNECT] - %q\n", phrase)

		go a.run(phrase)
	case "send":
		var req sendReq
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			log.Fatal(err)
		}

		log.Println(req)

		if err := a.sendTo(req.Address, req.Amount); err != nil {
			log.Println(err)
		}
	default:
		panic("unknown api call made")
	}
}

type ViewTX struct {
	Version  int32  `json:"version"`
	Index    int    `json:"index"`
	LockTime uint32 `json:"lock_time"`
	Hash     string `json:"hash"`
	Value    int64  `json:"value"`
}

func (a *app) addTX(tx *fakutil.Tx) {
	mtx := tx.MsgTx()

	var value int64
	for _, o := range mtx.TxOut {
		value += o.Value
	}

	js, err := json.Marshal(ViewTX{
		Version:  mtx.Version,
		Index:    tx.Index(),
		LockTime: mtx.LockTime,
		Hash:     tx.Hash().String(),
		Value:    value,
	})
	if err != nil {
		log.Println(err)
		return
	}

	a.view.Dispatch(func() {
		a.view.Eval(fmt.Sprintf("app.tableData.unshift(%s)", js))
	})
}

func main() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/", http.FileServer(assetFS()))
		mux.HandleFunc("/img/", func(w http.ResponseWriter, req *http.Request) {
			_, addr := filepath.Split(req.URL.Path)
			payAddr, err := fakutil.DecodeAddress(addr, &chaincfg.MainNetParams)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			png, err := qrcode.Encode(fmt.Sprintf("fakecoin:%s", payAddr.String()), qrcode.Medium, 256)
			if err != nil {
				log.Println(req.URL, ">", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			if _, err = w.Write(png); err != nil {
				log.Println(req.URL, ">", err)
			}
		})

		log.Fatal(http.Serve(ln, mux))
	}()

	log.Println("http://" + ln.Addr().String())

	a := new(app)
	view := webview.New(webview.Settings{
		Title:     "FAKit Desktop",
		URL:       "http://" + ln.Addr().String() + "/index.html",
		Width:     800,
		Height:    400,
		Resizable: true,
		Debug:     true,
		ExternalInvokeCallback: a.event,
	})
	a.view = view

	view.Run()
}
