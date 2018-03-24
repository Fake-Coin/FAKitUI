Vue.component('import-key', {
	template: `
<div>
	<p>Enter Your paper wallet key phrase below to continue.</p>
	<div class="ui internally celled grid wallet-input">
		<div v-for="i in [...Array(3).keys()]" class="row">
			<div v-for="j in [...Array(4).keys()]" class="four wide column ui transparent input">
				<input v-model="words[(i*4)+j]" v-bind:placeholder="(i*4)+j+1" type="text">
			</div>
		</div>
	</div>
	<div class="bip32-labled">
		<div v-for="w in words" v-if="w" class="ui label" v-bind:class="[bip32wordlist[w] ? 'green' : 'red']">{{ w }}</div>
	</div>
	<div class="row">
		<button class="ui primary basic button"@click="generatePhrase">New Wallet</button>
		<button class="ui secondary basic button" v-bind:class="{ disabled: !validPhrase() }" @click="connect">Connect</button>
	</div>
</div>
	`,
	data() {
		return {
			words: new Array(12).fill(""),
			bip32wordlist: bip32wordlist
		}
	},
	methods: {
		validPhrase: function() {
			return this.words.reduce((a,w) => a && this.bip32wordlist[w], true);
		},
		generatePhrase: function() {
			const bip32keys = Object.keys(this.bip32wordlist);
			const words = this.words;

			const indices = new Uint16Array(words.length);
			window.crypto.getRandomValues(indices);

			indices.forEach(function(r,i) {
				words[i] = bip32keys[r%bip32keys.length];
			});

			this.$forceUpdate();
		},
		connect: function() {
			window.external.invoke(JSON.stringify({fn: 'connect', data: this.words.join(' ')}));
		}
	}
});
