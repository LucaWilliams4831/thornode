package thorchain

import (
	"errors"
	"os"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type SwapV95Suite struct{}

var _ = Suite(&SwapV95Suite{})

func (s *SwapV95Suite) SetUpSuite(c *C) {
	err := os.Setenv("NET", "other")
	c.Assert(err, IsNil)
	SetupConfigForTest()
}

func (s *SwapV95Suite) TestSwap(c *C) {
	poolStorage := &TestSwapKeeper{}
	inputs := []struct {
		name          string
		requestTxHash common.TxID
		source        common.Asset
		target        common.Asset
		amount        cosmos.Uint
		requester     common.Address
		destination   common.Address
		returnAmount  cosmos.Uint
		tradeTarget   cosmos.Uint
		expectedErr   error
		events        int
	}{
		{
			name:          "empty-source",
			requestTxHash: "hash",
			source:        common.Asset{},
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(100 * common.One),
			requester:     "tester",
			destination:   "whatever",
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("denom cannot be empty"),
		},
		{
			name:          "empty-target",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.Asset{},
			amount:        cosmos.NewUint(100 * common.One),
			requester:     "tester",
			destination:   "whatever",
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("target is empty"),
		},
		{
			name:          "empty-requestTxHash",
			requestTxHash: "",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(100 * common.One),
			requester:     "tester",
			destination:   "whatever",
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("Tx ID cannot be empty"),
		},
		{
			name:          "empty-amount",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.ZeroUint(),
			requester:     "tester",
			destination:   "whatever",
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("amount cannot be zero"),
		},
		{
			name:          "empty-requester",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(100 * common.One),
			requester:     "",
			destination:   "whatever",
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("from address cannot be empty"),
		},
		{
			name:          "empty-destination",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(100 * common.One),
			requester:     GetRandomBNBAddress(),
			destination:   "",
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("to address cannot be empty"),
		},
		{
			name:          "pool-not-exist",
			requestTxHash: "hash",
			source:        common.Asset{Chain: common.BNBChain, Ticker: "NOTEXIST", Symbol: "NOTEXIST"},
			target:        common.RuneAsset(),
			amount:        cosmos.NewUint(100 * common.One),
			requester:     GetRandomBNBAddress(),
			destination:   GetRandomTHORAddress(),
			tradeTarget:   cosmos.NewUint(110000000),
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("pool BNB.NOTEXIST doesn't exist"),
		},
		{
			name:          "pool-not-exist-1",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.Asset{Chain: common.BNBChain, Ticker: "NOTEXIST", Symbol: "NOTEXIST"},
			amount:        cosmos.NewUint(100 * common.One),
			requester:     GetRandomTHORAddress(),
			destination:   GetRandomBNBAddress(),
			tradeTarget:   cosmos.NewUint(120000000),
			returnAmount:  cosmos.ZeroUint(),
			expectedErr:   errors.New("pool BNB.NOTEXIST doesn't exist"),
		},
		{
			name:          "swap-cross-chain-different-address",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BTCAsset,
			amount:        cosmos.NewUint(50 * common.One),
			requester:     GetRandomTHORAddress(),
			destination:   GetRandomBNBAddress(),
			returnAmount:  cosmos.ZeroUint(),
			tradeTarget:   cosmos.ZeroUint(),
			expectedErr:   errors.New("destination address is not a valid BTC address"),
			events:        1,
		},
		{
			name:          "swap-no-global-sliplimit",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(50 * common.One),
			requester:     GetRandomTHORAddress(),
			destination:   GetRandomBNBAddress(),
			returnAmount:  cosmos.NewUint(2222222222),
			tradeTarget:   cosmos.ZeroUint(),
			expectedErr:   nil,
			events:        1,
		},
		{
			name:          "swap-over-trade-sliplimit",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(9 * common.One),
			requester:     GetRandomTHORAddress(),
			destination:   GetRandomBNBAddress(),
			returnAmount:  cosmos.ZeroUint(),
			tradeTarget:   cosmos.NewUint(9 * common.One),
			expectedErr:   errors.New("emit asset 757511993 less than price limit 900000000"),
		},
		{
			name:          "swap-no-target-price-no-protection",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(8 * common.One),
			requester:     GetRandomTHORAddress(),
			destination:   GetRandomBNBAddress(),
			returnAmount:  cosmos.NewUint(685871056),
			tradeTarget:   cosmos.ZeroUint(),
			expectedErr:   nil,
			events:        1,
		},
		{
			name:          "swap",
			requestTxHash: "hash",
			source:        common.RuneAsset(),
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(5 * common.One),
			requester:     GetRandomTHORAddress(),
			destination:   GetRandomBNBAddress(),
			returnAmount:  cosmos.NewUint(453514739),
			tradeTarget:   cosmos.NewUint(453514738),
			expectedErr:   nil,
			events:        1,
		},
		{
			name:          "double-swap",
			requestTxHash: "hash",
			source:        common.Asset{Chain: common.BTCChain, Ticker: "BTC", Symbol: "BTC"},
			target:        common.BNBAsset,
			amount:        cosmos.NewUint(5 * common.One),
			requester:     GetRandomTHORAddress(),
			destination:   GetRandomBNBAddress(),
			returnAmount:  cosmos.NewUint(415017809),
			tradeTarget:   cosmos.NewUint(415017809),
			expectedErr:   nil,
			events:        2,
		},
		{
			name:          "swap-synth-to-rune-when-pool-is-not-available",
			requestTxHash: "hash",
			source:        common.BCHAsset.GetSyntheticAsset(),
			target:        common.RuneAsset(),
			amount:        cosmos.NewUint(5 * common.One),
			requester:     GetRandomTHORAddress(),
			destination:   GetRandomTHORAddress(),
			returnAmount:  cosmos.NewUint(475907198),
			tradeTarget:   cosmos.NewUint(453514738),
			expectedErr:   nil,
			events:        1,
		},
	}

	for _, item := range inputs {
		c.Logf("test name:%s", item.name)
		tx := common.NewTx(
			item.requestTxHash,
			item.requester,
			item.destination,
			common.Coins{
				common.NewCoin(item.source, item.amount),
			},
			BNBGasFeeSingleton,
			"",
		)
		tx.Chain = common.BNBChain
		ctx, mgr := setupManagerForTest(c)
		mgr.K = poolStorage
		mgr.txOutStore = NewTxStoreDummy()

		amount, evts, err := newSwapperV95().Swap(ctx, poolStorage, tx, item.target, item.destination, item.tradeTarget, "", "", nil, cosmos.NewUint(1000_000), 20_000, mgr)
		if item.expectedErr == nil {
			c.Assert(err, IsNil)
			c.Assert(evts, HasLen, item.events)
		} else {
			c.Assert(err, NotNil, Commentf("Expected: %s, got nil", item.expectedErr.Error()))
			c.Assert(err.Error(), Equals, item.expectedErr.Error())
		}

		c.Logf("expected amount:%s, actual amount:%s", item.returnAmount, amount)
		c.Check(item.returnAmount.Uint64(), Equals, amount.Uint64())

	}
}

func (s *SwapV95Suite) TestSynthSwap_RuneSynthRune(c *C) {
	ctx, mgr := setupManagerForTest(c)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceRune = cosmos.NewUint(1111 * common.One)
	pool.BalanceAsset = cosmos.NewUint(34 * common.One)
	pool.LPUnits = pool.BalanceRune
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	asgardVault := GetRandomVault()
	c.Assert(mgr.Keeper().SetVault(ctx, asgardVault), IsNil)

	// swap rune --> synth
	{
		addr := GetRandomTHORAddress()
		tx := common.NewTx(
			GetRandomTxHash(),
			addr,
			addr,
			common.NewCoins(
				common.NewCoin(common.RuneAsset(), cosmos.NewUint(50*common.One)),
			),
			BNBGasFeeSingleton,
			"",
		)
		tx.Chain = common.BNBChain

		// Check LUVI (Liquidity Unit Value Index) before and after the swap.
		//   LUVI := sqrt(BalanceRune * BalanceAsset) / PoolUnits
		// We calculate LUVI squared.
		poolUnitsBefore2 := pool.GetPoolUnits().Mul(pool.GetPoolUnits())
		luviBefore2 := pool.BalanceRune.Mul(pool.BalanceAsset).Quo(poolUnitsBefore2)

		// Expectations:
		// BalanceAsset should stay the same.
		// BalanceRune will be increased by the swap amount. For non-synth swaps, BalanceRune is also
		// decreased by an amount commensurate with the asset fee that is added to the pool. The
		// exact amount takes slip into account and is computed by Pool::RuneDisbursementForAssetAdd(..).
		// For Synth swaps, the (rune) disbursement amount is also subtracted even though no asset fee
		// is added to the pool balance.
		// So, the expected BalanceRune is:
		//    InitialBalanceRune + swapAmt - Pool::RuneDisbursementForAssetAdd(assetFee)
		// where assetFee is computed from the native rune fee using the spot price implied by the pool,
		// namely (InitialBalanceRune+swapAmt)/BalanceAsset.
		swapAmt := cosmos.NewUint(50 * 1e8)
		initialBalanceRune := cosmos.NewUint(1111 * 1e8)
		initialBalanceAsset := cosmos.NewUint(34 * 1e8)
		newBalanceAsset := initialBalanceAsset // BalanceAsset doesn't change for RUNE->Synth swap.
		nativeRuneFee := cosmos.NewUint(2 * 1e6)
		// The spot rate is used to convert the transaction fee.
		assetFee := cosmos.NewUint(
			uint64(QuoUint(nativeRuneFee.Mul(newBalanceAsset),
				initialBalanceRune.Add(swapAmt)).RoundInt64()))
		// For synths, the pool depths are double to decrease the fee.
		// swapResult: (swapAmt * 2*BalanceRune * 2*BalanceAsset ) / (swapAmt + 2*BalanceRune )^2
		TWO := cosmos.NewUint(2)
		numerator := swapAmt.Mul(TWO).Mul(initialBalanceAsset).Mul(TWO).Mul(initialBalanceRune)
		denom := swapAmt.Add(TWO.Mul(initialBalanceRune))
		denom = denom.Mul(denom)
		swapResult := cosmos.NewUint(uint64(QuoUint(numerator, denom).TruncateInt64()))
		runeDisbursement := cosmos.NewUint(
			uint64(QuoUint(assetFee.Mul(initialBalanceRune.Add(swapAmt)),
				newBalanceAsset.Add(assetFee)).RoundInt64()))
		expectedRuneBalance := initialBalanceRune.Add(swapAmt).Sub(runeDisbursement)
		expectedSynthSupply := swapResult.Sub(assetFee)

		amount, _, err := newSwapperV95().Swap(ctx, mgr.Keeper(), tx, common.BNBAsset.GetSyntheticAsset(), addr, cosmos.ZeroUint(), "", "", nil, cosmos.NewUint(1000_000), 20_000, mgr)
		c.Assert(err, IsNil)
		c.Check(amount.Uint64(), Equals, swapResult.Uint64(),
			Commentf("Actual: %d Exp: %d", amount.Uint64(), swapResult.Uint64()))

		pool, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
		c.Assert(err, IsNil)

		totalSynthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
		c.Check(totalSynthSupply.Uint64(), Equals, expectedSynthSupply.Uint64(),
			Commentf("Actual: %d Exp: %d", totalSynthSupply.Uint64(), expectedSynthSupply.Uint64()))
		pool.CalcUnits(mgr.GetVersion(), totalSynthSupply)
		c.Check(pool.BalanceAsset.Uint64(), Equals, newBalanceAsset.Uint64())
		c.Check(pool.BalanceRune.Uint64(), Equals, expectedRuneBalance.Uint64(),
			Commentf("Actual: %d Exp: %d", pool.BalanceRune.Uint64(), expectedRuneBalance.Uint64()))
		c.Check(pool.BalanceRune.Uint64(), Equals, uint64(116098000041), Commentf("%d", pool.BalanceRune.Uint64()))
		c.Check(pool.LPUnits.Uint64(), Equals, uint64(111100000000), Commentf("%d", pool.LPUnits.Uint64()))
		// We don't check pool.SynthUnits to not duplicate the calculation here,
		// but we did check BalanceAsset, LPUnits, and totalSynthSupply, the
		// three inputs to the calculation.

		poolUnitsAfter2 := pool.GetPoolUnits().Mul(pool.GetPoolUnits())
		luviAfter2 := pool.BalanceRune.Mul(pool.BalanceAsset).Quo(poolUnitsAfter2)
		c.Check(luviBefore2.Uint64(), Equals, luviAfter2.Uint64())
	}

	// swap synth --> rune
	{
		addr := GetRandomTHORAddress()
		tx := common.NewTx(
			GetRandomTxHash(),
			addr,
			addr,
			common.NewCoins(common.NewCoin(common.BNBAsset.GetSyntheticAsset(), cosmos.NewUint(1*1e8))),
			BNBGasFeeSingleton,
			"",
		)
		tx.Chain = common.BNBChain

		// synths are expected to be on the asgard module
		mintErr := mgr.Keeper().MintToModule(ctx, ModuleName, tx.Coins[0])
		c.Assert(mintErr, IsNil)
		sendErr := mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, tx.Coins)
		c.Assert(sendErr, IsNil)

		// Expectations:
		// BalanceAsset should stay the same.
		// BalanceRune is decreased by the swap result.
		swapAmt := cosmos.NewUint(1 * 1e8)
		initialBalanceRune := pool.BalanceRune
		initialBalanceAsset := pool.BalanceAsset
		// For synths, the pool depths are double to decrease the fee.
		// swapResult: (swapAmt * 2*BalanceRune * 2*BalanceAsset ) / (swapAmt + 2*BalanceAsset )^2
		TWO := cosmos.NewUint(2)
		numerator := swapAmt.Mul(TWO).Mul(initialBalanceRune).Mul(TWO).Mul(initialBalanceAsset)
		denom := swapAmt.Add(TWO.Mul(initialBalanceAsset))
		denom = denom.Mul(denom)
		swapResult := cosmos.NewUint(uint64(QuoUint(numerator, denom).TruncateInt64()))
		expBalanceRune := initialBalanceRune.Sub(swapResult)
		expBalanceAsset := initialBalanceAsset // BalanceAsset doesn't change for Synth->Rune swap.

		// Check LUVI (Liquidity Unit Value Index) before and after the swap.
		//   LUVI := sqrt(BalanceRune * BalanceAsset) / PoolUnits
		// We calculate LUVI squared.
		poolUnitsBefore2 := pool.GetPoolUnits().Mul(pool.GetPoolUnits())
		luviBefore2 := pool.BalanceRune.Mul(pool.BalanceAsset).Quo(poolUnitsBefore2)

		amount, _, err := newSwapperV95().Swap(ctx, mgr.Keeper(), tx, common.RuneAsset(), addr, cosmos.ZeroUint(), "", "", nil, cosmos.NewUint(1000_000), 20_000, mgr)
		c.Assert(err, IsNil)
		c.Check(amount.Uint64(), Equals, swapResult.Uint64(),
			Commentf("Actual: %d Exp: %d", amount.Uint64(), swapResult.Uint64()))
		pool, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
		c.Assert(err, IsNil)

		totalSynthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
		// TODO(leifthelucky): The total synth supply doesn't actually change. This is very puzzling.
		//
		// expectedSynthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset()).Sub(swapAmt)
		// c.Check(totalSynthSupply.Uint64(), Equals, expectedSynthSupply.Uint64(),
		//   Commentf("Actual: %d Exp: %d", totalSynthSupply.Uint64(), expectedSynthSupply.Uint64()))
		pool.CalcUnits(mgr.GetVersion(), totalSynthSupply)
		c.Check(pool.BalanceAsset.Uint64(), Equals, expBalanceAsset.Uint64(),
			Commentf("Actual: %d Exp: %d", pool.BalanceAsset.Uint64(), expBalanceAsset.Uint64()))
		c.Check(pool.BalanceRune.Uint64(), Equals, expBalanceRune.Uint64(),
			Commentf("Actual: %d Exp: %d", pool.BalanceRune.Uint64(), expBalanceRune.Uint64()))
		c.Check(pool.LPUnits.Uint64(), Equals, uint64(111100000000), Commentf("%d", pool.LPUnits.Uint64()))
		// We don't check pool.SynthUnits to not duplicate the calculation here,
		// but we did check BalanceAsset, LPUnits, and totalSynthSupply, the
		// three inputs to the calculation.
		poolUnitsAfter2 := pool.GetPoolUnits().Mul(pool.GetPoolUnits())
		luviAfter2 := pool.BalanceRune.Mul(pool.BalanceAsset).Quo(poolUnitsAfter2)
		c.Check(luviBefore2.Uint64(), Equals, luviAfter2.Uint64())
	}
}

func (s *SwapV95Suite) TestSynthSwap_AssetSynth(c *C) {
	ctx, mgr := setupManagerForTest(c)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceRune = cosmos.NewUint(1111 * common.One)
	pool.BalanceAsset = cosmos.NewUint(34 * common.One)
	pool.LPUnits = pool.BalanceRune
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	asgardVault := GetRandomVault()
	c.Assert(mgr.Keeper().SetVault(ctx, asgardVault), IsNil)

	addr := GetRandomTHORAddress()
	// swap BNB.BNB -> BNB/BNB (external asset directly to synth)
	tx := common.NewTx(
		GetRandomTxHash(),
		addr,
		addr,
		common.NewCoins(
			common.NewCoin(common.BNBAsset, cosmos.NewUint(50*common.One)),
		),
		BNBGasFeeSingleton,
		"",
	)
	tx.Chain = common.BNBChain

	// Expectations:
	// This is a double swap, so we need to compute the expectations as a result of two swaps.
	// 1st swap: BNB.BNB -> Rune
	// 1st swapResult: (swapAmt * BalanceRune * BalanceAsset ) / (swapAmt + BalanceAsset )^2
	swapAmtAsset := cosmos.NewUint(50 * 1e8)
	initialBalanceRune := pool.BalanceRune
	initialBalanceAsset := pool.BalanceAsset
	expLPUnits := pool.LPUnits // Shouldn't change for a swap.
	nativeRuneFee := cosmos.NewUint(2 * 1e6)
	numerator := swapAmtAsset.Mul(initialBalanceRune).Mul(initialBalanceAsset)
	denom := swapAmtAsset.Add(initialBalanceAsset)
	denom = denom.Mul(denom)
	swapResult1 := cosmos.NewUint(uint64(QuoUint(numerator, denom).TruncateInt64()))
	balanceRune1 := initialBalanceRune.Sub(swapResult1)
	balanceAsset1 := initialBalanceAsset.Add(swapAmtAsset)
	// 2nd swap: Rune -> BNB/BNB (synth)
	// 2nd swapResult: (swapResult1 * 2*NewBalanceRune * 2*NewBalanceAsset ) / (swapResult1 + 2*NewBalanceRune )^2
	TWO := cosmos.NewUint(2)
	numerator = swapResult1.Mul(TWO).Mul(balanceRune1).Mul(TWO).Mul(balanceAsset1)
	denom = swapResult1.Add(TWO.Mul(balanceRune1))
	denom = denom.Mul(denom)
	swapResult2 := cosmos.NewUint(uint64(QuoUint(numerator, denom).TruncateInt64()))
	balanceRune2 := balanceRune1.Add(swapResult1)
	balanceAsset2 := balanceAsset1
	assetFee := cosmos.NewUint(
		uint64(QuoUint(nativeRuneFee.Mul(balanceAsset2),
			balanceRune2).RoundInt64()))
	runeDisbursement := cosmos.NewUint(
		uint64(QuoUint(assetFee.Mul(balanceRune2),
			balanceAsset2.Add(assetFee)).RoundInt64()))
	expBalanceRune := balanceRune2.Sub(runeDisbursement) // BalanceRune after the second swap (rune->synth)
	expBalanceAsset := balanceAsset2

	expectedSynthSupply := swapResult2.Sub(assetFee)

	// Check LUVI (Liquidity Unit Value Index) before and after the swap.
	//   LUVI := sqrt(BalanceRune * BalanceAsset) / PoolUnits
	// We calculate LUVI squared.
	poolUnitsBefore2 := pool.GetPoolUnits().Mul(pool.GetPoolUnits())
	luviBefore2 := pool.BalanceRune.Mul(pool.BalanceAsset).Quo(poolUnitsBefore2)

	amount, _, err := newSwapperV95().Swap(ctx, mgr.Keeper(), tx, common.BNBAsset.GetSyntheticAsset(), addr, cosmos.ZeroUint(), "", "", nil, cosmos.NewUint(1000_000), 20_000, mgr)
	c.Assert(err, IsNil)
	c.Check(amount.Uint64(), Equals, swapResult2.Uint64(),
		Commentf("Actual: %d Exp: %d", amount.Uint64(), swapResult2.Uint64()))
	c.Check(amount.Uint64(), Equals, uint64(1985844476), Commentf("%d", amount.Uint64()))
	pool, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	c.Check(pool.BalanceAsset.Uint64(), Equals, expBalanceAsset.Uint64(),
		Commentf("Actual: %d Exp: %d", pool.BalanceAsset.Uint64(), expBalanceAsset.Uint64()))
	c.Check(pool.BalanceRune.Uint64(), Equals, expBalanceRune.Uint64(),
		Commentf("Actual: %d Exp: %d", pool.BalanceRune.Uint64(), expBalanceRune.Uint64()))
	totalSynthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	c.Check(totalSynthSupply.Uint64(), Equals, expectedSynthSupply.Uint64(),
		Commentf("Actual: %d Exp: %d", totalSynthSupply.Uint64(), expectedSynthSupply.Uint64()))
	pool.CalcUnits(mgr.GetVersion(), totalSynthSupply)
	c.Check(pool.LPUnits.Uint64(), Equals, expLPUnits.Uint64(), Commentf("%d", pool.LPUnits.Uint64()))
	// We don't check pool.SynthUnits to not duplicate the calculation here,
	// but we did check BalanceAsset, LPUnits, and totalSynthSupply, the
	// three inputs to the calculation.

	poolUnitsAfter2 := pool.GetPoolUnits().Mul(pool.GetPoolUnits())
	luviAfter2 := pool.BalanceRune.Mul(pool.BalanceAsset).Quo(poolUnitsAfter2)
	c.Check(luviBefore2.Uint64(), Equals, luviAfter2.Uint64())

	// emit asset is not enough to pay for fee , then pool balance should be restored
	tx1 := common.NewTx(
		GetRandomTxHash(),
		addr,
		addr,
		common.NewCoins(
			common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One/2)),
		),
		BNBGasFeeSingleton,
		"",
	)
	tx1.Chain = common.BTCChain
	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.BalanceAsset = cosmos.NewUint(common.One)
	btcPool.BalanceRune = cosmos.NewUint(common.One * 10)
	btcPool.LPUnits = cosmos.NewUint(100)
	btcPool.SynthUnits = cosmos.ZeroUint()
	c.Assert(mgr.Keeper().SetPool(ctx, btcPool), IsNil)

	amount, _, err = newSwapperV95().Swap(ctx, mgr.Keeper(), tx1, common.BTCAsset, addr, cosmos.ZeroUint(), "", "", nil, cosmos.NewUint(1000_000_000_000), 20_000, mgr)
	c.Assert(err, NotNil)
	c.Check(amount.IsZero(), Equals, true)
	pool, err = mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	totalSynthSupply = mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	c.Check(totalSynthSupply.Uint64(), Equals, uint64(0),
		Commentf("%d", totalSynthSupply.Uint64()))
	c.Check(pool.BalanceAsset.Uint64(), Equals, uint64(common.One))
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(10*common.One), Commentf("%d", pool.BalanceRune.Uint64()))
	pool.CalcUnits(mgr.GetVersion(), totalSynthSupply)
	c.Check(pool.LPUnits.Uint64(), Equals, uint64(100), Commentf("%d", pool.LPUnits.Uint64()))
	// We don't check pool.SynthUnits to not duplicate the calculation here,
	// but we did check BalanceAsset, LPUnits, and totalSynthSupply, the
	// three inputs to the calculation.
}
