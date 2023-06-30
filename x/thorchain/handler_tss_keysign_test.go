package thorchain

import (
	"errors"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerTssKeysignSuite struct{}

var _ = Suite(&HandlerTssKeysignSuite{})

type tssKeysignFailHandlerTestHelper struct {
	ctx           cosmos.Context
	version       semver.Version
	keeper        *tssKeysignKeeperHelper
	constAccessor constants.ConstantValues
	nodeAccount   NodeAccount
	mgr           Manager
	blame         Blame
	retiringVault Vault
}

type tssKeysignKeeperHelper struct {
	keeper.Keeper
	errListActiveAccounts           bool
	errGetTssVoter                  bool
	errFailToGetNodeAccountByPubKey bool
	errFailSetNodeAccount           bool
}

func newTssKeysignFailKeeperHelper(keeper keeper.Keeper) *tssKeysignKeeperHelper {
	return &tssKeysignKeeperHelper{
		Keeper: keeper,
	}
}

func (k *tssKeysignKeeperHelper) GetNodeAccountByPubKey(ctx cosmos.Context, pk common.PubKey) (NodeAccount, error) {
	if k.errFailToGetNodeAccountByPubKey {
		return NodeAccount{}, errKaboom
	}
	return k.Keeper.GetNodeAccountByPubKey(ctx, pk)
}

func (k *tssKeysignKeeperHelper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	if k.errFailSetNodeAccount {
		return errKaboom
	}
	return k.Keeper.SetNodeAccount(ctx, na)
}

func (k *tssKeysignKeeperHelper) GetTssKeysignFailVoter(ctx cosmos.Context, id string) (TssKeysignFailVoter, error) {
	if k.errGetTssVoter {
		return TssKeysignFailVoter{}, errKaboom
	}
	return k.Keeper.GetTssKeysignFailVoter(ctx, id)
}

func (k *tssKeysignKeeperHelper) ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error) {
	if k.errListActiveAccounts {
		return NodeAccounts{}, errKaboom
	}
	return k.Keeper.ListActiveValidators(ctx)
}

func signVoter(ctx cosmos.Context, keeper keeper.Keeper, except cosmos.AccAddress) (result []cosmos.AccAddress) {
	active, _ := keeper.ListActiveValidators(ctx)
	for _, na := range active {
		if na.NodeAddress.Equals(except) {
			continue
		}
		result = append(result, na.NodeAddress)
	}
	return
}

func newTssKeysignHandlerTestHelper(c *C) tssKeysignFailHandlerTestHelper {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(1023)
	keeperHelper := newTssKeysignFailKeeperHelper(k)
	// active account
	nodeAccount := GetRandomValidatorNode(NodeActive)
	nodeAccount.Bond = cosmos.NewUint(100 * common.One)
	c.Assert(keeperHelper.SetNodeAccount(ctx, nodeAccount), IsNil)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	mgr := NewDummyMgr()

	var members []Node
	for i := 0; i < 8; i++ {
		na := GetRandomValidatorNode(NodeActive)
		members = append(members, Node{Pubkey: na.PubKeySet.Secp256k1.String()})
		_ = keeperHelper.SetNodeAccount(ctx, na)
	}
	blame := Blame{
		FailReason: "whatever",
		BlameNodes: []Node{members[0], members[1]},
	}
	asgardVault := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	c.Assert(keeperHelper.SetVault(ctx, asgardVault), IsNil)
	retiringVault := NewVault(ctx.BlockHeight(), RetiringVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	for _, item := range members {
		retiringVault.Membership = append(retiringVault.Membership, item.Pubkey)
	}
	c.Assert(keeperHelper.SetVault(ctx, retiringVault), IsNil)
	return tssKeysignFailHandlerTestHelper{
		ctx:           ctx,
		version:       GetCurrentVersion(),
		keeper:        keeperHelper,
		constAccessor: constAccessor,
		nodeAccount:   nodeAccount,
		mgr:           mgr,
		blame:         blame,
		retiringVault: retiringVault,
	}
}

func (h HandlerTssKeysignSuite) TestTssKeysignFailHandler(c *C) {
	testCases := []struct {
		name           string
		messageCreator func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg
		runner         func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error)
		validator      func(helper tssKeysignFailHandlerTestHelper, msg cosmos.Msg, result *cosmos.Result, c *C)
		expectedResult error
	}{
		{
			name: "invalid message should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				return NewMsgNoOp(GetRandomObservedTx(), helper.nodeAccount.NodeAddress, "")
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: errInvalidMessage,
		},
		{
			name: "Not signed by an active account should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				msg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, GetRandomBech32Addr(), GetRandomPubKey())
				c.Assert(err, IsNil)
				return msg
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: se.ErrUnauthorized,
		},
		{
			name: "empty signer should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				msg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, cosmos.AccAddress{}, GetRandomPubKey())
				c.Assert(err, IsNil)
				return msg
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: se.ErrInvalidAddress,
		},
		{
			name: "empty id should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				tssMsg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress, GetRandomPubKey())
				c.Assert(err, IsNil)
				tssMsg.ID = ""
				return tssMsg
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: se.ErrUnknownRequest,
		},
		{
			name: "empty member pubkeys should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				msg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), Blame{
					FailReason: "",
					BlameNodes: []Node{},
				}, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress, GetRandomPubKey())
				c.Assert(err, IsNil)
				return msg
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: se.ErrUnknownRequest,
		},
		{
			name: "normal blame should works fine",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				msg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress, helper.retiringVault.PubKey)
				c.Assert(err, IsNil)
				return msg
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: nil,
		},
		{
			name: "when the same signer already sign the tss keysign failure , it should not do anything",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				msg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress, GetRandomPubKey())
				c.Assert(err, IsNil)
				voter, _ := helper.keeper.Keeper.GetTssKeysignFailVoter(helper.ctx, msg.ID)
				voter.Sign(msg.Signer)
				helper.keeper.Keeper.SetTssKeysignFailVoter(helper.ctx, voter)
				return msg
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: nil,
		},
		{
			name: "fail to list active node accounts should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				msg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress, GetRandomPubKey())
				c.Assert(err, IsNil)
				return msg
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error) {
				helper.keeper.errListActiveAccounts = true
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: errKaboom,
		},
		{
			name: "fail to get Tss Keysign fail voter should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				msg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress, GetRandomPubKey())
				c.Assert(err, IsNil)
				return msg
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error) {
				helper.keeper.errGetTssVoter = true
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: errKaboom,
		},
		{
			name: "fail to get node account should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				msg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress, GetRandomPubKey())
				c.Assert(err, IsNil)
				return msg
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error) {
				mmsg, ok := msg.(*MsgTssKeysignFail)
				c.Assert(ok, Equals, true)
				// prepopulate the voter with other signers
				voter, _ := helper.keeper.GetTssKeysignFailVoter(helper.ctx, mmsg.ID)
				signers := signVoter(helper.ctx, helper.keeper, mmsg.Signer)
				voter.Signers = make([]string, len(signers))
				for i, sign := range signers {
					voter.Signers[i] = sign.String()
				}
				helper.keeper.SetTssKeysignFailVoter(helper.ctx, voter)
				helper.keeper.errFailToGetNodeAccountByPubKey = true
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: errInternal,
		},
		{
			name: "without majority it should not take any actions",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				msg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress, helper.retiringVault.PubKey)
				c.Assert(err, IsNil)
				return msg
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error) {
				for i := 0; i < 3; i++ {
					na := GetRandomValidatorNode(NodeActive)
					if err := helper.keeper.SetNodeAccount(helper.ctx, na); err != nil {
						return nil, ErrInternal(err, "fail to set node account")
					}
				}
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: nil,
		},
		{
			name: "with majority it should take actions",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				msg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress, helper.retiringVault.PubKey)
				c.Assert(err, IsNil)
				return msg
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) (*cosmos.Result, error) {
				var na NodeAccount
				for i := 0; i < 3; i++ {
					na = GetRandomValidatorNode(NodeActive)
					if err := helper.keeper.SetNodeAccount(helper.ctx, na); err != nil {
						return nil, ErrInternal(err, "fail to set node account")
					}
				}
				_, err := handler.Run(helper.ctx, msg)
				if err != nil {
					return nil, err
				}
				msg, err = NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, na.NodeAddress, helper.retiringVault.PubKey)
				c.Assert(err, IsNil)
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: nil,
		},
	}
	for _, tc := range testCases {
		helper := newTssKeysignHandlerTestHelper(c)
		handler := NewTssKeysignHandler(NewDummyMgrWithKeeper(helper.keeper))
		msg := tc.messageCreator(helper)

		c.Logf(">Name: %s\n", tc.name)
		result, err := tc.runner(handler, msg, helper)
		if tc.expectedResult == nil {
			c.Logf("Name: %s, %s\n", tc.name, err)
			c.Assert(err, IsNil)
		} else {
			c.Assert(errors.Is(err, tc.expectedResult), Equals, true, Commentf("name:%s, %w", tc.name, err))
		}
		if tc.validator != nil {
			tc.validator(helper, msg, result, c)
		}
	}
}

func (h HandlerTssKeysignSuite) TestTssKeysignFailHandler_accept_standby_node_messages(c *C) {
	helper := newTssKeysignHandlerTestHelper(c)
	handler := NewTssKeysignHandler(NewDummyMgrWithKeeper(helper.keeper))
	vault := NewVault(1024, RetiringVault, AsgardVault, GetRandomPubKey(), []string{
		common.BNBChain.String(),
	}, []ChainContract{})
	accounts := NodeAccounts{}
	for i := 0; i < 8; i++ {
		na := GetRandomValidatorNode(NodeActive)
		_ = helper.keeper.SetNodeAccount(helper.ctx, na)
		vault.Membership = append(vault.Membership, na.PubKeySet.Secp256k1.String())
		accounts = append(accounts, na)
	}
	naStandby := GetRandomValidatorNode(NodeStandby)
	_ = helper.keeper.SetNodeAccount(helper.ctx, naStandby)
	vault.Membership = append(vault.Membership, naStandby.PubKeySet.Secp256k1.String())
	c.Assert(helper.keeper.SetVault(helper.ctx, vault), IsNil)
	for idx, item := range accounts {
		if idx >= 4 {
			break
		}
		msg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, item.NodeAddress, vault.PubKey)
		c.Assert(err, IsNil)
		result, err := handler.Run(helper.ctx, msg)
		c.Assert(result, NotNil)
		c.Assert(err, IsNil)
	}
	msg, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, naStandby.NodeAddress, vault.PubKey)
	c.Assert(err, IsNil)
	result, err := handler.Run(helper.ctx, msg)
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)

	msg1, err := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, naStandby.NodeAddress, vault.PubKey)
	msg1.Blame.BlameNodes = []Node{}
	c.Assert(err, IsNil)
	result, err = handler.Run(helper.ctx, msg1)
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)
	c.Assert(errors.Is(err, se.ErrUnknownRequest), Equals, true)
}
