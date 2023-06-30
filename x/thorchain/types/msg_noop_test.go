package types

import (
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	. "gopkg.in/check.v1"
)

type MsgNoopSuite struct{}

var _ = Suite(&MsgNoopSuite{})

func (MsgNoopSuite) TestMsgNoop(c *C) {
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
	m := NewMsgNoOp(tx, addr, "")
	c.Check(m.ValidateBasic(), IsNil)
	c.Check(m.Type(), Equals, "set_noop")
	EnsureMsgBasicCorrect(m, c)
	mEmpty := NewMsgNoOp(tx, cosmos.AccAddress{}, "")
	c.Assert(mEmpty.ValidateBasic(), NotNil)
}
