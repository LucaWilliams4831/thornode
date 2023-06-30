//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package avaxtokens

import (
	_ "embed"
)

//go:embed avax_mainnet_V95.json
var AVAXTokenListRawV95 []byte

//go:embed avax_mainnet_V101.json
var AVAXTokenListRawV101 []byte
