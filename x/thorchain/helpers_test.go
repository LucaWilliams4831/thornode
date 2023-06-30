package thorchain

import (
	"fmt"
	"strings"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type HelperSuite struct{}

var _ = Suite(&HelperSuite{})

type TestRefundBondKeeper struct {
	keeper.KVStoreDummy
	ygg     Vault
	pool    Pool
	na      NodeAccount
	vaults  Vaults
	modules map[string]int64
	consts  constants.ConstantValues
}

func (k *TestRefundBondKeeper) GetConfigInt64(ctx cosmos.Context, key constants.ConstantName) int64 {
	return k.consts.GetInt64Value(key)
}

func (k *TestRefundBondKeeper) GetAsgardVaultsByStatus(_ cosmos.Context, _ VaultStatus) (Vaults, error) {
	return k.vaults, nil
}

func (k *TestRefundBondKeeper) VaultExists(_ cosmos.Context, pk common.PubKey) bool {
	return true
}

func (k *TestRefundBondKeeper) GetVault(_ cosmos.Context, pk common.PubKey) (Vault, error) {
	if k.ygg.PubKey.Equals(pk) {
		return k.ygg, nil
	}
	return Vault{}, errKaboom
}

func (k *TestRefundBondKeeper) GetLeastSecure(ctx cosmos.Context, vaults Vaults, signingTransPeriod int64) Vault {
	return vaults[0]
}

func (k *TestRefundBondKeeper) GetPool(_ cosmos.Context, asset common.Asset) (Pool, error) {
	if k.pool.Asset.Equals(asset) {
		return k.pool, nil
	}
	return NewPool(), errKaboom
}

func (k *TestRefundBondKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	k.na = na
	return nil
}

func (k *TestRefundBondKeeper) SetPool(_ cosmos.Context, p Pool) error {
	if k.pool.Asset.Equals(p.Asset) {
		k.pool = p
		return nil
	}
	return errKaboom
}

func (k *TestRefundBondKeeper) DeleteVault(_ cosmos.Context, key common.PubKey) error {
	if k.ygg.PubKey.Equals(key) {
		k.ygg = NewVault(1, InactiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	}
	return nil
}

func (k *TestRefundBondKeeper) SetVault(ctx cosmos.Context, vault Vault) error {
	if k.ygg.PubKey.Equals(vault.PubKey) {
		k.ygg = vault
	}
	return nil
}

func (k *TestRefundBondKeeper) SetBondProviders(ctx cosmos.Context, _ BondProviders) error {
	return nil
}

func (k *TestRefundBondKeeper) GetBondProviders(ctx cosmos.Context, add cosmos.AccAddress) (BondProviders, error) {
	return BondProviders{}, nil
}

func (k *TestRefundBondKeeper) SendFromModuleToModule(_ cosmos.Context, from, to string, coins common.Coins) error {
	k.modules[from] -= int64(coins[0].Amount.Uint64())
	k.modules[to] += int64(coins[0].Amount.Uint64())
	return nil
}

func (s *HelperSuite) TestSubsidizePoolWithSlashBond(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ygg := GetRandomVault()
	c.Assert(subsidizePoolWithSlashBond(ctx, ygg, cosmos.NewUint(100*common.One), cosmos.ZeroUint(), mgr), IsNil)
	poolBNB := NewPool()
	poolBNB.Asset = common.BNBAsset
	poolBNB.BalanceRune = cosmos.NewUint(100 * common.One)
	poolBNB.BalanceAsset = cosmos.NewUint(100 * common.One)
	poolBNB.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolBNB), IsNil)

	poolTCAN := NewPool()
	tCanAsset, err := common.NewAsset("BNB.TCAN-014")
	c.Assert(err, IsNil)
	poolTCAN.Asset = tCanAsset
	poolTCAN.BalanceRune = cosmos.NewUint(200 * common.One)
	poolTCAN.BalanceAsset = cosmos.NewUint(200 * common.One)
	poolTCAN.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolTCAN), IsNil)

	poolBTC := NewPool()
	poolBTC.Asset = common.BTCAsset
	poolBTC.BalanceAsset = cosmos.NewUint(300 * common.One)
	poolBTC.BalanceRune = cosmos.NewUint(300 * common.One)
	poolBTC.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolBTC), IsNil)
	ygg.Type = YggdrasilVault
	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(1*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One)),            // 1
		common.NewCoin(tCanAsset, cosmos.NewUint(common.One).QuoUint64(2)),       // 0.5 TCAN
		common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One).QuoUint64(4)), // 0.25 BTC
	}
	totalRuneLeft, err := getTotalYggValueInRune(ctx, mgr.Keeper(), ygg)
	c.Assert(err, IsNil)

	totalRuneStolen := ygg.GetCoin(common.RuneAsset()).Amount
	slashAmt := totalRuneLeft.MulUint64(3).QuoUint64(2)

	FundModule(c, ctx, mgr.Keeper(), BondName, slashAmt.Uint64())
	asgardBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName)
	bondBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, BondName)
	poolsBeforeSlash := poolBNB.BalanceRune.Add(poolTCAN.BalanceRune).Add(poolBTC.BalanceRune)

	c.Assert(subsidizePoolWithSlashBond(ctx, ygg, totalRuneLeft, slashAmt, mgr), IsNil)

	slashAmt = common.SafeSub(slashAmt, totalRuneStolen)
	totalRuneLeft = common.SafeSub(totalRuneLeft, totalRuneStolen)

	amountBNBForBNBPool := slashAmt.Mul(poolBNB.AssetValueInRune(cosmos.NewUint(common.One))).Quo(totalRuneLeft)
	runeBNB := poolBNB.BalanceRune.Add(amountBNBForBNBPool)
	bnbPoolAsset := poolBNB.BalanceAsset.Sub(cosmos.NewUint(common.One))
	poolBNB, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(poolBNB.BalanceRune.Equal(runeBNB), Equals, true)
	c.Assert(poolBNB.BalanceAsset.Equal(bnbPoolAsset), Equals, true)
	amountRuneForTCANPool := slashAmt.Mul(poolTCAN.AssetValueInRune(cosmos.NewUint(common.One).QuoUint64(2))).Quo(totalRuneLeft)
	runeTCAN := poolTCAN.BalanceRune.Add(amountRuneForTCANPool)
	tcanPoolAsset := poolTCAN.BalanceAsset.Sub(cosmos.NewUint(common.One).QuoUint64(2))
	poolTCAN, err = mgr.Keeper().GetPool(ctx, tCanAsset)
	c.Assert(err, IsNil)
	c.Assert(poolTCAN.BalanceRune.Equal(runeTCAN), Equals, true)
	c.Assert(poolTCAN.BalanceAsset.Equal(tcanPoolAsset), Equals, true)
	amountRuneForBTCPool := slashAmt.Mul(poolBTC.AssetValueInRune(cosmos.NewUint(common.One).QuoUint64(4))).Quo(totalRuneLeft)
	runeBTC := poolBTC.BalanceRune.Add(amountRuneForBTCPool)
	btcPoolAsset := poolBTC.BalanceAsset.Sub(cosmos.NewUint(common.One).QuoUint64(4))
	poolBTC, err = mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(poolBTC.BalanceRune.Equal(runeBTC), Equals, true)
	c.Assert(poolBTC.BalanceAsset.Equal(btcPoolAsset), Equals, true)

	asgardAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName)
	bondAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, BondName)
	poolsAfterSlash := poolBNB.BalanceRune.Add(poolTCAN.BalanceRune).Add(poolBTC.BalanceRune)

	// subsidized RUNE should move from bond to asgard
	c.Assert(poolsAfterSlash.Sub(poolsBeforeSlash).Uint64(), Equals, asgardAfterSlash.Sub(asgardBeforeSlash).Uint64())
	c.Assert(asgardAfterSlash.Sub(asgardBeforeSlash).Uint64(), Equals, bondBeforeSlash.Sub(bondAfterSlash).Uint64())

	ygg1 := GetRandomVault()
	ygg1.Type = YggdrasilVault
	ygg1.Coins = common.Coins{
		common.NewCoin(tCanAsset, cosmos.NewUint(common.One*2)),       // 2 TCAN
		common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One*4)), // 4 BTC
	}
	totalRuneLeft, err = getTotalYggValueInRune(ctx, mgr.Keeper(), ygg1)
	c.Assert(err, IsNil)
	slashAmt = cosmos.NewUint(100 * common.One)
	c.Assert(subsidizePoolWithSlashBond(ctx, ygg1, totalRuneLeft, slashAmt, mgr), IsNil)
	amountRuneForTCANPool = slashAmt.Mul(poolTCAN.AssetValueInRune(cosmos.NewUint(common.One * 2))).Quo(totalRuneLeft)
	runeTCAN = poolTCAN.BalanceRune.Add(amountRuneForTCANPool)
	poolTCAN, err = mgr.Keeper().GetPool(ctx, tCanAsset)
	c.Assert(err, IsNil)
	c.Assert(poolTCAN.BalanceRune.Equal(runeTCAN), Equals, true)
	amountRuneForBTCPool = slashAmt.Mul(poolBTC.AssetValueInRune(cosmos.NewUint(common.One * 4))).Quo(totalRuneLeft)
	runeBTC = poolBTC.BalanceRune.Add(amountRuneForBTCPool)
	poolBTC, err = mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(poolBTC.BalanceRune.Equal(runeBTC), Equals, true)

	ygg2 := GetRandomVault()
	ygg2.Type = YggdrasilVault
	ygg2.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(2*common.One)),
		common.NewCoin(tCanAsset, cosmos.NewUint(0)),
	}
	totalRuneLeft, err = getTotalYggValueInRune(ctx, mgr.Keeper(), ygg2)
	c.Assert(err, IsNil)
	slashAmt = cosmos.NewUint(2 * common.One)
	c.Assert(subsidizePoolWithSlashBond(ctx, ygg2, totalRuneLeft, slashAmt, mgr), IsNil)

	// Skip subsidy if rune value of coin is 0 - can happen with an old, empty pool
	poolETH := NewPool()
	poolETH.Asset = common.ETHAsset
	poolETH.BalanceAsset = cosmos.ZeroUint()
	poolETH.BalanceRune = cosmos.NewUint(300 * common.One)
	poolETH.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolETH), IsNil)

	ygg3 := GetRandomVault()
	ygg3.Type = YggdrasilVault
	ygg3.Coins = common.Coins{
		common.NewCoin(common.ETHAsset, cosmos.NewUint(100)),
	}

	c.Assert(subsidizePoolWithSlashBond(ctx, ygg3, totalRuneLeft, slashAmt, mgr), IsNil)
	poolETH, err = mgr.Keeper().GetPool(ctx, common.ETHAsset)
	c.Assert(err, IsNil)
	c.Assert(poolETH.BalanceRune.Equal(cosmos.NewUint(300*common.One)), Equals, true)
}

func (s *HelperSuite) TestPausedLP(c *C) {
	ctx, mgr := setupManagerForTest(c)

	c.Check(isLPPaused(ctx, common.BNBChain, mgr), Equals, false)
	c.Check(isLPPaused(ctx, common.BTCChain, mgr), Equals, false)

	mgr.Keeper().SetMimir(ctx, "PauseLPBTC", 1)
	c.Check(isLPPaused(ctx, common.BTCChain, mgr), Equals, true)

	mgr.Keeper().SetMimir(ctx, "PauseLP", 1)
	c.Check(isLPPaused(ctx, common.BNBChain, mgr), Equals, true)
}

func (s *HelperSuite) TestRefundBondError(c *C) {
	ctx, _ := setupKeeperForTest(c)
	// active node should not refund bond
	pk := GetRandomPubKey()
	na := GetRandomValidatorNode(NodeActive)
	na.PubKeySet.Secp256k1 = pk
	na.Bond = cosmos.NewUint(100 * common.One)
	tx := GetRandomTx()
	tx.FromAddress = GetRandomTHORAddress()
	keeper1 := &TestRefundBondKeeper{
		modules: make(map[string]int64),
		consts:  constants.GetConstantValues(GetCurrentVersion()),
	}
	mgr := NewDummyMgrWithKeeper(keeper1)
	c.Assert(refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr), IsNil)

	// fail to get vault should return an error
	na.UpdateStatus(NodeStandby, ctx.BlockHeight())
	keeper1.na = na
	mgr.K = keeper1
	c.Assert(refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr), NotNil)

	// if the vault is not a yggdrasil pool , it should return an error
	ygg := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, pk, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.Coins = common.Coins{}
	keeper1.ygg = ygg
	mgr.K = keeper1
	c.Assert(refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr), NotNil)

	// fail to get pool should fail
	ygg = NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(27*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(27*common.One)),
	}
	keeper1.ygg = ygg
	mgr.K = keeper1
	c.Assert(refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr), NotNil)

	// when ygg asset in RUNE is more then bond , thorchain should slash the node account with all their bond
	keeper1.pool = Pool{
		Asset:        common.BNBAsset,
		BalanceRune:  cosmos.NewUint(1024 * common.One),
		BalanceAsset: cosmos.NewUint(167 * common.One),
	}
	mgr.K = keeper1
	c.Assert(refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr), IsNil)
	// make sure no tx has been generated for refund
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 0)
}

func (s *HelperSuite) TestRefundBondHappyPath(c *C) {
	ctx, _ := setupKeeperForTest(c)
	na := GetRandomValidatorNode(NodeActive)
	na.Bond = cosmos.NewUint(12098 * common.One)
	pk := GetRandomPubKey()
	na.PubKeySet.Secp256k1 = pk
	ygg := NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain}.Strings(), []ChainContract{})

	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(3946*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(27*common.One)),
	}
	keeper := &TestRefundBondKeeper{
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(23789 * common.One),
			BalanceAsset: cosmos.NewUint(167 * common.One),
		},
		ygg:     ygg,
		vaults:  Vaults{GetRandomVault()},
		modules: make(map[string]int64),
		consts:  constants.GetConstantValues(GetCurrentVersion()),
	}
	na.Status = NodeStandby
	mgr := NewDummyMgrWithKeeper(keeper)
	tx := GetRandomTx()
	tx.FromAddress, _ = common.NewAddress(na.BondAddress.String())
	yggAssetInRune, err := getTotalYggValueInRune(ctx, keeper, ygg)
	c.Assert(err, IsNil)
	err = refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr)
	c.Assert(err, IsNil)
	slashAmt := yggAssetInRune.MulUint64(3).QuoUint64(2)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)
	p, err := keeper.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	expectedPoolRune := cosmos.NewUint(23789 * common.One).Sub(cosmos.NewUint(3946 * common.One)).Add(slashAmt)
	c.Assert(p.BalanceRune.Equal(expectedPoolRune), Equals, true, Commentf("expect %s however we got %s", expectedPoolRune, p.BalanceRune))
	expectedPoolBNB := cosmos.NewUint(167 * common.One).Sub(cosmos.NewUint(27 * common.One))
	c.Assert(p.BalanceAsset.Equal(expectedPoolBNB), Equals, true, Commentf("expected BNB in pool %s , however we got %s", expectedPoolBNB, p.BalanceAsset))
}

func (s *HelperSuite) TestRefundBondDisableRequestToLeaveNode(c *C) {
	ctx, _ := setupKeeperForTest(c)
	na := GetRandomValidatorNode(NodeActive)
	na.Bond = cosmos.NewUint(12098 * common.One)
	pk := GetRandomPubKey()
	na.PubKeySet.Secp256k1 = pk
	ygg := NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain}.Strings(), []ChainContract{})

	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(3946*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(27*common.One)),
	}
	keeper := &TestRefundBondKeeper{
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(23789 * common.One),
			BalanceAsset: cosmos.NewUint(167 * common.One),
		},
		ygg:     ygg,
		vaults:  Vaults{GetRandomVault()},
		modules: make(map[string]int64),
		consts:  constants.GetConstantValues(GetCurrentVersion()),
	}
	na.Status = NodeStandby
	na.RequestedToLeave = true
	mgr := NewDummyMgrWithKeeper(keeper)
	tx := GetRandomTx()
	yggAssetInRune, err := getTotalYggValueInRune(ctx, keeper, ygg)
	c.Assert(err, IsNil)
	err = refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr)
	c.Assert(err, IsNil)
	slashAmt := yggAssetInRune.MulUint64(3).QuoUint64(2)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)
	p, err := keeper.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	expectedPoolRune := cosmos.NewUint(23789 * common.One).Sub(cosmos.NewUint(3946 * common.One)).Add(slashAmt)
	c.Assert(p.BalanceRune.Equal(expectedPoolRune), Equals, true, Commentf("expect %s however we got %s", expectedPoolRune, p.BalanceRune))
	expectedPoolBNB := cosmos.NewUint(167 * common.One).Sub(cosmos.NewUint(27 * common.One))
	c.Assert(p.BalanceAsset.Equal(expectedPoolBNB), Equals, true, Commentf("expected BNB in pool %s , however we got %s", expectedPoolBNB, p.BalanceAsset))
	c.Assert(keeper.na.Status == NodeDisabled, Equals, true)
}

func (s *HelperSuite) TestDollarsPerRune(c *C) {
	ctx, k := setupKeeperForTest(c)
	mgr := NewDummyMgrWithKeeper(k)
	mgr.Keeper().SetMimir(ctx, "TorAnchor-BNB-BUSD-BD1", 1) // enable BUSD pool as a TOR anchor
	busd, err := common.NewAsset("BNB.BUSD-BD1")
	c.Assert(err, IsNil)
	pool := NewPool()
	pool.Asset = busd
	pool.Status = PoolAvailable
	pool.BalanceRune = cosmos.NewUint(85515078103667)
	pool.BalanceAsset = cosmos.NewUint(709802235538353)
	pool.Decimals = 8
	c.Assert(k.SetPool(ctx, pool), IsNil)

	runeUSDPrice := telem(mgr.Keeper().DollarsPerRune(ctx))
	c.Assert(runeUSDPrice, Equals, float32(8.300317))

	// Now try with a second pool, identical depths.
	mgr.Keeper().SetMimir(ctx, "TorAnchor-ETH-USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48", 1) // enable USDC pool as a TOR anchor
	usdc, err := common.NewAsset("ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48")
	c.Assert(err, IsNil)
	pool = NewPool()
	pool.Asset = usdc
	pool.Status = PoolAvailable
	pool.BalanceRune = cosmos.NewUint(85515078103667)
	pool.BalanceAsset = cosmos.NewUint(709802235538353)
	pool.Decimals = 8
	c.Assert(k.SetPool(ctx, pool), IsNil)

	runeUSDPrice = telem(mgr.Keeper().DollarsPerRune(ctx))
	c.Assert(runeUSDPrice, Equals, float32(8.300317))
}

func (s *HelperSuite) TestTelem(c *C) {
	value := cosmos.NewUint(12047733)
	c.Assert(value.Uint64(), Equals, uint64(12047733))
	c.Assert(telem(value), Equals, float32(0.12047733))
}

type addGasFeesKeeperHelper struct {
	keeper.Keeper
	errGetNetwork bool
	errSetNetwork bool
	errGetPool    bool
	errSetPool    bool
}

func newAddGasFeesKeeperHelper(keeper keeper.Keeper) *addGasFeesKeeperHelper {
	return &addGasFeesKeeperHelper{
		Keeper: keeper,
	}
}

func (h *addGasFeesKeeperHelper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if h.errGetNetwork {
		return Network{}, errKaboom
	}
	return h.Keeper.GetNetwork(ctx)
}

func (h *addGasFeesKeeperHelper) SetNetwork(ctx cosmos.Context, data Network) error {
	if h.errSetNetwork {
		return errKaboom
	}
	return h.Keeper.SetNetwork(ctx, data)
}

func (h *addGasFeesKeeperHelper) SetPool(ctx cosmos.Context, pool Pool) error {
	if h.errSetPool {
		return errKaboom
	}
	return h.Keeper.SetPool(ctx, pool)
}

func (h *addGasFeesKeeperHelper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if h.errGetPool {
		return Pool{}, errKaboom
	}
	return h.Keeper.GetPool(ctx, asset)
}

type addGasFeeTestHelper struct {
	ctx cosmos.Context
	na  NodeAccount
	mgr Manager
}

func newAddGasFeeTestHelper(c *C) addGasFeeTestHelper {
	ctx, mgr := setupManagerForTest(c)
	keeper := newAddGasFeesKeeperHelper(mgr.Keeper())
	mgr.K = keeper
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	poolBTC := NewPool()
	poolBTC.Asset = common.BTCAsset
	poolBTC.BalanceAsset = cosmos.NewUint(100 * common.One)
	poolBTC.BalanceRune = cosmos.NewUint(100 * common.One)
	poolBTC.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolBTC), IsNil)

	na := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	yggVault := NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, na.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	c.Assert(mgr.Keeper().SetVault(ctx, yggVault), IsNil)
	version := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(version)
	mgr.gasMgr = newGasMgrV81(constAccessor, keeper)
	return addGasFeeTestHelper{
		ctx: ctx,
		mgr: mgr,
		na:  na,
	}
}

func (s *HelperSuite) TestAddGasFees(c *C) {
	testCases := []struct {
		name        string
		txCreator   func(helper addGasFeeTestHelper) ObservedTx
		runner      func(helper addGasFeeTestHelper, tx ObservedTx) error
		expectError bool
		validator   func(helper addGasFeeTestHelper, c *C)
	}{
		{
			name: "empty Gas should just return nil",
			txCreator: func(helper addGasFeeTestHelper) ObservedTx {
				return GetRandomObservedTx()
			},

			expectError: false,
		},
		{
			name: "normal BNB gas",
			txCreator: func(helper addGasFeeTestHelper) ObservedTx {
				tx := ObservedTx{
					Tx: common.Tx{
						ID:          GetRandomTxHash(),
						Chain:       common.BNBChain,
						FromAddress: GetRandomBNBAddress(),
						ToAddress:   GetRandomBNBAddress(),
						Coins: common.Coins{
							common.NewCoin(common.BNBAsset, cosmos.NewUint(5*common.One)),
							common.NewCoin(common.RuneAsset(), cosmos.NewUint(8*common.One)),
						},
						Gas: common.Gas{
							common.NewCoin(common.BNBAsset, BNBGasFeeSingleton[0].Amount),
						},
						Memo: "",
					},
					Status:         types.Status_done,
					OutHashes:      nil,
					BlockHeight:    helper.ctx.BlockHeight(),
					Signers:        []string{helper.na.NodeAddress.String()},
					ObservedPubKey: helper.na.PubKeySet.Secp256k1,
				}
				return tx
			},
			runner: func(helper addGasFeeTestHelper, tx ObservedTx) error {
				return addGasFees(helper.ctx, helper.mgr, tx)
			},
			expectError: false,
			validator: func(helper addGasFeeTestHelper, c *C) {
				expected := common.NewCoin(common.BNBAsset, BNBGasFeeSingleton[0].Amount)
				c.Assert(helper.mgr.GasMgr().GetGas(), HasLen, 1)
				c.Assert(helper.mgr.GasMgr().GetGas()[0].Equals(expected), Equals, true)
			},
		},
		{
			name: "normal BTC gas",
			txCreator: func(helper addGasFeeTestHelper) ObservedTx {
				tx := ObservedTx{
					Tx: common.Tx{
						ID:          GetRandomTxHash(),
						Chain:       common.BTCChain,
						FromAddress: GetRandomBTCAddress(),
						ToAddress:   GetRandomBTCAddress(),
						Coins: common.Coins{
							common.NewCoin(common.BTCAsset, cosmos.NewUint(5*common.One)),
						},
						Gas: common.Gas{
							common.NewCoin(common.BTCAsset, cosmos.NewUint(2000)),
						},
						Memo: "",
					},
					Status:         types.Status_done,
					OutHashes:      nil,
					BlockHeight:    helper.ctx.BlockHeight(),
					Signers:        []string{helper.na.NodeAddress.String()},
					ObservedPubKey: helper.na.PubKeySet.Secp256k1,
				}
				return tx
			},
			runner: func(helper addGasFeeTestHelper, tx ObservedTx) error {
				return addGasFees(helper.ctx, helper.mgr, tx)
			},
			expectError: false,
			validator: func(helper addGasFeeTestHelper, c *C) {
				expected := common.NewCoin(common.BTCAsset, cosmos.NewUint(2000))
				c.Assert(helper.mgr.GasMgr().GetGas(), HasLen, 1)
				c.Assert(helper.mgr.GasMgr().GetGas()[0].Equals(expected), Equals, true)
			},
		},
	}
	for _, tc := range testCases {
		helper := newAddGasFeeTestHelper(c)
		tx := tc.txCreator(helper)
		var err error
		if tc.runner == nil {
			err = addGasFees(helper.ctx, helper.mgr, tx)
		} else {
			err = tc.runner(helper, tx)
		}

		if err != nil && !tc.expectError {
			c.Errorf("test case: %s,didn't expect error however it got : %s", tc.name, err)
			c.FailNow()
		}
		if err == nil && tc.expectError {
			c.Errorf("test case: %s, expect error however it didn't", tc.name)
			c.FailNow()
		}
		if !tc.expectError && tc.validator != nil {
			tc.validator(helper, c)
			continue
		}
	}
}

func (s *HelperSuite) TestEmitPoolStageCostEvent(c *C) {
	ctx, mgr := setupManagerForTest(c)
	emitPoolBalanceChangedEvent(ctx,
		NewPoolMod(common.BTCAsset, cosmos.NewUint(1000), false, cosmos.ZeroUint(), false), "test", mgr)
	found := false
	for _, e := range ctx.EventManager().Events() {
		if strings.EqualFold(e.Type, types.PoolBalanceChangeEventType) {
			found = true
			break
		}
	}
	c.Assert(found, Equals, true)
}

func (s *HelperSuite) TestIsSynthMintPause(c *C) {
	ctx, mgr := setupManagerForTest(c)

	mgr.Keeper().SetMimir(ctx, constants.MaxSynthPerPoolDepth.String(), 1500)

	pool := types.Pool{
		Asset:        common.BTCAsset,
		BalanceAsset: cosmos.NewUint(100 * common.One),
		BalanceRune:  cosmos.NewUint(100 * common.One),
	}
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	coins := cosmos.NewCoins(cosmos.NewCoin("btc/btc", cosmos.NewInt(29*common.One))) // 29% utilization
	c.Assert(mgr.coinKeeper.MintCoins(ctx, ModuleName, coins), IsNil)

	c.Assert(isSynthMintPaused(ctx, mgr, common.BTCAsset, cosmos.ZeroUint()), IsNil)

	// A swap that outputs 0.5 synth BTC would not surpass the synth utilization cap (29% -> 29.5%)
	c.Assert(isSynthMintPaused(ctx, mgr, common.BTCAsset, cosmos.NewUint(0.5*common.One)), IsNil)
	// A swap that outputs 1 synth BTC would not surpass the synth utilization cap (29% -> 30%)
	c.Assert(isSynthMintPaused(ctx, mgr, common.BTCAsset, cosmos.NewUint(1*common.One)), IsNil)
	// A swap that outputs 1.1 synth BTC would surpass the synth utilization cap (29% -> 30.1%)
	c.Assert(isSynthMintPaused(ctx, mgr, common.BTCAsset, cosmos.NewUint(1.1*common.One)), NotNil)

	coins = cosmos.NewCoins(cosmos.NewCoin("btc/btc", cosmos.NewInt(1*common.One))) // 30% utilization
	c.Assert(mgr.coinKeeper.MintCoins(ctx, ModuleName, coins), IsNil)

	c.Assert(isSynthMintPaused(ctx, mgr, common.BTCAsset, cosmos.ZeroUint()), IsNil)

	coins = cosmos.NewCoins(cosmos.NewCoin("btc/btc", cosmos.NewInt(1*common.One))) // 31% utilization
	c.Assert(mgr.coinKeeper.MintCoins(ctx, ModuleName, coins), IsNil)

	c.Assert(isSynthMintPaused(ctx, mgr, common.BTCAsset, cosmos.ZeroUint()), NotNil)
}

func (s *HelperSuite) TestIsTradingHalt(c *C) {
	ctx, mgr := setupManagerForTest(c)
	txID := GetRandomTxHash()
	tx := common.NewTx(txID, GetRandomBTCAddress(), GetRandomBTCAddress(), common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(100))), common.Gas{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(100)),
	}, "swap:BNB.BNB:"+GetRandomBNBAddress().String())
	memo, err := ParseMemoWithTHORNames(ctx, mgr.Keeper(), tx.Memo)
	c.Assert(err, IsNil)
	m, err := getMsgSwapFromMemo(memo.(SwapMemo), NewObservedTx(tx, ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()), GetRandomBech32Addr())
	c.Assert(err, IsNil)

	txAddLiquidity := common.NewTx(txID, GetRandomBTCAddress(), GetRandomBTCAddress(), common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(100))), common.Gas{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(100)),
	}, "add:BTC.BTC:"+GetRandomTHORAddress().String())
	memoAddExternal, err := ParseMemoWithTHORNames(ctx, mgr.Keeper(), txAddLiquidity.Memo)
	c.Assert(err, IsNil)
	mAddExternal, err := getMsgAddLiquidityFromMemo(ctx,
		memoAddExternal.(AddLiquidityMemo),
		NewObservedTx(txAddLiquidity, ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()),
		GetRandomBech32Addr())

	c.Assert(err, IsNil)
	txAddRUNE := common.NewTx(txID, GetRandomTHORAddress(), GetRandomTHORAddress(), common.NewCoins(common.NewCoin(common.RuneNative, cosmos.NewUint(100))), common.Gas{
		common.NewCoin(common.RuneNative, cosmos.NewUint(100)),
	}, "add:BTC.BTC:"+GetRandomBTCAddress().String())
	memoAddRUNE, err := ParseMemoWithTHORNames(ctx, mgr.Keeper(), txAddRUNE.Memo)
	c.Assert(err, IsNil)
	mAddRUNE, err := getMsgAddLiquidityFromMemo(ctx,
		memoAddRUNE.(AddLiquidityMemo),
		NewObservedTx(txAddRUNE, ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()),
		GetRandomBech32Addr())
	c.Assert(err, IsNil)

	mgr.Keeper().SetTHORName(ctx, THORName{
		Name:              "testtest",
		ExpireBlockHeight: ctx.BlockHeight() + 1024,
		Owner:             GetRandomBech32Addr(),
		PreferredAsset:    common.BNBAsset,
		Aliases: []THORNameAlias{
			{
				Chain:   common.BNBChain,
				Address: GetRandomBNBAddress(),
			},
		},
	})
	txWithThorname := common.NewTx(txID, GetRandomBTCAddress(), GetRandomBTCAddress(), common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(100))), common.Gas{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(100)),
	}, "swap:BNB.BNB:testtest")
	memoWithThorname, err := ParseMemoWithTHORNames(ctx, mgr.Keeper(), txWithThorname.Memo)
	c.Assert(err, IsNil)
	mWithThorname, err := getMsgSwapFromMemo(memoWithThorname.(SwapMemo), NewObservedTx(txWithThorname, ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()), GetRandomBech32Addr())
	c.Assert(err, IsNil)

	txSynth := common.NewTx(txID, GetRandomTHORAddress(), GetRandomTHORAddress(),
		common.NewCoins(common.NewCoin(common.BNBAsset.GetSyntheticAsset(), cosmos.NewUint(100))),
		common.Gas{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))},
		"swap:ETH.ETH:"+GetRandomTHORAddress().String())
	memoRedeemSynth, err := ParseMemoWithTHORNames(ctx, mgr.Keeper(), txSynth.Memo)
	c.Assert(err, IsNil)
	mRedeemSynth, err := getMsgSwapFromMemo(memoRedeemSynth.(SwapMemo), NewObservedTx(txSynth, ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()), GetRandomBech32Addr())
	c.Assert(err, IsNil)

	c.Assert(isTradingHalt(ctx, m, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, false)

	mgr.Keeper().SetMimir(ctx, "HaltTrading", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, true)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "HaltTrading"), IsNil)

	mgr.Keeper().SetMimir(ctx, "HaltBNBTrading", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, true)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "HaltBNBTrading"), IsNil)

	mgr.Keeper().SetMimir(ctx, "HaltBTCTrading", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, false)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "HaltBTCTrading"), IsNil)

	mgr.Keeper().SetMimir(ctx, "SolvencyHaltBTCChain", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, false)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "SolvencyHaltBTCChain"), IsNil)
}

func (s *HelperSuite) TestUpdateTxOutGas(c *C) {
	ctx, mgr := setupManagerForTest(c)

	// Create ObservedVoter and add a TxOut
	txVoter := GetRandomObservedTxVoter()
	txOut := GetRandomTxOutItem()
	txVoter.Actions = append(txVoter.Actions, txOut)
	mgr.Keeper().SetObservedTxInVoter(ctx, txVoter)

	// Try to set new gas, should return error as TxOut InHash doesn't match
	newGas := common.Gas{common.NewCoin(common.LUNAAsset, cosmos.NewUint(2000000))}
	err := updateTxOutGas(ctx, mgr.K, txOut, newGas)
	c.Assert(err.Error(), Equals, fmt.Sprintf("fail to find tx out in ObservedTxVoter %s", txOut.InHash))

	// Update TxOut InHash to match, should update gas
	txOut.InHash = txVoter.TxID
	txVoter.Actions[1] = txOut
	mgr.Keeper().SetObservedTxInVoter(ctx, txVoter)

	// Err should be Nil
	err = updateTxOutGas(ctx, mgr.K, txOut, newGas)
	c.Assert(err, IsNil)

	// Keeper should have updated gas of TxOut in Actions
	txVoter, err = mgr.Keeper().GetObservedTxInVoter(ctx, txVoter.TxID)
	c.Assert(err, IsNil)

	didUpdate := false
	for _, item := range txVoter.Actions {
		if item.Equals(txOut) && item.MaxGas.Equals(newGas) {
			didUpdate = true
			break
		}
	}

	c.Assert(didUpdate, Equals, true)
}

func (s *HelperSuite) TestUpdateTxOutGasRate(c *C) {
	ctx, mgr := setupManagerForTest(c)

	// Create ObservedVoter and add a TxOut
	txVoter := GetRandomObservedTxVoter()
	txOut := GetRandomTxOutItem()
	txVoter.Actions = append(txVoter.Actions, txOut)
	mgr.Keeper().SetObservedTxInVoter(ctx, txVoter)

	// Try to set new gas rate, should return error as TxOut InHash doesn't match
	newGasRate := int64(25)
	err := updateTxOutGasRate(ctx, mgr.K, txOut, newGasRate)
	c.Assert(err.Error(), Equals, fmt.Sprintf("fail to find tx out in ObservedTxVoter %s", txOut.InHash))

	// Update TxOut InHash to match, should update gas
	txOut.InHash = txVoter.TxID
	txVoter.Actions[1] = txOut
	mgr.Keeper().SetObservedTxInVoter(ctx, txVoter)

	// Err should be Nil
	err = updateTxOutGasRate(ctx, mgr.K, txOut, newGasRate)
	c.Assert(err, IsNil)

	// Now that the actions have been updated (dependent on Equals which checks GasRate),
	// update the GasRate in the outbound queue item.
	txOut.GasRate = newGasRate

	// Keeper should have updated gas of TxOut in Actions
	txVoter, err = mgr.Keeper().GetObservedTxInVoter(ctx, txVoter.TxID)
	c.Assert(err, IsNil)

	didUpdate := false
	for _, item := range txVoter.Actions {
		if item.Equals(txOut) && item.GasRate == newGasRate {
			didUpdate = true
			break
		}
	}

	c.Assert(didUpdate, Equals, true)
}

func (s *HelperSuite) TestPOLPoolValue(c *C) {
	ctx, mgr := setupManagerForTest(c)

	polAddress, err := mgr.Keeper().GetModuleAddress(ReserveName)
	c.Assert(err, IsNil)

	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.BalanceRune = cosmos.NewUint(2000 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(20 * common.One)
	btcPool.LPUnits = cosmos.NewUint(1600)
	c.Assert(mgr.Keeper().SetPool(ctx, btcPool), IsNil)

	coin := common.NewCoin(common.BTCAsset.GetSyntheticAsset(), cosmos.NewUint(10*common.One))
	c.Assert(mgr.Keeper().MintToModule(ctx, ModuleName, coin), IsNil)

	lps := LiquidityProviders{
		{
			Asset:             btcPool.Asset,
			RuneAddress:       GetRandomBNBAddress(),
			AssetAddress:      GetRandomBTCAddress(),
			LastAddHeight:     5,
			Units:             btcPool.LPUnits.QuoUint64(2),
			PendingRune:       cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			AssetDepositValue: cosmos.ZeroUint(),
			RuneDepositValue:  cosmos.ZeroUint(),
		},
		{
			Asset:             btcPool.Asset,
			RuneAddress:       polAddress,
			AssetAddress:      common.NoAddress,
			LastAddHeight:     10,
			Units:             btcPool.LPUnits.QuoUint64(2),
			PendingRune:       cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			AssetDepositValue: cosmos.ZeroUint(),
			RuneDepositValue:  cosmos.ZeroUint(),
		},
	}
	for _, lp := range lps {
		mgr.Keeper().SetLiquidityProvider(ctx, lp)
	}

	value, err := polPoolValue(ctx, mgr)
	c.Assert(err, IsNil)
	c.Check(value.Uint64(), Equals, uint64(150023441162), Commentf("%d", value.Uint64()))
}

func (s *HelperSuite) TestSecurityBond(c *C) {
	nas := make(NodeAccounts, 0)
	c.Assert(getEffectiveSecurityBond(nas).Uint64(), Equals, uint64(0), Commentf("%d", getEffectiveSecurityBond(nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10)},
	}
	c.Assert(getEffectiveSecurityBond(nas).Uint64(), Equals, uint64(10), Commentf("%d", getEffectiveSecurityBond(nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10)},
		NodeAccount{Bond: cosmos.NewUint(20)},
	}
	c.Assert(getEffectiveSecurityBond(nas).Uint64(), Equals, uint64(30), Commentf("%d", getEffectiveSecurityBond(nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10)},
		NodeAccount{Bond: cosmos.NewUint(20)},
		NodeAccount{Bond: cosmos.NewUint(30)},
	}
	c.Assert(getEffectiveSecurityBond(nas).Uint64(), Equals, uint64(30), Commentf("%d", getEffectiveSecurityBond(nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10)},
		NodeAccount{Bond: cosmos.NewUint(20)},
		NodeAccount{Bond: cosmos.NewUint(30)},
		NodeAccount{Bond: cosmos.NewUint(40)},
	}
	c.Assert(getEffectiveSecurityBond(nas).Uint64(), Equals, uint64(60), Commentf("%d", getEffectiveSecurityBond(nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10)},
		NodeAccount{Bond: cosmos.NewUint(20)},
		NodeAccount{Bond: cosmos.NewUint(30)},
		NodeAccount{Bond: cosmos.NewUint(40)},
		NodeAccount{Bond: cosmos.NewUint(50)},
	}
	c.Assert(getEffectiveSecurityBond(nas).Uint64(), Equals, uint64(100), Commentf("%d", getEffectiveSecurityBond(nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10)},
		NodeAccount{Bond: cosmos.NewUint(20)},
		NodeAccount{Bond: cosmos.NewUint(30)},
		NodeAccount{Bond: cosmos.NewUint(40)},
		NodeAccount{Bond: cosmos.NewUint(50)},
		NodeAccount{Bond: cosmos.NewUint(60)},
	}
	c.Assert(getEffectiveSecurityBond(nas).Uint64(), Equals, uint64(100), Commentf("%d", getEffectiveSecurityBond(nas).Uint64()))
}

func (s *HelperSuite) TestGetHardBondCap(c *C) {
	nas := make(NodeAccounts, 0)
	c.Assert(getHardBondCap(nas).Uint64(), Equals, uint64(0), Commentf("%d", getHardBondCap(nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10)},
	}
	c.Assert(getHardBondCap(nas).Uint64(), Equals, uint64(10), Commentf("%d", getHardBondCap(nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10)},
		NodeAccount{Bond: cosmos.NewUint(20)},
	}
	c.Assert(getHardBondCap(nas).Uint64(), Equals, uint64(20), Commentf("%d", getHardBondCap(nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10)},
		NodeAccount{Bond: cosmos.NewUint(20)},
		NodeAccount{Bond: cosmos.NewUint(30)},
	}
	c.Assert(getHardBondCap(nas).Uint64(), Equals, uint64(20), Commentf("%d", getHardBondCap(nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10)},
		NodeAccount{Bond: cosmos.NewUint(20)},
		NodeAccount{Bond: cosmos.NewUint(30)},
		NodeAccount{Bond: cosmos.NewUint(40)},
	}
	c.Assert(getHardBondCap(nas).Uint64(), Equals, uint64(30), Commentf("%d", getHardBondCap(nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10)},
		NodeAccount{Bond: cosmos.NewUint(20)},
		NodeAccount{Bond: cosmos.NewUint(30)},
		NodeAccount{Bond: cosmos.NewUint(40)},
		NodeAccount{Bond: cosmos.NewUint(50)},
	}
	c.Assert(getHardBondCap(nas).Uint64(), Equals, uint64(40), Commentf("%d", getHardBondCap(nas).Uint64()))

	nas = NodeAccounts{
		NodeAccount{Bond: cosmos.NewUint(10)},
		NodeAccount{Bond: cosmos.NewUint(20)},
		NodeAccount{Bond: cosmos.NewUint(30)},
		NodeAccount{Bond: cosmos.NewUint(40)},
		NodeAccount{Bond: cosmos.NewUint(50)},
		NodeAccount{Bond: cosmos.NewUint(60)},
	}
	c.Assert(getHardBondCap(nas).Uint64(), Equals, uint64(40), Commentf("%d", getHardBondCap(nas).Uint64()))
}

func (HandlerSuite) TestIsSignedByActiveNodeAccounts(c *C) {
	ctx, mgr := setupManagerForTest(c)

	r := isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{})
	c.Check(r, Equals, false,
		Commentf("empty signers should return false"))

	nodeAddr := GetRandomBech32Addr()
	r = isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{nodeAddr})
	c.Check(r, Equals, false,
		Commentf("empty node account should return false"))

	nodeAccount1 := GetRandomValidatorNode(NodeWhiteListed)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, nodeAccount1), IsNil)
	r = isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{nodeAccount1.NodeAddress})
	c.Check(r, Equals, false,
		Commentf("non-active node account should return false"))

	nodeAccount1.Status = NodeActive
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, nodeAccount1), IsNil)
	r = isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{nodeAccount1.NodeAddress})
	c.Check(r, Equals, true,
		Commentf("active node account should return true"))

	r = isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{nodeAccount1.NodeAddress, nodeAddr})
	c.Check(r, Equals, false,
		Commentf("should return false if any signer is not an active validator"))

	nodeAccount1.Type = NodeTypeVault
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, nodeAccount1), IsNil)
	r = isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{nodeAccount1.NodeAddress})
	c.Check(r, Equals, false,
		Commentf("non-validator node should return false"))

	asgardAddr := mgr.Keeper().GetModuleAccAddress(AsgardName)
	r = isSignedByActiveNodeAccounts(ctx, mgr.Keeper(), []cosmos.AccAddress{asgardAddr})
	c.Check(r, Equals, true,
		Commentf("asgard module address should return true"))
}
