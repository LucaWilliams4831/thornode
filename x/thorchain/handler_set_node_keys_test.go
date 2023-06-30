package thorchain

import (
	"errors"
	"fmt"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerSetNodeKeysSuite struct{}

type TestSetNodeKeysKeeper struct {
	keeper.KVStoreDummy
	na     NodeAccount
	ensure error
}

func (k *TestSetNodeKeysKeeper) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coins common.Coins) error {
	return nil
}

func (k *TestSetNodeKeysKeeper) GetNodeAccount(ctx cosmos.Context, signer cosmos.AccAddress) (NodeAccount, error) {
	return k.na, nil
}

func (k *TestSetNodeKeysKeeper) EnsureNodeKeysUnique(_ cosmos.Context, _ string, _ common.PubKeySet) error {
	return k.ensure
}

func (k *TestSetNodeKeysKeeper) GetNetwork(ctx cosmos.Context) (Network, error) {
	return NewNetwork(), nil
}

func (k *TestSetNodeKeysKeeper) SetNetwork(ctx cosmos.Context, data Network) error {
	return nil
}

func (k *TestSetNodeKeysKeeper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	return nil
}

func (k *TestSetNodeKeysKeeper) SendFromModuleToModule(ctx cosmos.Context, from, to string, coins common.Coins) error {
	return nil
}

var _ = Suite(&HandlerSetNodeKeysSuite{})

func (s *HandlerSetNodeKeysSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestSetNodeKeysKeeper{
		na:     GetRandomValidatorNode(NodeStandby),
		ensure: nil,
	}
	keeper.na.PubKeySet = common.PubKeySet{}

	handler := NewSetNodeKeysHandler(NewDummyMgrWithKeeper(keeper))

	// happy path
	signer := GetRandomBech32Addr()
	c.Assert(signer.Empty(), Equals, false)
	consensPubKey := GetRandomBech32ConsensusPubKey()
	pubKeys := GetRandomPubKeySet()

	msg := NewMsgSetNodeKeys(pubKeys, consensPubKey, signer)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)
	result, err := handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// cannot set keys again
	keeper.na.PubKeySet = pubKeys
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)

	// cannot set node keys for active account
	keeper.na.Status = NodeActive
	msg = NewMsgSetNodeKeys(pubKeys, consensPubKey, keeper.na.NodeAddress)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)

	// cannot set node keys for disabled account
	keeper.na.Status = NodeDisabled
	msg = NewMsgSetNodeKeys(pubKeys, consensPubKey, keeper.na.NodeAddress)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)

	// cannot set node keys when duplicate
	keeper.na.Status = NodeStandby
	keeper.ensure = fmt.Errorf("duplicate keys")
	msg = NewMsgSetNodeKeys(keeper.na.PubKeySet, consensPubKey, keeper.na.NodeAddress)
	err = handler.validate(ctx, *msg)
	c.Assert(err, ErrorMatches, "duplicate keys")
	keeper.ensure = nil

	// invalid msg
	msg = &MsgSetNodeKeys{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	result, err = handler.Run(ctx, msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	result, err = handler.Run(ctx, NewMsgMimir("what", 1, GetRandomBech32Addr()))
	c.Check(err, NotNil)
	c.Check(result, IsNil)
}

type TestSetNodeKeysHandleKeeper struct {
	keeper.Keeper
	failGetNodeAccount bool
	failSetNodeAccount bool
	failGetNetwork     bool
	failSetNetwork     bool
}

func NewTestSetNodeKeysHandleKeeper(k keeper.Keeper) *TestSetNodeKeysHandleKeeper {
	return &TestSetNodeKeysHandleKeeper{
		Keeper: k,
	}
}

func (k *TestSetNodeKeysHandleKeeper) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coins common.Coins) error {
	return nil
}

func (k *TestSetNodeKeysHandleKeeper) GetNodeAccount(ctx cosmos.Context, signer cosmos.AccAddress) (NodeAccount, error) {
	if k.failGetNodeAccount {
		return NodeAccount{}, errKaboom
	}
	return k.Keeper.GetNodeAccount(ctx, signer)
}

func (k *TestSetNodeKeysHandleKeeper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	if k.failSetNodeAccount {
		return errKaboom
	}
	return k.Keeper.SetNodeAccount(ctx, na)
}

func (k *TestSetNodeKeysHandleKeeper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if k.failGetNetwork {
		return Network{}, errKaboom
	}
	return k.Keeper.GetNetwork(ctx)
}

func (k *TestSetNodeKeysHandleKeeper) SetNetwork(ctx cosmos.Context, data Network) error {
	if k.failSetNetwork {
		return errKaboom
	}
	return k.Keeper.SetNetwork(ctx, data)
}

func (k *TestSetNodeKeysHandleKeeper) EnsureNodeKeysUnique(_ cosmos.Context, consensPubKey string, pubKeys common.PubKeySet) error {
	return nil
}

func (s *HandlerSetNodeKeysSuite) TestHandle(c *C) {
	ctx, mgr := setupManagerForTest(c)
	helper := NewTestSetNodeKeysHandleKeeper(mgr.Keeper())
	mgr.K = helper
	handler := NewSetNodeKeysHandler(mgr)
	ctx = ctx.WithBlockHeight(1)
	signer := GetRandomBech32Addr()

	// add observer
	bepConsPubKey := GetRandomBech32ConsensusPubKey()
	bondAddr := GetRandomBNBAddress()
	pubKeys := GetRandomPubKeySet()
	emptyPubKeySet := common.PubKeySet{}

	msgNodeKeys := NewMsgSetNodeKeys(pubKeys, bepConsPubKey, signer)

	bond := cosmos.NewUint(common.One * 100)
	nodeAccount := NewNodeAccount(signer, NodeActive, emptyPubKeySet, "", bond, bondAddr, ctx.BlockHeight())
	c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAccount), IsNil)

	nodeAccount = NewNodeAccount(signer, NodeWhiteListed, emptyPubKeySet, "", bond, bondAddr, ctx.BlockHeight())
	c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAccount), IsNil)
	FundModule(c, ctx, helper, BondName, common.One*100)
	// happy path
	_, err := handler.handle(ctx, *msgNodeKeys)
	c.Assert(err, IsNil)
	na, err := helper.Keeper.GetNodeAccount(ctx, msgNodeKeys.Signer)
	c.Assert(err, IsNil)
	c.Assert(na.PubKeySet, Equals, pubKeys)
	c.Assert(na.ValidatorConsPubKey, Equals, bepConsPubKey)
	c.Assert(na.Status, Equals, NodeStandby)
	c.Assert(na.StatusSince, Equals, int64(1))

	testCases := []struct {
		name              string
		messageProvider   func(c *C, ctx cosmos.Context, helper *TestSetNodeKeysHandleKeeper) cosmos.Msg
		validator         func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *TestSetNodeKeysHandleKeeper, name string)
		skipForNativeRune bool
	}{
		{
			name: "fail to get node account should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *TestSetNodeKeysHandleKeeper) cosmos.Msg {
				helper.failGetNodeAccount = true
				return NewMsgSetNodeKeys(GetRandomPubKeySet(), GetRandomBech32ConsensusPubKey(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *TestSetNodeKeysHandleKeeper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
				c.Check(errors.Is(err, se.ErrUnauthorized), Equals, true)
			},
		},
		{
			name: "node account is empty should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *TestSetNodeKeysHandleKeeper) cosmos.Msg {
				return NewMsgSetNodeKeys(GetRandomPubKeySet(), GetRandomBech32ConsensusPubKey(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *TestSetNodeKeysHandleKeeper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
				c.Check(errors.Is(err, se.ErrUnauthorized), Equals, true)
			},
		},
		{
			name: "fail to save node account should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *TestSetNodeKeysHandleKeeper) cosmos.Msg {
				nodeAcct := GetRandomValidatorNode(NodeWhiteListed)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAcct), IsNil)
				helper.failSetNodeAccount = true
				return NewMsgSetNodeKeys(nodeAcct.PubKeySet, nodeAcct.ValidatorConsPubKey, nodeAcct.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *TestSetNodeKeysHandleKeeper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to get network data should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *TestSetNodeKeysHandleKeeper) cosmos.Msg {
				nodeAcct := GetRandomValidatorNode(NodeWhiteListed)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAcct), IsNil)
				helper.failGetNetwork = true
				return NewMsgSetNodeKeys(nodeAcct.PubKeySet, nodeAcct.ValidatorConsPubKey, nodeAcct.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *TestSetNodeKeysHandleKeeper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
			skipForNativeRune: true,
		},
		{
			name: "fail to set network data should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *TestSetNodeKeysHandleKeeper) cosmos.Msg {
				nodeAcct := GetRandomValidatorNode(NodeWhiteListed)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAcct), IsNil)
				helper.failSetNetwork = true
				return NewMsgSetNodeKeys(nodeAcct.PubKeySet, nodeAcct.ValidatorConsPubKey, nodeAcct.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *TestSetNodeKeysHandleKeeper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
			skipForNativeRune: true,
		},
	}
	for _, tc := range testCases {
		if common.RuneAsset().Native() != "" && tc.skipForNativeRune {
			continue
		}
		ctx, mgr := setupManagerForTest(c)
		helper := NewTestSetNodeKeysHandleKeeper(mgr.Keeper())
		mgr.K = helper
		handler := NewSetNodeKeysHandler(mgr)
		msg := tc.messageProvider(c, ctx, helper)
		result, err := handler.Run(ctx, msg)
		tc.validator(c, ctx, result, err, helper, tc.name)
	}
}
