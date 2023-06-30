//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package bsctokens

import (
	_ "embed"
)

//go:embed bsc_mainnet_V111.json
var BSCTokenListRawV111 []byte
