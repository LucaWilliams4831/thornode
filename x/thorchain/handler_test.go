package thorchain

import (
	"errors"
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/store"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	dbm "github.com/tendermint/tm-db"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"

	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	kv1 "gitlab.com/thorchain/thornode/x/thorchain/keeper/v1"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

var errKaboom = errors.New("kaboom")

type HandlerSuite struct{}

var _ = Suite(&HandlerSuite{})

func (s *HandlerSuite) SetUpSuite(*C) {
	SetupConfigForTest()
}

func FundModule(c *C, ctx cosmos.Context, k keeper.Keeper, name string, amt uint64) {
	coin := common.NewCoin(common.RuneNative, cosmos.NewUint(amt*common.One))
	err := k.MintToModule(ctx, ModuleName, coin)
	c.Assert(err, IsNil)
	err = k.SendFromModuleToModule(ctx, ModuleName, name, common.NewCoins(coin))
	c.Assert(err, IsNil)
}

func FundAccount(c *C, ctx cosmos.Context, k keeper.Keeper, addr cosmos.AccAddress, amt uint64) {
	coin := common.NewCoin(common.RuneNative, cosmos.NewUint(amt*common.One))
	err := k.MintToModule(ctx, ModuleName, coin)
	c.Assert(err, IsNil)
	err = k.SendFromModuleToAccount(ctx, ModuleName, addr, common.NewCoins(coin))
	c.Assert(err, IsNil)
}

// nolint: deadcode unused
// create a codec used only for testing
func makeTestCodec() *codec.LegacyAmino {
	return types.MakeTestCodec()
}

var keyThorchain = cosmos.NewKVStoreKey(StoreKey)

func setupManagerForTest(c *C) (cosmos.Context, *Mgrs) {
	SetupConfigForTest()
	keyAcc := cosmos.NewKVStoreKey(authtypes.StoreKey)
	keyBank := cosmos.NewKVStoreKey(banktypes.StoreKey)
	keyParams := cosmos.NewKVStoreKey(paramstypes.StoreKey)
	tkeyParams := cosmos.NewTransientStoreKey(paramstypes.TStoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyAcc, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyThorchain, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyBank, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, cosmos.StoreTypeTransient, db)
	err := ms.LoadLatestVersion()
	c.Assert(err, IsNil)

	ctx := cosmos.NewContext(ms, tmproto.Header{ChainID: "thorchain"}, false, log.NewNopLogger())
	ctx = ctx.WithBlockHeight(18)
	legacyCodec := makeTestCodec()
	marshaler := simapp.MakeTestEncodingConfig().Marshaler

	pk := paramskeeper.NewKeeper(marshaler, legacyCodec, keyParams, tkeyParams)
	ak := authkeeper.NewAccountKeeper(marshaler, keyAcc, pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{
		ModuleName:  {authtypes.Minter, authtypes.Burner},
		AsgardName:  {},
		BondName:    {},
		ReserveName: {},
		LendingName: {},
	})

	bk := bankkeeper.NewBaseKeeper(marshaler, keyBank, ak, pk.Subspace(banktypes.ModuleName), nil)
	c.Assert(bk.MintCoins(ctx, ModuleName, cosmos.Coins{
		cosmos.NewCoin(common.RuneAsset().Native(), cosmos.NewInt(200_000_000_00000000)),
	}), IsNil)
	k := keeper.NewKeeper(marshaler, bk, ak, keyThorchain)
	FundModule(c, ctx, k, ModuleName, 10000*common.One)
	FundModule(c, ctx, k, AsgardName, common.One)
	FundModule(c, ctx, k, ReserveName, 10000*common.One)
	c.Assert(k.SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
		Chain:              common.BNBChain,
		TransactionSize:    1,
		TransactionFeeRate: 37500,
	}), IsNil)

	c.Assert(k.SaveNetworkFee(ctx, common.TERRAChain, NetworkFee{
		Chain:              common.TERRAChain,
		TransactionSize:    1,
		TransactionFeeRate: 6423600,
	}), IsNil)
	os.Setenv("NET", "mocknet")
	mgr := NewManagers(k, marshaler, bk, ak, keyThorchain)
	constants.SWVersion = GetCurrentVersion()

	_, hasVerStored := k.GetVersionWithCtx(ctx)
	c.Assert(hasVerStored, Equals, false,
		Commentf("version should not be stored until BeginBlock"))

	c.Assert(mgr.BeginBlock(ctx), IsNil)
	mgr.gasMgr.BeginBlock(mgr)

	verStored, hasVerStored := k.GetVersionWithCtx(ctx)
	c.Assert(hasVerStored, Equals, true,
		Commentf("version should be stored"))
	verComputed := k.GetLowestActiveVersion(ctx)
	c.Assert(verStored.String(), Equals, verComputed.String(),
		Commentf("stored version should match computed version"))

	return ctx, mgr
}

func setupKeeperForTest(c *C) (cosmos.Context, keeper.Keeper) {
	SetupConfigForTest()
	keyAcc := cosmos.NewKVStoreKey(authtypes.StoreKey)
	keyBank := cosmos.NewKVStoreKey(banktypes.StoreKey)
	keyParams := cosmos.NewKVStoreKey(paramstypes.StoreKey)
	tkeyParams := cosmos.NewTransientStoreKey(paramstypes.TStoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyAcc, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyThorchain, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyBank, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, cosmos.StoreTypeTransient, db)
	err := ms.LoadLatestVersion()
	c.Assert(err, IsNil)

	ctx := cosmos.NewContext(ms, tmproto.Header{ChainID: "thorchain"}, false, log.NewNopLogger())
	ctx = ctx.WithBlockHeight(18)
	legacyCodec := makeTestCodec()
	marshaler := simapp.MakeTestEncodingConfig().Marshaler

	pk := paramskeeper.NewKeeper(marshaler, legacyCodec, keyParams, tkeyParams)
	ak := authkeeper.NewAccountKeeper(marshaler, keyAcc, pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{
		ModuleName:  {authtypes.Minter, authtypes.Burner},
		AsgardName:  {},
		BondName:    {},
		ReserveName: {},
		LendingName: {},
	})

	bk := bankkeeper.NewBaseKeeper(marshaler, keyBank, ak, pk.Subspace(banktypes.ModuleName), nil)
	c.Assert(bk.MintCoins(ctx, ModuleName, cosmos.Coins{
		cosmos.NewCoin(common.RuneAsset().Native(), cosmos.NewInt(200_000_000_00000000)),
	}), IsNil)
	k := kv1.NewKVStore(marshaler, bk, ak, keyThorchain, GetCurrentVersion())
	FundModule(c, ctx, k, ModuleName, 1000000*common.One)
	FundModule(c, ctx, k, AsgardName, common.One)
	FundModule(c, ctx, k, ReserveName, 10000*common.One)
	err = k.SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
		Chain:              common.BNBChain,
		TransactionSize:    1,
		TransactionFeeRate: 37500,
	})
	c.Assert(err, IsNil)
	err = k.SaveNetworkFee(ctx, common.TERRAChain, NetworkFee{
		Chain:              common.TERRAChain,
		TransactionSize:    1,
		TransactionFeeRate: 6423600,
	})
	c.Assert(err, IsNil)
	os.Setenv("NET", "mocknet")
	return ctx, k
}

type handlerTestWrapper struct {
	ctx                  cosmos.Context
	keeper               keeper.Keeper
	mgr                  Manager
	activeNodeAccount    NodeAccount
	notActiveNodeAccount NodeAccount
}

func getHandlerTestWrapper(c *C, height int64, withActiveNode, withActieBNBPool bool) handlerTestWrapper {
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(height)
	acc1 := GetRandomValidatorNode(NodeActive)
	acc1.Version = mgr.GetVersion().String()
	if withActiveNode {
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, acc1), IsNil)
	}
	if withActieBNBPool {
		p, err := mgr.Keeper().GetPool(ctx, common.BNBAsset)
		c.Assert(err, IsNil)
		p.Asset = common.BNBAsset
		p.Status = PoolAvailable
		p.BalanceRune = cosmos.NewUint(100 * common.One)
		p.BalanceAsset = cosmos.NewUint(100 * common.One)
		p.LPUnits = cosmos.NewUint(100 * common.One)
		c.Assert(mgr.Keeper().SetPool(ctx, p), IsNil)
	}

	FundModule(c, ctx, mgr.Keeper(), AsgardName, 100000000)

	c.Assert(mgr.ValidatorMgr().BeginBlock(ctx, mgr, nil), IsNil)

	return handlerTestWrapper{
		ctx:                  ctx,
		keeper:               mgr.Keeper(),
		mgr:                  mgr,
		activeNodeAccount:    acc1,
		notActiveNodeAccount: GetRandomValidatorNode(NodeDisabled),
	}
}

func (HandlerSuite) TestHandleTxInWithdrawLiquidityMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)

	vault := GetRandomVault()
	vault.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One)),
	}
	c.Assert(w.keeper.SetVault(w.ctx, vault), IsNil)
	vaultAddr, err := vault.PubKey.GetAddress(common.BNBChain)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.LPUnits = cosmos.NewUint(100)
	c.Assert(w.keeper.SetPool(w.ctx, pool), IsNil)

	runeAddr := GetRandomRUNEAddress()
	lp := LiquidityProvider{
		Asset:        common.BNBAsset,
		RuneAddress:  runeAddr,
		AssetAddress: GetRandomBNBAddress(),
		PendingRune:  cosmos.ZeroUint(),
		Units:        cosmos.NewUint(100),
	}
	w.keeper.SetLiquidityProvider(w.ctx, lp)

	tx := common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(1*common.One)),
		},
		Memo:        "withdraw:BNB.BNB",
		FromAddress: lp.RuneAddress,
		ToAddress:   vaultAddr,
		Gas:         BNBGasFeeSingleton,
	}

	msg := NewMsgWithdrawLiquidity(tx, lp.RuneAddress, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)

	handler := NewInternalHandler(w.mgr)

	FundModule(c, w.ctx, w.keeper, AsgardName, 500)
	c.Assert(w.keeper.SaveNetworkFee(w.ctx, common.BNBChain, NetworkFee{
		Chain:              common.BNBChain,
		TransactionSize:    1,
		TransactionFeeRate: bnbSingleTxFee.Uint64(),
	}), IsNil)

	_, err = handler(w.ctx, msg)
	c.Assert(err, IsNil)
	pool, err = w.keeper.GetPool(w.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.IsEmpty(), Equals, false)
	c.Check(pool.Status, Equals, PoolStaged)
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(0), Commentf("%d", pool.LPUnits.Uint64()))
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(0), Commentf("%d", pool.BalanceRune.Uint64()))
	remainGas := uint64(37500)
	c.Check(pool.BalanceAsset.Uint64(), Equals, remainGas, Commentf("%d", pool.BalanceAsset.Uint64())) // leave a little behind for gas
}

func (HandlerSuite) TestRefund(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)

	pool := Pool{
		Asset:        common.BNBAsset,
		BalanceRune:  cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
	}
	c.Assert(w.keeper.SetPool(w.ctx, pool), IsNil)

	vault := GetRandomVault()
	c.Assert(w.keeper.SetVault(w.ctx, vault), IsNil)

	txin := NewObservedTx(
		common.Tx{
			ID:    GetRandomTxHash(),
			Chain: common.BNBChain,
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
			},
			Memo:        "withdraw:BNB.BNB",
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		vault.PubKey, 1024,
	)
	txOutStore := w.mgr.TxOutStore()
	c.Assert(refundTx(w.ctx, txin, w.mgr, 0, "refund", ""), IsNil)
	items, err := txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)

	// check THORNode DONT create a refund transaction when THORNode don't have a pool for
	// the asset sent.
	lokiAsset, _ := common.NewAsset("BNB.LOKI")
	txin.Tx.Coins = common.Coins{
		common.NewCoin(lokiAsset, cosmos.NewUint(100*common.One)),
	}

	c.Assert(refundTx(w.ctx, txin, w.mgr, 0, "refund", ""), IsNil)
	items, err = txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)

	pool, err = w.keeper.GetPool(w.ctx, lokiAsset)
	c.Assert(err, IsNil)
	// pool should be zero since we drop coins we don't recognize on the floor
	c.Assert(pool.BalanceAsset.Equal(cosmos.ZeroUint()), Equals, true, Commentf("%d", pool.BalanceAsset.Uint64()))

	// doing it a second time should keep it at zero
	c.Assert(refundTx(w.ctx, txin, w.mgr, 0, "refund", ""), IsNil)
	items, err = txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
	pool, err = w.keeper.GetPool(w.ctx, lokiAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.BalanceAsset.Equal(cosmos.ZeroUint()), Equals, true)
}

func (HandlerSuite) TestGetMsgSwapFromMemo(c *C) {
	m, err := ParseMemo(GetCurrentVersion(), "swap:BNB.BNB")
	swapMemo, ok := m.(SwapMemo)
	c.Assert(ok, Equals, true)
	c.Assert(err, IsNil)

	txin := types.NewObservedTx(
		common.Tx{
			ID:    GetRandomTxHash(),
			Chain: common.BNBChain,
			Coins: common.Coins{
				common.NewCoin(
					common.RuneAsset(),
					cosmos.NewUint(100*common.One),
				),
			},
			Memo:        m.String(),
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		common.EmptyPubKey, 1024,
	)

	resultMsg1, err := getMsgSwapFromMemo(swapMemo, txin, GetRandomBech32Addr())
	c.Assert(resultMsg1, NotNil)
	c.Assert(err, IsNil)
}

func (HandlerSuite) TestGetMsgWithdrawFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	tx := GetRandomTx()
	tx.Memo = "withdraw:10000"
	if common.RuneAsset().Equals(common.RuneNative) {
		tx.FromAddress = GetRandomTHORAddress()
	}
	obTx := NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	_, isWithdraw := msg.(*MsgWithdrawLiquidity)
	c.Assert(isWithdraw, Equals, true)
}

func (HandlerSuite) TestGetMsgMigrationFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	tx := GetRandomTx()
	tx.Memo = "migrate:10"
	obTx := NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	_, isMigrate := msg.(*MsgMigrate)
	c.Assert(isMigrate, Equals, true)
}

func (HandlerSuite) TestGetMsgBondFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One)),
	}
	tx.Memo = "bond:" + GetRandomBech32Addr().String()
	obTx := NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	_, isBond := msg.(*MsgBond)
	c.Assert(isBond, Equals, true)
}

func (HandlerSuite) TestGetMsgUnBondFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One)),
	}
	tx.Memo = "unbond:" + GetRandomTHORAddress().String() + ":1000"
	obTx := NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	_, isUnBond := msg.(*MsgUnBond)
	c.Assert(isUnBond, Equals, true)
}

func (HandlerSuite) TestGetMsgLiquidityFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	// provide BNB, however THORNode send T-CAN as coin , which is incorrect, should result in an error
	m, err := ParseMemo(GetCurrentVersion(), fmt.Sprintf("add:BNB.BNB:%s", GetRandomRUNEAddress()))
	c.Assert(err, IsNil)
	lpMemo, ok := m.(AddLiquidityMemo)
	c.Assert(ok, Equals, true)
	tcanAsset, err := common.NewAsset("BNB.TCAN-014")
	c.Assert(err, IsNil)
	runeAsset := common.RuneAsset()
	c.Assert(err, IsNil)

	txin := types.NewObservedTx(
		common.Tx{
			ID:    GetRandomTxHash(),
			Chain: common.BNBChain,
			Coins: common.Coins{
				common.NewCoin(tcanAsset,
					cosmos.NewUint(100*common.One)),
				common.NewCoin(runeAsset,
					cosmos.NewUint(100*common.One)),
			},
			Memo:        m.String(),
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		common.EmptyPubKey, 1024,
	)

	msg, err := getMsgAddLiquidityFromMemo(w.ctx, lpMemo, txin, GetRandomBech32Addr())
	c.Assert(msg, NotNil)
	c.Assert(err, IsNil)

	// Asymentic liquidity provision should works fine, only RUNE
	txin.Tx.Coins = common.Coins{
		common.NewCoin(runeAsset,
			cosmos.NewUint(100*common.One)),
	}

	// provide only rune should be fine
	msg1, err1 := getMsgAddLiquidityFromMemo(w.ctx, lpMemo, txin, GetRandomBech32Addr())
	c.Assert(msg1, NotNil)
	c.Assert(err1, IsNil)

	bnbAsset, err := common.NewAsset("BNB.BNB")
	c.Assert(err, IsNil)
	txin.Tx.Coins = common.Coins{
		common.NewCoin(bnbAsset,
			cosmos.NewUint(100*common.One)),
	}

	// provide only token(BNB) should be fine
	msg2, err2 := getMsgAddLiquidityFromMemo(w.ctx, lpMemo, txin, GetRandomBech32Addr())
	c.Assert(msg2, NotNil)
	c.Assert(err2, IsNil)

	lokiAsset, _ := common.NewAsset("BNB.LOKI")
	// Make sure the RUNE Address and Asset Address set correctly
	txin.Tx.Coins = common.Coins{
		common.NewCoin(runeAsset,
			cosmos.NewUint(100*common.One)),
		common.NewCoin(lokiAsset,
			cosmos.NewUint(100*common.One)),
	}

	runeAddr := GetRandomRUNEAddress()
	lokiAddLiquidityMemo, err := ParseMemo(GetCurrentVersion(), fmt.Sprintf("add:BNB.LOKI:%s", runeAddr))
	c.Assert(err, IsNil)
	msg4, err4 := getMsgAddLiquidityFromMemo(w.ctx, lokiAddLiquidityMemo.(AddLiquidityMemo), txin, GetRandomBech32Addr())
	c.Assert(err4, IsNil)
	c.Assert(msg4, NotNil)
	msgAddLiquidity, ok := msg4.(*MsgAddLiquidity)
	c.Assert(ok, Equals, true)
	c.Assert(msgAddLiquidity, NotNil)
	c.Assert(msgAddLiquidity.RuneAddress, Equals, runeAddr)
	c.Assert(msgAddLiquidity.AssetAddress, Equals, txin.Tx.FromAddress)
}

func (HandlerSuite) TestMsgLeaveFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	addr := types.GetRandomBech32Addr()
	txin := types.NewObservedTx(
		common.Tx{
			ID:          GetRandomTxHash(),
			Chain:       common.BNBChain,
			Coins:       common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(1))},
			Memo:        fmt.Sprintf("LEAVE:%s", addr.String()),
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		common.EmptyPubKey, 1024,
	)

	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)
}

func (HandlerSuite) TestYggdrasilMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	addr := types.GetRandomBech32Addr()
	txin := types.NewObservedTx(
		common.Tx{
			ID:          GetRandomTxHash(),
			Chain:       common.BNBChain,
			Coins:       common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(1))},
			Memo:        "yggdrasil+:1024",
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		GetRandomPubKey(), 1024,
	)

	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)

	txin.Tx.Memo = "yggdrasil-:1024"
	msg, err = processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)
}

func (s *HandlerSuite) TestReserveContributor(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	addr := types.GetRandomBech32Addr()
	txin := types.NewObservedTx(
		common.Tx{
			ID:          GetRandomTxHash(),
			Chain:       common.BNBChain,
			Coins:       common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(1))},
			Memo:        "reserve",
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		GetRandomPubKey(), 1024,
	)

	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)
	_, isReserve := msg.(*MsgReserveContributor)
	c.Assert(isReserve, Equals, true)
}

func (s *HandlerSuite) TestSwitch(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	addr := types.GetRandomBech32Addr()
	txin := types.NewObservedTx(
		common.Tx{
			ID:          GetRandomTxHash(),
			Chain:       common.BNBChain,
			Coins:       common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(1))},
			Memo:        "switch:" + GetRandomBech32Addr().String(),
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		GetRandomPubKey(), 1024,
	)

	msg, err := processOneTxIn(w.ctx, GetCurrentVersion(), w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)
	_, isSwitch := msg.(*MsgSwitch)
	c.Assert(isSwitch, Equals, true)
}

func (s *HandlerSuite) TestExternalHandler(c *C) {
	ctx, mgr := setupManagerForTest(c)
	handler := NewExternalHandler(mgr)
	ctx = ctx.WithBlockHeight(1024)
	msg := NewMsgNetworkFee(1024, common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
	result, err := handler(ctx, msg)
	c.Check(err, NotNil)
	c.Check(errors.Is(err, se.ErrUnauthorized), Equals, true)
	c.Check(result, IsNil)
	na := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, 10*common.One)
	result, err = handler(ctx, NewMsgSetVersion("0.1.0", na.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
}

func (s *HandlerSuite) TestFuzzyMatching(c *C) {
	ctx, mgr := setupManagerForTest(c)
	k := mgr.Keeper()
	p1 := NewPool()
	p1.Asset = common.BNBAsset
	p1.BalanceRune = cosmos.NewUint(10 * common.One)
	c.Assert(k.SetPool(ctx, p1), IsNil)

	// real USDT
	p2 := NewPool()
	p2.Asset, _ = common.NewAsset("ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7")
	p2.BalanceRune = cosmos.NewUint(80 * common.One)
	c.Assert(k.SetPool(ctx, p2), IsNil)

	// fake USDT, attempt to clone end of contract address
	p3 := NewPool()
	p3.Asset, _ = common.NewAsset("ETH.USDT-0XD084B83C305DAFD76AE3E1B4E1F1FE213D831EC7")
	p3.BalanceRune = cosmos.NewUint(20 * common.One)
	c.Assert(k.SetPool(ctx, p3), IsNil)

	// fake USDT, bad contract address
	p4 := NewPool()
	p4.Asset, _ = common.NewAsset("ETH.USDT-0XD084B83C305DAFD76AE3E1B4E1F1FE2ECCCB3988")
	p4.BalanceRune = cosmos.NewUint(20 * common.One)
	c.Assert(k.SetPool(ctx, p4), IsNil)

	// fake USDT, on different chain
	p5 := NewPool()
	p5.Asset, _ = common.NewAsset("BSC.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7")
	p5.BalanceRune = cosmos.NewUint(30 * common.One)
	c.Assert(k.SetPool(ctx, p5), IsNil)

	// fake USDT, right contract address, wrong ticker
	p6 := NewPool()
	p6.Asset, _ = common.NewAsset("ETH.UST-0XDAC17F958D2EE523A2206206994597C13D831EC7")
	p6.BalanceRune = cosmos.NewUint(90 * common.One)
	c.Assert(k.SetPool(ctx, p6), IsNil)

	result := fuzzyAssetMatch(ctx, k, p1.Asset)
	c.Check(result.Equals(p1.Asset), Equals, true)
	result = fuzzyAssetMatch(ctx, k, p6.Asset)
	c.Check(result.Equals(p6.Asset), Equals, true)

	check, _ := common.NewAsset("ETH.USDT")
	result = fuzzyAssetMatch(ctx, k, check)
	c.Check(result.Equals(p2.Asset), Equals, true)
	check, _ = common.NewAsset("ETH.USDT-")
	result = fuzzyAssetMatch(ctx, k, check)
	c.Check(result.Equals(p2.Asset), Equals, true)
	check, _ = common.NewAsset("ETH.USDT-1EC7")
	result = fuzzyAssetMatch(ctx, k, check)
	c.Check(result.Equals(p2.Asset), Equals, true)

	check, _ = common.NewAsset("ETH/USDT-1EC7")
	result = fuzzyAssetMatch(ctx, k, check)
	c.Check(result.Synth, Equals, true)
	c.Check(result.Equals(p2.Asset.GetSyntheticAsset()), Equals, true)
}

func (s *HandlerSuite) TestMemoFetchAddress(c *C) {
	ctx, k := setupKeeperForTest(c)

	thorAddr := GetRandomTHORAddress()
	name := NewTHORName("hello", 50, []THORNameAlias{{Chain: common.THORChain, Address: thorAddr}})
	k.SetTHORName(ctx, name)

	bnbAddr := GetRandomBNBAddress()
	addr, err := FetchAddress(ctx, k, bnbAddr.String(), common.BNBChain)
	c.Assert(err, IsNil)
	c.Check(addr.Equals(bnbAddr), Equals, true)

	addr, err = FetchAddress(ctx, k, "hello", common.THORChain)
	c.Assert(err, IsNil)
	c.Check(addr.Equals(thorAddr), Equals, true)

	addr, err = FetchAddress(ctx, k, "hello.thor", common.THORChain)
	c.Assert(err, IsNil)
	c.Check(addr.Equals(thorAddr), Equals, true)
}

func (s *HandlerSuite) TestExternalAssetMatch(c *C) {
	v := GetCurrentVersion()

	c.Check(externalAssetMatch(v, common.ETHChain, "7a0"), Equals, "0xd601c6A3a36721320573885A8d8420746dA3d7A0")
	c.Check(externalAssetMatch(v, common.ETHChain, "foobar"), Equals, "foobar")
	c.Check(externalAssetMatch(v, common.ETHChain, "3"), Equals, "3")
	c.Check(externalAssetMatch(v, common.ETHChain, ""), Equals, "")
	c.Check(externalAssetMatch(v, common.BTCChain, "foo"), Equals, "foo")
}
