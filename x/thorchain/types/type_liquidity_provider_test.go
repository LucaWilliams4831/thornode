package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
)

type LiquidityProviderSuite struct{}

var _ = Suite(&LiquidityProviderSuite{})

func (LiquidityProviderSuite) TestLiquidityProvider(c *C) {
	lp := LiquidityProvider{
		Asset:         common.BNBAsset,
		RuneAddress:   GetRandomBNBAddress(),
		AssetAddress:  GetRandomBTCAddress(),
		LastAddHeight: 12,
	}
	c.Check(lp.Valid(), IsNil)
	c.Check(len(lp.Key()) > 0, Equals, true)
	lp1 := LiquidityProvider{
		Asset:         common.BNBAsset,
		RuneAddress:   GetRandomBNBAddress(),
		AssetAddress:  GetRandomBTCAddress(),
		LastAddHeight: 0,
	}
	c.Check(lp1.Valid(), NotNil)

	lp2 := LiquidityProvider{
		Asset:         common.BNBAsset,
		RuneAddress:   common.NoAddress,
		AssetAddress:  common.NoAddress,
		LastAddHeight: 100,
	}
	c.Check(lp2.Valid(), NotNil)
}
