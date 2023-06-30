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

type HandlerOutboundTxSuite struct{}

type TestOutboundTxKeeper struct {
	keeper.KVStoreDummy
	activeNodeAccount NodeAccount
	vault             Vault
}

func (k *TestOutboundTxKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.activeNodeAccount.NodeAddress.Equals(addr) {
		return k.activeNodeAccount, nil
	}
	return NodeAccount{}, nil
}

var _ = Suite(&HandlerOutboundTxSuite{})

func (s *HandlerOutboundTxSuite) SetUpSuite(_ *C) {
	SetupConfigForTest()
}

func (s *HandlerOutboundTxSuite) TestValidate(c *C) {
	ctx, mgr := setupManagerForTest(c)

	k := &TestOutboundTxKeeper{
		activeNodeAccount: GetRandomValidatorNode(NodeActive),
		vault:             GetRandomVault(),
	}

	mgr.K = k
	mgr.slasher = newSlasherV75(k, NewDummyEventMgr())

	handler := NewOutboundTxHandler(mgr)

	addr, err := k.vault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)

	tx := NewObservedTx(common.Tx{
		ID:          GetRandomTxHash(),
		Chain:       common.BNBChain,
		Coins:       common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One))},
		Memo:        "",
		FromAddress: GetRandomBNBAddress(),
		ToAddress:   addr,
		Gas:         BNBGasFeeSingleton,
	}, 12, GetRandomPubKey(), 12)

	msgOutboundTx := NewMsgOutboundTx(tx, tx.Tx.ID, k.activeNodeAccount.NodeAddress)
	sErr := handler.validate(ctx, *msgOutboundTx)
	c.Assert(sErr, IsNil)

	result, err := handler.handle(ctx, *msgOutboundTx)
	c.Check(result, IsNil)
	c.Check(err, NotNil)

	// invalid msg
	msgOutboundTx = &MsgOutboundTx{}
	sErr = handler.validate(ctx, *msgOutboundTx)
	c.Assert(sErr, NotNil)
}

type outboundTxHandlerTestHelper struct {
	ctx           cosmos.Context
	pool          Pool
	version       semver.Version
	keeper        *outboundTxHandlerKeeperHelper
	asgardVault   Vault
	yggVault      Vault
	constAccessor constants.ConstantValues
	nodeAccount   NodeAccount
	inboundTx     ObservedTx
	toi           TxOutItem
	mgr           Manager
}

type outboundTxHandlerKeeperHelper struct {
	keeper.Keeper
	observeTxVoterErrHash common.TxID
	errGetTxOut           bool
	errGetNodeAccount     bool
	errGetPool            bool
	errSetPool            bool
	errSetNodeAccount     bool
	errGetNetwork         bool
	errSetNetwork         bool
	vault                 Vault
}

func newOutboundTxHandlerKeeperHelper(keeper keeper.Keeper) *outboundTxHandlerKeeperHelper {
	return &outboundTxHandlerKeeperHelper{
		Keeper:                keeper,
		observeTxVoterErrHash: GetRandomTxHash(),
	}
}

func (k *outboundTxHandlerKeeperHelper) GetObservedTxInVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error) {
	if hash.Equals(k.observeTxVoterErrHash) {
		return ObservedTxVoter{}, errKaboom
	}
	return k.Keeper.GetObservedTxOutVoter(ctx, hash)
}

func (k *outboundTxHandlerKeeperHelper) GetTxOut(ctx cosmos.Context, height int64) (*TxOut, error) {
	if k.errGetTxOut {
		return nil, errKaboom
	}
	return k.Keeper.GetTxOut(ctx, height)
}

func (k *outboundTxHandlerKeeperHelper) GetNodeAccountByPubKey(ctx cosmos.Context, pk common.PubKey) (NodeAccount, error) {
	if k.errGetNodeAccount {
		return NodeAccount{}, errKaboom
	}
	return k.Keeper.GetNodeAccountByPubKey(ctx, pk)
}

func (k *outboundTxHandlerKeeperHelper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if k.errGetPool {
		return NewPool(), errKaboom
	}
	return k.Keeper.GetPool(ctx, asset)
}

func (k *outboundTxHandlerKeeperHelper) SetPool(ctx cosmos.Context, pool Pool) error {
	if k.errSetPool {
		return errKaboom
	}
	return k.Keeper.SetPool(ctx, pool)
}

func (k *outboundTxHandlerKeeperHelper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	if k.errSetNodeAccount {
		return errKaboom
	}
	return k.Keeper.SetNodeAccount(ctx, na)
}

func (k *outboundTxHandlerKeeperHelper) GetAsgardVaultsByStatus(ctx cosmos.Context, status VaultStatus) (Vaults, error) {
	return k.Keeper.GetAsgardVaultsByStatus(ctx, status)
}

func (k *outboundTxHandlerKeeperHelper) GetVault(_ cosmos.Context, _ common.PubKey) (Vault, error) {
	return k.vault, nil
}

func (k *outboundTxHandlerKeeperHelper) SetVault(_ cosmos.Context, v Vault) error {
	k.vault = v
	return nil
}

func (k *outboundTxHandlerKeeperHelper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if k.errGetNetwork {
		return Network{}, errKaboom
	}
	return k.Keeper.GetNetwork(ctx)
}

func (k *outboundTxHandlerKeeperHelper) SetNetwork(ctx cosmos.Context, data Network) error {
	if k.errSetNetwork {
		return errKaboom
	}
	return k.Keeper.SetNetwork(ctx, data)
}

// newOutboundTxHandlerTestHelper setup all the basic condition to test OutboundTxHandler
func newOutboundTxHandlerTestHelper(c *C) outboundTxHandlerTestHelper {
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(1023)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.LPUnits = pool.BalanceRune

	version := GetCurrentVersion()
	asgardVault := GetRandomVault()
	asgardVault.Membership = []string{asgardVault.PubKey.String()}
	c.Assert(mgr.Keeper().SetVault(ctx, asgardVault), IsNil)
	addr, err := asgardVault.PubKey.GetAddress(common.RuneAsset().Chain)
	yggVault := GetRandomVault()
	yggVault.Membership = []string{yggVault.PubKey.String()}
	c.Assert(err, IsNil)

	tx := NewObservedTx(common.Tx{
		ID:          GetRandomTxHash(),
		Chain:       common.RuneAsset().Chain,
		Coins:       common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One))},
		Memo:        "SWAP:" + common.RuneAsset().String(),
		FromAddress: GetRandomBNBAddress(),
		ToAddress:   addr,
		Gas:         BNBGasFeeSingleton,
	}, 12, GetRandomPubKey(), 12)

	keeperHelper := newOutboundTxHandlerKeeperHelper(mgr.Keeper())
	keeperHelper.vault = yggVault

	nodeAccount := GetRandomValidatorNode(NodeActive)
	nodeAccount.NodeAddress, err = yggVault.PubKey.GetThorAddress()
	c.Assert(err, IsNil)
	nodeAccount.Bond = cosmos.NewUint(100 * common.One)
	FundModule(c, ctx, mgr.Keeper(), BondName, nodeAccount.Bond.Uint64())
	nodeAccount.PubKeySet = common.NewPubKeySet(yggVault.PubKey, yggVault.PubKey)
	c.Assert(keeperHelper.SetNodeAccount(ctx, nodeAccount), IsNil)

	c.Assert(keeperHelper.SetPool(ctx, pool), IsNil)
	voter := NewObservedTxVoter(tx.Tx.ID, make(ObservedTxs, 0))
	voter.Add(tx, nodeAccount.NodeAddress)
	voter.FinalisedHeight = ctx.BlockHeight()
	voter.Tx = voter.GetTx(NodeAccounts{nodeAccount})
	keeperHelper.SetObservedTxOutVoter(ctx, voter)

	constAccessor := constants.GetConstantValues(version)
	txOutStorage := newTxOutStorageV83(keeperHelper, constAccessor, NewDummyEventMgr(), newGasMgrV81(constAccessor, keeperHelper))
	toi := TxOutItem{
		Chain:       common.BNBChain,
		ToAddress:   tx.Tx.FromAddress,
		VaultPubKey: yggVault.PubKey,
		Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One)),
		Memo:        NewOutboundMemo(tx.Tx.ID).String(),
		InHash:      tx.Tx.ID,
	}
	mgr.K = keeperHelper
	mgr.slasher = newSlasherV75(keeperHelper, NewDummyEventMgr())
	result, err := txOutStorage.TryAddTxOutItem(ctx, mgr, toi, cosmos.ZeroUint())
	c.Assert(err, IsNil)
	c.Check(result, Equals, true)

	return outboundTxHandlerTestHelper{
		ctx:           ctx,
		pool:          pool,
		version:       version,
		keeper:        keeperHelper,
		asgardVault:   asgardVault,
		yggVault:      yggVault,
		nodeAccount:   nodeAccount,
		inboundTx:     tx,
		toi:           toi,
		constAccessor: constAccessor,
		mgr:           mgr,
	}
}

func (s *HandlerOutboundTxSuite) TestOutboundTxHandlerShouldUpdateTxOut(c *C) {
	testCases := []struct {
		name           string
		messageCreator func(helper outboundTxHandlerTestHelper, tx ObservedTx) cosmos.Msg
		runner         func(handler OutboundTxHandler, helper outboundTxHandlerTestHelper, msg cosmos.Msg) (*cosmos.Result, error)
		expectedResult error
	}{
		{
			name: "invalid message should return an error",
			messageCreator: func(helper outboundTxHandlerTestHelper, tx ObservedTx) cosmos.Msg {
				return NewMsgNoOp(GetRandomObservedTx(), helper.nodeAccount.NodeAddress, "")
			},
			runner: func(handler OutboundTxHandler, helper outboundTxHandlerTestHelper, msg cosmos.Msg) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: errInvalidMessage,
		},
		{
			name: "fail to get observed TxVoter should result in an error",
			messageCreator: func(helper outboundTxHandlerTestHelper, tx ObservedTx) cosmos.Msg {
				return NewMsgOutboundTx(tx, helper.keeper.observeTxVoterErrHash, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler OutboundTxHandler, helper outboundTxHandlerTestHelper, msg cosmos.Msg) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: errInternal,
		},
		{
			name: "fail to get txout should result in an error",
			messageCreator: func(helper outboundTxHandlerTestHelper, tx ObservedTx) cosmos.Msg {
				return NewMsgOutboundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler OutboundTxHandler, helper outboundTxHandlerTestHelper, msg cosmos.Msg) (*cosmos.Result, error) {
				helper.keeper.errGetTxOut = true
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: se.ErrUnknownRequest,
		},
		{
			name: "valid outbound message, no event, no txout",
			messageCreator: func(helper outboundTxHandlerTestHelper, tx ObservedTx) cosmos.Msg {
				return NewMsgOutboundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler OutboundTxHandler, helper outboundTxHandlerTestHelper, msg cosmos.Msg) (*cosmos.Result, error) {
				return handler.Run(helper.ctx, msg)
			},
			expectedResult: nil,
		},
	}

	for _, tc := range testCases {
		helper := newOutboundTxHandlerTestHelper(c)
		handler := NewOutboundTxHandler(helper.mgr)
		fromAddr, err := helper.yggVault.PubKey.GetAddress(common.BNBChain)
		c.Assert(err, IsNil)
		tx := NewObservedTx(common.Tx{
			ID:    GetRandomTxHash(),
			Chain: common.BNBChain,
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
			},
			Memo:        NewOutboundMemo(helper.inboundTx.Tx.ID).String(),
			FromAddress: fromAddr,
			ToAddress:   helper.inboundTx.Tx.FromAddress,
			Gas:         BNBGasFeeSingleton,
		}, helper.ctx.BlockHeight(), helper.yggVault.PubKey, helper.ctx.BlockHeight())
		msg := tc.messageCreator(helper, tx)
		_, err = tc.runner(handler, helper, msg)
		if tc.expectedResult == nil {
			c.Check(err, IsNil)
		} else {
			c.Check(errors.Is(err, tc.expectedResult), Equals, true, Commentf("name: %s, Err: %s", tc.name, err))
		}
	}
}

func (s *HandlerOutboundTxSuite) TestOutboundTxNormalCase(c *C) {
	helper := newOutboundTxHandlerTestHelper(c)
	handler := NewOutboundTxHandler(helper.mgr)

	fromAddr, err := helper.yggVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(199887500)),
		},
		Memo:        NewOutboundMemo(helper.inboundTx.Tx.ID).String(),
		FromAddress: fromAddr,
		ToAddress:   helper.inboundTx.Tx.FromAddress,
		Gas:         BNBGasFeeSingleton,
	}, helper.ctx.BlockHeight(), helper.yggVault.PubKey, helper.ctx.BlockHeight())
	// valid outbound message, with event, with txout
	outMsg := NewMsgOutboundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
	_, err = handler.Run(helper.ctx, outMsg)
	c.Assert(err, IsNil)
	// txout should had been complete

	txOut, err := helper.keeper.GetTxOut(helper.ctx, helper.ctx.BlockHeight())
	c.Assert(err, IsNil)
	c.Assert(txOut.TxArray[0].OutHash.IsEmpty(), Equals, false)
}

func (s *HandlerOutboundTxSuite) TestOuboundTxHandlerSendExtraFundShouldBeSlashed(c *C) {
	helper := newOutboundTxHandlerTestHelper(c)
	handler := NewOutboundTxHandler(helper.mgr)
	fromAddr, err := helper.asgardVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(2*common.One)),
		},
		Memo:        NewOutboundMemo(helper.inboundTx.Tx.ID).String(),
		FromAddress: fromAddr,
		ToAddress:   helper.inboundTx.Tx.FromAddress,
		Gas:         BNBGasFeeSingleton,
	}, helper.ctx.BlockHeight(), helper.nodeAccount.PubKeySet.Secp256k1, helper.ctx.BlockHeight())
	// expectedBond := helper.nodeAccount.Bond.Sub(cosmos.NewUint(common.One * 2).Add(BNBGasFeeSingleton[0].Amount).MulUint64(3).QuoUint64(2))
	expectedBond := cosmos.NewUint(9699945439)
	expectedVaultTotalReserve := cosmos.NewUint(7766279631552373170)
	// valid outbound message, with event, with txout
	outMsg := NewMsgOutboundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
	_, err = handler.Run(helper.ctx, outMsg)
	c.Assert(err, IsNil)
	na, err := helper.keeper.GetNodeAccount(helper.ctx, helper.nodeAccount.NodeAddress)
	c.Assert(na.Bond.Equal(expectedBond), Equals, true, Commentf("expected bond:%s, real bond: %s", expectedBond, na.Bond))
	newReserve := helper.keeper.GetRuneBalanceOfModule(helper.ctx, ReserveName)
	c.Assert(err, IsNil)
	c.Assert(newReserve.Equal(expectedVaultTotalReserve), Equals, true)
}

func (s *HandlerOutboundTxSuite) TestOutboundTxHandlerSendAdditionalCoinsShouldBeSlashed(c *C) {
	helper := newOutboundTxHandlerTestHelper(c)
	handler := NewOutboundTxHandler(helper.mgr)
	fromAddr, err := helper.asgardVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(1*common.One)),
			common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One)),
		},
		Memo:        NewOutboundMemo(helper.inboundTx.Tx.ID).String(),
		FromAddress: fromAddr,
		ToAddress:   helper.inboundTx.Tx.FromAddress,
		Gas:         BNBGasFeeSingleton,
	}, helper.ctx.BlockHeight(), helper.nodeAccount.PubKeySet.Secp256k1, helper.ctx.BlockHeight())
	expectedBond := cosmos.NewUint(9700704420)
	// slash one BNB, and one rune
	outMsg := NewMsgOutboundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
	_, err = handler.Run(helper.ctx, outMsg)
	c.Assert(err, IsNil)
	na, err := helper.keeper.GetNodeAccount(helper.ctx, helper.nodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(na.Bond.Equal(expectedBond), Equals, true, Commentf("%d/%d", na.Bond.Uint64(), expectedBond.Uint64()))
}

func (s *HandlerOutboundTxSuite) TestOutboundTxHandlerInvalidObservedTxVoterShouldSlash(c *C) {
	helper := newOutboundTxHandlerTestHelper(c)
	handler := NewOutboundTxHandler(helper.mgr)
	fromAddr, err := helper.asgardVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(1*common.One)),
			common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One)),
		},
		Memo:        NewOutboundMemo(helper.inboundTx.Tx.ID).String(),
		FromAddress: fromAddr,
		ToAddress:   helper.inboundTx.Tx.FromAddress,
		Gas:         BNBGasFeeSingleton,
	}, helper.ctx.BlockHeight(), helper.nodeAccount.PubKeySet.Secp256k1, helper.ctx.BlockHeight())

	expectedBond := cosmos.NewUint(9700704420)

	// expected 0.5 slashed RUNE be added to reserve
	expectedVaultTotalReserve := cosmos.NewUint(7766279631552877459)
	pool, err := helper.keeper.GetPool(helper.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	poolBNB := common.SafeSub(pool.BalanceAsset, cosmos.NewUint(common.One).Add(BNBGasFeeSingleton[0].Amount))

	// given the outbound tx doesn't have relevant OservedTxVoter in system ,
	// thus it should be slashed with 1.5 * the full amount of assets
	outMsg := NewMsgOutboundTx(tx, tx.Tx.ID, helper.nodeAccount.NodeAddress)
	_, err = handler.Run(helper.ctx, outMsg)
	c.Assert(err, IsNil)
	na, err := helper.keeper.GetNodeAccount(helper.ctx, helper.nodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(na.Bond.Equal(expectedBond), Equals, true, Commentf("%d/%d", na.Bond.Uint64(), expectedBond.Uint64()))

	newReserve := helper.keeper.GetRuneBalanceOfModule(helper.ctx, ReserveName)
	c.Assert(newReserve.Equal(expectedVaultTotalReserve), Equals, true)
	pool, err = helper.keeper.GetPool(helper.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.BalanceRune.Equal(cosmos.NewUint(10100933578)), Equals, true, Commentf("%d/%s", pool.BalanceRune.Uint64(), cosmos.NewUint(10149884125)))
	c.Assert(pool.BalanceAsset.Equal(poolBNB), Equals, true, Commentf("%d/%d", pool.BalanceAsset.Uint64(), poolBNB.Uint64()))
}

func (s *HandlerOutboundTxSuite) TestOutboundTxHandlerETHChainSpendTooMuchGasShouldSlash(c *C) {
	helper := newOutboundTxHandlerTestHelper(c)
	handler := NewOutboundTxHandler(helper.mgr)
	pool := NewPool()
	pool.Asset = common.ETHAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.LPUnits = pool.BalanceRune
	c.Assert(helper.keeper.SetPool(helper.ctx, pool), IsNil)
	fromAddr, err := helper.yggVault.PubKey.GetAddress(common.ETHChain)
	c.Assert(err, IsNil)
	usdtAsset, err := common.NewAsset("ETH.USDT-0XA3910454BF2CB59B8B3A401589A3BACC5CA42306")
	c.Assert(err, IsNil)

	txOutStorage := newTxOutStorageV83(helper.keeper, helper.constAccessor, NewDummyEventMgr(), newGasMgrV81(helper.constAccessor, helper.keeper))
	pubKey := GetRandomPubKey()
	toAddr, err := pubKey.GetAddress(common.ETHChain)
	c.Assert(err, IsNil)
	toi := TxOutItem{
		Chain:       common.ETHChain,
		ToAddress:   toAddr,
		VaultPubKey: helper.yggVault.PubKey,
		Coin:        common.NewCoin(usdtAsset, cosmos.NewUint(2*common.One)),
		Memo:        NewOutboundMemo(helper.inboundTx.Tx.ID).String(),
		InHash:      helper.inboundTx.Tx.ID,
	}
	c.Assert(txOutStorage.UnSafeAddTxOutItem(helper.ctx, helper.mgr, toi), IsNil)
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.ETHChain,
		Coins: common.Coins{
			common.NewCoin(usdtAsset, cosmos.NewUint(2*common.One)),
		},
		Memo:        NewOutboundMemo(helper.inboundTx.Tx.ID).String(),
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Gas: common.Gas{
			common.NewCoin(common.ETHAsset, cosmos.NewUint(1*common.One)),
		},
	}, helper.ctx.BlockHeight(), helper.nodeAccount.PubKeySet.Secp256k1, helper.ctx.BlockHeight())
	expectedBond := cosmos.NewUint(9848484849)

	outMsg := NewMsgOutboundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
	_, err = handler.Run(helper.ctx, outMsg)
	c.Assert(err, IsNil)
	na, err := helper.keeper.GetNodeAccount(helper.ctx, helper.nodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(na.Bond.Equal(expectedBond), Equals, true, Commentf("%d/%d", na.Bond.Uint64(), expectedBond.Uint64()))
}

func (s *HandlerOutboundTxSuite) TestOutboundTxHandlerETHChainSpendTooMuchGasPerTHORNodeInstructionShouldNotSlash(c *C) {
	helper := newOutboundTxHandlerTestHelper(c)
	handler := NewOutboundTxHandler(helper.mgr)
	pool := NewPool()
	pool.Asset = common.ETHAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.LPUnits = pool.BalanceRune
	c.Assert(helper.keeper.SetPool(helper.ctx, pool), IsNil)
	fromAddr, err := helper.yggVault.PubKey.GetAddress(common.ETHChain)
	c.Assert(err, IsNil)
	usdtAsset, err := common.NewAsset("ETH.USDT-0XA3910454BF2CB59B8B3A401589A3BACC5CA42306")
	c.Assert(err, IsNil)

	txOutStorage := newTxOutStorageV83(helper.keeper, helper.constAccessor, NewDummyEventMgr(), newGasMgrV81(helper.constAccessor, helper.keeper))
	pubKey := GetRandomPubKey()
	toAddr, err := pubKey.GetAddress(common.ETHChain)
	c.Assert(err, IsNil)
	toi := TxOutItem{
		Chain:       common.ETHChain,
		ToAddress:   toAddr,
		VaultPubKey: helper.yggVault.PubKey,
		Coin:        common.NewCoin(usdtAsset, cosmos.NewUint(2*common.One)),
		Memo:        NewOutboundMemo(helper.inboundTx.Tx.ID).String(),
		InHash:      helper.inboundTx.Tx.ID,
		MaxGas: common.Gas{
			common.NewCoin(common.ETHAsset, cosmos.NewUint(1*common.One)),
		},
	}
	c.Assert(txOutStorage.UnSafeAddTxOutItem(helper.ctx, helper.mgr, toi), IsNil)
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.ETHChain,
		Coins: common.Coins{
			common.NewCoin(usdtAsset, cosmos.NewUint(2*common.One)),
		},
		Memo:        NewOutboundMemo(helper.inboundTx.Tx.ID).String(),
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Gas: common.Gas{
			common.NewCoin(common.ETHAsset, cosmos.NewUint(1*common.One)),
		},
	}, helper.ctx.BlockHeight(), helper.nodeAccount.PubKeySet.Secp256k1, helper.ctx.BlockHeight())
	expectedBond := cosmos.NewUint(10000000000)

	outMsg := NewMsgOutboundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
	_, err = handler.Run(helper.ctx, outMsg)
	c.Assert(err, IsNil)
	na, err := helper.keeper.GetNodeAccount(helper.ctx, helper.nodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(na.Bond.Equal(expectedBond), Equals, true, Commentf("%d/%d", na.Bond.Uint64(), expectedBond.Uint64()))
}

func (s *HandlerOutboundTxSuite) TestOutboundTxHandlerMismatchDecimalShouldNotSlash(c *C) {
	usdtAsset, err := common.NewAsset("ETH.USDT-0XA3910454BF2CB59B8B3A401589A3BACC5CA42306")
	c.Assert(err, IsNil)

	helper := newOutboundTxHandlerTestHelper(c)
	handler := NewOutboundTxHandler(helper.mgr)
	pool := NewPool()
	pool.Asset = usdtAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.Decimals = 6
	pool.LPUnits = pool.BalanceRune
	c.Assert(helper.keeper.SetPool(helper.ctx, pool), IsNil)
	fromAddr, err := helper.yggVault.PubKey.GetAddress(common.ETHChain)
	c.Assert(err, IsNil)

	txOutStorage := newTxOutStorageV85(helper.keeper, helper.constAccessor, NewDummyEventMgr(), newGasMgrV81(helper.constAccessor, helper.keeper))
	pubKey := GetRandomPubKey()
	toAddr, err := pubKey.GetAddress(common.ETHChain)
	c.Assert(err, IsNil)
	toi := TxOutItem{
		Chain:       common.ETHChain,
		ToAddress:   toAddr,
		VaultPubKey: helper.yggVault.PubKey,
		Coin:        common.NewCoin(usdtAsset, cosmos.NewUint(418847787978)),
		Memo:        NewOutboundMemo(helper.inboundTx.Tx.ID).String(),
		InHash:      helper.inboundTx.Tx.ID,
	}
	c.Assert(txOutStorage.UnSafeAddTxOutItem(helper.ctx, helper.mgr, toi), IsNil)
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.ETHChain,
		Coins: common.Coins{
			common.NewCoin(usdtAsset, cosmos.NewUint(418847787900)),
		},
		Memo:        NewOutboundMemo(helper.inboundTx.Tx.ID).String(),
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Gas: common.Gas{
			common.NewCoin(common.ETHAsset, cosmos.NewUint(1*common.One/100)),
		},
	}, helper.ctx.BlockHeight(), helper.nodeAccount.PubKeySet.Secp256k1, helper.ctx.BlockHeight())
	// no bond should be slashed
	expectedBond := cosmos.NewUint(10000000000)

	outMsg := NewMsgOutboundTx(tx, helper.inboundTx.Tx.ID, helper.nodeAccount.NodeAddress)
	_, err = handler.Run(helper.ctx, outMsg)
	c.Assert(err, IsNil)
	na, err := helper.keeper.GetNodeAccount(helper.ctx, helper.nodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(na.Bond.Equal(expectedBond), Equals, true, Commentf("%d/%d", na.Bond.Uint64(), expectedBond.Uint64()))
}
