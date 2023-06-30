package keeperv1

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/constants"
)

type KeeperConfigSuite struct{}

var _ = Suite(&KeeperConfigSuite{})

func (s *KeeperConfigSuite) TestGetConfigInt64(c *C) {
	ctx, k := setupKeeperForTest(c)

	c.Assert(k.GetConfigInt64(ctx, constants.EmissionCurve), Equals, int64(6))

	k.SetMimir(ctx, constants.EmissionCurve.String(), 10)
	c.Assert(k.GetConfigInt64(ctx, constants.EmissionCurve), Equals, int64(10))

	k.SetMimir(ctx, constants.EmissionCurve.String(), -1)
	c.Assert(k.GetConfigInt64(ctx, constants.EmissionCurve), Equals, int64(6))
}
