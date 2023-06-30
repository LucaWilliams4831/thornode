package thorchain

import (
	"errors"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerObservedTxInSuite struct{}

type TestObservedTxInValidateKeeper struct {
	keeper.KVStoreDummy
	activeNodeAccount NodeAccount
	standbyAccount    NodeAccount
}

func (k *TestObservedTxInValidateKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if addr.Equals(k.standbyAccount.NodeAddress) {
		return k.standbyAccount, nil
	}
	if addr.Equals(k.activeNodeAccount.NodeAddress) {
		return k.activeNodeAccount, nil
	}
	return NodeAccount{}, errKaboom
}

func (k *TestObservedTxInValidateKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	if na.NodeAddress.Equals(k.standbyAccount.NodeAddress) {
		k.standbyAccount = na
		return nil
	}
	return errKaboom
}

var _ = Suite(&HandlerObservedTxInSuite{})

func (s *HandlerObservedTxInSuite) TestValidate(c *C) {
	var err error
	ctx, _ := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	standbyAccount := GetRandomValidatorNode(NodeStandby)
	keeper := &TestObservedTxInValidateKeeper{
		activeNodeAccount: activeNodeAccount,
		standbyAccount:    standbyAccount,
	}

	handler := NewObservedTxInHandler(NewDummyMgrWithKeeper(keeper))

	// happy path
	pk := GetRandomPubKey()
	txs := ObservedTxs{NewObservedTx(GetRandomTx(), 12, pk, 12)}
	txs[0].Tx.ToAddress, err = pk.GetAddress(txs[0].Tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	msg := NewMsgObservedTxIn(txs, activeNodeAccount.NodeAddress)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// inactive node account
	msg = NewMsgObservedTxIn(txs, GetRandomBech32Addr())
	err = handler.validate(ctx, *msg)
	c.Assert(errors.Is(err, se.ErrUnauthorized), Equals, true)

	// invalid msg
	msg = &MsgObservedTxIn{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

type TestObservedTxInFailureKeeper struct {
	keeper.KVStoreDummy
	pool Pool
}

func (k *TestObservedTxInFailureKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	if k.pool.IsEmpty() {
		return NewPool(), nil
	}
	return k.pool, nil
}

func (s *HandlerObservedTxInSuite) TestFailure(c *C) {
	ctx, _ := setupKeeperForTest(c)
	// w := getHandlerTestWrapper(c, 1, true, false)

	keeper := &TestObservedTxInFailureKeeper{
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
	}
	mgr := NewDummyMgrWithKeeper(keeper)

	tx := NewObservedTx(GetRandomTx(), 12, GetRandomPubKey(), 12)
	err := refundTx(ctx, tx, mgr, CodeInvalidMemo, "Invalid memo", "")
	c.Assert(err, IsNil)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 1)
}

type TestObservedTxInHandleKeeper struct {
	keeper.KVStoreDummy
	nas                  NodeAccounts
	voter                ObservedTxVoter
	yggExists            bool
	height               int64
	msg                  MsgSwap
	pool                 Pool
	observing            []cosmos.AccAddress
	vault                Vault
	yggVault             Vault
	txOut                *TxOut
	setLastObserveHeight bool
}

func (k *TestObservedTxInHandleKeeper) SetSwapQueueItem(_ cosmos.Context, msg MsgSwap, _ int) error {
	k.msg = msg
	return nil
}

func (k *TestObservedTxInHandleKeeper) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	return k.nas, nil
}

func (k *TestObservedTxInHandleKeeper) GetObservedTxInVoter(_ cosmos.Context, _ common.TxID) (ObservedTxVoter, error) {
	return k.voter, nil
}

func (k *TestObservedTxInHandleKeeper) SetObservedTxInVoter(_ cosmos.Context, voter ObservedTxVoter) {
	k.voter = voter
}

func (k *TestObservedTxInHandleKeeper) VaultExists(_ cosmos.Context, _ common.PubKey) bool {
	return k.yggExists
}

func (k *TestObservedTxInHandleKeeper) SetLastChainHeight(_ cosmos.Context, _ common.Chain, height int64) error {
	k.height = height
	return nil
}

func (k *TestObservedTxInHandleKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	if k.pool.IsEmpty() {
		return NewPool(), nil
	}
	return k.pool, nil
}

func (k *TestObservedTxInHandleKeeper) AddObservingAddresses(_ cosmos.Context, addrs []cosmos.AccAddress) error {
	k.observing = addrs
	return nil
}

func (k *TestObservedTxInHandleKeeper) GetVault(_ cosmos.Context, key common.PubKey) (Vault, error) {
	if k.vault.PubKey.Equals(key) {
		return k.vault, nil
	} else if k.yggVault.PubKey.Equals(key) {
		return k.yggVault, nil
	}
	return GetRandomVault(), errKaboom
}

func (k *TestObservedTxInHandleKeeper) GetAsgardVaults(_ cosmos.Context) (Vaults, error) {
	return Vaults{k.vault}, nil
}

func (k *TestObservedTxInHandleKeeper) SetVault(_ cosmos.Context, vault Vault) error {
	if k.vault.PubKey.Equals(vault.PubKey) {
		k.vault = vault
		return nil
	} else if k.yggVault.PubKey.Equals(vault.PubKey) {
		k.yggVault = vault
	}
	return errKaboom
}

func (k *TestObservedTxInHandleKeeper) GetLowestActiveVersion(_ cosmos.Context) semver.Version {
	return GetCurrentVersion()
}

func (k *TestObservedTxInHandleKeeper) IsActiveObserver(_ cosmos.Context, addr cosmos.AccAddress) bool {
	return addr.Equals(k.nas[0].NodeAddress)
}

func (k *TestObservedTxInHandleKeeper) GetTxOut(ctx cosmos.Context, blockHeight int64) (*TxOut, error) {
	if k.txOut != nil && k.txOut.Height == blockHeight {
		return k.txOut, nil
	}
	return nil, errKaboom
}

func (k *TestObservedTxInHandleKeeper) SetTxOut(ctx cosmos.Context, blockOut *TxOut) error {
	if k.txOut.Height == blockOut.Height {
		k.txOut = blockOut
		return nil
	}
	return errKaboom
}

func (k *TestObservedTxInHandleKeeper) SetLastObserveHeight(ctx cosmos.Context, chain common.Chain, address cosmos.AccAddress, height int64) error {
	k.setLastObserveHeight = true
	return nil
}

func (s *HandlerObservedTxInSuite) TestHandle(c *C) {
	s.testHandleWithVersion(c)
	s.testHandleWithConfirmation(c)
}

func (s *HandlerObservedTxInSuite) testHandleWithConfirmation(c *C) {
	var err error
	ctx, mgr := setupManagerForTest(c)
	tx := GetRandomTx()
	tx.Memo = "SWAP:BTC.BTC:" + GetRandomBTCAddress().String()
	obTx := NewObservedTx(tx, 12, GetRandomPubKey(), 15)
	txs := ObservedTxs{obTx}
	pk := GetRandomPubKey()
	txs[0].Tx.ToAddress, err = pk.GetAddress(txs[0].Tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	vault := GetRandomVault()
	vault.PubKey = obTx.ObservedPubKey

	keeper := &TestObservedTxInHandleKeeper{
		nas: NodeAccounts{
			GetRandomValidatorNode(NodeActive),
			GetRandomValidatorNode(NodeActive),
			GetRandomValidatorNode(NodeActive),
			GetRandomValidatorNode(NodeActive),
		},
		vault: vault,
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
		yggExists: true,
	}
	mgr.K = keeper
	handler := NewObservedTxInHandler(mgr)

	// first not confirmed message
	msg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	voter, err := keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.Txs, HasLen, 1)
	// tx has not reach consensus yet, thus fund should not be credit to vault
	c.Assert(keeper.vault.HasFunds(), Equals, false)
	c.Assert(voter.UpdatedVault, Equals, false)
	c.Assert(voter.FinalisedHeight, Equals, int64(0))
	c.Assert(voter.Height, Equals, int64(0))
	mgr.ObMgr().EndBlock(ctx, keeper)

	// second not confirmed message
	msg1 := NewMsgObservedTxIn(txs, keeper.nas[1].NodeAddress)
	_, err = handler.handle(ctx, *msg1)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.Txs, HasLen, 1)
	c.Assert(voter.UpdatedVault, Equals, false)
	c.Assert(voter.FinalisedHeight, Equals, int64(0))
	c.Assert(voter.Height, Equals, int64(0))
	c.Assert(keeper.vault.HasFunds(), Equals, false)

	// third not confirmed message
	msg2 := NewMsgObservedTxIn(txs, keeper.nas[2].NodeAddress)
	_, err = handler.handle(ctx, *msg2)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.Txs, HasLen, 1)
	c.Assert(voter.UpdatedVault, Equals, true)
	c.Assert(voter.FinalisedHeight, Equals, int64(0))
	c.Check(keeper.height, Equals, int64(12))
	// make sure fund has been credit to vault correctly
	bnbCoin := keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.OneUint()), Equals, true)
	// make sure the logic has not been processed , as tx has not been finalised , still waiting for confirmation
	c.Check(keeper.msg.Tx.ID.Equals(tx.ID), Equals, false)

	// fourth not confirmed message
	msg3 := NewMsgObservedTxIn(txs, keeper.nas[3].NodeAddress)
	_, err = handler.handle(ctx, *msg3)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.Txs, HasLen, 1)
	c.Assert(voter.UpdatedVault, Equals, true)
	c.Assert(voter.FinalisedHeight, Equals, int64(0))
	c.Check(keeper.height, Equals, int64(12))
	// make sure fund has not been doubled
	bnbCoin = keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.OneUint()), Equals, true)
	c.Check(keeper.msg.Tx.ID.Equals(tx.ID), Equals, false)

	//  first finalised message
	txs[0].BlockHeight = 15
	fMsg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, *fMsg)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.UpdatedVault, Equals, true)
	c.Assert(voter.FinalisedHeight, Equals, int64(0))
	c.Assert(voter.Height, Equals, int64(18))
	// make sure fund has not been doubled
	bnbCoin = keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.OneUint()), Equals, true)
	c.Check(keeper.msg.Tx.ID.Equals(tx.ID), Equals, false)

	// second finalised message
	fMsg1 := NewMsgObservedTxIn(txs, keeper.nas[1].NodeAddress)
	_, err = handler.handle(ctx, *fMsg1)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.UpdatedVault, Equals, true)
	c.Assert(voter.FinalisedHeight, Equals, int64(0))
	c.Assert(voter.Height, Equals, int64(18))
	// make sure fund has not been doubled
	bnbCoin = keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.OneUint()), Equals, true)
	c.Check(keeper.msg.Tx.ID.Equals(tx.ID), Equals, false)

	// third finalised message
	fMsg2 := NewMsgObservedTxIn(txs, keeper.nas[2].NodeAddress)
	_, err = handler.handle(ctx, *fMsg2)
	c.Assert(err, IsNil)
	voter, err = keeper.GetObservedTxInVoter(ctx, tx.ID)
	c.Assert(err, IsNil)
	c.Assert(voter.UpdatedVault, Equals, true)
	c.Assert(voter.FinalisedHeight, Equals, int64(18))
	c.Assert(voter.Height, Equals, int64(18))
	// make sure fund has not been doubled
	bnbCoin = keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.OneUint()), Equals, true)
	c.Check(keeper.msg.Tx.ID.Equals(tx.ID), Equals, true)
}

func (s *HandlerObservedTxInSuite) testHandleWithVersion(c *C) {
	var err error
	ctx, mgr := setupManagerForTest(c)

	tx := GetRandomTx()
	tx.Memo = "SWAP:BTC.BTC:" + GetRandomBTCAddress().String()
	obTx := NewObservedTx(tx, 12, GetRandomPubKey(), 12)
	txs := ObservedTxs{obTx}
	pk := GetRandomPubKey()
	txs[0].Tx.ToAddress, err = pk.GetAddress(txs[0].Tx.Coins[0].Asset.Chain)

	vault := GetRandomVault()
	vault.PubKey = obTx.ObservedPubKey

	keeper := &TestObservedTxInHandleKeeper{
		nas:   NodeAccounts{GetRandomValidatorNode(NodeActive)},
		voter: NewObservedTxVoter(tx.ID, make(ObservedTxs, 0)),
		vault: vault,
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
		yggExists: true,
	}
	mgr.K = keeper
	handler := NewObservedTxInHandler(mgr)

	c.Assert(err, IsNil)
	msg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	mgr.ObMgr().EndBlock(ctx, keeper)
	c.Check(keeper.msg.Tx.ID.Equals(tx.ID), Equals, true)
	c.Check(keeper.observing, HasLen, 1)
	c.Check(keeper.height, Equals, int64(12))
	bnbCoin := keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.OneUint()), Equals, true)
}

// Test migrate memo
func (s *HandlerObservedTxInSuite) TestMigrateMemo(c *C) {
	var err error
	ctx, _ := setupKeeperForTest(c)

	vault := GetRandomVault()
	addr, err := vault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	newVault := GetRandomVault()
	txout := NewTxOut(12)
	newVaultAddr, err := newVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)

	txout.TxArray = append(txout.TxArray, TxOutItem{
		Chain:       common.BNBChain,
		InHash:      common.BlankTxID,
		ToAddress:   newVaultAddr,
		VaultPubKey: vault.PubKey,
		Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		Memo:        NewMigrateMemo(1).String(),
	})
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		},
		Memo:        NewMigrateMemo(12).String(),
		FromAddress: addr,
		ToAddress:   newVaultAddr,
		Gas:         BNBGasFeeSingleton,
	}, 13, vault.PubKey, 13)

	txs := ObservedTxs{tx}
	keeper := &TestObservedTxInHandleKeeper{
		nas:   NodeAccounts{GetRandomValidatorNode(NodeActive)},
		voter: NewObservedTxVoter(tx.Tx.ID, make(ObservedTxs, 0)),
		vault: vault,
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
		yggExists: true,
		txOut:     txout,
	}

	handler := NewObservedTxInHandler(NewDummyMgrWithKeeper(keeper))

	c.Assert(err, IsNil)
	msg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
}

type ObservedTxInHandlerTestHelper struct {
	keeper.Keeper
	failListActiveValidators bool
	failVaultExist           bool
	failGetObservedTxInVote  bool
	failGetVault             bool
	failSetVault             bool
}

func NewObservedTxInHandlerTestHelper(k keeper.Keeper) *ObservedTxInHandlerTestHelper {
	return &ObservedTxInHandlerTestHelper{
		Keeper: k,
	}
}

func (h *ObservedTxInHandlerTestHelper) ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error) {
	if h.failListActiveValidators {
		return NodeAccounts{}, errKaboom
	}
	return h.Keeper.ListActiveValidators(ctx)
}

func (h *ObservedTxInHandlerTestHelper) VaultExists(ctx cosmos.Context, pk common.PubKey) bool {
	if h.failVaultExist {
		return false
	}
	return h.Keeper.VaultExists(ctx, pk)
}

func (h *ObservedTxInHandlerTestHelper) GetObservedTxInVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error) {
	if h.failGetObservedTxInVote {
		return ObservedTxVoter{}, errKaboom
	}
	return h.Keeper.GetObservedTxInVoter(ctx, hash)
}

func (h *ObservedTxInHandlerTestHelper) GetVault(ctx cosmos.Context, pk common.PubKey) (Vault, error) {
	if h.failGetVault {
		return Vault{}, errKaboom
	}
	return h.Keeper.GetVault(ctx, pk)
}

func (h *ObservedTxInHandlerTestHelper) SetVault(ctx cosmos.Context, vault Vault) error {
	if h.failSetVault {
		return errKaboom
	}
	return h.Keeper.SetVault(ctx, vault)
}

func setupAnLegitObservedTx(ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper, c *C) *MsgObservedTxIn {
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	pk := GetRandomPubKey()
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*3)),
	}
	tx.Memo = "SWAP:RUNE"
	addr, err := pk.GetAddress(tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	tx.ToAddress = addr
	obTx := NewObservedTx(tx, ctx.BlockHeight(), pk, ctx.BlockHeight())
	txs := ObservedTxs{obTx}
	txs[0].Tx.ToAddress, err = pk.GetAddress(txs[0].Tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	vault := GetRandomVault()
	vault.PubKey = obTx.ObservedPubKey
	c.Assert(helper.Keeper.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	c.Assert(helper.SetVault(ctx, vault), IsNil)
	p := NewPool()
	p.Asset = common.BNBAsset
	p.BalanceRune = cosmos.NewUint(100 * common.One)
	p.BalanceAsset = cosmos.NewUint(100 * common.One)
	p.Status = PoolAvailable
	c.Assert(helper.Keeper.SetPool(ctx, p), IsNil)
	return NewMsgObservedTxIn(ObservedTxs{
		obTx,
	}, activeNodeAccount.NodeAddress)
}

func (HandlerObservedTxInSuite) TestObservedTxHandler_validations(c *C) {
	testCases := []struct {
		name            string
		messageProvider func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg
		validator       func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string)
	}{
		{
			name: "invalid message should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				return NewMsgNetworkFee(ctx.BlockHeight(), common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil)
				c.Check(errors.Is(err, errInvalidMessage), Equals, true)
			},
		},
		{
			name: "message fail validation should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				return NewMsgObservedTxIn(ObservedTxs{
					NewObservedTx(GetRandomTx(), ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()),
				}, GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil)
			},
		},
		{
			name: "signer vote for the same tx should be slashed , and not doing anything else",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				voter, err := helper.Keeper.GetObservedTxInVoter(ctx, m.Txs[0].Tx.ID)
				c.Assert(err, IsNil)
				voter.Add(m.Txs[0], m.Signer)
				helper.Keeper.SetObservedTxInVoter(ctx, voter)
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to list active node accounts should result in an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				helper.failListActiveValidators = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "vault not exist should not result in an error, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				helper.failVaultExist = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to get observedTxInVoter should not result in an error, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				helper.failGetObservedTxInVote = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "empty memo should not result in an error, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				m.Txs[0].Tx.Memo = ""
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
				txOut, err := helper.GetTxOut(ctx, ctx.BlockHeight())
				c.Assert(err, IsNil, Commentf(name))
				c.Assert(txOut.IsEmpty(), Equals, false)
			},
		},
		{
			name: "fail to get vault, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				helper.failGetVault = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to set vault, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				helper.failSetVault = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "if the vault is not asgard, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				vault, err := helper.Keeper.GetVault(ctx, m.Txs[0].ObservedPubKey)
				c.Assert(err, IsNil)
				vault.Type = YggdrasilVault
				c.Assert(helper.Keeper.SetVault(ctx, vault), IsNil)
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "inactive vault, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				vault, err := helper.Keeper.GetVault(ctx, m.Txs[0].ObservedPubKey)
				c.Assert(err, IsNil)
				vault.Status = InactiveVault
				c.Assert(helper.Keeper.SetVault(ctx, vault), IsNil)
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "chain halt, it should refund",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				helper.Keeper.SetMimir(ctx, "HaltTrading", 1)
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
				txOut, err := helper.GetTxOut(ctx, ctx.BlockHeight())
				c.Assert(err, IsNil, Commentf(name))
				c.Assert(txOut.IsEmpty(), Equals, false)
			},
		},
		{
			name: "normal provision, it should success",
			messageProvider: func(c *C, ctx cosmos.Context, helper *ObservedTxInHandlerTestHelper) cosmos.Msg {
				m := setupAnLegitObservedTx(ctx, helper, c)
				m.Txs[0].Tx.Memo = "add:BNB.BNB"
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ObservedTxInHandlerTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
	}
	versions := []semver.Version{
		GetCurrentVersion(),
	}
	for _, tc := range testCases {
		for _, ver := range versions {
			ctx, mgr := setupManagerForTest(c)
			helper := NewObservedTxInHandlerTestHelper(mgr.Keeper())
			mgr.K = helper
			mgr.currentVersion = ver
			handler := NewObservedTxInHandler(mgr)
			msg := tc.messageProvider(c, ctx, helper)
			result, err := handler.Run(ctx, msg)
			tc.validator(c, ctx, result, err, helper, tc.name)
		}
	}
}

func (s HandlerObservedTxInSuite) TestSwapWithAffiliate(c *C) {
	ctx, mgr := setupManagerForTest(c)

	queue := newSwapQv58(mgr.Keeper())
	handler := NewObservedTxInHandler(mgr)

	msg := NewMsgSwap(common.Tx{
		ID:          common.TxID("5E1DF027321F1FE37CA19B9ECB11C2B4ABEC0D8322199D335D9CE4C39F85F115"),
		FromAddress: GetRandomBNBAddress(),
		ToAddress:   GetRandomBNBAddress(),
		Gas:         BNBGasFeeSingleton,
		Chain:       common.BNBChain,
		Coins:       common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One))},
	}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), GetRandomTHORAddress(), cosmos.NewUint(1000),
		"",
		"", nil,
		MarketOrder,
		GetRandomBech32Addr(),
	)
	handler.addSwap(ctx, *msg)
	swaps, err := queue.FetchQueue(ctx)
	c.Assert(err, IsNil)
	c.Assert(swaps, HasLen, 2, Commentf("%d", len(swaps)))
	c.Check(swaps[0].msg.Tx.Coins[0].Amount.Uint64(), Equals, uint64(180000000))
	c.Check(swaps[1].msg.Tx.Coins[0].Amount.Uint64(), Equals, uint64(20000000))
}

func (s *HandlerObservedTxInSuite) TestVaultStatus(c *C) {
	testCases := []struct {
		name                 string
		statusAtConsensus    VaultStatus
		statusAtFinalisation VaultStatus
	}{
		{
			name:                 "should observe if active on consensus and finalisation",
			statusAtConsensus:    ActiveVault,
			statusAtFinalisation: ActiveVault,
		}, {
			name:                 "should observe if active on consensus, inactive on finalisation",
			statusAtConsensus:    ActiveVault,
			statusAtFinalisation: InactiveVault,
		}, {
			name:                 "should not observe if inactive on consensus",
			statusAtConsensus:    InactiveVault,
			statusAtFinalisation: InactiveVault,
		},
	}
	for _, tc := range testCases {
		var err error
		ctx, mgr := setupManagerForTest(c)
		tx := GetRandomTx()
		tx.Memo = "SWAP:BTC.BTC:" + GetRandomBTCAddress().String()
		obTx := NewObservedTx(tx, 12, GetRandomPubKey(), 15)
		txs := ObservedTxs{obTx}
		vault := GetRandomVault()
		vault.PubKey = obTx.ObservedPubKey
		keeper := &TestObservedTxInHandleKeeper{
			nas:       NodeAccounts{GetRandomValidatorNode(NodeActive)},
			voter:     NewObservedTxVoter(tx.ID, make(ObservedTxs, 0)),
			vault:     vault,
			yggExists: true,
		}
		mgr.K = keeper
		handler := NewObservedTxInHandler(mgr)

		keeper.vault.Status = tc.statusAtConsensus
		msg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
		_, err = handler.handle(ctx, *msg)
		c.Assert(err, IsNil, Commentf(tc.name))
		c.Check(keeper.voter.Height, Equals, int64(18), Commentf(tc.name))

		c.Check(keeper.voter.UpdatedVault, Equals, true, Commentf(tc.name))
		c.Check(keeper.vault.InboundTxCount, Equals, int64(0), Commentf(tc.name))

		keeper.vault.Status = tc.statusAtFinalisation
		txs[0].BlockHeight = 15
		msg = NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
		ctx = ctx.WithBlockHeight(30)
		_, err = handler.handle(ctx, *msg)
		c.Assert(err, IsNil, Commentf(tc.name))
		c.Check(keeper.voter.FinalisedHeight, Equals, int64(30), Commentf(tc.name))

		c.Check(keeper.voter.UpdatedVault, Equals, true, Commentf(tc.name))
		c.Check(keeper.vault.InboundTxCount, Equals, int64(1), Commentf(tc.name))
	}
}

func (s *HandlerObservedTxInSuite) TestYggFundedOnlyFromAsgard(c *C) {
	ctx, mgr := setupManagerForTest(c)

	tx := GetRandomTx()
	vault := GetRandomVault()
	obTx := NewObservedTx(tx, 12, GetRandomPubKey(), 15)

	vault.PubKey = obTx.ObservedPubKey
	asgardFromAddr, err := vault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	obTx.Tx.FromAddress = asgardFromAddr

	keeper := &TestObservedTxInHandleKeeper{
		nas:       NodeAccounts{GetRandomValidatorNode(NodeActive)},
		voter:     NewObservedTxVoter(tx.ID, make(ObservedTxs, 0)),
		vault:     vault,
		yggExists: true,
	}
	mgr.K = keeper
	handler := NewObservedTxInHandler(mgr)

	// Test isFromAsgard func
	isFromAsgard, err := handler.isFromAsgard(ctx, obTx)
	c.Assert(err, IsNil)
	c.Assert(isFromAsgard, Equals, true)

	obTx.Tx.FromAddress = GetRandomBNBAddress()
	isFromAsgard, err = handler.isFromAsgard(ctx, obTx)
	c.Assert(err, IsNil)
	c.Assert(isFromAsgard, Equals, false)

	// TX is not from Asgard, shouldn't fund Ygg
	fundTx := GetRandomTx()
	fundTx.Memo = "yggdrasil+:15"
	obTx = NewObservedTx(fundTx, 12, GetRandomPubKey(), 15)
	ygg := GetRandomVault()
	ygg.Type = YggdrasilVault
	ygg.PubKey = obTx.ObservedPubKey

	txValue := tx.Coins[0].Amount
	yggBnbBalanceBefore := ygg.GetCoin(common.BNBAsset).Amount

	keeper.yggVault = ygg
	keeper.voter = NewObservedTxVoter(tx.ID, make(ObservedTxs, 0))
	mgr.K = keeper

	handler = NewObservedTxInHandler(mgr)
	txs := ObservedTxs{obTx}
	txs[0].BlockHeight = 15
	msg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)

	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Assert(keeper.yggVault.GetCoin(common.BNBAsset).Amount.Sub(yggBnbBalanceBefore).Uint64(), Equals, cosmos.ZeroUint().Uint64())

	// TX is from asgard, should fund Ygg
	keeper.voter = NewObservedTxVoter(tx.ID, make(ObservedTxs, 0))
	mgr.K = keeper

	handler = NewObservedTxInHandler(mgr)
	txs[0].Tx.FromAddress = asgardFromAddr
	msg = NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)

	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Assert(keeper.yggVault.GetCoin(common.BNBAsset).Amount.Sub(yggBnbBalanceBefore).Uint64(), Equals, txValue.Uint64())
}
