package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type GasManagerTestSuiteV108 struct{}

var _ = Suite(&GasManagerTestSuiteV108{})

func (GasManagerTestSuiteV108) TestGasManagerV108(c *C) {
	ctx, mgr := setupManagerForTest(c)
	k := mgr.K
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	gasMgr := newGasMgrV108(constAccessor, k)
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

func (GasManagerTestSuiteV108) TestGetFee(c *C) {
	ctx, mgr := setupManagerForTest(c)
	k := mgr.Keeper()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	gasMgr := newGasMgrV108(constAccessor, k)
	gasMgr.BeginBlock(mgr)
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
	c.Assert(fee.Uint64(), Equals, bnbSingleTxFee.Uint64()*2, Commentf("%d vs %d", fee.Uint64(), bnbSingleTxFee.Uint64()*3))

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
	c.Assert(fee.Uint64(), Equals, uint64(70*50*2))

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

	// when MinimumL1OutboundFeeUSD set to something higher, it should override the network fee
	busdAsset, err := common.NewAsset("BNB.BUSD-BD1")
	c.Assert(err, IsNil)
	c.Assert(k.SetPool(ctx, Pool{
		BalanceRune:  cosmos.NewUint(500 * common.One),
		BalanceAsset: cosmos.NewUint(500 * common.One),
		Decimals:     8,
		Asset:        busdAsset,
		Status:       PoolAvailable,
	}), IsNil)
	k.SetMimir(ctx, constants.MinimumL1OutboundFeeUSD.String(), 1_0000_0000)
	k.SetMimir(ctx, "TorAnchor-BNB-BUSD-BD1", 1) // enable BUSD pool as a TOR anchor

	fee = gasMgr.GetFee(ctx, common.BTCChain, common.BTCAsset)
	c.Assert(fee.Uint64(), Equals, uint64(20000000), Commentf("%d", fee.Uint64()))

	// when network fee is higher than MinimumL1OutboundFeeUSD , then choose network fee
	networkFee = NewNetworkFee(common.BTCChain, 1000, 50000)
	c.Assert(k.SaveNetworkFee(ctx, common.BTCChain, networkFee), IsNil)
	fee = gasMgr.GetFee(ctx, common.BTCChain, common.BTCAsset)
	c.Assert(fee.Uint64(), Equals, uint64(100000000))
}

func (GasManagerTestSuiteV108) TestDifferentValidations(c *C) {
	ctx, mgr := setupManagerForTest(c)
	k := mgr.Keeper()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	gasMgr := newGasMgrV108(constAccessor, k)
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

func (GasManagerTestSuiteV108) TestGetMaxGas(c *C) {
	ctx, k := setupKeeperForTest(c)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	gasMgr := newGasMgrV108(constAccessor, k)
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

func (GasManagerTestSuiteV108) TestOutboundFeeMultiplier(c *C) {
	ctx, k := setupKeeperForTest(c)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	gasMgr := newGasMgrV108(constAccessor, k)

	targetSurplus := cosmos.NewUint(100_00000000) // 100 $RUNE
	minMultiplier := cosmos.NewUint(15_000)
	maxMultiplier := cosmos.NewUint(20_000)
	gasSpent := cosmos.ZeroUint()
	gasWithheld := cosmos.ZeroUint()

	// No surplus to start, should return maxMultiplier
	m := gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, maxMultiplier.Uint64())

	// More gas spent than withheld, use maxMultiplier
	gasSpent = cosmos.NewUint(1000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, maxMultiplier.Uint64())

	gasSpent = cosmos.NewUint(100_00000000)
	gasWithheld = cosmos.NewUint(110_00000000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(19_500), Commentf("%d", m.Uint64()))

	// 50% surplus vs target, reduce multiplier by 50%
	gasSpent = cosmos.NewUint(100_00000000)
	gasWithheld = cosmos.NewUint(150_00000000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(17_500), Commentf("%d", m.Uint64()))

	// 75% surplus vs target, reduce multiplier by 75%
	gasSpent = cosmos.NewUint(100_00000000)
	gasWithheld = cosmos.NewUint(175_00000000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(16_250), Commentf("%d", m.Uint64()))

	// 99% surplus vs target, reduce multiplier by 99%
	gasSpent = cosmos.NewUint(100_00000000)
	gasWithheld = cosmos.NewUint(199_00000000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(15_050), Commentf("%d", m.Uint64()))

	// 100% surplus vs target, reduce multiplier by 100%
	gasSpent = cosmos.NewUint(100_00000000)
	gasWithheld = cosmos.NewUint(200_00000000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(15_000), Commentf("%d", m.Uint64()))

	// 110% surplus vs target, still reduce multiplier by 100%
	gasSpent = cosmos.NewUint(100_00000000)
	gasWithheld = cosmos.NewUint(210_00000000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(15_000))

	// If min multiplier somehow gets set above max multiplier, multiplier should return old default (3x)
	maxMultiplier = cosmos.NewUint(10_000)
	m = gasMgr.CalcOutboundFeeMultiplier(ctx, targetSurplus, gasSpent, gasWithheld, maxMultiplier, minMultiplier)
	c.Assert(m.Uint64(), Equals, uint64(30_000), Commentf("%d", m.Uint64()))
}
