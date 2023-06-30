package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerIPAddressSuite struct{}

type TestIPAddresslKeeper struct {
	keeper.KVStoreDummy
	na        NodeAccount
	vaultNode NodeAccount
}

func (k *TestIPAddresslKeeper) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coins common.Coins) error {
	return nil
}

func (k *TestIPAddresslKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.vaultNode.NodeAddress.Equals(addr) {
		return NodeAccount{Type: NodeTypeVault}, nil
	}
	return k.na, nil
}

func (k *TestIPAddresslKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	k.na = na
	return nil
}

func (k *TestIPAddresslKeeper) GetNetwork(ctx cosmos.Context) (Network, error) {
	return NewNetwork(), nil
}

func (k *TestIPAddresslKeeper) SetNetwork(ctx cosmos.Context, data Network) error {
	return nil
}

func (k *TestIPAddresslKeeper) SendFromModuleToModule(ctx cosmos.Context, from, to string, coins common.Coins) error {
	return nil
}

var _ = Suite(&HandlerIPAddressSuite{})

func (s *HandlerIPAddressSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestIPAddresslKeeper{
		na:        GetRandomValidatorNode(NodeActive),
		vaultNode: GetRandomVaultNode(NodeActive),
	}

	handler := NewIPAddressHandler(NewDummyMgrWithKeeper(keeper))
	// happy path
	msg := NewMsgSetIPAddress("8.8.8.8", keeper.na.NodeAddress)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// invalid msg
	msg = &MsgSetIPAddress{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)

	// vault nodes can't set ip address
	msg = NewMsgSetIPAddress("8.8.8.8", keeper.vaultNode.NodeAddress)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

func (s *HandlerIPAddressSuite) TestHandle(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestIPAddresslKeeper{
		na: GetRandomValidatorNode(NodeActive),
	}

	handler := NewIPAddressHandler(NewDummyMgrWithKeeper(keeper))

	msg := NewMsgSetIPAddress("192.168.0.1", GetRandomBech32Addr())
	err := handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Check(keeper.na.IPAddress, Equals, "192.168.0.1")
}

type HandlerIPAddressTestHelper struct {
	keeper.Keeper
	failGetNodeAccount  bool
	failSaveNodeAccount bool
}

func NewHandlerIPAddressTestHelper(k keeper.Keeper) *HandlerIPAddressTestHelper {
	return &HandlerIPAddressTestHelper{
		Keeper: k,
	}
}

func (h *HandlerIPAddressTestHelper) GetNodeAccount(ctx cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if h.failGetNodeAccount {
		return NodeAccount{}, errKaboom
	}
	return h.Keeper.GetNodeAccount(ctx, addr)
}

func (h *HandlerIPAddressTestHelper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	if h.failSaveNodeAccount {
		return errKaboom
	}
	return h.Keeper.SetNodeAccount(ctx, na)
}

func (s *HandlerIPAddressSuite) TestHandlerSetIPAddress_validation(c *C) {
	testCases := []struct {
		name            string
		messageProvider func(ctx cosmos.Context, helper *HandlerIPAddressTestHelper) cosmos.Msg
		validator       func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerIPAddressTestHelper, name string)
	}{
		{
			name: "invalid message should return an error",
			messageProvider: func(ctx cosmos.Context, helper *HandlerIPAddressTestHelper) cosmos.Msg {
				return NewMsgNetworkFee(1024, common.BTCChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerIPAddressTestHelper, name string) {
				c.Assert(err, NotNil)
				c.Assert(result, IsNil)
			},
		},
		{
			name: "message fail validation should return an error",
			messageProvider: func(ctx cosmos.Context, helper *HandlerIPAddressTestHelper) cosmos.Msg {
				return NewMsgSetIPAddress("whatever", GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerIPAddressTestHelper, name string) {
				c.Assert(err, NotNil)
				c.Assert(result, IsNil)
			},
		},
		{
			name: "fail to get node account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *HandlerIPAddressTestHelper) cosmos.Msg {
				helper.failGetNodeAccount = true
				return NewMsgSetIPAddress("192.168.0.1", GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerIPAddressTestHelper, name string) {
				c.Assert(err, NotNil)
				c.Assert(result, IsNil)
			},
		},
		{
			name: "empty node account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *HandlerIPAddressTestHelper) cosmos.Msg {
				return NewMsgSetIPAddress("192.168.0.1", GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerIPAddressTestHelper, name string) {
				c.Assert(err, NotNil)
				c.Assert(result, IsNil)
			},
		},
		{
			name: "fail to save node account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *HandlerIPAddressTestHelper) cosmos.Msg {
				helper.failSaveNodeAccount = true
				nodeAccount := GetRandomValidatorNode(NodeWhiteListed)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAccount), IsNil)
				return NewMsgSetIPAddress("192.168.0.1", nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerIPAddressTestHelper, name string) {
				c.Assert(err, NotNil)
				c.Assert(result, IsNil)
			},
		},
		{
			name: "all good - happy path",
			messageProvider: func(ctx cosmos.Context, helper *HandlerIPAddressTestHelper) cosmos.Msg {
				nodeAccount := GetRandomValidatorNode(NodeWhiteListed)
				FundModule(c, ctx, helper, BondName, common.One*100)
				c.Assert(helper.SendFromModuleToAccount(ctx, ModuleName, nodeAccount.NodeAddress, common.Coins{
					common.NewCoin(common.RuneAsset(), cosmos.NewUint(1000*common.One)),
				}), IsNil)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAccount), IsNil)
				return NewMsgSetIPAddress("192.168.0.1", nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerIPAddressTestHelper, name string) {
				c.Assert(err, IsNil)
				c.Assert(result, NotNil)
			},
		},
	}
	for _, tc := range testCases {
		ctx, mgr := setupManagerForTest(c)
		c.Logf("Name: %s", tc.name)
		helper := NewHandlerIPAddressTestHelper(mgr.Keeper())
		mgr.K = helper
		handler := NewIPAddressHandler(mgr)
		msg := tc.messageProvider(ctx, helper)
		result, err := handler.Run(ctx, msg)
		tc.validator(c, ctx, result, err, helper, tc.name)
	}
}
