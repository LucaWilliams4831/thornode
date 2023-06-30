package keeperv1

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperNetworkSuite struct{}

var _ = Suite(&KeeperNetworkSuite{})

func (KeeperNetworkSuite) TestNetwork(c *C) {
	ctx, k := setupKeeperForTest(c)
	vd, err := k.GetNetwork(ctx)
	c.Check(err, IsNil)
	c.Check(vd.BondRewardRune.Equal(cosmos.ZeroUint()), Equals, true)

	vd1 := NewNetwork()
	vd1.BondRewardRune = cosmos.NewUint(common.One * 100)
	err1 := k.SetNetwork(ctx, vd1)
	c.Assert(err1, IsNil)

	vd2, err2 := k.GetNetwork(ctx)
	c.Check(err2, IsNil)
	c.Check(vd2.BondRewardRune.Equal(vd1.BondRewardRune), Equals, true)
}

func (KeeperNetworkSuite) TestPOL(c *C) {
	ctx, k := setupKeeperForTest(c)
	pol, err := k.GetPOL(ctx)
	c.Check(err, IsNil)
	c.Check(pol.RuneDeposited.Equal(cosmos.ZeroUint()), Equals, true)

	pol.RuneDeposited = cosmos.NewUint(common.One * 100)
	err1 := k.SetPOL(ctx, pol)
	c.Assert(err1, IsNil)

	pol2, err2 := k.GetPOL(ctx)
	c.Check(err2, IsNil)
	c.Check(pol2.RuneDeposited.Uint64(), Equals, uint64(100*common.One))
}
