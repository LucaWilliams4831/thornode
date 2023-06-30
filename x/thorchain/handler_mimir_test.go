package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type HandlerMimirSuite struct{}

var _ = Suite(&HandlerMimirSuite{})

func (s *HandlerMimirSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (s *HandlerMimirSuite) TestValidate(c *C) {
	ctx, keeper := setupKeeperForTest(c)

	addr, _ := cosmos.AccAddressFromBech32(ADMINS[0])
	handler := NewMimirHandler(NewDummyMgrWithKeeper(keeper))
	// happy path
	msg := NewMsgMimir("foo", 44, addr)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// invalid msg
	msg = &MsgMimir{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

func (s *HandlerMimirSuite) TestMimirHandle(c *C) {
	ctx, keeper := setupKeeperForTest(c)
	handler := NewMimirHandler(NewDummyMgrWithKeeper(keeper))
	addr, err := cosmos.AccAddressFromBech32(ADMINS[0])
	c.Assert(err, IsNil)
	msg := NewMsgMimir("foo", 55, addr)
	sdkErr := handler.handle(ctx, *msg)
	c.Assert(sdkErr, IsNil)
	val, err := keeper.GetMimir(ctx, "foo")
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(55))

	invalidMsg := NewMsgNetworkFee(ctx.BlockHeight(), common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
	result, err := handler.Run(ctx, invalidMsg)
	c.Check(err, NotNil)
	c.Check(result, IsNil)

	msg.Signer = GetRandomBech32Addr()
	result, err = handler.Run(ctx, msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	msg1 := NewMsgMimir("hello", 1, addr)
	result, err = handler.Run(ctx, msg1)
	c.Check(err, IsNil)
	c.Check(result, NotNil)

	val, err = keeper.GetMimir(ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(1))

	// delete mimir
	msg1 = NewMsgMimir("hello", -3, addr)
	result, err = handler.Run(ctx, msg1)
	c.Check(err, IsNil)
	c.Check(result, NotNil)
	val, err = keeper.GetMimir(ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(-1))

	// node set mimir
	FundModule(c, ctx, keeper, BondName, 100*common.One)
	ver := "1.92.0"
	na1 := GetRandomValidatorNode(NodeActive)
	na1.Version = ver
	na2 := GetRandomValidatorNode(NodeActive)
	na2.Version = ver
	na3 := GetRandomValidatorNode(NodeActive)
	na3.Version = ver
	c.Assert(keeper.SetNodeAccount(ctx, na1), IsNil)
	c.Assert(keeper.SetNodeAccount(ctx, na2), IsNil)
	c.Assert(keeper.SetNodeAccount(ctx, na3), IsNil)

	// first node set mimir , no consensus
	result, err = handler.Run(ctx, NewMsgMimir("node-mimir", 1, na1.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	mvalue, err := keeper.GetMimir(ctx, "node-mimir")
	c.Assert(err, IsNil)
	c.Assert(mvalue, Equals, int64(-1))

	// second node set mimir, reach consensus
	result, err = handler.Run(ctx, NewMsgMimir("node-mimir", 1, na2.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	mvalue, err = keeper.GetMimir(ctx, "node-mimir")
	c.Assert(err, IsNil)
	c.Assert(mvalue, Equals, int64(1))

	// third node set mimir, reach consensus
	result, err = handler.Run(ctx, NewMsgMimir("node-mimir", 1, na3.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	mvalue, err = keeper.GetMimir(ctx, "node-mimir")
	c.Assert(err, IsNil)
	c.Assert(mvalue, Equals, int64(1))

	// third node vote mimir to a different value, it should not change the admin mimir value
	result, err = handler.Run(ctx, NewMsgMimir("node-mimir", 0, na3.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	mvalue, err = keeper.GetMimir(ctx, "node-mimir")
	c.Assert(err, IsNil)
	c.Assert(mvalue, Equals, int64(1))

	// second node vote mimir to a different value , it should update admin mimir
	result, err = handler.Run(ctx, NewMsgMimir("node-mimir", 0, na2.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	mvalue, err = keeper.GetMimir(ctx, "node-mimir")
	c.Assert(err, IsNil)
	c.Assert(mvalue, Equals, int64(0))

	result, err = handler.Run(ctx, NewMsgMimir("node-mimir-1", 0, na2.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
}
