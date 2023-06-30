package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

var _ = Suite(&HandlerSwitchSuite{})

type HandlerSwitchSuite struct{}

func (s *HandlerSwitchSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (s *HandlerSwitchSuite) TestValidate(c *C) {
	ctx, k := setupKeeperForTest(c)

	na := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, na), IsNil)
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.Rune67CAsset, cosmos.NewUint(100*common.One)),
	}

	handler := NewSwitchHandler(NewDummyMgrWithKeeper(k))

	destination := GetRandomTHORAddress()
	// happy path
	msg := NewMsgSwitch(tx, destination, na.NodeAddress)
	result, err := handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// invalid msg
	msg = &MsgSwitch{}
	result, err = handler.Run(ctx, msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
}

func (s *HandlerSwitchSuite) TestValidateKillSwitch(c *C) {
	ctx, mgr := setupManagerForTest(c)

	mgr.Keeper().SetMimir(ctx, constants.KillSwitchStart.String(), 1)
	mgr.Keeper().SetMimir(ctx, constants.KillSwitchDuration.String(), 1)

	na := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.Rune67CAsset, cosmos.NewUint(100*common.One)),
	}
	destination := GetRandomBNBAddress()

	handler := NewSwitchHandler(mgr)

	msg := NewMsgSwitch(tx, destination, na.NodeAddress)
	err := handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	c.Check(err.Error(), Equals, "switch is deprecated")
}

func (s *HandlerSwitchSuite) TestGettingNativeTokens(c *C) {
	ctx, k := setupKeeperForTest(c)

	na := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, na), IsNil)
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.Rune67CAsset, cosmos.NewUint(100*common.One)),
	}
	destination := GetRandomTHORAddress()

	handler := NewSwitchHandler(NewDummyMgrWithKeeper(k))

	msg := NewMsgSwitch(tx, destination, na.NodeAddress)
	_, err := handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	coin, err := common.NewCoin(common.RuneNative, cosmos.NewUint(100*common.One)).Native()
	c.Assert(err, IsNil)
	addr, err := cosmos.AccAddressFromBech32(destination.String())
	c.Assert(err, IsNil)
	c.Check(k.HasCoins(ctx, addr, cosmos.NewCoins(coin)), Equals, true, Commentf("%s", k.GetBalance(ctx, addr)))

	// check that we can add more an account
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	coin, err = common.NewCoin(common.RuneNative, cosmos.NewUint(200*common.One)).Native()
	c.Assert(err, IsNil)
	c.Check(k.HasCoins(ctx, addr, cosmos.NewCoins(coin)), Equals, true)
}

type HandlerSwitchTestHelper struct {
	keeper.Keeper
}

func NewHandlerSwitchTestHelper(k keeper.Keeper) *HandlerSwitchTestHelper {
	return &HandlerSwitchTestHelper{
		Keeper: k,
	}
}

func (s *HandlerSwitchSuite) getAValidSwitchMsg(ctx cosmos.Context, helper *HandlerSwitchTestHelper) *MsgSwitch {
	na := GetRandomValidatorNode(NodeActive)
	from := GetRandomBNBAddress()
	tx := GetRandomTx()
	tx.FromAddress = common.Address(from.String())
	tx.Coins = common.Coins{
		common.NewCoin(common.BEP2RuneAsset(), cosmos.NewUint(100*common.One)),
	}
	destination := GetRandomBech32Addr()
	_ = helper.Keeper.SetNodeAccount(ctx, na)
	coin, _ := common.NewCoin(common.RuneNative, cosmos.NewUint(800*common.One)).Native()
	_ = helper.Keeper.AddCoins(ctx, destination, cosmos.NewCoins(coin))
	vault := GetRandomVault()
	vault.Type = AsgardVault
	vault.Status = ActiveVault
	vault.AddFunds(common.Coins{
		common.NewCoin(common.BEP2RuneAsset(), cosmos.NewUint(100*common.One)),
	})
	_ = helper.Keeper.SetVault(ctx, vault)
	return NewMsgSwitch(tx, common.Address(destination.String()), na.NodeAddress)
}

func (s *HandlerSwitchSuite) TestSwitchHandlerDifferentValidations(c *C) {
	testCases := []struct {
		name            string
		messageProvider func(c *C, ctx cosmos.Context, helper *HandlerSwitchTestHelper) cosmos.Msg
		validator       func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerSwitchTestHelper, name string)
		nativeRune      bool
	}{
		{
			name: "invalid msg should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerSwitchTestHelper) cosmos.Msg {
				return NewMsgMimir("what", 1, GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerSwitchTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "Not enough RUNE to pay for fees should not fail",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerSwitchTestHelper) cosmos.Msg {
				m := s.getAValidSwitchMsg(ctx, helper)
				m.Tx.Coins[0].Amount = cosmos.NewUint(common.One).QuoUint64(2)
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerSwitchTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
			nativeRune: true,
		},
	}

	for _, tc := range testCases {
		if len(common.RuneAsset().Native()) == 0 && tc.nativeRune {
			continue
		}

		ctx, mgr := setupManagerForTest(c)
		helper := NewHandlerSwitchTestHelper(mgr.Keeper())
		mgr.K = helper
		handler := NewSwitchHandler(mgr)
		msg := tc.messageProvider(c, ctx, helper)
		result, err := handler.Run(ctx, msg)
		tc.validator(c, ctx, result, err, helper, tc.name)
	}
}

func (s *HandlerSwitchSuite) TestCalcCoin(c *C) {
	ctx, mgr := setupManagerForTest(c)
	handler := NewSwitchHandler(mgr)
	in := cosmos.NewUint(100)

	// no kill switch
	c.Check(handler.calcCoin(ctx, in).Uint64(), Equals, in.Uint64())

	// with kill switch
	mgr.Keeper().SetMimir(ctx, constants.KillSwitchStart.String(), 1)
	mgr.Keeper().SetMimir(ctx, constants.KillSwitchDuration.String(), 101)
	ctx = ctx.WithBlockHeight(1)
	amt := handler.calcCoin(ctx, in).Uint64()
	c.Check(amt, Equals, uint64(100), Commentf("%d", amt))

	ctx = ctx.WithBlockHeight(25 + 1)
	amt = handler.calcCoin(ctx, in).Uint64()
	c.Check(amt, Equals, uint64(75), Commentf("%d", amt))

	ctx = ctx.WithBlockHeight(50 + 1)
	amt = handler.calcCoin(ctx, in).Uint64()
	c.Check(amt, Equals, uint64(50), Commentf("%d", amt))

	ctx = ctx.WithBlockHeight(75 + 1)
	amt = handler.calcCoin(ctx, in).Uint64()
	c.Check(amt, Equals, uint64(26), Commentf("%d", amt))

	ctx = ctx.WithBlockHeight(100 + 1)
	amt = handler.calcCoin(ctx, in).Uint64()
	c.Check(amt, Equals, uint64(1), Commentf("%d", amt))

	ctx = ctx.WithBlockHeight(200 + 1)
	amt = handler.calcCoin(ctx, in).Uint64()
	c.Check(amt, Equals, uint64(0), Commentf("%d", amt))

	// with kill switch but not yet active
	mgr.Keeper().SetMimir(ctx, constants.KillSwitchStart.String(), 100)
	mgr.Keeper().SetMimir(ctx, constants.KillSwitchDuration.String(), 201)
	ctx = ctx.WithBlockHeight(50)
	amt = handler.calcCoin(ctx, in).Uint64()
	c.Check(amt, Equals, uint64(100), Commentf("%d", amt))
}
