package common

import (
	"github.com/btcsuite/btcd/chaincfg"
	dogchaincfg "github.com/eager7/dogd/chaincfg"
	ltcchaincfg "github.com/ltcsuite/ltcd/chaincfg"
	btypes "gitlab.com/thorchain/binance-sdk/common/types"
	. "gopkg.in/check.v1"
)

type ChainSuite struct{}

var _ = Suite(&ChainSuite{})

func (s ChainSuite) TestChain(c *C) {
	bnbChain, err := NewChain("bnb")
	c.Assert(err, IsNil)
	c.Check(bnbChain.Equals(BNBChain), Equals, true)
	c.Check(bnbChain.IsBNB(), Equals, true)
	c.Check(bnbChain.IsEmpty(), Equals, false)
	c.Check(bnbChain.String(), Equals, "BNB")

	_, err = NewChain("B") // too short
	c.Assert(err, NotNil)

	chains := Chains{"BNB", "BNB", "BTC"}
	c.Check(chains.Has("BTC"), Equals, true)
	c.Check(chains.Has("ETH"), Equals, false)
	uniq := chains.Distinct()
	c.Assert(uniq, HasLen, 2)

	algo := bnbChain.GetSigningAlgo()
	c.Assert(algo, Equals, SigningAlgoSecp256k1)

	c.Assert(BNBChain.GetGasAsset(), Equals, BNBAsset)
	c.Assert(BTCChain.GetGasAsset(), Equals, BTCAsset)
	c.Assert(ETHChain.GetGasAsset(), Equals, ETHAsset)
	c.Assert(LTCChain.GetGasAsset(), Equals, LTCAsset)
	c.Assert(BCHChain.GetGasAsset(), Equals, BCHAsset)
	c.Assert(DOGEChain.GetGasAsset(), Equals, DOGEAsset)
	c.Assert(EmptyChain.GetGasAsset(), Equals, EmptyAsset)

	c.Assert(BNBChain.AddressPrefix(MockNet), Equals, btypes.TestNetwork.Bech32Prefixes())
	c.Assert(BNBChain.AddressPrefix(TestNet), Equals, btypes.TestNetwork.Bech32Prefixes())
	c.Assert(BNBChain.AddressPrefix(MainNet), Equals, btypes.ProdNetwork.Bech32Prefixes())
	c.Assert(BNBChain.AddressPrefix(StageNet), Equals, btypes.ProdNetwork.Bech32Prefixes())

	c.Assert(BTCChain.AddressPrefix(MockNet), Equals, chaincfg.RegressionNetParams.Bech32HRPSegwit)
	c.Assert(BTCChain.AddressPrefix(TestNet), Equals, chaincfg.TestNet3Params.Bech32HRPSegwit)
	c.Assert(BTCChain.AddressPrefix(MainNet), Equals, chaincfg.MainNetParams.Bech32HRPSegwit)
	c.Assert(BTCAsset.Chain.AddressPrefix(StageNet), Equals, chaincfg.MainNetParams.Bech32HRPSegwit)

	c.Assert(LTCChain.AddressPrefix(MockNet), Equals, ltcchaincfg.RegressionNetParams.Bech32HRPSegwit)
	c.Assert(LTCChain.AddressPrefix(TestNet), Equals, ltcchaincfg.TestNet4Params.Bech32HRPSegwit)
	c.Assert(LTCChain.AddressPrefix(MainNet), Equals, ltcchaincfg.MainNetParams.Bech32HRPSegwit)
	c.Assert(LTCChain.AddressPrefix(StageNet), Equals, ltcchaincfg.MainNetParams.Bech32HRPSegwit)

	c.Assert(DOGEChain.AddressPrefix(MockNet), Equals, dogchaincfg.RegressionNetParams.Bech32HRPSegwit)
	c.Assert(DOGEChain.AddressPrefix(TestNet), Equals, dogchaincfg.TestNet3Params.Bech32HRPSegwit)
	c.Assert(DOGEChain.AddressPrefix(MainNet), Equals, dogchaincfg.MainNetParams.Bech32HRPSegwit)
	c.Assert(DOGEChain.AddressPrefix(StageNet), Equals, dogchaincfg.MainNetParams.Bech32HRPSegwit)
}
