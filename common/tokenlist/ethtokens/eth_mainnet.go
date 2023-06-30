//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package ethtokens

import (
	_ "embed"
)

//go:embed eth_mainnet_V114.json
var ETHTokenListRawV114 []byte

//go:embed eth_mainnet_V108.json
var ETHTokenListRawV108 []byte

//go:embed eth_mainnet_V101.json
var ETHTokenListRawV101 []byte

//go:embed eth_mainnet_V97.json
var ETHTokenListRawV97 []byte

//go:embed eth_mainnet_V93.json
var ETHTokenListRawV93 []byte

//go:embed eth_mainnet_V95.json
var ETHTokenListRawV95 []byte
