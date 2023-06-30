package thorchain

import (
	. "gopkg.in/check.v1"
)

type HandlerNoOpSuite struct{}

var _ = Suite(&HandlerNoOpSuite{})

func (HandlerNoOpSuite) TestNoOp(c *C) {
	w := getHandlerTestWrapper(c, 1, true, true)
	h := NewNoOpHandler(w.mgr)
	m := NewMsgNoOp(GetRandomObservedTx(), w.activeNodeAccount.NodeAddress, "novault")
	result, err := h.Run(w.ctx, m)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
}
