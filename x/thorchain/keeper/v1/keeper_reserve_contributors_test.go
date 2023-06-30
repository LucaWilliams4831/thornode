package keeperv1

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperReserveContributorsSuite struct{}

var _ = Suite(&KeeperReserveContributorsSuite{})

func (KeeperReserveContributorsSuite) TestReserveContributors(c *C) {
	ctx, k := setupKeeperForTest(c)

	poolFee := cosmos.NewUint(common.One * 100)
	FundModule(c, ctx, k, AsgardName, poolFee.Uint64())
	asgardBefore := k.GetRuneBalanceOfModule(ctx, AsgardName)
	reserveBefore := k.GetRuneBalanceOfModule(ctx, ReserveName)

	c.Assert(k.AddPoolFeeToReserve(ctx, poolFee), IsNil)

	asgardAfter := k.GetRuneBalanceOfModule(ctx, AsgardName)
	reserveAfter := k.GetRuneBalanceOfModule(ctx, ReserveName)
	c.Assert(asgardAfter.String(), Equals, asgardBefore.Sub(poolFee).String())
	c.Assert(reserveAfter.String(), Equals, reserveBefore.Add(poolFee).String())

	bondFee := cosmos.NewUint(common.One * 200)
	FundModule(c, ctx, k, BondName, bondFee.Uint64())
	bondBefore := k.GetRuneBalanceOfModule(ctx, BondName)
	reserveBefore = reserveAfter

	c.Assert(k.AddBondFeeToReserve(ctx, bondFee), IsNil)

	bondAfter := k.GetRuneBalanceOfModule(ctx, BondName)
	reserveAfter = k.GetRuneBalanceOfModule(ctx, ReserveName)
	c.Assert(bondAfter.String(), Equals, bondBefore.Sub(bondFee).String())
	c.Assert(reserveAfter.String(), Equals, reserveBefore.Add(bondFee).String())
}
