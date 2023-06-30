package keeperv1

import (
	. "gopkg.in/check.v1"
)

type KeeperTxInSuite struct{}

var _ = Suite(&KeeperTxInSuite{})

func (s *KeeperTxInSuite) TestTxInVoter(c *C) {
	ctx, k := setupKeeperForTest(c)

	tx := GetRandomTx()
	voter := NewObservedTxVoter(
		tx.ID,
		ObservedTxs{NewObservedTx(tx, 12, GetRandomPubKey(), 12)},
	)

	k.SetObservedTxInVoter(ctx, voter)
	voter, err := k.GetObservedTxInVoter(ctx, voter.TxID)
	c.Assert(err, IsNil)
	c.Check(voter.TxID.Equals(tx.ID), Equals, true)

	voterOut, err := k.GetObservedTxOutVoter(ctx, voter.TxID)
	c.Assert(err, IsNil)
	c.Assert(voterOut.TxID.Equals(tx.ID), Equals, true)
	c.Assert(voterOut.Tx.IsEmpty(), Equals, true)

	voter1 := NewObservedTxVoter(
		tx.ID,
		ObservedTxs{
			NewObservedTx(tx, 12, GetRandomPubKey(), 12),
		},
	)
	k.SetObservedTxOutVoter(ctx, voter1)

	voterOut1, err := k.GetObservedTxOutVoter(ctx, voter1.TxID)
	c.Assert(err, IsNil)
	c.Assert(voterOut1.TxID.Equals(tx.ID), Equals, true)

	// ensure that if the voter doesn't exist, we DON'T error
	tx = GetRandomTx()
	voter, err = k.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Check(voter.TxID.Equals(tx.ID), Equals, true)

	iter := k.GetObservedTxInVoterIterator(ctx)
	c.Check(iter, NotNil)
	iter.Close()

	iter1 := k.GetObservedTxOutVoterIterator(ctx)
	c.Check(iter1, NotNil)
	iter1.Close()
}

func (s *KeeperTxInSuite) TestObservedLink(c *C) {
	ctx, k := setupKeeperForTest(c)

	inhash := GetRandomTxHash()
	outhash1 := GetRandomTxHash()
	outhash2 := GetRandomTxHash()
	outhash3 := GetRandomTxHash()

	// empty hashes
	hashes := k.GetObservedLink(ctx, inhash)
	c.Assert(hashes, HasLen, 0)

	// single hash
	k.SetObservedLink(ctx, inhash, outhash1)
	hashes = k.GetObservedLink(ctx, inhash)
	c.Assert(hashes, HasLen, 1)
	c.Check(hashes[0].Equals(outhash1), Equals, true, Commentf("%s/%s", hashes[0], outhash1))

	// dedup works
	k.SetObservedLink(ctx, inhash, outhash1)
	hashes = k.GetObservedLink(ctx, inhash)
	c.Assert(hashes, HasLen, 1)
	c.Check(hashes[0].Equals(outhash1), Equals, true)

	// dedup works
	k.SetObservedLink(ctx, inhash, outhash2)
	k.SetObservedLink(ctx, inhash, outhash3)
	hashes = k.GetObservedLink(ctx, inhash)
	c.Assert(hashes, HasLen, 3)
	c.Check(hashes[0].Equals(outhash1), Equals, true)
	c.Check(hashes[1].Equals(outhash2), Equals, true)
	c.Check(hashes[2].Equals(outhash3), Equals, true)
}
