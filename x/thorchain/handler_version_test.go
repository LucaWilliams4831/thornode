package thorchain

import (
	"errors"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerVersionSuite struct{}

type TestVersionlKeeper struct {
	keeper.KVStoreDummy
	na                  NodeAccount
	failNodeAccount     NodeAccount
	emptyNodeAccount    NodeAccount
	vaultNodeAccount    NodeAccount
	failSaveNodeAccount bool
	failGetNetwork      bool
	failSetNetwork      bool
}

func (k *TestVersionlKeeper) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coins common.Coins) error {
	return nil
}

func (k *TestVersionlKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.failNodeAccount.NodeAddress.Equals(addr) {
		return NodeAccount{}, errKaboom
	}
	if k.emptyNodeAccount.NodeAddress.Equals(addr) {
		return NodeAccount{}, nil
	}
	if k.vaultNodeAccount.NodeAddress.Equals(addr) {
		return NodeAccount{Type: NodeTypeVault}, nil
	}
	return k.na, nil
}

func (k *TestVersionlKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	if k.failSaveNodeAccount {
		return errKaboom
	}
	k.na = na
	return nil
}

func (k *TestVersionlKeeper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if k.failGetNetwork {
		return NewNetwork(), errKaboom
	}
	return NewNetwork(), nil
}

func (k *TestVersionlKeeper) SetNetwork(ctx cosmos.Context, data Network) error {
	if k.failSetNetwork {
		return errKaboom
	}
	return nil
}

func (k *TestVersionlKeeper) SendFromModuleToModule(ctx cosmos.Context, from, to string, coins common.Coins) error {
	return nil
}

var _ = Suite(&HandlerVersionSuite{})

func (s *HandlerVersionSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)
	ver := GetCurrentVersion()

	keeper := &TestVersionlKeeper{
		na:               GetRandomValidatorNode(NodeActive),
		failNodeAccount:  GetRandomValidatorNode(NodeActive),
		emptyNodeAccount: GetRandomValidatorNode(NodeStandby),
		vaultNodeAccount: GetRandomVaultNode(NodeActive),
	}

	handler := NewVersionHandler(NewDummyMgrWithKeeper(keeper))
	// happy path
	msg := NewMsgSetVersion(ver.String(), keeper.na.NodeAddress)
	result, err := handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// invalid msg
	msg = &MsgSetVersion{}
	result, err = handler.Run(ctx, msg)
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	// fail to get node account should fail
	msg1 := NewMsgSetVersion(ver.String(), keeper.failNodeAccount.NodeAddress)
	result, err = handler.Run(ctx, msg1)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// node account empty should fail
	msg2 := NewMsgSetVersion(ver.String(), keeper.emptyNodeAccount.NodeAddress)
	result, err = handler.Run(ctx, msg2)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
	c.Assert(errors.Is(err, se.ErrUnauthorized), Equals, true)

	// vault node should fail
	msg3 := NewMsgSetVersion(ver.String(), keeper.vaultNodeAccount.NodeAddress)
	result, err = handler.Run(ctx, msg3)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
	c.Assert(errors.Is(err, se.ErrUnauthorized), Equals, true)
}

func (s *HandlerVersionSuite) TestHandle(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestVersionlKeeper{
		na: GetRandomValidatorNode(NodeActive),
	}

	handler := NewVersionHandler(NewDummyMgrWithKeeper(keeper))

	msg := NewMsgSetVersion("2.0.0", GetRandomBech32Addr())
	err := handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Check(keeper.na.Version, Equals, "2.0.0")

	// fail to set node account should return an error
	keeper.failSaveNodeAccount = true
	result, err := handler.Run(ctx, msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
	keeper.failSaveNodeAccount = false

	if !common.RuneAsset().Equals(common.RuneNative) {
		// BEP2 RUNE
		keeper.failGetNetwork = true
		result, err = handler.Run(ctx, msg)
		c.Assert(err, NotNil)
		c.Assert(result, IsNil)
		keeper.failGetNetwork = false
		keeper.failSetNetwork = true
		result, err = handler.Run(ctx, msg)
		c.Assert(err, NotNil)
		c.Assert(result, IsNil)
		keeper.failSetNetwork = false
	}
}
