package types

import (
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	. "gopkg.in/check.v1"
)

type MsgConsolidateSuite struct{}

var _ = Suite(&MsgConsolidateSuite{})

func (MsgConsolidateSuite) TestMsgConsolidate(c *C) {
	addr := GetRandomBech32Addr()
	c.Check(addr.Empty(), Equals, false)
	tx := ObservedTx{
		Tx:             GetRandomTx(),
		Status:         Status_done,
		OutHashes:      nil,
		BlockHeight:    1,
		Signers:        []string{addr.String()},
		ObservedPubKey: GetRandomPubKey(),
		FinaliseHeight: 1,
	}
	m := NewMsgConsolidate(tx, addr)
	c.Check(m.ValidateBasic(), IsNil)
	c.Check(m.Type(), Equals, "consolidate")
	EnsureMsgBasicCorrect(m, c)
	mEmpty := NewMsgConsolidate(tx, cosmos.AccAddress{})
	c.Assert(mEmpty.ValidateBasic(), NotNil)
}
