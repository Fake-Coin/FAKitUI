package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	core "fakco.in/FAKitCore"
	"fakco.in/fakutil"

	"github.com/zserge/webview"
)

type app struct {
	view webview.WebView
}

func (a *app) run(keyPhrase string) {
	privKey := core.BRBIP39DeriveKey(keyPhrase)
	wallet := a.InitWallet(privKey)
	pm := InitPM(wallet)
	go pm.Connect()

	for range time.NewTicker(500 * time.Millisecond).C {
		p := pm.Progress()
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
	})
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

func appConnect(w webview.WebView, data string) {
	log.Println(data)
	a := &app{w}
	go a.run(data)
}

func main() {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	startPage := filepath.Join(dir, "index.html")

	view := webview.New(webview.Settings{
		Title:     "FAKit Desktop",
		URL:       startPage,
		Width:     800,
		Height:    600,
		Resizable: true,
		Debug:     true,
		ExternalInvokeCallback: appConnect,
	})

	view.Run()
}
