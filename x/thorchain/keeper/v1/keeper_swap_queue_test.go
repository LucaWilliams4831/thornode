package keeperv1

import (
	. "gopkg.in/check.v1"
)

type KeeperSwapQueueSuite struct{}

var _ = Suite(&KeeperSwapQueueSuite{})

func (s *KeeperSwapQueueSuite) TestKeeperSwapQueue(c *C) {
	ctx, k := setupKeeperForTest(c)

	// not found
	_, err := k.GetSwapQueueItem(ctx, GetRandomTxHash(), 0)
	c.Assert(err, NotNil)

	msg := MsgSwap{
		Tx: GetRandomTx(),
	}

	c.Assert(k.SetSwapQueueItem(ctx, msg, 0), IsNil)
	msg2, err := k.GetSwapQueueItem(ctx, msg.Tx.ID, 0)
	c.Assert(err, IsNil)
	c.Check(msg2.Tx.ID.Equals(msg.Tx.ID), Equals, true)

	iter := k.GetSwapQueueIterator(ctx)
	defer iter.Close()

	// test remove
	k.RemoveSwapQueueItem(ctx, msg.Tx.ID, 0)
	_, err = k.GetSwapQueueItem(ctx, msg.Tx.ID, 0)
	c.Check(err, NotNil)
}
