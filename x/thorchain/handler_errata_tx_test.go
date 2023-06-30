package thorchain

import (
	"fmt"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

var _ = Suite(&HandlerErrataTxSuite{})

type HandlerErrataTxSuite struct{}

type TestErrataTxKeeper struct {
	keeper.KVStoreDummy
	observedTx ObservedTxVoter
	pool       Pool
	na         NodeAccount
	lps        LiquidityProviders
	err        error
}

func (k *TestErrataTxKeeper) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{k.na}, k.err
}

func (k *TestErrataTxKeeper) GetNodeAccount(_ cosmos.Context, _ cosmos.AccAddress) (NodeAccount, error) {
	return k.na, k.err
}

func (k *TestErrataTxKeeper) GetObservedTxInVoter(_ cosmos.Context, txID common.TxID) (ObservedTxVoter, error) {
	return k.observedTx, k.err
}

func (k *TestErrataTxKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	return k.pool, k.err
}

func (k *TestErrataTxKeeper) SetPool(_ cosmos.Context, pool Pool) error {
	k.pool = pool
	return k.err
}

func (k *TestErrataTxKeeper) GetLiquidityProvider(_ cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	for _, lp := range k.lps {
		if lp.RuneAddress.Equals(addr) {
			return lp, k.err
		}
	}
	return LiquidityProvider{}, k.err
}

func (k *TestErrataTxKeeper) SetLiquidityProvider(_ cosmos.Context, lp LiquidityProvider) {
	for i, skr := range k.lps {
		if skr.RuneAddress.Equals(lp.RuneAddress) {
			k.lps[i] = lp
		}
	}
}

func (k *TestErrataTxKeeper) GetErrataTxVoter(_ cosmos.Context, txID common.TxID, chain common.Chain) (ErrataTxVoter, error) {
	return NewErrataTxVoter(txID, chain), k.err
}

func (s *HandlerErrataTxSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestErrataTxKeeper{
		na: GetRandomValidatorNode(NodeActive),
	}

	handler := NewErrataTxHandler(NewDummyMgrWithKeeper(keeper))
	// happy path
	msg := NewMsgErrataTx(GetRandomTxHash(), common.BNBChain, keeper.na.NodeAddress)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// invalid msg
	msg = &MsgErrataTx{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

func (s *HandlerErrataTxSuite) TestErrataHandlerHappyPath(c *C) {
	ctx, mgr := setupManagerForTest(c)

	txID := GetRandomTxHash()
	na := GetRandomValidatorNode(NodeActive)
	addr := GetRandomBNBAddress()
	totalUnits := cosmos.NewUint(1600)
	observedTx := ObservedTx{
		Tx: common.Tx{
			ID:          txID,
			Chain:       common.BNBChain,
			FromAddress: addr,
			Coins: common.Coins{
				common.NewCoin(common.RuneAsset(), cosmos.NewUint(30*common.One)),
			},
			Memo: fmt.Sprintf("ADD:BNB.BNB:%s", GetRandomRUNEAddress()),
		},
	}
	keeper := &TestErrataTxKeeper{
		na: na,
		observedTx: ObservedTxVoter{
			Tx:     observedTx,
			Txs:    ObservedTxs{observedTx},
			Height: 1024,
		},
		pool: Pool{
			Asset:        common.BNBAsset,
			LPUnits:      totalUnits,
			BalanceRune:  cosmos.NewUint(100 * common.One),
			BalanceAsset: cosmos.NewUint(100 * common.One),
		},
		lps: LiquidityProviders{
			{
				RuneAddress:   addr,
				LastAddHeight: 5,
				Units:         totalUnits.QuoUint64(2),
				PendingRune:   cosmos.ZeroUint(),
			},
			{
				RuneAddress:   GetRandomBNBAddress(),
				LastAddHeight: 10,
				Units:         totalUnits.QuoUint64(2),
				PendingRune:   cosmos.ZeroUint(),
			},
		},
	}
	mgr.K = keeper

	handler := NewErrataTxHandler(mgr)
	msg := NewMsgErrataTx(txID, common.BNBChain, na.NodeAddress)
	_, err := handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Check(keeper.pool.BalanceRune.Equal(cosmos.NewUint(70*common.One)), Equals, true)
	c.Check(keeper.pool.BalanceAsset.Equal(cosmos.NewUint(100*common.One)), Equals, true)
	c.Check(keeper.lps[0].Units.IsZero(), Equals, true, Commentf("%d", keeper.lps[0].Units.Uint64()))
	c.Check(keeper.lps[0].LastAddHeight, Equals, int64(18))
}

type ErrataTxHandlerTestHelper struct {
	keeper.Keeper
	failListActiveNodeAccount bool
	failGetErrataTxVoter      bool
	failGetObserveTxVoter     bool
	failGetPool               bool
	failGetLiquidityProvider  bool
	failSetPool               bool
}

func NewErrataTxHandlerTestHelper(k keeper.Keeper) *ErrataTxHandlerTestHelper {
	return &ErrataTxHandlerTestHelper{
		Keeper: k,
	}
}

func (k *ErrataTxHandlerTestHelper) ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error) {
	if k.failListActiveNodeAccount {
		return NodeAccounts{}, errKaboom
	}
	return k.Keeper.ListActiveValidators(ctx)
}

func (k *ErrataTxHandlerTestHelper) GetErrataTxVoter(ctx cosmos.Context, txID common.TxID, chain common.Chain) (ErrataTxVoter, error) {
	if k.failGetErrataTxVoter {
		return ErrataTxVoter{}, errKaboom
	}
	return k.Keeper.GetErrataTxVoter(ctx, txID, chain)
}

func (k *ErrataTxHandlerTestHelper) GetObservedTxInVoter(ctx cosmos.Context, txID common.TxID) (ObservedTxVoter, error) {
	if k.failGetObserveTxVoter {
		return ObservedTxVoter{}, errKaboom
	}
	return k.Keeper.GetObservedTxInVoter(ctx, txID)
}

func (k *ErrataTxHandlerTestHelper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if k.failGetPool {
		return NewPool(), errKaboom
	}
	return k.Keeper.GetPool(ctx, asset)
}

func (k *ErrataTxHandlerTestHelper) GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	if k.failGetLiquidityProvider {
		return LiquidityProvider{}, errKaboom
	}
	return k.Keeper.GetLiquidityProvider(ctx, asset, addr)
}

func (k *ErrataTxHandlerTestHelper) SetPool(ctx cosmos.Context, pool Pool) error {
	if k.failSetPool {
		return errKaboom
	}
	return k.Keeper.SetPool(ctx, pool)
}

func (s *HandlerErrataTxSuite) TestErrataHandlerDifferentError(c *C) {
	testCases := []struct {
		name            string
		messageProvider func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg
		validator       func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string)
	}{
		{
			name: "invalid message should return an error",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				return NewMsgNetworkFee(1024, common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
		},
		{
			name: "message fail validation should return an error",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				return NewMsgErrataTx(GetRandomTxHash(), common.BTCChain, GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to list active account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				// add an active node account
				nodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAccount), IsNil)
				helper.failListActiveNodeAccount = true
				return NewMsgErrataTx(GetRandomTxHash(), common.BTCChain, nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to get errata tx voter should return an error",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				// add an active node account
				nodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAccount), IsNil)
				helper.failGetErrataTxVoter = true
				return NewMsgErrataTx(GetRandomTxHash(), common.BTCChain, nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
		},
		{
			name: "if voter already sign the errata tx voter it should not do anything",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				// add an active node account
				nodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAccount), IsNil)
				txID := GetRandomTxHash()
				voter, _ := helper.Keeper.GetErrataTxVoter(ctx, txID, common.BTCChain)
				voter.Sign(nodeAccount.NodeAddress)
				helper.Keeper.SetErrataTxVoter(ctx, voter)
				return NewMsgErrataTx(txID, common.BTCChain, nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, NotNil, Commentf(name))
				c.Check(err, IsNil, Commentf(name))
			},
		},
		{
			name: "if voter doesn't have consensus it should not do anything",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				// add an active node account
				nodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAccount), IsNil)
				txID := GetRandomTxHash()
				nodeAcct1 := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAcct1), IsNil)
				return NewMsgErrataTx(txID, common.BTCChain, nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, NotNil, Commentf(name))
				c.Check(err, IsNil, Commentf(name))
			},
		},
		{
			name: "if voter had been processed it should not do anything",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				// add an active node account
				nodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAccount), IsNil)
				txID := GetRandomTxHash()
				voter, _ := helper.Keeper.GetErrataTxVoter(ctx, txID, common.BTCChain)
				voter.BlockHeight = ctx.BlockHeight()
				helper.Keeper.SetErrataTxVoter(ctx, voter)
				return NewMsgErrataTx(txID, common.BTCChain, nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, NotNil, Commentf(name))
				c.Check(err, IsNil, Commentf(name))
			},
		},
		{
			name: "if fail to get observed tx in it should return err",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				// add an active node account
				nodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAccount), IsNil)
				helper.failGetObserveTxVoter = true
				return NewMsgErrataTx(GetRandomTxHash(), common.BTCChain, nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
		},
		{
			name: "if observed tx is empty it should return err",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				// add an active node account
				nodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAccount), IsNil)
				return NewMsgErrataTx(GetRandomTxHash(), common.BTCChain, nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
		},
		{
			name: "if chain doesn't match it should not do anything",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				// add an active node account
				nodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAccount), IsNil)
				observedTx := GetRandomObservedTx()
				voter := ObservedTxVoter{
					TxID:   observedTx.Tx.ID,
					Tx:     observedTx,
					Txs:    ObservedTxs{observedTx},
					Height: observedTx.BlockHeight,
				}
				helper.Keeper.SetObservedTxInVoter(ctx, voter)
				return NewMsgErrataTx(voter.TxID, common.BTCChain, nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, NotNil, Commentf(name))
				c.Check(err, IsNil, Commentf(name))
			},
		},
		{
			name: "if the tx is not swap nor provide liquidity, it should not do anything",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				// add an active node account
				nodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAccount), IsNil)
				observedTx := GetRandomObservedTx()
				observedTx.Tx.Chain = common.BTCChain
				observedTx.Tx.Memo = "withdraw"
				voter := ObservedTxVoter{
					TxID:   observedTx.Tx.ID,
					Tx:     observedTx,
					Txs:    ObservedTxs{observedTx},
					Height: observedTx.BlockHeight,
				}
				helper.Keeper.SetObservedTxInVoter(ctx, voter)
				return NewMsgErrataTx(voter.TxID, common.BTCChain, nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, NotNil, Commentf(name))
				c.Check(err, IsNil, Commentf(name))
			},
		},
		{
			name: "if it fail to get pool it should return an error",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				// add an active node account
				nodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAccount), IsNil)
				observedTx := GetRandomObservedTx()
				observedTx.Tx.Chain = common.BTCChain
				observedTx.Tx.Memo = "swap:BNB"
				helper.failGetPool = true
				voter := ObservedTxVoter{
					TxID:   observedTx.Tx.ID,
					Tx:     observedTx,
					Txs:    ObservedTxs{observedTx},
					Height: observedTx.BlockHeight,
				}
				helper.Keeper.SetObservedTxInVoter(ctx, voter)
				return NewMsgErrataTx(voter.TxID, common.BTCChain, nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
		},
		{
			name: "if fail to get liquidity provider it should return an error",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				// add an active node account
				nodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAccount), IsNil)
				observedTx := GetRandomObservedTx()
				observedTx.Tx.Chain = common.BTCChain
				observedTx.Tx.Memo = "add:BTC:" + observedTx.Tx.FromAddress.String()
				lp := LiquidityProvider{
					Asset:         common.BTCAsset,
					AssetAddress:  GetRandomBNBAddress(),
					LastAddHeight: 1024,
					RuneAddress:   observedTx.Tx.FromAddress,
				}
				helper.SetLiquidityProvider(ctx, lp)
				helper.failGetLiquidityProvider = true
				pool := NewPool()
				pool.Asset = common.BTCAsset
				pool.BalanceRune = cosmos.NewUint(common.One * 100)
				pool.BalanceAsset = cosmos.NewUint(common.One * 100)
				pool.Status = PoolAvailable
				c.Assert(helper.Keeper.SetPool(ctx, pool), IsNil)
				voter := ObservedTxVoter{
					TxID:   observedTx.Tx.ID,
					Tx:     observedTx,
					Txs:    ObservedTxs{observedTx},
					Height: observedTx.BlockHeight,
				}
				helper.Keeper.SetObservedTxInVoter(ctx, voter)
				return NewMsgErrataTx(voter.TxID, common.BTCChain, nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, IsNil, Commentf(name))
				c.Check(err, NotNil, Commentf(name))
			},
		},
		{
			name: " fail to save pool should not error out",
			messageProvider: func(ctx cosmos.Context, helper *ErrataTxHandlerTestHelper) cosmos.Msg {
				// add an active node account
				nodeAccount := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, nodeAccount), IsNil)
				observedTx := GetRandomObservedTx()
				observedTx.Tx.Chain = common.BTCChain
				observedTx.Tx.Memo = "swap:BTC"
				helper.failSetPool = true
				pool := NewPool()
				pool.Asset = common.BTCAsset
				pool.BalanceRune = cosmos.NewUint(common.One * 100)
				pool.BalanceAsset = cosmos.NewUint(common.One * 100)
				pool.Status = PoolAvailable
				c.Assert(helper.Keeper.SetPool(ctx, pool), IsNil)
				voter := ObservedTxVoter{
					TxID:   observedTx.Tx.ID,
					Tx:     observedTx,
					Txs:    ObservedTxs{observedTx},
					Height: observedTx.BlockHeight,
				}
				helper.Keeper.SetObservedTxInVoter(ctx, voter)
				return NewMsgErrataTx(voter.TxID, common.BTCChain, nodeAccount.NodeAddress)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *ErrataTxHandlerTestHelper, name string) {
				c.Check(result, NotNil, Commentf(name))
				c.Check(err, IsNil, Commentf(name))
			},
		},
	}

	for _, tc := range testCases {
		ctx, mgr := setupManagerForTest(c)
		helper := NewErrataTxHandlerTestHelper(mgr.Keeper())
		mgr.K = helper
		msg := tc.messageProvider(ctx, helper)
		handler := NewErrataTxHandler(mgr)
		result, err := handler.Run(ctx, msg)
		tc.validator(c, ctx, result, err, helper, tc.name)
	}
}

func (*HandlerErrataTxSuite) TestProcessErrortaOutboundTx(c *C) {
	ctx, mgr := setupManagerForTest(c)
	helper := NewErrataTxHandlerTestHelper(mgr.Keeper())
	handler := NewErrataTxHandler(mgr)
	node1 := GetRandomValidatorNode(NodeActive)
	node2 := GetRandomValidatorNode(NodeActive)
	node3 := GetRandomValidatorNode(NodeActive)
	c.Assert(helper.Keeper.SetNodeAccount(ctx, node1), IsNil)
	c.Assert(helper.Keeper.SetNodeAccount(ctx, node2), IsNil)
	c.Assert(helper.Keeper.SetNodeAccount(ctx, node3), IsNil)

	// fail to get observed tx out voter
	txID := GetRandomTxHash()
	msg := NewMsgErrataTx(txID, common.LTCChain, node1.NodeAddress)
	result, err := handler.processErrataOutboundTx(ctx, *msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	observedPubKey := GetRandomPubKey()
	tx := common.NewTx(txID, GetRandomLTCAddress(), GetRandomLTCAddress(),
		common.Coins{
			common.NewCoin(common.LTCAsset, cosmos.NewUint(102400000)),
		},
		common.Gas{
			common.NewCoin(common.LTCAsset, cosmos.NewUint(1000)),
		}, "swap:LTC.LTC")
	observedTx := []ObservedTx{
		NewObservedTx(
			tx,
			1024, observedPubKey, 1024),
	}
	txOutVoter := NewObservedTxVoter(txID, observedTx)
	helper.Keeper.SetObservedTxOutVoter(ctx, txOutVoter)
	// Tx is empty , it should fail
	result, err = handler.processErrataOutboundTx(ctx, *msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
	txOutVoter.Add(observedTx[0], node2.NodeAddress)
	txOutVoter.Add(observedTx[0], node3.NodeAddress)
	tx1 := txOutVoter.GetTx(NodeAccounts{node1, node2, node3})
	c.Assert(tx1.IsEmpty(), Equals, false)
	helper.Keeper.SetObservedTxOutVoter(ctx, txOutVoter)

	// not outbound tx , it should fail
	result, err = handler.processErrataOutboundTx(ctx, *msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// fail to get vault
	txInID := GetRandomTxHash()
	txOutVoter.Tx.Tx.Memo = "OUT:" + txInID.String()
	helper.Keeper.SetObservedTxOutVoter(ctx, txOutVoter)
	result, err = handler.processErrataOutboundTx(ctx, *msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// Active Asgard vault, TxInVoter not exist
	asgardVault := NewVault(1, types.VaultStatus_ActiveVault, AsgardVault, observedPubKey, []string{
		common.LTCChain.String(),
		common.BTCChain.String(),
		common.BNBChain.String(),
	}, []ChainContract{})
	c.Assert(helper.Keeper.SetVault(ctx, asgardVault), IsNil)
	result, err = handler.processErrataOutboundTx(ctx, *msg)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// inactive vault , cause it to compensate asgard with reserve
	asgardVault.UpdateStatus(types.VaultStatus_InactiveVault, 1024)
	c.Assert(helper.Keeper.SetVault(ctx, asgardVault), IsNil)
	result, err = handler.processErrataOutboundTx(ctx, *msg)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	// vault should be set back to retiring
	v, err := helper.Keeper.GetVault(ctx, asgardVault.PubKey)
	c.Assert(err, IsNil)
	c.Assert(v.Status == types.VaultStatus_RetiringVault, Equals, true)

	// With POOL , but no txin voter
	pool := NewPool()
	pool.Asset = common.LTCAsset
	pool.BalanceAsset = cosmos.NewUint(1024 * common.One)
	pool.BalanceRune = cosmos.NewUint(1024 * common.One)
	pool.Status = PoolAvailable
	c.Assert(helper.Keeper.SetPool(ctx, pool), IsNil)
	result, err = handler.processErrataOutboundTx(ctx, *msg)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	// yggdrasil vault
	asgardVault.Type = YggdrasilVault

	c.Assert(helper.Keeper.SetVault(ctx, asgardVault), IsNil)
	result, err = handler.processErrataOutboundTx(ctx, *msg)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	txInbound := common.NewTx(txInID, GetRandomLTCAddress(), GetRandomLTCAddress(),
		common.Coins{
			common.NewCoin(common.LTCAsset, cosmos.NewUint(102400000)),
		},
		common.Gas{
			common.NewCoin(common.LTCAsset, cosmos.NewUint(1000)),
		}, "swap:LTC.LTC")
	observedTxInbound := []ObservedTx{
		NewObservedTx(
			txInbound,
			1024, observedPubKey, 1024),
	}
	txInVoter := NewObservedTxVoter(txInID, observedTxInbound)
	txInVoter.Actions = []TxOutItem{
		{
			Chain:     common.LTCChain,
			InHash:    txInID,
			ToAddress: GetRandomLTCAddress(),
			Coin:      common.NewCoin(common.LTCAsset, cosmos.NewUint(10240000)),
			Memo:      "OUT:" + txInID.String(),
			OutHash:   txID,
		},
	}
	helper.Keeper.SetObservedTxInVoter(ctx, txInVoter)
	newActiveAsgardVault := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), []string{
		common.BTCChain.String(),
		common.LTCChain.String(),
		common.ETHChain.String(),
	}, []ChainContract{})
	newActiveAsgardVault.AddFunds(common.Coins{
		common.NewCoin(common.LTCAsset, cosmos.NewUint(1024*common.One)),
	})
	c.Assert(helper.Keeper.SetVault(ctx, newActiveAsgardVault), IsNil)
	c.Assert(helper.Keeper.SaveNetworkFee(ctx, common.LTCChain, NetworkFee{
		Chain:              common.LTCChain,
		TransactionSize:    250,
		TransactionFeeRate: 10,
	}), IsNil)
	result, err = handler.processErrataOutboundTx(ctx, *msg)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	txOut, err := helper.Keeper.GetTxOut(ctx, ctx.BlockHeight())
	c.Assert(err, IsNil)
	c.Assert(txOut.TxArray, HasLen, 1)
}

func (*HandlerErrataTxSuite) TestProcessErrortaOutboundTx_EnsureMigrateTxWillSetInactiveVaultBackToRetiring(c *C) {
	ctx, mgr := setupManagerForTest(c)
	helper := NewErrataTxHandlerTestHelper(mgr.Keeper())
	mgr.K = helper
	handler := NewErrataTxHandler(mgr)
	node1 := GetRandomValidatorNode(NodeActive)
	node2 := GetRandomValidatorNode(NodeActive)
	node3 := GetRandomValidatorNode(NodeActive)
	c.Assert(helper.Keeper.SetNodeAccount(ctx, node1), IsNil)
	c.Assert(helper.Keeper.SetNodeAccount(ctx, node2), IsNil)
	c.Assert(helper.Keeper.SetNodeAccount(ctx, node3), IsNil)

	retiredPubKey := GetRandomPubKey()
	activePubKey := GetRandomPubKey()
	// inactive vault, TxInVoter not exist
	inactiveVault := NewVault(1, types.VaultStatus_InactiveVault, AsgardVault, retiredPubKey, []string{
		common.LTCChain.String(),
		common.BTCChain.String(),
		common.BNBChain.String(),
	}, []ChainContract{})

	activeVault := NewVault(1, types.VaultStatus_ActiveVault, AsgardVault, activePubKey, []string{
		common.LTCChain.String(),
		common.BTCChain.String(),
		common.BNBChain.String(),
	}, []ChainContract{})
	c.Assert(helper.Keeper.SetVault(ctx, inactiveVault), IsNil)
	c.Assert(helper.Keeper.SetVault(ctx, activeVault), IsNil)
	// migration , internal tx , should cause the vault to be set back to retiring

	txID1 := GetRandomTxHash()
	internalMigrationTx := NewMsgErrataTx(txID1, common.LTCChain, node1.NodeAddress)

	migrateTx := common.NewTx(txID1,
		GetRandomLTCAddress(),
		GetRandomLTCAddress(),
		common.Coins{
			common.NewCoin(common.LTCAsset, cosmos.NewUint(102400000)),
		},
		common.Gas{
			common.NewCoin(common.LTCAsset, cosmos.NewUint(1000)),
		}, "migrate:111")

	// observed outbound tx in the retired vault
	observedTx := []ObservedTx{
		NewObservedTx(
			migrateTx,
			1024, retiredPubKey, 1024),
	}
	txOutVoter := NewObservedTxVoter(txID1, observedTx)
	txOutVoter.Add(observedTx[0], node2.NodeAddress)
	txOutVoter.Add(observedTx[0], node3.NodeAddress)
	c.Assert(txOutVoter.GetTx(NodeAccounts{node1, node2, node3}), NotNil)
	helper.Keeper.SetObservedTxOutVoter(ctx, txOutVoter)

	// observed inbound tx in the active vault
	observedInboundTx := []ObservedTx{
		NewObservedTx(
			migrateTx,
			1024, retiredPubKey, 1024),
	}
	txInVoter := NewObservedTxVoter(txID1, observedTx)
	txInVoter.Add(observedInboundTx[0], node2.NodeAddress)
	txInVoter.Add(observedInboundTx[0], node3.NodeAddress)
	c.Assert(txInVoter.GetTx(NodeAccounts{node1, node2, node3}), NotNil)
	helper.Keeper.SetObservedTxInVoter(ctx, txInVoter)

	result, err := handler.processErrataOutboundTx(ctx, *internalMigrationTx)
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	v, err := helper.Keeper.GetVault(ctx, retiredPubKey)
	c.Assert(err, IsNil)
	c.Assert(v.Status == types.VaultStatus_RetiringVault, Equals, true)
	c.Assert(v.HasFunds(), Equals, true)
	ltcCoin := v.GetCoin(common.LTCAsset)
	c.Assert(ltcCoin.Equals(common.NewCoin(common.LTCAsset, cosmos.NewUint(102400000))), Equals, true)

	// reset the inactive vault
	c.Assert(helper.Keeper.SetVault(ctx, inactiveVault), IsNil)
	errataVoter := NewErrataTxVoter(txID1, common.LTCChain)
	errataVoter.Sign(node2.NodeAddress)
	errataVoter.Sign(node3.NodeAddress)
	helper.Keeper.SetErrataTxVoter(ctx, errataVoter)
	result, err = handler.handle(ctx, *internalMigrationTx)
	c.Assert(result, NotNil)
	c.Assert(err, IsNil)
	v, err = helper.Keeper.GetVault(ctx, retiredPubKey)
	c.Assert(err, IsNil)
	c.Assert(v.Status == types.VaultStatus_RetiringVault, Equals, true)
	c.Assert(v.HasFunds(), Equals, true)
	ltcCoin = v.GetCoin(common.LTCAsset)
	c.Assert(ltcCoin.Equals(common.NewCoin(common.LTCAsset, cosmos.NewUint(102400000))), Equals, true)
}
