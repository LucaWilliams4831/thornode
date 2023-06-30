package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

func withdraw(ctx cosmos.Context, msg MsgWithdrawLiquidity, mgr Manager) (cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, error) {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.102.0")):
		return withdrawV102(ctx, msg, mgr)
	case version.GTE(semver.MustParse("1.98.0")):
		return withdrawV98(ctx, msg, mgr)
	case version.GTE(semver.MustParse("1.91.0")):
		return withdrawV91(ctx, msg, mgr)
	case version.GTE(semver.MustParse("1.89.0")):
		return withdrawV89(ctx, msg, mgr)
	case version.GTE(semver.MustParse("1.84.0")):
		return withdrawV84(ctx, msg, mgr)
	case version.GTE(semver.MustParse("0.76.0")):
		return withdrawV76(ctx, msg, mgr)
	}
	zero := cosmos.ZeroUint()
	return zero, zero, zero, zero, zero, errInvalidVersion
}

// Performs the withdraw for the provided MsgWithdrawLiquidity message.
// Returns: runeAmt, assetAmount, protectionRuneAmt, units, lastWithdraw, err
func withdrawV102(ctx cosmos.Context, msg MsgWithdrawLiquidity, mgr Manager) (cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint, error) {
	if err := validateWithdrawV1(ctx, mgr.Keeper(), msg); err != nil {
		ctx.Logger().Error("msg withdraw failed validation", "error", err)
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), err
	}

	pool, err := mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		ctx.Logger().Error("failed to get pool", "error", err)
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), err
	}
	synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	pool.CalcUnits(mgr.GetVersion(), synthSupply)

	lp, err := mgr.Keeper().GetLiquidityProvider(ctx, msg.Asset, msg.WithdrawAddress)
	if err != nil {
		ctx.Logger().Error("failed to find liquidity provider", "error", err)
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), err

	}

	poolRune := pool.BalanceRune
	poolAsset := pool.BalanceAsset
	originalLiquidityProviderUnits := lp.Units
	fLiquidityProviderUnit := lp.Units
	if lp.Units.IsZero() {
		if !lp.PendingRune.IsZero() || !lp.PendingAsset.IsZero() {
			mgr.Keeper().RemoveLiquidityProvider(ctx, lp)
			pool.PendingInboundRune = common.SafeSub(pool.PendingInboundRune, lp.PendingRune)
			pool.PendingInboundAsset = common.SafeSub(pool.PendingInboundAsset, lp.PendingAsset)
			if err := mgr.Keeper().SetPool(ctx, pool); err != nil {
				ctx.Logger().Error("failed to save pool pending inbound funds", "error", err)
			}
			// remove lp

			return lp.PendingRune, cosmos.RoundToDecimal(lp.PendingAsset, pool.Decimals), cosmos.ZeroUint(), lp.Units, cosmos.ZeroUint(), nil
		}
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errNoLiquidityUnitLeft
	}

	// fail if the last add height less than the lockup period in the past
	cv := mgr.GetConstants()
	height := ctx.BlockHeight()
	lockupBlocks := mgr.Keeper().GetConfigInt64(ctx, constants.LiquidityLockUpBlocks)
	if height < (lp.LastAddHeight + lockupBlocks) {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errWithdrawLockup
	}

	ctx.Logger().Info("pool before withdraw", "pool units", pool.GetPoolUnits(), "balance RUNE", poolRune, "balance asset", poolAsset)
	ctx.Logger().Info("liquidity provider before withdraw", "liquidity provider unit", fLiquidityProviderUnit)

	pauseAsym, _ := mgr.Keeper().GetMimir(ctx, fmt.Sprintf("PauseAsymWithdrawal-%s", pool.Asset.GetChain()))
	assetToWithdraw := assetToWithdrawV89(msg, lp, pauseAsym)

	if pool.Status == PoolAvailable && lp.RuneDepositValue.IsZero() && lp.AssetDepositValue.IsZero() {
		lp.RuneDepositValue = lp.RuneDepositValue.Add(common.GetSafeShare(lp.Units, pool.GetPoolUnits(), pool.BalanceRune))
		lp.AssetDepositValue = lp.AssetDepositValue.Add(common.GetSafeShare(lp.Units, pool.GetPoolUnits(), pool.BalanceAsset))
	}

	// calculate any impermanent loss or not
	protectionRuneAmount := cosmos.ZeroUint()
	extraUnits := cosmos.ZeroUint()
	fullProtectionLine, err := mgr.Keeper().GetMimir(ctx, constants.FullImpLossProtectionBlocks.String())
	if fullProtectionLine < 0 || err != nil {
		fullProtectionLine = cv.GetInt64Value(constants.FullImpLossProtectionBlocks)
	}
	ilpPoolMimirKey := fmt.Sprintf("ILP-DISABLED-%s", pool.Asset)
	ilpDisabled, err := mgr.Keeper().GetMimir(ctx, ilpPoolMimirKey)
	if err != nil {
		ctx.Logger().Error("fail to get ILP-DISABLED mimir", "error", err, "key", ilpPoolMimirKey)
		ilpDisabled = 0
	}
	ilpCutoff := mgr.Keeper().GetConfigInt64(ctx, constants.ILPCutoff)

	if (ilpCutoff <= 0 || ilpCutoff > lp.LastAddHeight) && // ilp cutoff must be after the last add height
		fullProtectionLine > 0 && // full protection line must be greater than 0
		pool.Status == PoolAvailable && // pool must be available
		!(ilpDisabled > 0 && !pool.Asset.IsVaultAsset()) { // ilp must not be disabled for this pool

		lastAddHeight := lp.LastAddHeight
		if lastAddHeight < pool.StatusSince {
			lastAddHeight = pool.StatusSince
		}
		protectionBasisPoints := calcImpLossProtectionAmtV1(ctx, lastAddHeight, fullProtectionLine)
		implProtectionRuneAmount, depositValue, redeemValue := calcImpLossV91(lp, msg.BasisPoints, protectionBasisPoints, pool)
		ctx.Logger().Info("imp loss calculation", "deposit value", depositValue, "redeem value", redeemValue, "protection", implProtectionRuneAmount)
		if !implProtectionRuneAmount.IsZero() {
			protectionRuneAmount = implProtectionRuneAmount
			_, extraUnits, err = calculatePoolUnitsV1(pool.GetPoolUnits(), poolRune, poolAsset, implProtectionRuneAmount, cosmos.ZeroUint())
			if err != nil {
				return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), err
			}
			ctx.Logger().Info("liquidity provider granted imp loss protection", "extra provider units", extraUnits, "extra rune", implProtectionRuneAmount)
			poolRune = poolRune.Add(implProtectionRuneAmount)
			fLiquidityProviderUnit = fLiquidityProviderUnit.Add(extraUnits)
			pool.LPUnits = pool.LPUnits.Add(extraUnits)
		}
	}

	var withdrawRune, withDrawAsset, unitAfter cosmos.Uint
	if pool.Asset.IsVaultAsset() {
		withdrawRune, withDrawAsset, unitAfter = calculateVaultWithdrawV1(pool.GetPoolUnits(), poolAsset, originalLiquidityProviderUnits, msg.BasisPoints)
	} else {
		withdrawRune, withDrawAsset, unitAfter, err = calculateWithdrawV76(pool.GetPoolUnits(), poolRune, poolAsset, originalLiquidityProviderUnits, extraUnits, msg.BasisPoints, assetToWithdraw)
		if err != nil {
			ctx.Logger().Error("fail to withdraw", "error", err)
			return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errWithdrawFail
		}
	}
	if !pool.Asset.IsVaultAsset() {
		if (withdrawRune.Equal(poolRune) && !withDrawAsset.Equal(poolAsset)) || (!withdrawRune.Equal(poolRune) && withDrawAsset.Equal(poolAsset)) {
			ctx.Logger().Error("fail to withdraw: cannot withdraw 100% of only one side of the pool")
			return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errWithdrawFail
		}
	}
	withDrawAsset = cosmos.RoundToDecimal(withDrawAsset, pool.Decimals)
	gasAsset := cosmos.ZeroUint()
	// If the pool is empty, and there is a gas asset, subtract required gas
	if common.SafeSub(pool.GetPoolUnits(), fLiquidityProviderUnit).Add(unitAfter).IsZero() {
		maxGas, err := mgr.GasMgr().GetMaxGas(ctx, pool.Asset.GetChain())
		if err != nil {
			ctx.Logger().Error("fail to get gas for asset", "asset", pool.Asset, "error", err)
			return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errWithdrawFail
		}
		// minus gas costs for our transactions
		// TODO: chain specific logic should be in a single location
		if pool.Asset.IsBNB() && !common.RuneAsset().Chain.Equals(common.THORChain) {
			originalAsset := withDrawAsset
			withDrawAsset = common.SafeSub(
				withDrawAsset,
				maxGas.Amount.MulUint64(2), // RUNE asset is on binance chain
			)
			gasAsset = originalAsset.Sub(withDrawAsset)
		} else if pool.Asset.GetChain().GetGasAsset().Equals(pool.Asset) {
			gasAsset = maxGas.Amount
			if gasAsset.GT(withDrawAsset) {
				gasAsset = withDrawAsset
			}
			withDrawAsset = common.SafeSub(withDrawAsset, gasAsset)
		}
	}

	ctx.Logger().Info("client withdraw", "RUNE", withdrawRune, "asset", withDrawAsset, "units left", unitAfter)
	// update pool
	pool.LPUnits = common.SafeSub(pool.LPUnits, common.SafeSub(fLiquidityProviderUnit, unitAfter))
	pool.BalanceRune = common.SafeSub(poolRune, withdrawRune)
	pool.BalanceAsset = common.SafeSub(poolAsset, withDrawAsset)

	ctx.Logger().Info("pool after withdraw", "pool unit", pool.GetPoolUnits(), "balance RUNE", pool.BalanceRune, "balance asset", pool.BalanceAsset)

	lp.LastWithdrawHeight = ctx.BlockHeight()
	maxPts := cosmos.NewUint(uint64(MaxWithdrawBasisPoints))
	lp.RuneDepositValue = common.SafeSub(lp.RuneDepositValue, common.GetSafeShare(msg.BasisPoints, maxPts, lp.RuneDepositValue))
	lp.AssetDepositValue = common.SafeSub(lp.AssetDepositValue, common.GetSafeShare(msg.BasisPoints, maxPts, lp.AssetDepositValue))
	lp.Units = unitAfter

	// sanity check, we don't increase LP units
	if unitAfter.GTE(originalLiquidityProviderUnits) {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), ErrInternal(err, fmt.Sprintf("sanity check: LP units cannot increase during a withdrawal: %d --> %d", originalLiquidityProviderUnits.Uint64(), unitAfter.Uint64()))
	}

	// Create a pool event if THORNode have no rune or assets
	if (pool.BalanceAsset.IsZero() || pool.BalanceRune.IsZero()) && !pool.Asset.IsVaultAsset() {
		poolEvt := NewEventPool(pool.Asset, PoolStaged)
		if err := mgr.EventMgr().EmitEvent(ctx, poolEvt); nil != err {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
		pool.Status = PoolStaged
	}

	if err := mgr.Keeper().SetPool(ctx, pool); err != nil {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), ErrInternal(err, "fail to save pool")
	}
	if mgr.Keeper().RagnarokInProgress(ctx) {
		mgr.Keeper().SetLiquidityProvider(ctx, lp)
	} else {
		if !lp.Units.Add(lp.PendingAsset).Add(lp.PendingRune).IsZero() {
			mgr.Keeper().SetLiquidityProvider(ctx, lp)
		} else {
			mgr.Keeper().RemoveLiquidityProvider(ctx, lp)
		}
	}
	// add rune from the reserve to the asgard module, to cover imp loss protection
	if !protectionRuneAmount.IsZero() {
		err := mgr.Keeper().SendFromModuleToModule(ctx, ReserveName, AsgardName, common.NewCoins(common.NewCoin(common.RuneAsset(), protectionRuneAmount)))
		if err != nil {
			return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), ErrInternal(err, "fail to move imp loss protection rune from the reserve to asgard")
		}
	}
	return withdrawRune, withDrawAsset, protectionRuneAmount, common.SafeSub(originalLiquidityProviderUnits, unitAfter), gasAsset, nil
}

// calcImpLossV91 if there needs to add some imp loss protection, in rune
func calcImpLossV91(lp LiquidityProvider, withdrawBasisPoints cosmos.Uint, protectionBasisPoints int64, pool Pool) (cosmos.Uint, cosmos.Uint, cosmos.Uint) {
	/*
		A0 = assetDepositValue; R0 = runeDepositValue;

		liquidityUnits = units the member wishes to redeem after applying withdrawBasisPoints
		A1 = GetUncappedShare(liquidityUnits, lpUnits, assetDepth);
		R1 = GetUncappedShare(liquidityUnits, lpUnits, runeDepth);
		P1 = R1/A1
		coverage = ((A0 * P1) + R0) - ((A1 * P1) + R1) => ((A0 * R1/A1) + R0) - (R1 + R1)
	*/
	A0 := lp.AssetDepositValue
	R0 := lp.RuneDepositValue
	poolUnits := pool.GetPoolUnits()
	A1 := common.GetSafeShare(lp.Units, poolUnits, pool.BalanceAsset)
	R1 := common.GetSafeShare(lp.Units, poolUnits, pool.BalanceRune)
	if A1.IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint()
	}
	depositValue := A0.Mul(R1).Quo(A1).Add(R0)
	redeemValue := R1.Add(R1)
	coverage := common.SafeSub(depositValue, redeemValue)

	// taking withdrawBasisPoints, calculate how much of the coverage the user should receives
	coverage = common.GetSafeShare(withdrawBasisPoints, cosmos.NewUint(10000), coverage)

	// taking protection basis points, calculate how much of the coverage the user actually receives
	result := coverage.MulUint64(uint64(protectionBasisPoints)).QuoUint64(10000)
	return result, depositValue, redeemValue
}

func assetToWithdrawV89(msg MsgWithdrawLiquidity, lp LiquidityProvider, pauseAsym int64) common.Asset {
	if lp.RuneAddress.IsEmpty() {
		return msg.Asset
	}
	if lp.AssetAddress.IsEmpty() {
		return common.RuneAsset()
	}
	if pauseAsym > 0 {
		return common.EmptyAsset
	}

	return msg.WithdrawalAsset
}

func calculateWithdrawV76(poolUnits, poolRune, poolAsset, lpUnits, extraUnits, withdrawBasisPoints cosmos.Uint, withdrawalAsset common.Asset) (cosmos.Uint, cosmos.Uint, cosmos.Uint, error) {
	if poolUnits.IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("poolUnits can't be zero")
	}
	if poolRune.IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("pool rune balance can't be zero")
	}
	if poolAsset.IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("pool asset balance can't be zero")
	}
	if lpUnits.IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("liquidity provider unit can't be zero")
	}
	if withdrawBasisPoints.GT(cosmos.NewUint(MaxWithdrawBasisPoints)) {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint(), fmt.Errorf("withdraw basis point %s is not valid", withdrawBasisPoints.String())
	}

	unitsToClaim := common.GetSafeShare(withdrawBasisPoints, cosmos.NewUint(10000), lpUnits)
	unitAfter := common.SafeSub(lpUnits, unitsToClaim)
	unitsToClaim = unitsToClaim.Add(extraUnits)
	if withdrawalAsset.IsEmpty() {
		withdrawRune := common.GetSafeShare(unitsToClaim, poolUnits, poolRune)
		withdrawAsset := common.GetSafeShare(unitsToClaim, poolUnits, poolAsset)
		return withdrawRune, withdrawAsset, unitAfter, nil
	}
	if withdrawalAsset.IsRune() {
		return calcAsymWithdrawalV1(unitsToClaim, poolUnits, poolRune), cosmos.ZeroUint(), unitAfter, nil
	}
	return cosmos.ZeroUint(), calcAsymWithdrawalV1(unitsToClaim, poolUnits, poolAsset), unitAfter, nil
}

func calculateVaultWithdrawV1(vaultUnits, assetAmt, lpUnits, withdrawBasisPoints cosmos.Uint) (cosmos.Uint, cosmos.Uint, cosmos.Uint) {
	if vaultUnits.IsZero() || lpUnits.IsZero() || assetAmt.IsZero() || withdrawBasisPoints.IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), cosmos.ZeroUint()
	}

	unitsToClaim := common.GetSafeShare(withdrawBasisPoints, cosmos.NewUint(MaxWithdrawBasisPoints), lpUnits)
	unitAfter := common.SafeSub(lpUnits, unitsToClaim)
	withdrawAsset := common.GetSafeShare(unitsToClaim, vaultUnits, assetAmt)
	return cosmos.ZeroUint(), withdrawAsset, unitAfter
}

func validateWithdrawV1(ctx cosmos.Context, keeper keeper.Keeper, msg MsgWithdrawLiquidity) error {
	if msg.WithdrawAddress.IsEmpty() {
		return errors.New("empty withdraw address")
	}
	if msg.Tx.ID.IsEmpty() {
		return errors.New("request tx hash is empty")
	}
	if msg.Asset.IsEmpty() {
		return errors.New("empty asset")
	}
	withdrawBasisPoints := msg.BasisPoints
	// when BasisPoints is zero, it will be override in parse memo, so if a message can get here
	// the witdrawBasisPoints must between 1~MaxWithdrawBasisPoints
	if !withdrawBasisPoints.GT(cosmos.ZeroUint()) || withdrawBasisPoints.GT(cosmos.NewUint(MaxWithdrawBasisPoints)) {
		return fmt.Errorf("withdraw basis points %s is invalid", msg.BasisPoints)
	}
	if !keeper.PoolExist(ctx, msg.Asset) {
		// pool doesn't exist
		return fmt.Errorf("pool-%s doesn't exist", msg.Asset)
	}
	return nil
}

func calcAsymWithdrawalV1(s, t, a cosmos.Uint) cosmos.Uint {
	// share = (s * A * (2 * T^2 - 2 * T * s + s^2))/T^3
	// s = liquidity provider units for member (after factoring in withdrawBasisPoints)
	// T = totalPoolUnits for pool
	// A = assetDepth to be withdrawn
	// (part1 * (part2 - part3 + part4)) / part5
	part1 := s.Mul(a)
	part2 := t.Mul(t).MulUint64(2)
	part3 := t.Mul(s).MulUint64(2)
	part4 := s.Mul(s)
	numerator := part1.Mul(common.SafeSub(part2, part3).Add(part4))
	part5 := t.Mul(t).Mul(t)
	return numerator.Quo(part5)
}

// calculate percentage (in basis points) of the amount of impermanent loss protection
func calcImpLossProtectionAmtV1(ctx cosmos.Context, lastDepositHeight, target int64) int64 {
	age := ctx.BlockHeight() - lastDepositHeight
	if age < ILPMinimumBlocks {
		return 0
	}
	if age >= target {
		return 10000
	}
	return (age * 10000) / target
}
