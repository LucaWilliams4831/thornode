package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type GasManagerTestSuiteV81 struct{}

var _ = Suite(&GasManagerTestSuiteV81{})

func (GasManagerTestSuiteV81) TestGasManagerV81(c *C) {
	ctx, mgr := setupManagerForTest(c)
	k := mgr.Keeper()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	gasMgr := newGasMgrV81(constAccessor, k)
	gasEvent := gasMgr.gasEvent
	c.Assert(gasMgr, NotNil)
	gasMgr.BeginBlock(mgr)
	c.Assert(gasEvent != gasMgr.gasEvent, Equals, true)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	c.Assert(k.SetPool(ctx, pool), IsNil)
	pool.Asset = common.BTCAsset
	c.Assert(k.SetPool(ctx, pool), IsNil)

	gasMgr.AddGasAsset(common.Gas{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(37500)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(1000)),
	}, true)
	c.Assert(gasMgr.GetGas(), HasLen, 2)
	gasMgr.AddGasAsset(common.Gas{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(38500)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(2000)),
	}, true)
	c.Assert(gasMgr.GetGas(), HasLen, 2)
	gasMgr.AddGasAsset(common.Gas{
		common.NewCoin(common.ETHAsset, cosmos.NewUint(38500)),
	}, true)
	c.Assert(gasMgr.GetGas(), HasLen, 3)
	eventMgr := newEventMgrV1()
	gasMgr.EndBlock(ctx, k, eventMgr)
}

func (GasManagerTestSuiteV81) TestGetFee(c *C) {
	ctx, k := setupKeeperForTest(c)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	gasMgr := newGasMgrV81(constAccessor, k)
	fee := gasMgr.GetFee(ctx, common.BNBChain, common.RuneAsset())
	defaultTxFee := uint64(constAccessor.GetInt64Value(constants.OutboundTransactionFee))
	// when there is no network fee available, it should just get from the constants
	c.Assert(fee.Uint64(), Equals, defaultTxFee)
	networkFee := NewNetworkFee(common.BNBChain, 1, bnbSingleTxFee.Uint64())
	c.Assert(k.SaveNetworkFee(ctx, common.BNBChain, networkFee), IsNil)
	fee = gasMgr.GetFee(ctx, common.BNBChain, common.RuneAsset())
	c.Assert(fee.Uint64(), Equals, defaultTxFee)
	c.Assert(k.SetPool(ctx, Pool{
		BalanceRune:  cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BNBAsset,
		Status:       PoolAvailable,
	}), IsNil)
	fee = gasMgr.GetFee(ctx, common.BNBChain, common.RuneAsset())
	c.Assert(fee.Uint64(), Equals, bnbSingleTxFee.Uint64()*3, Commentf("%d vs %d", fee.Uint64(), bnbSingleTxFee.Uint64()*3))

	// BTC chain
	networkFee = NewNetworkFee(common.BTCChain, 70, 50)
	c.Assert(k.SaveNetworkFee(ctx, common.BTCChain, networkFee), IsNil)
	fee = gasMgr.GetFee(ctx, common.BTCChain, common.RuneAsset())
	c.Assert(fee.Uint64(), Equals, defaultTxFee)
	c.Assert(k.SetPool(ctx, Pool{
		BalanceRune:  cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BTCAsset,
		Status:       PoolAvailable,
	}), IsNil)
	fee = gasMgr.GetFee(ctx, common.BTCChain, common.RuneAsset())
	c.Assert(fee.Uint64(), Equals, uint64(70*50*3))

	// Synth asset (BTC/BTC)
	sBTC, err := common.NewAsset("BTC/BTC")
	c.Assert(err, IsNil)

	// change the pool balance
	c.Assert(k.SetPool(ctx, Pool{
		BalanceRune:  cosmos.NewUint(500 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BTCAsset,
		Status:       PoolAvailable,
	}), IsNil)
	synthAssetFee := gasMgr.GetFee(ctx, common.THORChain, sBTC)
	c.Assert(synthAssetFee.Uint64(), Equals, uint64(400000))
}

type gasManagerTestHelper struct {
	keeper.Keeper
	failGetNetwork bool
	failGetPool    bool
	failSetPool    bool
}

func newGasManagerTestHelper(k keeper.Keeper) *gasManagerTestHelper {
	return &gasManagerTestHelper{
		Keeper: k,
	}
}

func (g *gasManagerTestHelper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if g.failGetNetwork {
		return Network{}, errKaboom
	}
	return g.Keeper.GetNetwork(ctx)
}

func (g *gasManagerTestHelper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if g.failGetPool {
		return NewPool(), errKaboom
	}
	return g.Keeper.GetPool(ctx, asset)
}

func (g *gasManagerTestHelper) SetPool(ctx cosmos.Context, p Pool) error {
	if g.failSetPool {
		return errKaboom
	}
	return g.Keeper.SetPool(ctx, p)
}

func (GasManagerTestSuiteV81) TestDifferentValidations(c *C) {
	ctx, mgr := setupManagerForTest(c)
	k := mgr.Keeper()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	gasMgr := newGasMgrV81(constAccessor, k)
	gasMgr.BeginBlock(mgr)
	helper := newGasManagerTestHelper(k)
	eventMgr := newEventMgrV1()
	gasMgr.EndBlock(ctx, helper, eventMgr)

	helper.failGetNetwork = true
	gasMgr.EndBlock(ctx, helper, eventMgr)
	helper.failGetNetwork = false

	helper.failGetPool = true
	gasMgr.AddGasAsset(common.Gas{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(37500)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(1000)),
		common.NewCoin(common.ETHAsset, cosmos.ZeroUint()),
	}, true)
	gasMgr.EndBlock(ctx, helper, eventMgr)
	helper.failGetPool = false
	helper.failSetPool = true
	p := NewPool()
	p.Asset = common.BNBAsset
	p.BalanceAsset = cosmos.NewUint(common.One * 100)
	p.BalanceRune = cosmos.NewUint(common.One * 100)
	p.Status = PoolAvailable
	c.Assert(helper.Keeper.SetPool(ctx, p), IsNil)
	gasMgr.AddGasAsset(common.Gas{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(37500)),
	}, true)
	gasMgr.EndBlock(ctx, helper, eventMgr)
}

func (GasManagerTestSuiteV81) TestGetMaxGas(c *C) {
	ctx, k := setupKeeperForTest(c)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	gasMgr := newGasMgrV81(constAccessor, k)
	gasCoin, err := gasMgr.GetMaxGas(ctx, common.BTCChain)
	c.Assert(err, IsNil)
	c.Assert(gasCoin.Amount.IsZero(), Equals, true)
	networkFee := NewNetworkFee(common.BTCChain, 1000, 127)
	c.Assert(k.SaveNetworkFee(ctx, common.BTCChain, networkFee), IsNil)
	gasCoin, err = gasMgr.GetMaxGas(ctx, common.BTCChain)
	c.Assert(err, IsNil)
	c.Assert(gasCoin.Amount.Uint64(), Equals, uint64(127*1000*3/2))

	networkFee = NewNetworkFee(common.TERRAChain, 123, 127)
	c.Assert(k.SaveNetworkFee(ctx, common.TERRAChain, networkFee), IsNil)
	gasCoin, err = gasMgr.GetMaxGas(ctx, common.TERRAChain)
	c.Assert(err, IsNil)
	c.Assert(gasCoin.Amount.Uint64(), Equals, uint64(23400))
}
