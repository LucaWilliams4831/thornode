//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package cmd

const (
	Bech32PrefixAccAddr  = "thor"
	Bech32PrefixAccPub   = "thorpub"
	Bech32PrefixValAddr  = "thorv"
	Bech32PrefixValPub   = "thorvpub"
	Bech32PrefixConsAddr = "thorc"
	Bech32PrefixConsPub  = "thorcpub"
)
