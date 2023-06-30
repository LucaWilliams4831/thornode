package keeperv1

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperSwapSlipSuite struct{}

var _ = Suite(&KeeperSwapSlipSuite{})

func (s *KeeperSwapSlipSuite) TestSwapSlip(c *C) {
	ctx, k := setupKeeperForTest(c)

	ctx = ctx.WithBlockHeight(10)
	height := ctx.BlockHeight()
	err := k.AddToSwapSlip(ctx, common.BTCAsset, cosmos.NewInt(200))
	c.Assert(err, IsNil)
	err = k.AddToSwapSlip(ctx, common.BNBAsset, cosmos.NewInt(300))
	c.Assert(err, IsNil)

	i, err := k.GetPoolSwapSlip(ctx, height, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Check(i.Int64(), Equals, int64(200), Commentf("%d", i.Int64()))

	i, err = k.GetPoolSwapSlip(ctx, height, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Check(i.Int64(), Equals, int64(300), Commentf("%d", i.Int64()))
}

func (s *KeeperSwapSlipSuite) TestRollupSwapSlip(c *C) {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(10)

	asset := common.BNBAsset
	targetCount := int64(5)

	for i := int64(1); i <= targetCount+1; i++ {
		ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
		err := k.AddToSwapSlip(ctx, asset, cosmos.NewInt(i))
		c.Assert(err, IsNil)

		amt, err := k.RollupSwapSlip(ctx, targetCount, asset)
		c.Assert(err, IsNil)
		switch i {
		case 1:
			// no swap slips in previous block, therefore its zero
			c.Assert(amt.Int64(), Equals, int64(0), Commentf("%d", amt.Int64()))
		case 2:
			// previous block was 1
			c.Assert(amt.Int64(), Equals, int64(1), Commentf("%d", amt.Int64()))
		case 3:
			// 1 + 2
			c.Assert(amt.Int64(), Equals, int64(3), Commentf("%d", amt.Int64()))
		case 4:
			// 1 + 2 + 3
			c.Assert(amt.Int64(), Equals, int64(6), Commentf("%d", amt.Int64()))
		case 5:
			// 1 + 2 + 3 + 4
			c.Assert(amt.Int64(), Equals, int64(10), Commentf("%d", amt.Int64()))
		case 6:
			// 1 + 2 + 3 + 4 + 5 - 1
			// this IS NOT 15 because we should shave off the first swap slip which was one
			c.Assert(amt.Int64(), Equals, int64(14), Commentf("%d", amt.Int64()))
		}
	}

	// test reset, by setting the target count to a lower number than it was
	// before
	amt, err := k.RollupSwapSlip(ctx, 1, asset)
	c.Assert(err, IsNil)
	c.Assert(amt.IsZero(), Equals, true)
}
