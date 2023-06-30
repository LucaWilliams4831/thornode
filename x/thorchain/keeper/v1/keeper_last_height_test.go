package keeperv1

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
)

type KeeperLastHeightSuite struct{}

var _ = Suite(&KeeperLastHeightSuite{})

func (s *KeeperLastHeightSuite) TestLastHeight_EmptyKeeper(c *C) {
	ctx, k := setupKeeperForTest(c)

	last, err := k.GetLastSignedHeight(ctx)
	c.Assert(err, IsNil)
	c.Check(last, Equals, int64(0))

	last, err = k.GetLastChainHeight(ctx, common.BNBChain)
	c.Assert(err, IsNil)
	c.Check(last, Equals, int64(0))
}

func (s *KeeperLastHeightSuite) TestLastHeight_SetKeeperSingleChain(c *C) {
	ctx, k := setupKeeperForTest(c)

	err := k.SetLastSignedHeight(ctx, 12)
	c.Assert(err, IsNil)
	last, err := k.GetLastSignedHeight(ctx)
	c.Assert(err, IsNil)
	c.Check(last, Equals, int64(12))
	c.Check(k.SetLastSignedHeight(ctx, 10), IsNil)

	err = k.SetLastChainHeight(ctx, common.BNBChain, 14)
	c.Assert(err, IsNil)
	last, err = k.GetLastChainHeight(ctx, common.BNBChain)
	c.Assert(err, IsNil)
	c.Check(last, Equals, int64(14))
}

func (s *KeeperLastHeightSuite) TestLastHeight_SetKeeperMultipleChains(c *C) {
	ctx, k := setupKeeperForTest(c)
	err := k.SetLastChainHeight(ctx, common.BTCChain, 23)
	c.Assert(err, IsNil)
	err = k.SetLastChainHeight(ctx, common.BNBChain, 14)
	c.Assert(err, IsNil)
	last, err := k.GetLastChainHeight(ctx, common.BTCChain)
	c.Assert(err, IsNil)
	c.Check(last, Equals, int64(23))
	last, err = k.GetLastChainHeight(ctx, common.BNBChain)
	c.Assert(err, IsNil)
	c.Check(last, Equals, int64(14))
	c.Check(k.SetLastChainHeight(ctx, common.BTCChain, 20), NotNil)
}

func (s *KeeperLastHeightSuite) TestGetLastChainHeights(c *C) {
	ctx, k := setupKeeperForTest(c)
	err := k.SetLastChainHeight(ctx, common.BTCChain, 23)
	c.Assert(err, IsNil)
	err = k.SetLastChainHeight(ctx, common.BNBChain, 14)
	c.Assert(err, IsNil)
	last, err := k.GetLastChainHeight(ctx, common.BTCChain)
	c.Assert(err, IsNil)
	c.Check(last, Equals, int64(23))
	last, err = k.GetLastChainHeight(ctx, common.BNBChain)
	c.Assert(err, IsNil)
	c.Check(last, Equals, int64(14))
	result, err := k.GetLastChainHeights(ctx)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
}

func (s *KeeperLastHeightSuite) TestSetLastObserveHeight(c *C) {
	ctx, k := setupKeeperForTest(c)
	na := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetLastObserveHeight(ctx, common.BTCChain, na.NodeAddress, 1024), IsNil)
	c.Assert(k.SetLastObserveHeight(ctx, common.BTCChain, na.NodeAddress, 1025), IsNil)
	result, err := k.GetLastObserveHeight(ctx, na.NodeAddress)
	c.Assert(err, IsNil)
	h, ok := result[common.BTCChain]
	c.Assert(ok, Equals, true)
	c.Assert(h, Equals, int64(1025))
	h, ok = result[common.BNBChain]
	c.Assert(ok, Equals, false)
	c.Assert(h, Equals, int64(0))

	c.Assert(k.SetLastObserveHeight(ctx, common.BNBChain, na.NodeAddress, 1114), IsNil)
	result, err = k.GetLastObserveHeight(ctx, na.NodeAddress)
	c.Assert(err, IsNil)
	h, ok = result[common.BTCChain]
	c.Assert(ok, Equals, true)
	c.Assert(h, Equals, int64(1025))
	h, ok = result[common.BNBChain]
	c.Assert(ok, Equals, true)
	c.Assert(h, Equals, int64(1114))
}
