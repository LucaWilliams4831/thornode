//go:build testnet
// +build testnet

package aggregators

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
)

func DexAggregators(version semver.Version) []Aggregator {
	switch {
	case version.GTE(semver.MustParse("1.93.0")):
		return []Aggregator{
			// uniswap v2
			{common.ETHChain, `0x942c6dA485FD6cEf255853ef83a149d43A73F18a`},
			// uniswap v3
			{common.ETHChain, `0x7236D46c894Be8Af0C6b26Dd97608E396Db0f339`},
			// sushiswap
			{common.ETHChain, `0x7fD9bd7A2Cab44820DD2874859E461640F04542D`},
			// THORSwap Faucet
			{common.ETHChain, `0x83b0c5136790dDf6cA8D3fb3d220C757e0a91fBe`},
			// RangoThorchainOutputAggUniV2
			{common.ETHChain, `0x4e071DA486bB901BC802758f6d63BCFb7D88345b`},
			// RangoThorchainOutputAggUniV3
			{common.ETHChain, `0x128766D155615b53C7655cD78A1A870C110bfdE6`},
		}
	case version.GTE(semver.MustParse("0.1.0")):
		return []Aggregator{
			// uniswap v2
			{common.ETHChain, `0x942c6dA485FD6cEf255853ef83a149d43A73F18a`},
			// uniswap v3
			{common.ETHChain, `0x7236D46c894Be8Af0C6b26Dd97608E396Db0f339`},
			// sushiswap
			{common.ETHChain, `0x7fD9bd7A2Cab44820DD2874859E461640F04542D`},
			// THORSwap Faucet
			{common.ETHChain, `0x83b0c5136790dDf6cA8D3fb3d220C757e0a91fBe`},
		}
	default:
		return nil
	}
}
