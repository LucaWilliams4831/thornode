package types

import (
	. "gopkg.in/check.v1"
)

type MsgNodePauseChainTestSuite struct{}

var _ = Suite(&MsgNodePauseChainTestSuite{})

func (MsgNodePauseChainTestSuite) TestMsgNodePauseChain(c *C) {
	acc := GetRandomBech32Addr()

	msg := NewMsgNodePauseChain(12, acc)
	c.Assert(msg.Route(), Equals, RouterKey)
	c.Assert(msg.Type(), Equals, "node_pause_chain")
	c.Assert(msg.ValidateBasic(), IsNil)
	c.Assert(len(msg.GetSignBytes()) > 0, Equals, true)
	c.Assert(msg.GetSigners(), NotNil)
	c.Assert(msg.GetSigners()[0].String(), Equals, acc.String())

	msg = NewMsgNodePauseChain(12, nil)
	c.Assert(msg.ValidateBasic(), NotNil)
}
