package keeperv1

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperPoolSuite struct{}

var _ = Suite(&KeeperPoolSuite{})

func (s *KeeperPoolSuite) TestPool(c *C) {
	ctx, k := setupKeeperForTest(c)

	c.Check(k.SetPool(ctx, Pool{}), NotNil) // empty asset should error

	pool := NewPool()
	pool.Asset = common.BNBAsset

	err := k.SetPool(ctx, pool)
	c.Assert(err, IsNil)
	pool, err = k.GetPool(ctx, pool.Asset)
	c.Assert(err, IsNil)
	c.Check(pool.Asset.Equals(common.BNBAsset), Equals, true)
	c.Check(k.PoolExist(ctx, common.BNBAsset), Equals, true)
	c.Check(k.PoolExist(ctx, common.BTCAsset), Equals, false)

	pools, err := k.GetPools(ctx)
	c.Assert(err, IsNil)
	c.Assert(pools, HasLen, 1)

	p, err := k.GetPool(ctx, common.BTCAsset)
	c.Check(err, IsNil)
	c.Check(p.Valid(), NotNil)
}

func (s *KeeperPoolSuite) TestPoolLUVI(c *C) {
	luvi := cosmos.NewUint(12345)

	ctx, k := setupKeeperForTest(c)
	k.SetPoolLUVI(ctx, common.BTCAsset, luvi)
	luvi2, err := k.GetPoolLUVI(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(luvi.Uint64(), Equals, luvi2.Uint64())
}
