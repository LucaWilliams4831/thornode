package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
	. "gopkg.in/check.v1"
)

type HandlerCommonOutboundSuite struct{}

var _ = Suite(&HandlerCommonOutboundSuite{})

func (s *HandlerCommonOutboundSuite) TestIsOutboundFakeGasTX(c *C) {
	coins := common.Coins{
		common.NewCoin(common.ETHAsset, cosmos.NewUint(1)),
	}
	gas := common.Gas{
		{Asset: common.ETHAsset, Amount: cosmos.NewUint(1)},
	}
	fakeGasTx := types.ObservedTx{
		Tx: common.NewTx("123", "0xabc", "0x123", coins, gas, "=:AVAX.AVAX:0x123"),
	}

	c.Assert(isOutboundFakeGasTX(fakeGasTx), Equals, true)

	coins = common.Coins{
		common.NewCoin(common.ETHAsset, cosmos.NewUint(100000)),
	}
	theftTx := types.ObservedTx{
		Tx: common.NewTx("123", "0xabc", "0x123", coins, gas, "=:AVAX.AVAX:0x123"),
	}
	c.Assert(isOutboundFakeGasTX(theftTx), Equals, false)

	coins = common.Coins{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(1)),
	}
	theftTx2 := types.ObservedTx{
		Tx: common.NewTx("123", "0xabc", "0x123", coins, gas, "=:AVAX.AVAX:0x123"),
	}
	c.Assert(isOutboundFakeGasTX(theftTx2), Equals, false)
}
