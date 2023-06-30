package types

import (
	. "gopkg.in/check.v1"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type ProtocolOwnedLiquiditySuite struct{}

var _ = Suite(&ProtocolOwnedLiquiditySuite{})

func (s *ProtocolOwnedLiquiditySuite) TestCalcNodeRewards(c *C) {
	pol := NewProtocolOwnedLiquidity()
	c.Check(pol.RuneDeposited.Uint64(), Equals, cosmos.ZeroUint().Uint64())
	c.Check(pol.RuneWithdrawn.Uint64(), Equals, cosmos.ZeroUint().Uint64())
}

func (s *ProtocolOwnedLiquiditySuite) TestCurrentDeposit(c *C) {
	pol := NewProtocolOwnedLiquidity()
	pol.RuneDeposited = cosmos.NewUint(100)
	pol.RuneWithdrawn = cosmos.NewUint(25)
	c.Check(pol.CurrentDeposit().Int64(), Equals, int64(75))

	pol = NewProtocolOwnedLiquidity()
	pol.RuneDeposited = cosmos.NewUint(25)
	pol.RuneWithdrawn = cosmos.NewUint(100)
	c.Check(pol.CurrentDeposit().Int64(), Equals, int64(-75))
}

func (s *ProtocolOwnedLiquiditySuite) PnL(c *C) {
	pol := NewProtocolOwnedLiquidity()
	pol.RuneDeposited = cosmos.NewUint(100)
	pol.RuneWithdrawn = cosmos.NewUint(25)
	c.Check(pol.PnL(cosmos.NewUint(30)).Int64(), Equals, int64(-45))

	pol = NewProtocolOwnedLiquidity()
	pol.RuneDeposited = cosmos.NewUint(25)
	pol.RuneWithdrawn = cosmos.NewUint(10)
	c.Check(pol.PnL(cosmos.NewUint(30)).Int64(), Equals, int64(15))
}
