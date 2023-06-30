package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type YggdrasilManagerV79Suite struct{}

var _ = Suite(&YggdrasilManagerV79Suite{})

func (s YggdrasilManagerV79Suite) TestCalcTargetAmounts(c *C) {
	var pools []Pool
	p := NewPool()
	p.Asset = common.BNBAsset
	p.BalanceRune = cosmos.NewUint(1000 * common.One)
	p.BalanceAsset = cosmos.NewUint(500 * common.One)
	pools = append(pools, p)

	p = NewPool()
	p.Asset = common.BTCAsset
	p.BalanceRune = cosmos.NewUint(3000 * common.One)
	p.BalanceAsset = cosmos.NewUint(225 * common.One)
	pools = append(pools, p)

	ygg := GetRandomVault()
	ygg.Type = YggdrasilVault

	minRuneDepth := cosmos.NewUint(1000 * common.One)

	totalBond := cosmos.NewUint(8000 * common.One)
	bond := cosmos.NewUint(200 * common.One)
	ymgr := newYggMgrV79(keeper.KVStoreDummy{})
	yggFundLimit := cosmos.NewUint(50)
	coins, err := ymgr.calcTargetYggCoins(pools, ygg, bond, totalBond, yggFundLimit, minRuneDepth)
	c.Assert(err, IsNil)
	c.Assert(coins, HasLen, 2)
	c.Check(coins[0].Asset.String(), Equals, common.BNBAsset.String())
	c.Check(coins[0].Amount.Uint64(), Equals, cosmos.NewUint(6.25*common.One).Uint64(), Commentf("%d vs %d", coins[0].Amount.Uint64(), cosmos.NewUint(6.25*common.One).Uint64()))
	c.Check(coins[1].Asset.String(), Equals, common.BTCAsset.String())
	c.Check(coins[1].Amount.Uint64(), Equals, cosmos.NewUint(2.8125*common.One).Uint64(), Commentf("%d vs %d", coins[1].Amount.Uint64(), cosmos.NewUint(2.8125*common.One).Uint64()))
}

func (s YggdrasilManagerV79Suite) TestCalcTargetAmounts2(c *C) {
	// Adding specific test per PR request
	// https://gitlab.com/thorchain/thornode/merge_requests/246#note_241913460
	var pools []Pool
	p := NewPool()
	p.Asset = common.BNBAsset
	p.BalanceRune = cosmos.NewUint(1000000 * common.One)
	p.BalanceAsset = cosmos.NewUint(1 * common.One)
	pools = append(pools, p)

	ygg := GetRandomVault()
	ygg.Type = YggdrasilVault

	minRuneDepth := cosmos.NewUint(50_000 * common.One)

	totalBond := cosmos.NewUint(3000000 * common.One)
	bond := cosmos.NewUint(1000000 * common.One)
	ymgr := newYggMgrV79(keeper.KVStoreDummy{})
	yggFundLimit := cosmos.NewUint(50)
	coins, err := ymgr.calcTargetYggCoins(pools, ygg, bond, totalBond, yggFundLimit, minRuneDepth)
	c.Assert(err, IsNil)
	c.Assert(coins, HasLen, 1)
	c.Check(coins[0].Asset.String(), Equals, common.BNBAsset.String())
	c.Check(coins[0].Amount.Uint64(), Equals, cosmos.NewUint(0.16666667*common.One).Uint64(), Commentf("%d vs %d", coins[0].Amount.Uint64(), cosmos.NewUint(0.16666667*common.One).Uint64()))
}

func (s YggdrasilManagerV79Suite) TestCalcTargetAmounts3(c *C) {
	// pre populate the yggdrasil vault with funds already, ensure we don't
	// double up on funds.
	var pools []Pool
	p := NewPool()
	p.Asset = common.BNBAsset
	p.BalanceRune = cosmos.NewUint(1000 * common.One)
	p.BalanceAsset = cosmos.NewUint(500 * common.One)
	pools = append(pools, p)

	p = NewPool()
	p.Asset = common.BTCAsset
	p.BalanceRune = cosmos.NewUint(3000 * common.One)
	p.BalanceAsset = cosmos.NewUint(225 * common.One)
	pools = append(pools, p)

	ygg := GetRandomVault()
	ygg.Type = YggdrasilVault
	ygg.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(6.25*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(1.8125*common.One)),
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(30*common.One)),
	}

	minRuneDepth := cosmos.NewUint(1000 * common.One)

	totalBond := cosmos.NewUint(8000 * common.One)
	bond := cosmos.NewUint(200 * common.One)
	ymgr := newYggMgrV79(keeper.KVStoreDummy{})
	yggFundLimit := cosmos.NewUint(50)
	coins, err := ymgr.calcTargetYggCoins(pools, ygg, bond, totalBond, yggFundLimit, minRuneDepth)
	c.Assert(err, IsNil)
	c.Assert(coins, HasLen, 1, Commentf("%d", len(coins)))
	c.Check(coins[0].Asset.String(), Equals, common.BTCAsset.String())
	c.Check(coins[0].Amount.Uint64(), Equals, cosmos.NewUint(1*common.One).Uint64(), Commentf("%d vs %d", coins[0].Amount.Uint64(), cosmos.NewUint(2.8125*common.One).Uint64()))
}

func (s YggdrasilManagerV79Suite) TestCalcTargetAmounts4(c *C) {
	// test under bonded scenario
	var pools []Pool
	p := NewPool()
	p.Asset = common.BNBAsset
	p.BalanceRune = cosmos.NewUint(1000 * common.One)
	p.BalanceAsset = cosmos.NewUint(500 * common.One)
	pools = append(pools, p)

	p = NewPool()
	p.Asset = common.BTCAsset
	p.BalanceRune = cosmos.NewUint(3000 * common.One)
	p.BalanceAsset = cosmos.NewUint(225 * common.One)
	pools = append(pools, p)

	ygg := GetRandomVault()
	ygg.Type = YggdrasilVault
	ygg.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(6.25*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(1.8125*common.One)),
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(30*common.One)),
	}

	minRuneDepth := cosmos.NewUint(1000 * common.One)

	totalBond := cosmos.NewUint(2000 * common.One)
	bond := cosmos.NewUint(200 * common.One)
	ymgr := newYggMgrV79(keeper.KVStoreDummy{})
	yggFundLimit := cosmos.NewUint(50)
	coins, err := ymgr.calcTargetYggCoins(pools, ygg, bond, totalBond, yggFundLimit, minRuneDepth)
	c.Assert(err, IsNil)
	c.Assert(coins, HasLen, 1, Commentf("%d", len(coins)))
	c.Check(coins[0].Asset.String(), Equals, common.BTCAsset.String())
	c.Check(coins[0].Amount.Uint64(), Equals, cosmos.NewUint(1*common.One).Uint64(), Commentf("%d vs %d", coins[0].Amount.Uint64(), cosmos.NewUint(2.8125*common.One).Uint64()))
}

func (s YggdrasilManagerV79Suite) TestCalcTargetAmounts5(c *C) {
	var pools []Pool
	p := NewPool()
	p.Asset = common.BNBAsset
	p.BalanceRune = cosmos.NewUint(1000000 * common.One)
	p.BalanceAsset = cosmos.NewUint(1 * common.One)
	pools = append(pools, p)

	p2 := NewPool()
	p2.Asset = common.ETHAsset
	p2.BalanceRune = cosmos.NewUint(30_000 * common.One)
	p2.BalanceAsset = cosmos.NewUint(2 * common.One)
	pools = append(pools, p2)

	ygg := GetRandomVault()
	ygg.Type = YggdrasilVault

	minRuneDepth := cosmos.NewUint(50_000 * common.One)

	totalBond := cosmos.NewUint(3000000 * common.One)
	bond := cosmos.NewUint(1000000 * common.One)
	ymgr := newYggMgrV79(keeper.KVStoreDummy{})
	yggFundLimit := cosmos.NewUint(50)
	coins, err := ymgr.calcTargetYggCoins(pools, ygg, bond, totalBond, yggFundLimit, minRuneDepth)

	// Ygg should only have BNB, since ETH pool does not have enough RUNE to be sent out from Asgard
	c.Assert(err, IsNil)
	c.Assert(coins, HasLen, 1)
	c.Check(coins[0].Asset.String(), Equals, common.BNBAsset.String())
	c.Check(coins[0].Amount.Uint64(), Equals, cosmos.NewUint(0.16666667*common.One).Uint64(), Commentf("%d vs %d", coins[0].Amount.Uint64(), cosmos.NewUint(0.16666667*common.One).Uint64()))
}

func (s YggdrasilManagerV79Suite) TestFund(c *C) {
	ctx, k := setupKeeperForTest(c)
	vault := GetRandomVault()
	vault.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(10000*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(10000*common.One)),
	}
	c.Assert(k.SetVault(ctx, vault), IsNil)
	mgr := NewDummyMgr()

	// setup 6 active nodes
	for i := 0; i < 6; i++ {
		na := GetRandomValidatorNode(NodeActive)
		na.Bond = cosmos.NewUint(common.One * 1000000)
		c.Assert(k.SetNodeAccount(ctx, na), IsNil)
	}
	ymgr := newYggMgrV79(k)
	ymgr.keeper.SetMimir(ctx, "PoolDepthForYggFundingMin", 100000_00000000)
	err := ymgr.Fund(ctx, mgr)
	c.Assert(err, IsNil)
	na1 := GetRandomValidatorNode(NodeActive)
	na1.Bond = cosmos.NewUint(1000000 * common.One)
	c.Assert(k.SetNodeAccount(ctx, na1), IsNil)
	bnbPool := NewPool()
	bnbPool.Asset = common.BNBAsset
	bnbPool.BalanceAsset = cosmos.NewUint(100000 * common.One)
	bnbPool.BalanceRune = cosmos.NewUint(100000 * common.One)
	c.Assert(k.SetPool(ctx, bnbPool), IsNil)
	err1 := ymgr.Fund(ctx, mgr)
	c.Assert(err1, IsNil)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
}

func (s YggdrasilManagerV79Suite) TestNotAvailablePoolAssetWillNotFundYggdrasil(c *C) {
	ctx, k := setupKeeperForTest(c)
	vault := GetRandomVault()
	asset, err := common.NewAsset("BNB.BUSD-BD1")
	c.Assert(err, IsNil)
	vault.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(10000*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(10000*common.One)),
		common.NewCoin(asset, cosmos.NewUint(10000*common.One)),
	}
	c.Assert(k.SetVault(ctx, vault), IsNil)
	mgr := NewDummyMgr()

	// setup 6 active nodes
	for i := 0; i < 6; i++ {
		na := GetRandomValidatorNode(NodeActive)
		na.Bond = cosmos.NewUint(common.One * 1000000)
		c.Assert(k.SetNodeAccount(ctx, na), IsNil)
	}
	ymgr := newYggMgrV79(k)
	ymgr.keeper.SetMimir(ctx, "PoolDepthForYggFundingMin", 100000_00000000)
	err = ymgr.Fund(ctx, mgr)
	c.Assert(err, IsNil)
	na1 := GetRandomValidatorNode(NodeActive)
	na1.Bond = cosmos.NewUint(1000000 * common.One)
	c.Assert(k.SetNodeAccount(ctx, na1), IsNil)
	bnbPool := NewPool()
	bnbPool.Asset = common.BNBAsset
	bnbPool.BalanceAsset = cosmos.NewUint(100000 * common.One)
	bnbPool.BalanceRune = cosmos.NewUint(100000 * common.One)
	c.Assert(k.SetPool(ctx, bnbPool), IsNil)

	busdPool := NewPool()
	busdPool.Asset = asset
	busdPool.BalanceRune = cosmos.NewUint(100000 * common.One)
	busdPool.BalanceAsset = cosmos.NewUint(10000 * common.One)
	busdPool.Status = PoolStaged
	c.Assert(k.SetPool(ctx, busdPool), IsNil)

	err1 := ymgr.Fund(ctx, mgr)
	c.Assert(err1, IsNil)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
}

func (s YggdrasilManagerV79Suite) TestChainTradingHaltWillNotFundYggdrasil(c *C) {
	ctx, k := setupKeeperForTest(c)
	vault := GetRandomVault()
	asset, err := common.NewAsset("BNB.BUSD-BD1")
	c.Assert(err, IsNil)
	vault.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(10000*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(10000*common.One)),
		common.NewCoin(asset, cosmos.NewUint(10000*common.One)),
		common.NewCoin(common.ETHAsset, cosmos.NewUint(10000*common.One)),
	}
	c.Assert(k.SetVault(ctx, vault), IsNil)
	mgr := NewDummyMgr()

	// setup 6 active nodes
	for i := 0; i < 6; i++ {
		na := GetRandomValidatorNode(NodeActive)
		na.Bond = cosmos.NewUint(common.One * 1000000)
		c.Assert(k.SetNodeAccount(ctx, na), IsNil)
	}
	ymgr := newYggMgrV79(k)
	ymgr.keeper.SetMimir(ctx, "PoolDepthForYggFundingMin", 100000_00000000)

	na1 := GetRandomValidatorNode(NodeActive)
	na1.Bond = cosmos.NewUint(1000000 * common.One)
	c.Assert(k.SetNodeAccount(ctx, na1), IsNil)
	bnbPool := NewPool()
	bnbPool.Asset = common.BNBAsset
	bnbPool.BalanceAsset = cosmos.NewUint(100000 * common.One)
	bnbPool.BalanceRune = cosmos.NewUint(100000 * common.One)
	c.Assert(k.SetPool(ctx, bnbPool), IsNil)

	busdPool := NewPool()
	busdPool.Asset = asset
	busdPool.BalanceRune = cosmos.NewUint(100000 * common.One)
	busdPool.BalanceAsset = cosmos.NewUint(10000 * common.One)
	busdPool.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, busdPool), IsNil)

	ethPool := NewPool()
	ethPool.Asset = common.ETHAsset
	ethPool.BalanceRune = cosmos.NewUint(100000 * common.One)
	ethPool.BalanceAsset = cosmos.NewUint(10000 * common.One)
	ethPool.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, ethPool), IsNil)
	ymgr.keeper.SetMimir(ctx, "HaltETHTrading", 1)
	err1 := ymgr.Fund(ctx, mgr)
	c.Assert(err1, IsNil)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 2)
}

func (s YggdrasilManagerV79Suite) TestAbandonYggdrasil(c *C) {
	ctx, mgr := setupManagerForTest(c)
	vault := GetRandomVault()
	vault.Membership = []string{vault.PubKey.String()}
	vault.Coins = common.Coins{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(10000*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(10000*common.One)),
	}
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	// add a queue , if we don't have pool , we don't know how to slash
	bnbPool := NewPool()
	bnbPool.Asset = common.BNBAsset
	bnbPool.BalanceRune = cosmos.NewUint(1000 * common.One)
	bnbPool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	c.Assert(mgr.Keeper().SetPool(ctx, bnbPool), IsNil)
	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.BalanceRune = cosmos.NewUint(1000 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	c.Assert(mgr.Keeper().SetPool(ctx, btcPool), IsNil)
	// setup 6 active nodes ,  so it will fund yggdrasil
	for i := 0; i < 6; i++ {
		na := GetRandomValidatorNode(NodeActive)
		na.Bond = cosmos.NewUint(common.One * 1000000)
		FundModule(c, ctx, mgr.Keeper(), BondName, na.Bond.Uint64())
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	}
	naDisabled := GetRandomValidatorNode(NodeDisabled)
	naDisabled.RequestedToLeave = true
	naDisabled.Bond = cosmos.NewUint(common.One * 1000000)
	FundModule(c, ctx, mgr.Keeper(), BondName, naDisabled.Bond.Uint64())
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, naDisabled), IsNil)

	yggdrasilVault := GetRandomVault()
	yggdrasilVault.PubKey = naDisabled.PubKeySet.Secp256k1
	yggdrasilVault.Membership = []string{yggdrasilVault.PubKey.String()}
	yggdrasilVault.Coins = common.Coins{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(250*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(200*common.One)),
	}
	yggdrasilVault.Type = YggdrasilVault
	yggdrasilVault.Status = ActiveVault
	c.Assert(mgr.Keeper().SetVault(ctx, yggdrasilVault), IsNil)
	ymgr := newYggMgrV79(mgr.Keeper())
	err := ymgr.Fund(ctx, mgr)
	c.Assert(err, IsNil)
	// make sure the yggdrasil vault had been removed
	c.Assert(mgr.Keeper().VaultExists(ctx, naDisabled.PubKeySet.Secp256k1), Equals, false)
	// make sure the node account had been slashed with bond
	naDisabled, err = mgr.Keeper().GetNodeAccount(ctx, naDisabled.NodeAddress)
	c.Assert(err, IsNil)
	c.Check(naDisabled.Bond.Equal(cosmos.NewUint(99932511250000)), Equals, true, Commentf("%d != %d", naDisabled.Bond.Uint64(), 99932511250000))
}

func (s YggdrasilManagerV79Suite) TestAbandonYggdrasilWithDifferentConditions(c *C) {
	ctx, mgr := setupManagerForTest(c)
	vault := GetRandomVault()
	vault.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(10000*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(10000*common.One)),
	}
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	// add a queue , if we don't have pool , we don't know how to slash
	bnbPool := NewPool()
	bnbPool.Asset = common.BNBAsset
	bnbPool.BalanceRune = cosmos.NewUint(1000 * common.One)
	bnbPool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	c.Assert(mgr.Keeper().SetPool(ctx, bnbPool), IsNil)
	// setup 6 active nodes ,  so it will fund yggdrasil
	for i := 0; i < 6; i++ {
		na := GetRandomValidatorNode(NodeActive)
		na.Bond = cosmos.NewUint(common.One * 1000000)
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	}
	naDisabled := GetRandomValidatorNode(NodeDisabled)
	naDisabled.RequestedToLeave = true
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, naDisabled), IsNil)

	yggdrasilVault := GetRandomVault()
	yggdrasilVault.PubKey = naDisabled.PubKeySet.Secp256k1
	yggdrasilVault.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(250*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(200*common.One)),
	}
	yggdrasilVault.Type = YggdrasilVault
	yggdrasilVault.Status = ActiveVault
	c.Assert(mgr.Keeper().SetVault(ctx, yggdrasilVault), IsNil)

	kh := &abandonYggdrasilTestHelper{
		Keeper:                       mgr.Keeper(),
		failToGetAsgardVaultByStatus: true,
	}
	ymgr := newYggMgrV79(kh)
	c.Assert(ymgr.abandonYggdrasilVaults(ctx, mgr), NotNil)

	kh = &abandonYggdrasilTestHelper{
		Keeper:               mgr.Keeper(),
		failToGetNodeAccount: true,
	}
	ymgr = newYggMgrV79(kh)
	c.Assert(ymgr.abandonYggdrasilVaults(ctx, mgr), IsNil)
	c.Assert(mgr.Keeper().VaultExists(ctx, naDisabled.PubKeySet.Secp256k1), Equals, true)

	// when bond is zero , it shouldn't do anything
	naDisabled.Bond = cosmos.ZeroUint()
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, naDisabled), IsNil)
	ymgr = newYggMgrV79(mgr.Keeper())
	c.Assert(ymgr.abandonYggdrasilVaults(ctx, mgr), IsNil)
	c.Assert(mgr.Keeper().VaultExists(ctx, naDisabled.PubKeySet.Secp256k1), Equals, true)

	// when Node account belongs to one of the retiring vault should not slash yet
	naDisabled.Bond = cosmos.NewUint(1000 * common.One)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, naDisabled), IsNil)
	asgardVault := GetRandomVault()
	asgardVault.Status = RetiringVault
	asgardVault.Type = AsgardVault
	asgardVault.Membership = common.PubKeys{
		GetRandomPubKey(),
		naDisabled.PubKeySet.Secp256k1,
	}.Strings()
	c.Assert(mgr.Keeper().SetVault(ctx, asgardVault), IsNil)

	ymgr = newYggMgrV79(mgr.Keeper())
	c.Assert(ymgr.abandonYggdrasilVaults(ctx, mgr), IsNil)
	c.Assert(mgr.Keeper().VaultExists(ctx, naDisabled.PubKeySet.Secp256k1), Equals, true)
}
