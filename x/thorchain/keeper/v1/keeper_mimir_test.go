package keeperv1

import (
	. "gopkg.in/check.v1"
)

type KeeperMimirSuite struct{}

var _ = Suite(&KeeperMimirSuite{})

func (s *KeeperMimirSuite) TestMimir(c *C) {
	ctx, k := setupKeeperForTest(c)

	k.SetMimir(ctx, "foo", 14)

	val, err := k.GetMimir(ctx, "foo")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(14))

	val, err = k.GetMimir(ctx, "bogus")
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(-1))

	// test that releasing the kraken is ignored (has no effect on other mimir keys)
	k.SetMimir(ctx, KRAKEN, 0)
	val, err = k.GetMimir(ctx, "foo")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(14))
	c.Check(k.GetMimirIterator(ctx), NotNil)

	addr := GetRandomBech32Addr()
	k.SetNodePauseChain(ctx, addr)
	pause := k.GetNodePauseChain(ctx, addr)
	c.Assert(pause, Equals, int64(18))
}
