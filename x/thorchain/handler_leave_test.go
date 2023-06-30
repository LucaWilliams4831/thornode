package thorchain

import (
	"errors"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerLeaveSuite struct{}

var _ = Suite(&HandlerLeaveSuite{})

func (HandlerLeaveSuite) TestLeaveHandler_NotActiveNodeLeave(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	vault := GetRandomVault()
	c.Assert(w.keeper.SetVault(w.ctx, vault), IsNil)
	leaveHandler := NewLeaveHandler(NewDummyMgrWithKeeper(w.keeper))
	acc2 := GetRandomValidatorNode(NodeStandby)
	acc2.Bond = cosmos.NewUint(100 * common.One)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc2), IsNil)
	ygg := NewVault(w.ctx.BlockHeight(), ActiveVault, YggdrasilVault, acc2.PubKeySet.Secp256k1, common.Chains{common.RuneAsset().Chain}.Strings(), []ChainContract{})
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	FundModule(c, w.ctx, w.keeper, BondName, 100)

	txID := GetRandomTxHash()
	tx := common.NewTx(
		txID,
		acc2.BondAddress,
		GetRandomBNBAddress(),
		common.Coins{common.NewCoin(common.RuneAsset(), cosmos.OneUint())},
		BNBGasFeeSingleton,
		"LEAVE",
	)
	msgLeave := NewMsgLeave(tx, acc2.NodeAddress, w.activeNodeAccount.NodeAddress)
	_, err := leaveHandler.Run(w.ctx, msgLeave)
	c.Assert(err, IsNil)
	accAfterLeave, err := w.keeper.GetNodeAccount(w.ctx, acc2.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(accAfterLeave.Status, Equals, NodeDisabled)
}

func (HandlerLeaveSuite) TestLeaveHandler_ActiveNodeLeave(c *C) {
	var err error
	w := getHandlerTestWrapper(c, 1, true, false)
	leaveHandler := NewLeaveHandler(NewDummyMgrWithKeeper(w.keeper))
	acc2 := GetRandomValidatorNode(NodeActive)
	acc2.Bond = cosmos.NewUint(100 * common.One)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc2), IsNil)
	txID := GetRandomTxHash()
	tx := common.NewTx(
		txID,
		acc2.BondAddress,
		GetRandomBNBAddress(),
		common.Coins{common.NewCoin(common.RuneAsset(), cosmos.OneUint())},
		BNBGasFeeSingleton,
		"",
	)
	msgLeave := NewMsgLeave(tx, acc2.NodeAddress, w.activeNodeAccount.NodeAddress)
	_, err = leaveHandler.Run(w.ctx, msgLeave)
	c.Assert(err, IsNil)

	acc2, err = w.keeper.GetNodeAccount(w.ctx, acc2.NodeAddress)
	c.Assert(err, IsNil)
	c.Check(acc2.Bond.Equal(cosmos.NewUint(10000000001)), Equals, true, Commentf("Bond:%d\n", acc2.Bond.Uint64()))
}

func (HandlerLeaveSuite) TestLeaveJail(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	vault := GetRandomVault()
	c.Assert(w.keeper.SetVault(w.ctx, vault), IsNil)
	leaveHandler := NewLeaveHandler(NewDummyMgrWithKeeper(w.keeper))
	acc2 := GetRandomValidatorNode(NodeStandby)
	acc2.Bond = cosmos.NewUint(100 * common.One)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc2), IsNil)

	c.Assert(w.keeper.SetNodeAccountJail(w.ctx, acc2.NodeAddress, w.ctx.BlockHeight()+100, "test it"), IsNil)

	ygg := NewVault(w.ctx.BlockHeight(), ActiveVault, YggdrasilVault, acc2.PubKeySet.Secp256k1, common.Chains{common.RuneAsset().Chain}.Strings(), []ChainContract{})
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	FundModule(c, w.ctx, w.keeper, BondName, 100)

	txID := GetRandomTxHash()
	tx := common.NewTx(
		txID,
		acc2.BondAddress,
		GetRandomBNBAddress(),
		common.Coins{common.NewCoin(common.RuneAsset(), cosmos.OneUint())},
		BNBGasFeeSingleton,
		"LEAVE",
	)
	msgLeave := NewMsgLeave(tx, acc2.NodeAddress, w.activeNodeAccount.NodeAddress)
	_, err := leaveHandler.Run(w.ctx, msgLeave)
	c.Assert(err, NotNil)
}

func (HandlerLeaveSuite) TestLeaveValidation(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	testCases := []struct {
		name          string
		msgLeave      *MsgLeave
		expectedError error
	}{
		{
			name: "empty from address should fail",
			msgLeave: NewMsgLeave(common.Tx{
				ID:          GetRandomTxHash(),
				Chain:       common.BNBChain,
				FromAddress: "",
				ToAddress:   GetRandomBNBAddress(),
				Coins: common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Gas: common.Gas{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Memo: "",
			}, w.activeNodeAccount.NodeAddress, w.activeNodeAccount.NodeAddress),
			expectedError: se.ErrInvalidAddress,
		},
		{
			name: "non-matching from address should fail",
			msgLeave: NewMsgLeave(common.Tx{
				ID:          GetRandomTxHash(),
				Chain:       common.BNBChain,
				FromAddress: GetRandomBNBAddress(),
				ToAddress:   GetRandomBNBAddress(),
				Coins: common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Gas: common.Gas{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Memo: "",
			}, w.activeNodeAccount.NodeAddress, w.activeNodeAccount.NodeAddress),
			expectedError: se.ErrUnauthorized,
		},
		{
			name: "empty tx id should fail",
			msgLeave: NewMsgLeave(common.Tx{
				ID:          common.TxID(""),
				Chain:       common.BNBChain,
				FromAddress: w.activeNodeAccount.BondAddress,
				ToAddress:   GetRandomBNBAddress(),
				Coins: common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Gas: common.Gas{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Memo: "",
			}, w.activeNodeAccount.NodeAddress, w.activeNodeAccount.NodeAddress),
			expectedError: se.ErrUnknownRequest,
		},
		{
			name: "empty signer should fail",
			msgLeave: NewMsgLeave(common.Tx{
				ID:          GetRandomTxHash(),
				Chain:       common.BNBChain,
				FromAddress: w.activeNodeAccount.BondAddress,
				ToAddress:   GetRandomBNBAddress(),
				Coins: common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Gas: common.Gas{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Memo: "",
			}, w.activeNodeAccount.NodeAddress, cosmos.AccAddress{}),
			expectedError: se.ErrInvalidAddress,
		},
	}
	for _, item := range testCases {
		c.Log(item.name)
		leaveHandler := NewLeaveHandler(NewDummyMgrWithKeeper(w.keeper))
		_, err := leaveHandler.Run(w.ctx, item.msgLeave)
		c.Check(errors.Is(err, item.expectedError), Equals, true, Commentf("name:%s, %s", item.name, err))
	}
}

type LeaveHandlerTestHelper struct {
	keeper.Keeper
	failGetNodeAccount bool
	failGetVault       bool
	failSetNodeAccount bool
}

func NewLeaveHandlerTestHelper(k keeper.Keeper) *LeaveHandlerTestHelper {
	return &LeaveHandlerTestHelper{
		Keeper: k,
	}
}

func (h *LeaveHandlerTestHelper) GetNodeAccount(ctx cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if h.failGetNodeAccount {
		return NodeAccount{}, errKaboom
	}
	return h.Keeper.GetNodeAccount(ctx, addr)
}

func (h *LeaveHandlerTestHelper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	if h.failSetNodeAccount {
		return errKaboom
	}
	return h.Keeper.SetNodeAccount(ctx, na)
}

func (h *LeaveHandlerTestHelper) GetVault(ctx cosmos.Context, pk common.PubKey) (Vault, error) {
	if h.failGetVault {
		return Vault{}, errKaboom
	}
	return h.Keeper.GetVault(ctx, pk)
}

func (HandlerLeaveSuite) TestLeaveDifferentValidations(c *C) {
	testCases := []struct {
		name            string
		messageProvider func(ctx cosmos.Context, helper *LeaveHandlerTestHelper) cosmos.Msg
		validator       func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *LeaveHandlerTestHelper, name string, msg cosmos.Msg)
	}{
		{
			name: "invalid message type should return an error",
			messageProvider: func(ctx cosmos.Context, helper *LeaveHandlerTestHelper) cosmos.Msg {
				return NewMsgNetworkFee(1024, common.BTCChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *LeaveHandlerTestHelper, name string, msg cosmos.Msg) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "fail to get node account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *LeaveHandlerTestHelper) cosmos.Msg {
				helper.failGetNodeAccount = true
				return NewMsgLeave(GetRandomTx(), GetRandomBech32Addr(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *LeaveHandlerTestHelper, name string, msg cosmos.Msg) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "empty node account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *LeaveHandlerTestHelper) cosmos.Msg {
				return NewMsgLeave(GetRandomTx(), GetRandomBech32Addr(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *LeaveHandlerTestHelper, name string, msg cosmos.Msg) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "fail to refund bond should return an error",
			messageProvider: func(ctx cosmos.Context, helper *LeaveHandlerTestHelper) cosmos.Msg {
				nodeAccount := GetRandomValidatorNode(NodeStandby)
				activeNodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, activeNodeAccount), IsNil)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAccount), IsNil)
				tx := GetRandomTx()
				tx.FromAddress = nodeAccount.BondAddress
				// when there is no asgard vault to refund, refund should fail
				return NewMsgLeave(tx, nodeAccount.NodeAddress, GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *LeaveHandlerTestHelper, name string, msg cosmos.Msg) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "vault not exist should refund bond",
			messageProvider: func(ctx cosmos.Context, helper *LeaveHandlerTestHelper) cosmos.Msg {
				nodeAccount := GetRandomValidatorNode(NodeStandby)
				activeNodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, activeNodeAccount), IsNil)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAccount), IsNil)
				tx := GetRandomTx()
				tx.FromAddress = nodeAccount.BondAddress
				// add an asgard vault , otherwise we won't be able to send out fund
				vault := GetRandomVault()
				c.Assert(helper.SetVault(ctx, vault), IsNil)
				return NewMsgLeave(tx, nodeAccount.NodeAddress, GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *LeaveHandlerTestHelper, name string, msg cosmos.Msg) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to get vault should return an error",
			messageProvider: func(ctx cosmos.Context, helper *LeaveHandlerTestHelper) cosmos.Msg {
				nodeAccount := GetRandomValidatorNode(NodeStandby)
				activeNodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, activeNodeAccount), IsNil)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAccount), IsNil)
				tx := GetRandomTx()
				tx.FromAddress = nodeAccount.BondAddress
				vault := NewVault(1024, ActiveVault, YggdrasilVault, nodeAccount.PubKeySet.Secp256k1, common.Chains{common.BNBChain, common.BTCChain}.Strings(), []ChainContract{})
				c.Assert(helper.Keeper.SetVault(ctx, vault), IsNil)
				helper.failGetVault = true
				return NewMsgLeave(tx, nodeAccount.NodeAddress, GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *LeaveHandlerTestHelper, name string, msg cosmos.Msg) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "when vault still has fund , it should request yggdrasil return",
			messageProvider: func(ctx cosmos.Context, helper *LeaveHandlerTestHelper) cosmos.Msg {
				nodeAccount := GetRandomValidatorNode(NodeStandby)
				activeNodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, activeNodeAccount), IsNil)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAccount), IsNil)
				tx := GetRandomTx()
				tx.FromAddress = nodeAccount.BondAddress
				vault := NewVault(1024, ActiveVault, YggdrasilVault, nodeAccount.PubKeySet.Secp256k1, common.Chains{common.BNBChain, common.BTCChain}.Strings(), []ChainContract{})
				vault.AddFunds(common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*100)),
				})
				c.Assert(helper.Keeper.SetVault(ctx, vault), IsNil)
				asgardVault := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain, common.BTCChain}.Strings(), []ChainContract{})
				c.Assert(helper.Keeper.SetVault(ctx, asgardVault), IsNil)
				return NewMsgLeave(tx, nodeAccount.NodeAddress, GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *LeaveHandlerTestHelper, name string, msg cosmos.Msg) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to save node account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *LeaveHandlerTestHelper) cosmos.Msg {
				nodeAccount := GetRandomValidatorNode(NodeStandby)
				activeNodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, activeNodeAccount), IsNil)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAccount), IsNil)
				tx := GetRandomTx()
				tx.FromAddress = nodeAccount.BondAddress
				asgardVault := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain, common.BTCChain}.Strings(), []ChainContract{})
				c.Assert(helper.Keeper.SetVault(ctx, asgardVault), IsNil)
				helper.failSetNodeAccount = true
				return NewMsgLeave(tx, nodeAccount.NodeAddress, GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *LeaveHandlerTestHelper, name string, msg cosmos.Msg) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "when node account is still belongs to a retiring vault , don't return bond",
			messageProvider: func(ctx cosmos.Context, helper *LeaveHandlerTestHelper) cosmos.Msg {
				nodeAccount := GetRandomValidatorNode(NodeDisabled)
				nodeAccount.Bond = cosmos.NewUint(100)
				activeNodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, activeNodeAccount), IsNil)
				c.Assert(helper.Keeper.SetNodeAccount(ctx, nodeAccount), IsNil)
				tx := GetRandomTx()
				tx.FromAddress = nodeAccount.BondAddress
				asgardVault := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain, common.BTCChain}.Strings(), []ChainContract{})
				c.Assert(helper.Keeper.SetVault(ctx, asgardVault), IsNil)

				retiringVault := NewVault(1000, RetiringVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain, common.BTCChain}.Strings(), []ChainContract{})
				retiringVault.Membership = common.PubKeys{
					nodeAccount.PubKeySet.Secp256k1,
					GetRandomPubKey(),
				}.Strings()
				c.Assert(helper.Keeper.SetVault(ctx, retiringVault), IsNil)
				return NewMsgLeave(tx, nodeAccount.NodeAddress, GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err2 error, helper *LeaveHandlerTestHelper, name string, msg cosmos.Msg) { // nolint
				leaveMsg, ok := msg.(*MsgLeave)
				c.Assert(ok, Equals, true)
				na, err := helper.GetNodeAccount(ctx, leaveMsg.NodeAddress)
				c.Assert(err, IsNil)
				c.Assert(na.Bond.Equal(cosmos.NewUint(100)), Equals, true)
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
	}

	for _, tc := range testCases {
		ctx, mgr := setupManagerForTest(c)
		FundModule(c, ctx, mgr.Keeper(), BondName, 1000)
		helper := NewLeaveHandlerTestHelper(mgr.Keeper())
		mgr.K = helper
		handler := NewLeaveHandler(mgr)
		msg := tc.messageProvider(ctx, helper)
		result, err := handler.Run(ctx, msg)
		tc.validator(c, ctx, result, err, helper, tc.name, msg)
	}
}

func (HandlerLeaveSuite) TestLeaveHandler_AbandonYggdrasilVault(c *C) {
	SetupConfigForTest()
	ctx, mgr := setupManagerForTest(c)
	pool := NewPool()
	pool.Asset = common.ETHAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(1000 * common.One)
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, 1000)

	c.Assert(mgr.Keeper().SaveNetworkFee(ctx, common.ETHChain, NetworkFee{
		Chain:              common.ETHChain,
		TransactionSize:    80000,
		TransactionFeeRate: 300,
	}), IsNil)
	nodeAccount := GetRandomValidatorNode(NodeStandby)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNodeAccount), IsNil)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, nodeAccount), IsNil)
	tx := GetRandomTx()
	tx.FromAddress = nodeAccount.BondAddress
	vault := NewVault(1024, ActiveVault, YggdrasilVault, nodeAccount.PubKeySet.Secp256k1, common.Chains{common.BNBChain, common.BTCChain, common.ETHChain}.Strings(), []ChainContract{})
	vault.AddFunds(common.Coins{
		common.NewCoin(common.ETHAsset, cosmos.NewUint(12000000)),
	})
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	asgardVault := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain, common.BTCChain, common.ETHChain}.Strings(), []ChainContract{})
	c.Assert(mgr.Keeper().SetVault(ctx, asgardVault), IsNil)
	msgLeave := NewMsgLeave(tx, nodeAccount.NodeAddress, GetRandomBech32Addr())
	handler := NewLeaveHandler(mgr)
	result, err := handler.Run(ctx, msgLeave)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	// make sure the yggdrasil vault still there
	v, err := mgr.Keeper().GetVault(ctx, nodeAccount.PubKeySet.Secp256k1)
	c.Assert(err, NotNil)
	c.Assert(errors.Is(err, kvTypes.ErrVaultNotFound), Equals, true)
	c.Assert(v.HasFunds(), Equals, false)
}

func (HandlerLeaveSuite) TestLeaveHandler_WhenTooMuchGasAssetLeft_LeaveShouldNotAbandonYggdrasil(c *C) {
	SetupConfigForTest()
	ctx, mgr := setupManagerForTest(c)
	pool := NewPool()
	pool.Asset = common.ETHAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(1000 * common.One)
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, 1000)
	c.Assert(mgr.Keeper().SaveNetworkFee(ctx, common.ETHChain, NetworkFee{
		Chain:              common.ETHChain,
		TransactionSize:    80000,
		TransactionFeeRate: 300,
	}), IsNil)
	nodeAccount := GetRandomValidatorNode(NodeStandby)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNodeAccount), IsNil)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, nodeAccount), IsNil)
	tx := GetRandomTx()
	tx.FromAddress = nodeAccount.BondAddress
	vault := NewVault(1024, ActiveVault, YggdrasilVault, nodeAccount.PubKeySet.Secp256k1, common.Chains{common.BNBChain, common.BTCChain, common.ETHChain}.Strings(), []ChainContract{})
	vault.AddFunds(common.Coins{
		common.NewCoin(common.ETHAsset, cosmos.NewUint(500000000)),
	})
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	asgardVault := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain, common.BTCChain, common.ETHChain}.Strings(), []ChainContract{})
	c.Assert(mgr.Keeper().SetVault(ctx, asgardVault), IsNil)
	msgLeave := NewMsgLeave(tx, nodeAccount.NodeAddress, GetRandomBech32Addr())
	handler := NewLeaveHandler(mgr)
	result, err := handler.Run(ctx, msgLeave)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	// make sure the yggdrasil vault still there
	v, err := mgr.Keeper().GetVault(ctx, nodeAccount.PubKeySet.Secp256k1)
	c.Assert(err, IsNil)
	c.Assert(v.HasFunds(), Equals, true)
}
