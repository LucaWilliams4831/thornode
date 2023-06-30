package keeperv1

import (
	. "gopkg.in/check.v1"
)

type KeeperTssSuite struct{}

var _ = Suite(&KeeperTssSuite{})

func (s *KeeperTssSuite) TestTssVoter(c *C) {
	ctx, k := setupKeeperForTest(c)

	pk := GetRandomPubKey()
	voter := NewTssVoter("hello", nil, pk)

	v, err1 := k.GetTssVoter(ctx, voter.ID)
	c.Check(err1, IsNil)
	c.Check(v.IsEmpty(), Equals, true)

	k.SetTssVoter(ctx, voter)
	voter, err := k.GetTssVoter(ctx, voter.ID)
	c.Assert(err, IsNil)
	c.Check(voter.ID, Equals, "hello")
	c.Check(voter.PoolPubKey.Equals(pk), Equals, true)
	iter := k.GetTssVoterIterator(ctx)
	c.Check(iter, NotNil)
	iter.Close()
}

func (s *KeeperTssSuite) TestTssKeygenMetric(c *C) {
	ctx, k := setupKeeperForTest(c)
	pk := GetRandomPubKey()
	metric, err := k.GetTssKeygenMetric(ctx, pk)
	c.Assert(err, IsNil)
	c.Assert(metric, NotNil)
	metric.AddNodeTssTime(GetRandomBech32Addr(), 1024)
	k.SetTssKeygenMetric(ctx, metric)

	metric1, err := k.GetTssKeygenMetric(ctx, pk)
	c.Assert(err, IsNil)
	c.Assert(metric1, NotNil)
	c.Assert(metric1.NodeTssTimes, HasLen, 1)
}

func (s *KeeperTssSuite) TestTssKeysignMetric(c *C) {
	ctx, k := setupKeeperForTest(c)
	txID := GetRandomTxHash()
	metric, err := k.GetTssKeysignMetric(ctx, txID)
	c.Assert(err, IsNil)
	c.Assert(metric, NotNil)
	metric.AddNodeTssTime(GetRandomBech32Addr(), 1024)
	k.SetTssKeysignMetric(ctx, metric)

	metric1, err := k.GetTssKeysignMetric(ctx, txID)
	c.Assert(err, IsNil)
	c.Assert(metric1, NotNil)
	c.Assert(metric1.NodeTssTimes, HasLen, 1)
}
