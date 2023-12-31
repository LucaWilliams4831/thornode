package keeperv1

import (
	. "gopkg.in/check.v1"
)

type KeeperKeygenSuite struct{}

var _ = Suite(&KeeperKeygenSuite{})

func (s *KeeperKeygenSuite) TestKeeperKeygen(c *C) {
	var err error
	ctx, k := setupKeeperForTest(c)

	keygenBlock := NewKeygenBlock(1)
	keygenMembers := []string{GetRandomPubKey().String(), GetRandomPubKey().String(), GetRandomPubKey().String()}
	keygen, err := NewKeygen(ctx.BlockHeight(), keygenMembers, AsgardKeygen)
	c.Assert(err, IsNil)
	c.Assert(keygen.IsEmpty(), Equals, false)
	keygenBlock.Keygens = append(keygenBlock.Keygens, keygen)
	k.SetKeygenBlock(ctx, keygenBlock)

	keygenBlock, err = k.GetKeygenBlock(ctx, 1)
	c.Assert(err, IsNil)
	c.Assert(keygenBlock, NotNil)
	c.Assert(keygenBlock.Height, Equals, int64(1))

	keygenBlock, err = k.GetKeygenBlock(ctx, 100)
	c.Assert(err, IsNil)
	c.Assert(keygenBlock, NotNil)

	iter := k.GetKeygenBlockIterator(ctx)
	defer iter.Close()
}
