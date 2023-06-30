//go:build stagenet
// +build stagenet

package ethereum

import (
	"gitlab.com/thorchain/thornode/common"
)

var whitelistSmartContractAddress = append(
	[]common.Address{
		// XRUNE
		common.Address(`0x69fa0feE221AD11012BAb0FdB45d444D3D2Ce71c`),
		// THORSwap Faucet
		common.Address(`0xB73B8E66196f2AF0762833304e3f15dB2e8Df0c3`),
		// RangoV1
		common.Address(`0x0e3EB2eAB0e524b69C79E24910f4318dB46bAa9c`),
	},
	LatestAggregatorContracts()...,
)
