//go:build mocknet
// +build mocknet

package aggregators

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
)

func DexAggregators(version semver.Version) []Aggregator {
	if version.GTE(semver.MustParse("0.1.0")) {
		return []Aggregator{
			// mocknet mock aggregator
			{common.ETHChain, `0x69800327b38A4CeF30367Dec3f64c2f2386f3848`},
			// mocknet avax aggregator
			{common.AVAXChain, `0x1429859428C0aBc9C2C47C8Ee9FBaf82cFA0F20f`},

			// the following contracts are latest mainnet - for forked EVM testing

			// TSAggregatorGeneric
			{common.ETHChain, `0xd31f7e39afECEc4855fecc51b693F9A0Cec49fd2`},
			// TSAggregator2LegUniswapV2 USDC
			{common.ETHChain, `0x3660dE6C56cFD31998397652941ECe42118375DA`},
			// RangoThorchainOutputAggUniV2
			{common.ETHChain, `0x2a7813412b8da8d18Ce56FE763B9eb264D8e28a8`},
			// RangoThorchainOutputAggUniV3
			{common.ETHChain, `0xbB8De86F3b041B3C084431dcf3159fE4827c5F0D`},
			// PangolinAggregator
			{common.AVAXChain, `0x7a68c37D8AFA3078f3Ad51D98eA23Fe57a8Ae21a`},
			// TSAggregatorUniswapV2 - short notation
			{common.ETHChain, `0x86904eb2b3c743400d03f929f2246efa80b91215`},
			// TSAggregatorSushiswap - short notation
			{common.ETHChain, `0xbf365e79aa44a2164da135100c57fdb6635ae870`},
			// TSAggregatorUniswapV3 100 - short notation
			{common.ETHChain, `0xbd68cbe6c247e2c3a0e36b8f0e24964914f26ee8`},
			// TSAggregatorUniswapV3 500 - short notation
			{common.ETHChain, `0xe4ddca21881bac219af7f217703db0475d2a9f02`},
			// TSAggregatorUniswapV3 3000 - short notation
			{common.ETHChain, `0x11733abf0cdb43298f7e949c930188451a9a9ef2`},
			// TSAggregatorUniswapV3 10000 - short notation
			{common.ETHChain, `0xb33874810e5395eb49d8bd7e912631db115d5a03`},
			// TSAggregatorPangolin
			{common.AVAXChain, `0x942c6dA485FD6cEf255853ef83a149d43A73F18a`},
			// TSAggregatorTraderJoe
			{common.AVAXChain, `0x3b7DbdD635B99cEa39D3d95Dbd0217F05e55B212`},
			// TSAggregatorAvaxGeneric
			{common.AVAXChain, `0x7C38b8B2efF28511ECc14a621e263857Fb5771d3`},
			// XDEFIAggregatorEthGeneric
			{common.ETHChain, `0x53E4DD4072A9a8ed56289e048f5BD5AA51c9Bf6E`},
			// XDEFIAggregatorEthUniswapV2
			{common.ETHChain, `0xeEe520b0DA1F8a9e4a0480F92CC4c5f6C027ef1E`},
			// XDEFIAggregatorAvaxGeneric
			{common.AVAXChain, `0xd0269244A876F7Bc600D1f38B03a9916864b73C6`},
			// XDEFIAggregatorAvaxTraderJoe
			{common.AVAXChain, `0x4ab34123A077aE294A39844f3e8df418d2A3D8c4`},
			// XDEFIAggregatorUniswapV3 100 - short notation
			{common.ETHChain, `0x88100E08e5287bA3445F95d448ABfF3113d82a4C`},
			// XDEFIAggregatorUniswapV3 500 - short notation
			{common.ETHChain, `0xC1faA12981160945903E0725888828E2d6a15821`},
			// XDEFIAggregatorUniswapV3 3000 - short notation
			{common.ETHChain, `0x7E019988299cd8038091D8d7fe38f7a1dd3f90F1`},
			// XDEFIAggregatorUniswapV3 10000 - short notation
			{common.ETHChain, `0x95B6b888a9fCc5BCA4A3004Df5E9498B63195F48`},
		}
	}
	return nil
}
