//go:build testnet || mocknet
// +build testnet mocknet

package ethereum

import (
	"gitlab.com/thorchain/thornode/common"
)

var whitelistSmartContractAddress = append(
	[]common.Address{
		// THORSwap Faucet
		common.Address(`0x83b0c5136790dDf6cA8D3fb3d220C757e0a91fBe`),
		// RangoV1
		common.Address(`0x0e81E5F5555Ece18b5F86F52a736476790eEF82d`),
	},
	LatestAggregatorContracts()...,
)
