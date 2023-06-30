package thorchain

import (
	"errors"
	"fmt"
	"strings"

	se "github.com/cosmos/cosmos-sdk/types/errors"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/types"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerSwapSuite struct{}

var _ = Suite(&HandlerSwapSuite{})

func (s *HandlerSwapSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestSwapHandleKeeper{
		activeNodeAccount: GetRandomValidatorNode(NodeActive),
	}

	handler := NewSwapHandler(NewDummyMgrWithKeeper(keeper))

	txID := GetRandomTxHash()
	signerBNBAddr := GetRandomBNBAddress()
	observerAddr := keeper.activeNodeAccount.NodeAddress
	tx := common.NewTx(
		txID,
		signerBNBAddr,
		signerBNBAddr,
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.OneUint()),
		},
		BNBGasFeeSingleton,
		"",
	)
	msg := NewMsgSwap(tx, common.BNBAsset, signerBNBAddr, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, MarketOrder, observerAddr)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// bad aggregator reference
	msg.Aggregator = "zzzzzz"
	c.Assert(handler.validate(ctx, *msg), NotNil)

	// invalid msg
	msg = &MsgSwap{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

type TestSwapHandleKeeper struct {
	keeper.KVStoreDummy
	pools             map[common.Asset]Pool
	activeNodeAccount NodeAccount
	synthSupply       cosmos.Uint
	haltChain         int64
	derivedAssets     bool
}

func (k *TestSwapHandleKeeper) GetConfigInt64(ctx cosmos.Context, key constants.ConstantName) int64 {
	val, _ := k.GetMimir(ctx, key.String())
	return val
}

func (k *TestSwapHandleKeeper) PoolExist(_ cosmos.Context, asset common.Asset) bool {
	asset = asset.GetLayer1Asset()
	_, ok := k.pools[asset]
	return ok
}

func (k *TestSwapHandleKeeper) GetPool(_ cosmos.Context, asset common.Asset) (Pool, error) {
	asset = asset.GetLayer1Asset()
	if pool, ok := k.pools[asset]; ok {
		return pool, nil
	}
	pool := NewPool()
	pool.Asset = asset
	return pool, nil
}

func (k *TestSwapHandleKeeper) GetPools(_ cosmos.Context) (pools Pools, err error) {
	for _, v := range k.pools {
		pools = append(pools, v)
	}
	return
}

func (k *TestSwapHandleKeeper) SetPool(_ cosmos.Context, pool Pool) error {
	asset := pool.Asset.GetLayer1Asset()
	k.pools[asset] = pool
	return nil
}

// IsActiveObserver see whether it is an active observer
func (k *TestSwapHandleKeeper) IsActiveObserver(_ cosmos.Context, addr cosmos.AccAddress) bool {
	return k.activeNodeAccount.NodeAddress.Equals(addr)
}

func (k *TestSwapHandleKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.activeNodeAccount.NodeAddress.Equals(addr) {
		return k.activeNodeAccount, nil
	}
	return NodeAccount{}, errors.New("not exist")
}

func (k *TestSwapHandleKeeper) AddToLiquidityFees(_ cosmos.Context, _ common.Asset, _ cosmos.Uint) error {
	return nil
}

func (k *TestSwapHandleKeeper) AddToSwapSlip(ctx cosmos.Context, asset common.Asset, fs cosmos.Int) error {
	return nil
}

func (k *TestSwapHandleKeeper) GetTotalSupply(_ cosmos.Context, _ common.Asset) cosmos.Uint {
	return k.synthSupply
}

func (k *TestSwapHandleKeeper) GetMimir(ctx cosmos.Context, key string) (int64, error) {
	if key == "EnableDerivedAssets" {
		if k.derivedAssets {
			return 1, nil
		}
		return 0, nil
	}
	return k.haltChain, nil
}

func (k *TestSwapHandleKeeper) MintToModule(_ cosmos.Context, _ string, _ common.Coin) error {
	return nil
}

func (k *TestSwapHandleKeeper) BurnFromModule(_ cosmos.Context, _ string, _ common.Coin) error {
	return nil
}

func (k *TestSwapHandleKeeper) SendFromModuleToModule(_ cosmos.Context, _, _ string, _ common.Coins) error {
	return nil
}

func (k *TestSwapHandleKeeper) GetModuleAddress(_ string) (common.Address, error) {
	return GetRandomTHORAddress(), nil
}

func (s *HandlerSwapSuite) TestValidation(c *C) {
	ctx, mgr := setupManagerForTest(c)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pools := make(map[common.Asset]Pool)
	pools[pool.Asset] = pool
	keeper := &TestSwapHandleKeeper{
		pools:             pools,
		activeNodeAccount: GetRandomValidatorNode(NodeActive),
		synthSupply:       cosmos.ZeroUint(),
	}
	mgr.K = keeper
	mgr.txOutStore = NewTxStoreDummy()

	handler := NewSwapHandler(mgr)

	txID := GetRandomTxHash()
	signerBNBAddr := GetRandomBNBAddress()
	observerAddr := keeper.activeNodeAccount.NodeAddress
	tx := common.NewTx(
		txID,
		signerBNBAddr,
		signerBNBAddr,
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(common.One*100)),
		},
		BNBGasFeeSingleton,
		"",
	)
	msg := NewMsgSwap(tx, common.BNBAsset.GetSyntheticAsset(), GetRandomTHORAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, MarketOrder, observerAddr)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// check that derived assets are no allowed
	msg.TargetAsset = common.BTCAsset.GetDerivedAsset()
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
	// enable derived assets and try again
	keeper.derivedAssets = true
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)
	msg.TargetAsset = common.BNBAsset.GetSyntheticAsset()

	// check that minting synths halts after hitting pool limit
	keeper.synthSupply = cosmos.NewUint(common.One * 200)
	mgr.K = keeper
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

func (s *HandlerSwapSuite) TestHandle(c *C) {
	ctx, mgr := setupManagerForTest(c)
	keeper := &TestSwapHandleKeeper{
		pools:             make(map[common.Asset]Pool),
		activeNodeAccount: GetRandomValidatorNode(NodeActive),
		synthSupply:       cosmos.ZeroUint(),
	}
	mgr.K = keeper
	mgr.txOutStore = NewTxStoreDummy()
	handler := NewSwapHandler(mgr)

	result, err := handler.Run(ctx, NewMsgMimir("what", 1, GetRandomBech32Addr()))
	c.Check(err, NotNil)
	c.Check(result, IsNil)
	c.Check(errors.Is(err, errInvalidMessage), Equals, true)

	txID := GetRandomTxHash()
	signerBNBAddr := GetRandomBNBAddress()
	observerAddr := keeper.activeNodeAccount.NodeAddress
	// no pool
	tx := common.NewTx(
		txID,
		signerBNBAddr,
		signerBNBAddr,
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.OneUint()),
		},
		BNBGasFeeSingleton,
		"",
	)
	msg := NewMsgSwap(tx, common.BNBAsset, signerBNBAddr, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, MarketOrder, observerAddr)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	c.Assert(keeper.SetPool(ctx, pool), IsNil)
	// fund is not enough to pay for transaction fee
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, NotNil)

	tx = common.NewTx(txID, signerBNBAddr, signerBNBAddr,
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(2*common.One)),
		},
		BNBGasFeeSingleton,
		"",
	)
	msgSwapPriceProtection := NewMsgSwap(tx, common.BNBAsset, signerBNBAddr, cosmos.NewUint(2*common.One), common.NoAddress, cosmos.ZeroUint(), "", "", nil, MarketOrder, observerAddr)
	result, err = handler.Run(ctx, msgSwapPriceProtection)
	c.Assert(err.Error(), Equals, errors.New("emit asset 192233756 less than price limit 200000000").Error())
	c.Assert(result, IsNil)

	poolTCAN := NewPool()
	tCanAsset, err := common.NewAsset("BNB.TCAN-014")
	c.Assert(err, IsNil)
	poolTCAN.Asset = tCanAsset
	poolTCAN.BalanceAsset = cosmos.NewUint(334850000)
	poolTCAN.BalanceRune = cosmos.NewUint(2349500000)
	c.Assert(keeper.SetPool(ctx, poolTCAN), IsNil)
	bnbAddr := GetRandomBNBAddress()
	m, err := ParseMemo(mgr.GetVersion(), "swap:BNB.BNB:"+bnbAddr.String()+":121893238")
	c.Assert(err, IsNil)
	txIn := NewObservedTx(
		common.NewTx(GetRandomTxHash(), signerBNBAddr, GetRandomBNBAddress(),
			common.Coins{
				common.NewCoin(tCanAsset, cosmos.NewUint(20000000)),
			},
			BNBGasFeeSingleton,
			"swap:BNB.BNB:"+signerBNBAddr.String()+":121893238",
		),
		1,
		GetRandomPubKey(), 1,
	)
	msgSwapFromTxIn, err := getMsgSwapFromMemo(m.(SwapMemo), txIn, observerAddr)
	c.Assert(err, IsNil)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)

	c.Assert(keeper.SetPool(ctx, pool), IsNil) // reset the pool
	_, err = handler.Run(ctx, msgSwapFromTxIn.(*MsgSwap))
	c.Assert(err, IsNil)
	items, err = mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)

	result, err = handler.Run(ctx, msgSwapFromTxIn)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
	msgSwap := NewMsgSwap(GetRandomTx(), common.EmptyAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, MarketOrder, GetRandomBech32Addr())
	result, err = handler.Run(ctx, msgSwap)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	msgSwap2 := NewMsgSwap(GetRandomTx(), common.Rune67CAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, MarketOrder, GetRandomBech32Addr())
	result, err = handler.Run(ctx, msgSwap2)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, fmt.Sprintf("target asset can't be %s", msgSwap2.TargetAsset.String()))
	c.Assert(result, IsNil)

	// When chain is halted , swap should respect it
	keeper.haltChain = 1
	result, err = handler.Run(ctx, msgSwapFromTxIn)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
	keeper.haltChain = 0
}

func (s *HandlerSwapSuite) TestSwapSynthERC20(c *C) {
	ctx, mgr := setupManagerForTest(c)
	mgr.txOutStore = NewTxStoreDummy()
	handler := NewSwapHandler(mgr)

	pool := NewPool()
	asset, err := common.NewAsset("ETH.AAVE-0X7FC66500C84A76AD7E9C93437BFC5AC33E2DDAE9")
	c.Assert(err, IsNil)
	pool.Asset = asset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	c.Assert(mgr.K.SetPool(ctx, pool), IsNil)

	m, err := ParseMemo(mgr.GetVersion(), "=:ETH/AAVE-0X7FC66:thor1x0jkvqdh2hlpeztd5zyyk70n3efx6mhudkmnn2::thor1a427q3v96psuj4fnughdw8glt5r7j38lj7rkp8:100")
	c.Assert(err, IsNil)
	swapM, ok := m.(SwapMemo)
	c.Assert(ok, Equals, true)
	swapM.Asset = fuzzyAssetMatch(ctx, mgr.K, swapM.Asset)
	txIn := NewObservedTx(
		common.NewTx("832B575FC2E92057BE1E1D69277B5AF690ADDF3E98E76FFC67232F846D87CB45", "bnb1psc68r72zlj6uhqyqda6hl8l8028u3c7jnk6lp", "bnb1tsqqch9ak73e44aumfeqda6d2vhusple4ffydk",
			common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(20000000)),
			},
			BNBGasFeeSingleton,
			"=:ETH/AAVE-0X7FC66:thor1x0jkvqdh2hlpeztd5zyyk70n3efx6mhudkmnn2::thor1a427q3v96psuj4fnughdw8glt5r7j38lj7rkp8:100",
		),
		1,
		GetRandomPubKey(), 1,
	)
	observerAddr, err := GetRandomTHORAddress().AccAddress()
	c.Assert(err, IsNil)
	msgSwapFromTxIn, err := getMsgSwapFromMemo(m.(SwapMemo), txIn, observerAddr)
	c.Assert(err, IsNil)
	res, err := handler.Run(ctx, msgSwapFromTxIn)
	c.Assert(res, IsNil)
	c.Assert(err, NotNil)
}

func (s *HandlerSwapSuite) TestDoubleSwap(c *C) {
	ctx, mgr := setupManagerForTest(c)
	keeper := &TestSwapHandleKeeper{
		pools:             make(map[common.Asset]Pool),
		activeNodeAccount: GetRandomValidatorNode(NodeActive),
		synthSupply:       cosmos.ZeroUint(),
	}
	mgr.K = keeper
	mgr.txOutStore = NewTxStoreDummy()
	handler := NewSwapHandler(mgr)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	c.Assert(keeper.SetPool(ctx, pool), IsNil)

	poolTCAN := NewPool()
	tCanAsset, err := common.NewAsset("BNB.TCAN-014")
	c.Assert(err, IsNil)
	poolTCAN.Asset = tCanAsset
	poolTCAN.BalanceAsset = cosmos.NewUint(334850000)
	poolTCAN.BalanceRune = cosmos.NewUint(2349500000)
	c.Assert(keeper.SetPool(ctx, poolTCAN), IsNil)

	signerBNBAddr := GetRandomBNBAddress()
	observerAddr := keeper.activeNodeAccount.NodeAddress

	// double swap - happy path
	m, err := ParseMemo(mgr.GetVersion(), "swap:BNB.BNB:"+signerBNBAddr.String())
	c.Assert(err, IsNil)
	txIn := NewObservedTx(
		common.NewTx(GetRandomTxHash(), signerBNBAddr, GetRandomBNBAddress(),
			common.Coins{
				common.NewCoin(tCanAsset, cosmos.NewUint(20000000)),
			},
			BNBGasFeeSingleton,
			"swap:BNB.BNB:"+signerBNBAddr.String(),
		),
		1,
		GetRandomPubKey(), 1,
	)
	msgSwapFromTxIn, err := getMsgSwapFromMemo(m.(SwapMemo), txIn, observerAddr)
	c.Assert(err, IsNil)

	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)

	_, err = handler.Run(ctx, msgSwapFromTxIn)
	c.Assert(err, IsNil)

	items, err = mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
	// double swap , RUNE not enough to pay for transaction fee
	testnetBNBAddr := GetRandomBNBAddress()
	m1, err := ParseMemo(mgr.GetVersion(), "swap:BNB.BNB:"+testnetBNBAddr.String())
	c.Assert(err, IsNil)
	txIn1 := NewObservedTx(
		common.NewTx(GetRandomTxHash(), signerBNBAddr, GetRandomBNBAddress(),
			common.Coins{
				common.NewCoin(tCanAsset, cosmos.NewUint(100000)),
			},
			BNBGasFeeSingleton,
			"swap:BNB.BNB:"+testnetBNBAddr.String(),
		),
		1,
		GetRandomPubKey(), 1,
	)
	msgSwapFromTxIn1, err := getMsgSwapFromMemo(m1.(SwapMemo), txIn1, observerAddr)
	c.Assert(err, IsNil)
	mgr.TxOutStore().ClearOutboundItems(ctx)
	_, err = handler.Run(ctx, msgSwapFromTxIn1)
	c.Assert(err, Equals, errSwapFailNotEnoughFee)

	items, err = mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)
}

func (s *HandlerSwapSuite) TestSwapOutDexIntegration(c *C) {
	ctx, mgr := setupManagerForTest(c)
	mgr.txOutStore = NewTxStoreDummy()
	handler := NewSwapHandler(mgr)

	pool := NewPool()
	asset, err := common.NewAsset("ETH.ETH")
	c.Assert(err, IsNil)
	pool.Asset = asset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	c.Assert(mgr.K.SetPool(ctx, pool), IsNil)

	swapMemo := "swap:ETH.ETH:" + types.GetRandomETHAddress().String() + "::::2f2386f3848:" + types.GetRandomETHAddress().String()
	m, err := ParseMemoWithTHORNames(ctx, mgr.Keeper(), swapMemo)
	c.Assert(err, IsNil)

	txIn := NewObservedTx(
		common.NewTx(GetRandomTxHash(), GetRandomTHORAddress(), GetRandomTHORAddress(),
			common.Coins{
				common.NewCoin(common.RuneNative, cosmos.NewUint(2000000000)),
			},
			common.Gas{
				common.NewCoin(common.RuneNative, cosmos.NewUint(20000000)),
			},
			swapMemo,
		),
		1,
		GetRandomPubKey(), 1,
	)

	observerAddr, err := GetRandomTHORAddress().AccAddress()
	c.Assert(err, IsNil)
	msgSwapFromTxIn, err := getMsgSwapFromMemo(m.(SwapMemo), txIn, observerAddr)
	c.Assert(err, IsNil)
	// when SwapOut Dex integration has been disabled by mimir , it should return an error cause refund
	mgr.Keeper().SetMimir(ctx, constants.SwapOutDexAggregationDisabled.String(), 1)
	res, err := handler.Run(ctx, msgSwapFromTxIn)
	c.Assert(res, IsNil)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "swap out dex integration disabled")

	mgr.Keeper().SetMimir(ctx, constants.SwapOutDexAggregationDisabled.String(), 0)

	// when target asset address is empty , swap should fail
	swapM, ok := m.(SwapMemo)
	c.Assert(ok, Equals, true)
	swapM.DexTargetAddress = ""
	msgSwapFromTxIn, err = getMsgSwapFromMemo(swapM, txIn, observerAddr)
	c.Assert(err, IsNil)
	res, err = handler.Run(ctx, msgSwapFromTxIn)
	c.Assert(err, NotNil)
	c.Assert(res, IsNil)
	c.Assert(errors.Is(err, se.ErrUnknownRequest), Equals, true)
	c.Assert(strings.HasPrefix(err.Error(), "aggregator target asset address is empty"), Equals, true)

	// When the target asset is not ETH.ETH, it should fail
	swapM, ok = m.(SwapMemo)
	c.Assert(ok, Equals, true)
	AAVEAsset, err := common.NewAsset("ETH.AAVE-0X7FC66500C84A76AD7E9C93437BFC5AC33E2DDAE9")
	c.Assert(err, IsNil)
	swapM.Asset = AAVEAsset
	msgSwapFromTxIn, err = getMsgSwapFromMemo(swapM, txIn, observerAddr)
	c.Assert(err, IsNil)
	res, err = handler.Run(ctx, msgSwapFromTxIn)
	c.Assert(err, NotNil)
	c.Assert(res, IsNil)
	c.Assert(err.Error(), Equals, "target asset (ETH.AAVE-0X7FC66500C84A76AD7E9C93437BFC5AC33E2DDAE9) is not gas asset , can't use dex feature")

	// when specified aggregator is not white list , swap should fail
	swapM, ok = m.(SwapMemo)
	c.Assert(ok, Equals, true)
	swapM.DexAggregator = "whatever"
	msgSwapFromTxIn, err = getMsgSwapFromMemo(swapM, txIn, observerAddr)
	c.Assert(err, IsNil)
	res, err = handler.Run(ctx, msgSwapFromTxIn)
	c.Assert(err, NotNil)
	c.Assert(res, IsNil)
	c.Assert(err.Error(), Equals, "whatever aggregator not found")

	// when aggregator target address is not valid , but we don't care
	swapM, ok = m.(SwapMemo)
	c.Assert(ok, Equals, true)
	swapM.DexTargetAddress = "whatever"
	msgSwapFromTxIn, err = getMsgSwapFromMemo(swapM, txIn, observerAddr)
	c.Assert(err, IsNil)
	res, err = handler.Run(ctx, msgSwapFromTxIn)
	c.Assert(err, IsNil)
	c.Assert(res, NotNil)

	// when aggregator target address and target chain doesn't match , don't care
	swapM, ok = m.(SwapMemo)
	c.Assert(ok, Equals, true)
	swapM.DexTargetAddress = GetRandomBNBAddress().String()
	msgSwapFromTxIn, err = getMsgSwapFromMemo(swapM, txIn, observerAddr)
	c.Assert(err, IsNil)
	res, err = handler.Run(ctx, msgSwapFromTxIn)
	c.Assert(err, IsNil)
	c.Assert(res, NotNil)

	mgr.TxOutStore().ClearOutboundItems(ctx)
	// normal swap with DEX
	swapM, ok = m.(SwapMemo)
	c.Assert(ok, Equals, true)
	msgSwapFromTxIn, err = getMsgSwapFromMemo(swapM, txIn, observerAddr)
	c.Assert(err, IsNil)
	res, err = handler.Run(ctx, msgSwapFromTxIn)
	c.Assert(err, IsNil)
	c.Assert(res, NotNil)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
	c.Assert(items[0].Aggregator, Equals, "0x69800327b38A4CeF30367Dec3f64c2f2386f3848")
	c.Assert(items[0].AggregatorTargetAsset, Equals, swapM.DexTargetAddress)
	c.Assert(items[0].AggregatorTargetLimit, IsNil)
}
