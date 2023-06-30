package thorchain

import (
	"errors"
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h AddLiquidityHandler) validateV110(ctx cosmos.Context, msg MsgAddLiquidity) error {
	if err := msg.ValidateBasicV98(); err != nil {
		ctx.Logger().Error(err.Error())
		return errAddLiquidityFailValidation
	}

	// TODO on hard fork move network check to ValidateBasic
	if !msg.AssetAddress.IsEmpty() {
		if !common.CurrentChainNetwork.SoftEquals(msg.AssetAddress.GetNetwork(h.mgr.GetVersion(), msg.AssetAddress.GetChain())) {
			return fmt.Errorf("address(%s) is not same network", msg.AssetAddress)
		}
	}

	// The Ragnarok key for the TERRA.LUNA pool would be RAGNAROK-TERRA-LUNA .
	k := "RAGNAROK-" + msg.Asset.MimirString()
	v, err := h.mgr.Keeper().GetMimir(ctx, k)
	if err != nil {
		ctx.Logger().Error("fail to get mimir value", "mimir", k, "error", err)
	}
	if v >= 1 {
		return fmt.Errorf("cannot add liquidity to Ragnaroked pool (%s)", msg.Asset.String())
	}

	// Note that GetChain() without GetLayer1Asset() would indicate THORChain for synthetic assets.
	gasAsset := msg.Asset.GetLayer1Asset().GetChain().GetGasAsset()
	// Even if a destination gas asset pool is empty, the first add liquidity has to be symmetrical,
	// and so there is no need to check at this stage for whether the addition is of RUNE or Asset or with needsSwap.
	if !msg.Asset.Equals(gasAsset) {
		gasPool, err := h.mgr.Keeper().GetPool(ctx, gasAsset)
		// Note that for a synthetic asset msg.Asset.Chain (unlike msg.Asset.GetChain())
		// is intentionally used to be the external chain rather than THOR.
		// Any destination asset starting with THOR should be rejected for no THOR.RUNE
		// gas asset pool existing.
		if err != nil {
			return ErrInternal(err, "fail to get gas pool")
		}
		// Note that NewPool from GetPool would return a pool with status;
		// use IsEmpty to check for prior existence.
		if gasPool.IsEmpty() {
			return fmt.Errorf("asset (%s)'s gas asset pool (%s) does not exist yet", msg.Asset.String(), gasAsset.String())
		}
	}

	if msg.Asset.IsDerivedAsset() {
		return fmt.Errorf("asset cannot be a derived asset")
	}

	if msg.Asset.IsVaultAsset() {
		if !msg.Asset.GetLayer1Asset().IsGasAsset() {
			return fmt.Errorf("asset must be a gas asset for the layer1 protocol")
		}
		if !msg.AssetAddress.IsChain(msg.Asset.GetLayer1Asset().GetChain()) {
			return fmt.Errorf("asset address must be layer1 chain")
		}
		if !msg.RuneAmount.IsZero() {
			return fmt.Errorf("cannot deposit rune into a vault")
		}
	}

	if !msg.RuneAddress.IsEmpty() && !msg.RuneAddress.IsChain(common.THORChain) {
		ctx.Logger().Error("rune address must be THORChain")
		return errAddLiquidityFailValidation
	}

	if !msg.AssetAddress.IsEmpty() {
		polAddress, err := h.mgr.Keeper().GetModuleAddress(ReserveName)
		if err != nil {
			return err
		}
		if msg.RuneAddress.Equals(polAddress) {
			return fmt.Errorf("pol lp cannot have asset address")
		}
	}

	// check if swap meets standards
	if h.needsSwap(msg) {
		if !msg.Asset.IsVaultAsset() {
			return fmt.Errorf("swap & add liquidity is only available for synthetic pools")
		}
		if !msg.Asset.GetLayer1Asset().Equals(msg.Tx.Coins[0].Asset) {
			return fmt.Errorf("deposit asset must be the layer1 equivalent for the synthetic asset")
		}
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	if err := pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	// if the pool is THORChain no need to check economic security
	if msg.Asset.IsVaultAsset() || !ensureLiquidityNoLargerThanBond {
		return nil
	}

	// the following  only applicable for chaosnet
	totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity RUNE")
	}

	// total liquidity RUNE after current add liquidity
	totalLiquidityRUNE = totalLiquidityRUNE.Add(msg.RuneAmount)
	totalLiquidityRUNE = totalLiquidityRUNE.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityRune.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityRune)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	if !ensureLiquidityNoLargerThanBond {
		return nil
	}
	securityBond, err := h.getEffectiveSecurityBond(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get security bond RUNE")
	}
	if totalLiquidityRUNE.GT(securityBond) {
		ctx.Logger().Info("total liquidity RUNE is more than effective security bond", "rune", totalLiquidityRUNE.String(), "bond", securityBond.String())
		return errAddLiquidityRUNEMoreThanBond
	}

	return nil
}

func (h AddLiquidityHandler) validateV98(ctx cosmos.Context, msg MsgAddLiquidity) error {
	if err := msg.ValidateBasicV98(); err != nil {
		ctx.Logger().Error(err.Error())
		return errAddLiquidityFailValidation
	}

	if msg.Asset.IsVaultAsset() {
		if !msg.Asset.GetLayer1Asset().IsGasAsset() {
			return fmt.Errorf("asset must be a gas asset for the layer1 protocol")
		}
		if !msg.AssetAddress.IsChain(msg.Asset.GetLayer1Asset().GetChain()) {
			return fmt.Errorf("asset address must be layer1 chain")
		}
		if !msg.RuneAmount.IsZero() {
			return fmt.Errorf("cannot deposit rune into a vault")
		}
	}

	if !msg.RuneAddress.IsEmpty() && !msg.RuneAddress.IsChain(common.THORChain) {
		ctx.Logger().Error("rune address must be THORChain")
		return errAddLiquidityFailValidation
	}

	if !msg.AssetAddress.IsEmpty() {
		polAddress, err := h.mgr.Keeper().GetModuleAddress(ReserveName)
		if err != nil {
			return err
		}
		if msg.RuneAddress.Equals(polAddress) {
			return fmt.Errorf("pol lp cannot have asset address")
		}
	}

	// check if swap meets standards
	if h.needsSwap(msg) {
		if !msg.Asset.IsVaultAsset() {
			return fmt.Errorf("swap & add liquidity is only available for synthetic pools")
		}
		if !msg.Asset.GetLayer1Asset().Equals(msg.Tx.Coins[0].Asset) {
			return fmt.Errorf("deposit asset must be the layer1 equivalent for the synthetic asset")
		}
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	if err := pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	// if the pool is THORChain no need to check economic security
	if msg.Asset.IsVaultAsset() || !ensureLiquidityNoLargerThanBond {
		return nil
	}

	// the following  only applicable for chaosnet
	totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity RUNE")
	}

	// total liquidity RUNE after current add liquidity
	totalLiquidityRUNE = totalLiquidityRUNE.Add(msg.RuneAmount)
	totalLiquidityRUNE = totalLiquidityRUNE.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityRune.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityRune)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	if !ensureLiquidityNoLargerThanBond {
		return nil
	}
	securityBond, err := h.getEffectiveSecurityBond(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get security bond RUNE")
	}
	if totalLiquidityRUNE.GT(securityBond) {
		ctx.Logger().Info("total liquidity RUNE is more than effective security bond", "rune", totalLiquidityRUNE.String(), "bond", securityBond.String())
		return errAddLiquidityRUNEMoreThanBond
	}

	return nil
}

func (h AddLiquidityHandler) validateV96(ctx cosmos.Context, msg MsgAddLiquidity) error {
	if err := msg.ValidateBasicV93(); err != nil {
		ctx.Logger().Error(err.Error())
		return errAddLiquidityFailValidation
	}

	if msg.Asset.IsVaultAsset() {
		if !msg.Asset.GetLayer1Asset().IsGasAsset() {
			return fmt.Errorf("asset must be a gas asset for the layer1 protocol")
		}
		if !msg.AssetAddress.IsChain(common.THORChain) {
			return fmt.Errorf("asset address must be a thor address")
		}
		if !msg.RuneAmount.IsZero() {
			return fmt.Errorf("cannot deposit rune into a vault")
		}
	}

	if !msg.RuneAddress.IsEmpty() && !msg.RuneAddress.IsChain(common.THORChain) {
		ctx.Logger().Error("rune address must be THORChain")
		return errAddLiquidityFailValidation
	}

	// check if swap meets standards
	if h.needsSwap(msg) {
		if !msg.Asset.IsVaultAsset() {
			return fmt.Errorf("swap & add liquidity is only available for synthetic pools")
		}
		if !msg.Asset.GetLayer1Asset().Equals(msg.Tx.Coins[0].Asset) {
			return fmt.Errorf("deposit asset must be the layer1 equivalent for the synthetic asset")
		}
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	if err := pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	// if the pool is THORChain no need to check economic security
	if msg.Asset.IsVaultAsset() || !ensureLiquidityNoLargerThanBond {
		return nil
	}

	// the following  only applicable for chaosnet
	totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity RUNE")
	}

	// total liquidity RUNE after current add liquidity
	totalLiquidityRUNE = totalLiquidityRUNE.Add(msg.RuneAmount)
	totalLiquidityRUNE = totalLiquidityRUNE.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityRune.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityRune)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	if !ensureLiquidityNoLargerThanBond {
		return nil
	}
	securityBond, err := h.getEffectiveSecurityBond(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get security bond RUNE")
	}
	if totalLiquidityRUNE.GT(securityBond) {
		ctx.Logger().Info("total liquidity RUNE is more than effective security bond", "rune", totalLiquidityRUNE.String(), "bond", securityBond.String())
		return errAddLiquidityRUNEMoreThanBond
	}

	return nil
}

func (h AddLiquidityHandler) validateV95(ctx cosmos.Context, msg MsgAddLiquidity) error {
	if err := msg.ValidateBasicV93(); err != nil {
		ctx.Logger().Error(err.Error())
		return errAddLiquidityFailValidation
	}

	if msg.Asset.IsSyntheticAsset() {
		ctx.Logger().Error("asset cannot be synth", "error", errAddLiquidityFailValidation)
		return errAddLiquidityFailValidation
	}

	if !msg.RuneAddress.IsEmpty() && !msg.RuneAddress.IsChain(common.THORChain) {
		ctx.Logger().Error("rune address must be THORChain")
		return errAddLiquidityFailValidation
	}

	// check if swap meets standards
	if h.needsSwap(msg) {
		if !msg.Asset.IsSyntheticAsset() {
			return fmt.Errorf("swap & add liquidity is only available for synthetic pools")
		}
		if !msg.Asset.GetLayer1Asset().Equals(msg.Tx.Coins[0].Asset) {
			return fmt.Errorf("deposit asset must be the layer1 equivalent for the synthetic asset")
		}
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	if err := pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	// the following  only applicable for chaosnet
	totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity RUNE")
	}

	// total liquidity RUNE after current add liquidity
	totalLiquidityRUNE = totalLiquidityRUNE.Add(msg.RuneAmount)
	totalLiquidityRUNE = totalLiquidityRUNE.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityRune.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityRune)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	if !ensureLiquidityNoLargerThanBond {
		return nil
	}
	securityBond, err := h.getEffectiveSecurityBond(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get security bond RUNE")
	}
	if totalLiquidityRUNE.GT(securityBond) {
		ctx.Logger().Info("total liquidity RUNE is more than effective security bond", "rune", totalLiquidityRUNE.String(), "bond", securityBond.String())
		return errAddLiquidityRUNEMoreThanBond
	}

	return nil
}

func (h AddLiquidityHandler) validateV93(ctx cosmos.Context, msg MsgAddLiquidity) error {
	if err := msg.ValidateBasicV93(); err != nil {
		ctx.Logger().Error(err.Error())
		return errAddLiquidityFailValidation
	}

	if msg.Asset.IsSyntheticAsset() {
		ctx.Logger().Error("asset cannot be synth", "error", errAddLiquidityFailValidation)
		return errAddLiquidityFailValidation
	}

	if !msg.RuneAddress.IsEmpty() && !msg.RuneAddress.IsChain(common.THORChain) {
		ctx.Logger().Error("rune address must be THORChain")
		return errAddLiquidityFailValidation
	}

	// check if swap meets standards
	if h.needsSwap(msg) {
		if !msg.Asset.IsSyntheticAsset() {
			return fmt.Errorf("swap & add liquidity is only available for synthetic pools")
		}
		if !msg.Asset.GetLayer1Asset().Equals(msg.Tx.Coins[0].Asset) {
			return fmt.Errorf("deposit asset must be the layer1 equivalent for the synthetic asset")
		}
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	if err := pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	// the following  only applicable for chaosnet
	totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity RUNE")
	}

	// total liquidity RUNE after current add liquidity
	totalLiquidityRUNE = totalLiquidityRUNE.Add(msg.RuneAmount)
	totalLiquidityRUNE = totalLiquidityRUNE.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityRune.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityRune)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	if !ensureLiquidityNoLargerThanBond {
		return nil
	}
	totalBondRune, err := h.getTotalActiveBond(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total bond RUNE")
	}
	if totalLiquidityRUNE.GT(totalBondRune) {
		ctx.Logger().Info("total liquidity RUNE is more than total Bond", "rune", totalLiquidityRUNE.String(), "bond", totalBondRune.String())
		return errAddLiquidityRUNEMoreThanBond
	}

	return nil
}

func (h AddLiquidityHandler) validateV76(ctx cosmos.Context, msg MsgAddLiquidity) error {
	if err := msg.ValidateBasicV63(); err != nil {
		ctx.Logger().Error(err.Error())
		return errAddLiquidityFailValidation
	}

	if msg.Asset.IsSyntheticAsset() {
		ctx.Logger().Error("asset cannot be synth", "error", errAddLiquidityFailValidation)
		return errAddLiquidityFailValidation
	}

	// Synths coins are not compatible with add liquidity
	if msg.Tx.Coins.HasSynthetic() {
		ctx.Logger().Error("asset coins cannot be synth", "error", errAddLiquidityFailValidation)
		return errAddLiquidityFailValidation
	}

	if !msg.AssetAddress.IsEmpty() && !msg.AssetAddress.IsChain(msg.Asset.Chain) {
		ctx.Logger().Error("asset address must match asset chain")
		return errAddLiquidityFailValidation
	}

	if !msg.RuneAddress.IsEmpty() && !msg.RuneAddress.IsChain(common.THORChain) {
		ctx.Logger().Error("rune address must be THORChain")
		return errAddLiquidityFailValidation
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	// the following  only applicable for chaosnet
	totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity RUNE")
	}

	// total liquidity RUNE after current add liquidity
	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	totalLiquidityRUNE = totalLiquidityRUNE.Add(msg.RuneAmount)
	totalLiquidityRUNE = totalLiquidityRUNE.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityRune.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityRune)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	if !ensureLiquidityNoLargerThanBond {
		return nil
	}
	totalBondRune, err := h.getTotalActiveBond(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total bond RUNE")
	}
	if totalLiquidityRUNE.GT(totalBondRune) {
		ctx.Logger().Info("total liquidity RUNE is more than total Bond", "rune", totalLiquidityRUNE.String(), "bond", totalBondRune.String())
		return errAddLiquidityRUNEMoreThanBond
	}

	return nil
}

func (h AddLiquidityHandler) handleV98(ctx cosmos.Context, msg MsgAddLiquidity) (errResult error) {
	// check if we need to swap before adding asset
	if h.needsSwap(msg) {
		return h.swapV93(ctx, msg)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}

	if pool.IsEmpty() {
		ctx.Logger().Info("pool doesn't exist yet, creating a new one...", "symbol", msg.Asset.String(), "creator", msg.RuneAddress)
		pool.Asset = msg.Asset
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			return ErrInternal(err, "fail to save pool to key value store")
		}
	}

	// if the pool decimals hasn't been set, it will still be 0. If we have a
	// pool asset coin, get the decimals from that transaction. This will only
	// set the decimals once.
	if pool.Decimals == 0 {
		coin := msg.GetTx().Coins.GetCoin(pool.Asset)
		if !coin.IsEmpty() {
			if coin.Decimals > 0 {
				pool.Decimals = coin.Decimals
			}
			ctx.Logger().Info("try update pool decimals", "asset", msg.Asset, "pool decimals", pool.Decimals)
			if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
				return ErrInternal(err, "fail to save pool to key value store")
			}
		}
	}

	// figure out if we need to stage the funds and wait for a follow on
	// transaction to commit all funds atomically. For pools of native assets
	// only, stage is always false
	stage := false
	if !msg.Asset.IsVaultAsset() {
		if !msg.AssetAddress.IsEmpty() && msg.AssetAmount.IsZero() {
			stage = true
		}
		if !msg.RuneAddress.IsEmpty() && msg.RuneAmount.IsZero() {
			stage = true
		}
	}

	if msg.AffiliateBasisPoints.IsZero() {
		return h.addLiquidity(
			ctx,
			msg.Asset,
			msg.RuneAmount,
			msg.AssetAmount,
			msg.RuneAddress,
			msg.AssetAddress,
			msg.Tx.ID,
			stage,
			h.mgr.GetConstants())
	}

	// add liquidity has an affiliate fee, add liquidity for both the user and their affiliate
	affiliateRune := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.RuneAmount)
	affiliateAsset := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.AssetAmount)
	userRune := common.SafeSub(msg.RuneAmount, affiliateRune)
	userAsset := common.SafeSub(msg.AssetAmount, affiliateAsset)

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		userRune,
		userAsset,
		msg.RuneAddress,
		msg.AssetAddress,
		msg.Tx.ID,
		stage,
		h.mgr.GetConstants(),
	)
	if err != nil {
		return err
	}

	affiliateRuneAddress := common.NoAddress
	affiliateAssetAddress := common.NoAddress
	if msg.AffiliateAddress.IsChain(common.THORChain) {
		affiliateRuneAddress = msg.AffiliateAddress
	} else {
		affiliateAssetAddress = msg.AffiliateAddress
	}

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		affiliateRune,
		affiliateAsset,
		affiliateRuneAddress,
		affiliateAssetAddress,
		msg.Tx.ID,
		false,
		h.mgr.GetConstants(),
	)
	if err != nil {
		ctx.Logger().Error("fail to add liquidity for affiliate", "address", msg.AffiliateAddress, "error", err)
		return err
	}
	return nil
}

func (h AddLiquidityHandler) handleV96(ctx cosmos.Context, msg MsgAddLiquidity) (errResult error) {
	// check if we need to swap before adding asset
	if h.needsSwap(msg) {
		return h.swapV93(ctx, msg)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}

	if pool.IsEmpty() {
		ctx.Logger().Info("pool doesn't exist yet, creating a new one...", "symbol", msg.Asset.String(), "creator", msg.RuneAddress)
		pool.Asset = msg.Asset
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			return ErrInternal(err, "fail to save pool to key value store")
		}
	}

	// if the pool decimals hasn't been set, it will still be 0. If we have a
	// pool asset coin, get the decimals from that transaction. This will only
	// set the decimals once.
	if pool.Decimals == 0 {
		coin := msg.GetTx().Coins.GetCoin(pool.Asset)
		if !coin.IsEmpty() {
			if coin.Decimals > 0 {
				pool.Decimals = coin.Decimals
			}
			ctx.Logger().Info("try update pool decimals", "asset", msg.Asset, "pool decimals", pool.Decimals)
			if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
				return ErrInternal(err, "fail to save pool to key value store")
			}
		}
	}

	// figure out if we need to stage the funds and wait for a follow on
	// transaction to commit all funds atomically. For pools of native assets
	// only, stage is always false
	stage := false
	if !msg.Asset.IsVaultAsset() {
		if !msg.AssetAddress.IsEmpty() && msg.AssetAmount.IsZero() {
			stage = true
		}
		if !msg.RuneAddress.IsEmpty() && msg.RuneAmount.IsZero() {
			stage = true
		}
	}

	if msg.AffiliateBasisPoints.IsZero() {
		return h.addLiquidity(
			ctx,
			msg.Asset,
			msg.RuneAmount,
			msg.AssetAmount,
			msg.RuneAddress,
			msg.AssetAddress,
			msg.Tx.ID,
			stage,
			h.mgr.GetConstants())
	}

	// add liquidity has an affiliate fee, add liquidity for both the user and their affiliate
	affiliateRune := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.RuneAmount)
	affiliateAsset := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.AssetAmount)
	userRune := common.SafeSub(msg.RuneAmount, affiliateRune)
	userAsset := common.SafeSub(msg.AssetAmount, affiliateAsset)

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		userRune,
		userAsset,
		msg.RuneAddress,
		msg.AssetAddress,
		msg.Tx.ID,
		stage,
		h.mgr.GetConstants(),
	)
	if err != nil {
		return err
	}

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		affiliateRune,
		affiliateAsset,
		msg.AffiliateAddress,
		common.NoAddress,
		msg.Tx.ID,
		stage,
		h.mgr.GetConstants(),
	)
	if err != nil {
		// we swallow this error so we don't trigger a refund, when we've
		// already successfully added liquidity for the user. If we were to
		// refund here, funds could be leaked from the network. In order, to
		// error here, we would need to revert the user addLiquidity
		// function first (TODO).
		ctx.Logger().Error("fail to add liquidity for affiliate", "address", msg.AffiliateAddress, "error", err)
	}
	return nil
}

func (h AddLiquidityHandler) handleV93(ctx cosmos.Context, msg MsgAddLiquidity) (errResult error) {
	// check if we need to swap before adding asset
	if h.needsSwap(msg) {
		return h.swapV93(ctx, msg)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}

	if pool.IsEmpty() {
		ctx.Logger().Info("pool doesn't exist yet, creating a new one...", "symbol", msg.Asset.String(), "creator", msg.RuneAddress)
		pool.Asset = msg.Asset
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			return ErrInternal(err, "fail to save pool to key value store")
		}
	}

	// if the pool decimals hasn't been set, it will still be 0. If we have a
	// pool asset coin, get the decimals from that transaction. This will only
	// set the decimals once.
	if pool.Decimals == 0 {
		coin := msg.GetTx().Coins.GetCoin(pool.Asset)
		if !coin.IsEmpty() {
			if coin.Decimals > 0 {
				pool.Decimals = coin.Decimals
			}
			ctx.Logger().Info("try update pool decimals", "asset", msg.Asset, "pool decimals", pool.Decimals)
			if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
				return ErrInternal(err, "fail to save pool to key value store")
			}
		}
	}

	// figure out if we need to stage the funds and wait for a follow on
	// transaction to commit all funds atomically
	stage := false
	if !msg.AssetAddress.IsEmpty() && msg.AssetAmount.IsZero() {
		stage = true
	}
	if !msg.RuneAddress.IsEmpty() && msg.RuneAmount.IsZero() {
		stage = true
	}

	if msg.AffiliateBasisPoints.IsZero() {
		return h.addLiquidity(
			ctx,
			msg.Asset,
			msg.RuneAmount,
			msg.AssetAmount,
			msg.RuneAddress,
			msg.AssetAddress,
			msg.Tx.ID,
			stage,
			h.mgr.GetConstants())
	}

	// add liquidity has an affiliate fee, add liquidity for both the user and their affiliate
	affiliateRune := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.RuneAmount)
	affiliateAsset := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.AssetAmount)
	userRune := common.SafeSub(msg.RuneAmount, affiliateRune)
	userAsset := common.SafeSub(msg.AssetAmount, affiliateAsset)

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		userRune,
		userAsset,
		msg.RuneAddress,
		msg.AssetAddress,
		msg.Tx.ID,
		stage,
		h.mgr.GetConstants(),
	)
	if err != nil {
		return err
	}

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		affiliateRune,
		affiliateAsset,
		msg.AffiliateAddress,
		common.NoAddress,
		msg.Tx.ID,
		stage,
		h.mgr.GetConstants(),
	)
	if err != nil {
		// we swallow this error so we don't trigger a refund, when we've
		// already successfully added liquidity for the user. If we were to
		// refund here, funds could be leaked from the network. In order, to
		// error here, we would need to revert the user addLiquidity
		// function first (TODO).
		ctx.Logger().Error("fail to add liquidity for affiliate", "address", msg.AffiliateAddress, "error", err)
	}
	return nil
}

func (h AddLiquidityHandler) handleV63(ctx cosmos.Context, msg MsgAddLiquidity) (errResult error) {
	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}

	if pool.IsEmpty() {
		ctx.Logger().Info("pool doesn't exist yet, creating a new one...", "symbol", msg.Asset.String(), "creator", msg.RuneAddress)
		pool.Asset = msg.Asset
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			return ErrInternal(err, "fail to save pool to key value store")
		}
	}

	// if the pool decimals hasn't been set, it will still be 0. If we have a
	// pool asset coin, get the decimals from that transaction. This will only
	// set the decimals once.
	if pool.Decimals == 0 {
		coin := msg.GetTx().Coins.GetCoin(pool.Asset)
		if !coin.IsEmpty() {
			if coin.Decimals > 0 {
				pool.Decimals = coin.Decimals
			}
			ctx.Logger().Info("try update pool decimals", "asset", msg.Asset, "pool decimals", pool.Decimals)
			if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
				return ErrInternal(err, "fail to save pool to key value store")
			}
		}
	}

	if err := pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	// figure out if we need to stage the funds and wait for a follow on
	// transaction to commit all funds atomically
	stage := false
	if !msg.AssetAddress.IsEmpty() && msg.AssetAmount.IsZero() {
		stage = true
	}
	if !msg.RuneAddress.IsEmpty() && msg.RuneAmount.IsZero() {
		stage = true
	}

	if msg.AffiliateBasisPoints.IsZero() {
		return h.addLiquidity(
			ctx,
			msg.Asset,
			msg.RuneAmount,
			msg.AssetAmount,
			msg.RuneAddress,
			msg.AssetAddress,
			msg.Tx.ID,
			stage,
			h.mgr.GetConstants())
	}

	// add liquidity has an affiliate fee, add liquidity for both the user and their affiliate
	affiliateRune := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.RuneAmount)
	affiliateAsset := common.GetSafeShare(msg.AffiliateBasisPoints, cosmos.NewUint(10000), msg.AssetAmount)
	userRune := common.SafeSub(msg.RuneAmount, affiliateRune)
	userAsset := common.SafeSub(msg.AssetAmount, affiliateAsset)

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		userRune,
		userAsset,
		msg.RuneAddress,
		msg.AssetAddress,
		msg.Tx.ID,
		stage,
		h.mgr.GetConstants(),
	)
	if err != nil {
		return err
	}

	err = h.addLiquidity(
		ctx,
		msg.Asset,
		affiliateRune,
		affiliateAsset,
		msg.AffiliateAddress,
		common.NoAddress,
		msg.Tx.ID,
		stage,
		h.mgr.GetConstants(),
	)
	if err != nil {
		// we swallow this error so we don't trigger a refund, when we've
		// already successfully added liquidity for the user. If we were to
		// refund here, funds could be leaked from the network. In order, to
		// error here, we would need to revert the user addLiquidity
		// function first (TODO).
		ctx.Logger().Error("fail to add liquidity for affiliate", "address", msg.AffiliateAddress, "error", err)
	}
	return nil
}

// r = rune provided;
// a = asset provided
// R = rune Balance (before)
// A = asset Balance (before)
// P = existing Pool Units
// slipAdjustment = (1 - ABS((R a - r A)/((r + R) (a + A))))
// units = ((P (a R + A r))/(2 A R))*slidAdjustment
func calculatePoolUnitsV1(oldPoolUnits, poolRune, poolAsset, addRune, addAsset cosmos.Uint) (cosmos.Uint, cosmos.Uint, error) {
	if addRune.Add(poolRune).IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("total RUNE in the pool is zero")
	}
	if addAsset.Add(poolAsset).IsZero() {
		return cosmos.ZeroUint(), cosmos.ZeroUint(), errors.New("total asset in the pool is zero")
	}
	if poolRune.IsZero() || poolAsset.IsZero() {
		return addRune, addRune, nil
	}
	P := cosmos.NewDecFromBigInt(oldPoolUnits.BigInt())
	R := cosmos.NewDecFromBigInt(poolRune.BigInt())
	A := cosmos.NewDecFromBigInt(poolAsset.BigInt())
	r := cosmos.NewDecFromBigInt(addRune.BigInt())
	a := cosmos.NewDecFromBigInt(addAsset.BigInt())

	// (r + R) (a + A)
	slipAdjDenominator := (r.Add(R)).Mul(a.Add(A))
	// ABS((R a - r A)/((r + R) (a + A)))
	var slipAdjustment cosmos.Dec
	if R.Mul(a).GT(r.Mul(A)) {
		slipAdjustment = R.Mul(a).Sub(r.Mul(A)).Quo(slipAdjDenominator)
	} else {
		slipAdjustment = r.Mul(A).Sub(R.Mul(a)).Quo(slipAdjDenominator)
	}
	// (1 - ABS((R a - r A)/((r + R) (a + A))))
	slipAdjustment = cosmos.NewDec(1).Sub(slipAdjustment)

	// (P (a R + A r))
	numerator := P.Mul(a.Mul(R).Add(A.Mul(r)))
	// 2AR
	denominator := cosmos.NewDec(2).Mul(A).Mul(R)
	liquidityUnits := numerator.Quo(denominator).Mul(slipAdjustment)
	newPoolUnit := P.Add(liquidityUnits)

	pUnits := cosmos.NewUintFromBigInt(newPoolUnit.TruncateInt().BigInt())
	sUnits := cosmos.NewUintFromBigInt(liquidityUnits.TruncateInt().BigInt())

	return pUnits, sUnits, nil
}

func (h AddLiquidityHandler) addLiquidityV98(ctx cosmos.Context,
	asset common.Asset,
	addRuneAmount, addAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	requestTxHash common.TxID,
	stage bool,
	constAccessor constants.ConstantValues,
) (err error) {
	ctx.Logger().Info("liquidity provision", "asset", asset, "rune amount", addRuneAmount, "asset amount", addAssetAmount)
	if err := h.validateAddLiquidityMessage(ctx, h.mgr.Keeper(), asset, requestTxHash, runeAddr, assetAddr); err != nil {
		return fmt.Errorf("add liquidity message fail validation: %w", err)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	synthSupply := h.mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	originalUnits := pool.CalcUnits(h.mgr.GetVersion(), synthSupply)

	// if THORNode have no balance, set the default pool status
	if originalUnits.IsZero() {
		defaultPoolStatus := PoolAvailable.String()
		// if the pools is for gas asset on the chain, automatically enable it
		if !pool.Asset.Equals(pool.Asset.GetChain().GetGasAsset()) &&
			!pool.Asset.IsVaultAsset() {
			defaultPoolStatus = constAccessor.GetStringValue(constants.DefaultPoolStatus)
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)
	}

	fetchAddr := runeAddr
	if fetchAddr.IsEmpty() {
		fetchAddr = assetAddr
	}
	su, err := h.mgr.Keeper().GetLiquidityProvider(ctx, asset, fetchAddr)
	if err != nil {
		return ErrInternal(err, "fail to get liquidity provider")
	}

	su.LastAddHeight = ctx.BlockHeight()
	if su.Units.IsZero() {
		if su.PendingTxID.IsEmpty() {
			if su.RuneAddress.IsEmpty() {
				su.RuneAddress = runeAddr
			}
			if su.AssetAddress.IsEmpty() {
				su.AssetAddress = assetAddr
			}
		}

		if asset.IsVaultAsset() {
			// new SU, by default, places the thor address to the rune address,
			// but here we want it to be on the asset address only
			su.AssetAddress = assetAddr
			su.RuneAddress = common.NoAddress // no rune to add/withdraw
		} else {
			// ensure input addresses match LP position addresses
			if !runeAddr.Equals(su.RuneAddress) {
				return errAddLiquidityMismatchAddr
			}
			if !assetAddr.Equals(su.AssetAddress) {
				return errAddLiquidityMismatchAddr
			}
		}
	}

	if asset.IsVaultAsset() {
		if su.AssetAddress.IsEmpty() || !su.AssetAddress.IsChain(asset.GetLayer1Asset().GetChain()) {
			return errAddLiquidityMismatchAddr
		}
	} else if !assetAddr.IsEmpty() && !su.AssetAddress.Equals(assetAddr) {
		// mismatch of asset addresses from what is known to the address
		// given. Refund it.
		return errAddLiquidityMismatchAddr
	}

	// get tx hashes
	runeTxID := requestTxHash
	assetTxID := requestTxHash
	if addRuneAmount.IsZero() {
		runeTxID = su.PendingTxID
	} else {
		assetTxID = su.PendingTxID
	}

	pendingRuneAmt := su.PendingRune.Add(addRuneAmount)
	pendingAssetAmt := su.PendingAsset.Add(addAssetAmount)

	// if we have an asset address and no asset amount, put the rune pending
	if stage && pendingAssetAmt.IsZero() {
		pool.PendingInboundRune = pool.PendingInboundRune.Add(addRuneAmount)
		su.PendingRune = pendingRuneAmt
		su.PendingTxID = requestTxHash
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound rune", "error", err)
		}

		// add pending liquidity event
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.RuneAddress, addRuneAmount, su.AssetAddress, cosmos.ZeroUint(), requestTxHash, common.TxID(""))
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	// if we have a rune address and no rune asset, put the asset in pending
	if stage && pendingRuneAmt.IsZero() {
		pool.PendingInboundAsset = pool.PendingInboundAsset.Add(addAssetAmount)
		su.PendingAsset = pendingAssetAmt
		su.PendingTxID = requestTxHash
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound asset", "error", err)
		}
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.RuneAddress, cosmos.ZeroUint(), su.AssetAddress, addAssetAmount, common.TxID(""), requestTxHash)
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	pool.PendingInboundRune = common.SafeSub(pool.PendingInboundRune, su.PendingRune)
	pool.PendingInboundAsset = common.SafeSub(pool.PendingInboundAsset, su.PendingAsset)
	su.PendingAsset = cosmos.ZeroUint()
	su.PendingRune = cosmos.ZeroUint()
	su.PendingTxID = ""

	ctx.Logger().Info("pre add liquidity", "pool", pool.Asset, "rune", pool.BalanceRune, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits)
	ctx.Logger().Info("adding liquidity", "rune", addRuneAmount, "asset", addAssetAmount)

	balanceRune := pool.BalanceRune
	balanceAsset := pool.BalanceAsset

	oldPoolUnits := pool.GetPoolUnits()
	var newPoolUnits, liquidityUnits cosmos.Uint
	if asset.IsVaultAsset() {
		pendingRuneAmt = cosmos.ZeroUint() // sanity check
		newPoolUnits, liquidityUnits = calculateVaultUnitsV1(oldPoolUnits, balanceAsset, pendingAssetAmt)
	} else {
		newPoolUnits, liquidityUnits, err = h.calculatePoolUnits(oldPoolUnits, balanceRune, balanceAsset, pendingRuneAmt, pendingAssetAmt)
		if err != nil {
			return ErrInternal(err, "fail to calculate pool unit")
		}
	}

	ctx.Logger().Info("current pool status", "pool units", newPoolUnits, "liquidity units", liquidityUnits)
	poolRune := balanceRune.Add(pendingRuneAmt)
	poolAsset := balanceAsset.Add(pendingAssetAmt)
	pool.LPUnits = pool.LPUnits.Add(liquidityUnits)
	pool.BalanceRune = poolRune
	pool.BalanceAsset = poolAsset
	ctx.Logger().Info("post add liquidity", "pool", pool.Asset, "rune", pool.BalanceRune, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits, "add liquidity units", liquidityUnits)
	if (pool.BalanceRune.IsZero() && !asset.IsVaultAsset()) || pool.BalanceAsset.IsZero() {
		return ErrInternal(err, "pool cannot have zero rune or asset balance")
	}
	if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
		return ErrInternal(err, "fail to save pool")
	}
	if originalUnits.IsZero() && !pool.GetPoolUnits().IsZero() {
		poolEvent := NewEventPool(pool.Asset, pool.Status)
		if err := h.mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
	}

	su.Units = su.Units.Add(liquidityUnits)
	if pool.Status == PoolAvailable {
		if su.AssetDepositValue.IsZero() && su.RuneDepositValue.IsZero() {
			su.RuneDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceRune)
			su.AssetDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceAsset)
		} else {
			su.RuneDepositValue = su.RuneDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceRune))
			su.AssetDepositValue = su.AssetDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceAsset))
		}
	}
	h.mgr.Keeper().SetLiquidityProvider(ctx, su)

	evt := NewEventAddLiquidity(asset, liquidityUnits, su.RuneAddress, pendingRuneAmt, pendingAssetAmt, runeTxID, assetTxID, su.AssetAddress)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
		return ErrInternal(err, "fail to emit add liquidity event")
	}

	// if its the POL is adding, track rune added
	polAddress, err := h.mgr.Keeper().GetModuleAddress(ReserveName)
	if err != nil {
		return err
	}

	if polAddress.Equals(su.RuneAddress) {
		pol, err := h.mgr.Keeper().GetPOL(ctx)
		if err != nil {
			return err
		}
		pol.RuneDeposited = pol.RuneDeposited.Add(pendingRuneAmt)

		if err := h.mgr.Keeper().SetPOL(ctx, pol); err != nil {
			return err
		}

		ctx.Logger().Info("POL deposit", "pool", pool.Asset, "rune", pendingRuneAmt)
		telemetry.IncrCounterWithLabels(
			[]string{"thornode", "pol", "pool", "rune_deposited"},
			telem(pendingRuneAmt),
			[]metrics.Label{telemetry.NewLabel("pool", pool.Asset.String())},
		)
	}
	return nil
}

func (h AddLiquidityHandler) addLiquidityV96(ctx cosmos.Context,
	asset common.Asset,
	addRuneAmount, addAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	requestTxHash common.TxID,
	stage bool,
	constAccessor constants.ConstantValues,
) (err error) {
	ctx.Logger().Info("liquidity provision", "asset", asset, "rune amount", addRuneAmount, "asset amount", addAssetAmount)
	if err := h.validateAddLiquidityMessage(ctx, h.mgr.Keeper(), asset, requestTxHash, runeAddr, assetAddr); err != nil {
		return fmt.Errorf("add liquidity message fail validation: %w", err)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	synthSupply := h.mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	originalUnits := pool.CalcUnits(h.mgr.GetVersion(), synthSupply)

	// if THORNode have no balance, set the default pool status
	if originalUnits.IsZero() {
		defaultPoolStatus := PoolAvailable.String()
		// if the pools is for gas asset on the chain, automatically enable it
		if !pool.Asset.Equals(pool.Asset.GetChain().GetGasAsset()) {
			defaultPoolStatus = constAccessor.GetStringValue(constants.DefaultPoolStatus)
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)
	}

	fetchAddr := runeAddr
	if fetchAddr.IsEmpty() {
		fetchAddr = assetAddr
	}
	su, err := h.mgr.Keeper().GetLiquidityProvider(ctx, asset, fetchAddr)
	if err != nil {
		return ErrInternal(err, "fail to get liquidity provider")
	}

	su.LastAddHeight = ctx.BlockHeight()
	if su.Units.IsZero() {
		if su.PendingTxID.IsEmpty() {
			if su.RuneAddress.IsEmpty() {
				su.RuneAddress = runeAddr
			}
			if su.AssetAddress.IsEmpty() {
				su.AssetAddress = assetAddr
			}
		}

		if asset.IsVaultAsset() {
			// new SU, by default, places the thor address to the rune address,
			// but here we want it to be on the asset address only
			su.AssetAddress = assetAddr
			su.RuneAddress = common.NoAddress // no rune to add/withdraw
		} else {
			// ensure input addresses match LP position addresses
			if !runeAddr.Equals(su.RuneAddress) {
				return errAddLiquidityMismatchAddr
			}
			if !assetAddr.Equals(su.AssetAddress) {
				return errAddLiquidityMismatchAddr
			}
		}
	}

	if !assetAddr.IsEmpty() && !su.AssetAddress.Equals(assetAddr) && !asset.IsVaultAsset() {
		// mismatch of asset addresses from what is known to the address
		// given. Refund it.
		return errAddLiquidityMismatchAddr
	}

	// get tx hashes
	runeTxID := requestTxHash
	assetTxID := requestTxHash
	if addRuneAmount.IsZero() {
		runeTxID = su.PendingTxID
	} else {
		assetTxID = su.PendingTxID
	}

	pendingRuneAmt := su.PendingRune.Add(addRuneAmount)
	pendingAssetAmt := su.PendingAsset.Add(addAssetAmount)

	// if we have an asset address and no asset amount, put the rune pending
	if stage && pendingAssetAmt.IsZero() {
		pool.PendingInboundRune = pool.PendingInboundRune.Add(addRuneAmount)
		su.PendingRune = pendingRuneAmt
		su.PendingTxID = requestTxHash
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound rune", "error", err)
		}

		// add pending liquidity event
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.RuneAddress, addRuneAmount, su.AssetAddress, cosmos.ZeroUint(), requestTxHash, common.TxID(""))
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	// if we have a rune address and no rune asset, put the asset in pending
	if stage && pendingRuneAmt.IsZero() {
		pool.PendingInboundAsset = pool.PendingInboundAsset.Add(addAssetAmount)
		su.PendingAsset = pendingAssetAmt
		su.PendingTxID = requestTxHash
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound asset", "error", err)
		}
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.RuneAddress, cosmos.ZeroUint(), su.AssetAddress, addAssetAmount, common.TxID(""), requestTxHash)
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	pool.PendingInboundRune = common.SafeSub(pool.PendingInboundRune, su.PendingRune)
	pool.PendingInboundAsset = common.SafeSub(pool.PendingInboundAsset, su.PendingAsset)
	su.PendingAsset = cosmos.ZeroUint()
	su.PendingRune = cosmos.ZeroUint()
	su.PendingTxID = ""

	ctx.Logger().Info("pre add liquidity", "pool", pool.Asset, "rune", pool.BalanceRune, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits)
	ctx.Logger().Info("adding liquidity", "rune", addRuneAmount, "asset", addAssetAmount)

	balanceRune := pool.BalanceRune
	balanceAsset := pool.BalanceAsset

	oldPoolUnits := pool.GetPoolUnits()
	var newPoolUnits, liquidityUnits cosmos.Uint
	if asset.IsVaultAsset() {
		pendingRuneAmt = cosmos.ZeroUint() // sanity check
		newPoolUnits, liquidityUnits = calculateVaultUnitsV1(oldPoolUnits, balanceAsset, pendingAssetAmt)
	} else {
		newPoolUnits, liquidityUnits, err = calculatePoolUnitsV1(oldPoolUnits, balanceRune, balanceAsset, pendingRuneAmt, pendingAssetAmt)
		if err != nil {
			return ErrInternal(err, "fail to calculate pool unit")
		}
	}

	ctx.Logger().Info("current pool status", "pool units", newPoolUnits, "liquidity units", liquidityUnits)
	poolRune := balanceRune.Add(pendingRuneAmt)
	poolAsset := balanceAsset.Add(pendingAssetAmt)
	pool.LPUnits = pool.LPUnits.Add(liquidityUnits)
	pool.BalanceRune = poolRune
	pool.BalanceAsset = poolAsset
	ctx.Logger().Info("post add liquidity", "pool", pool.Asset, "rune", pool.BalanceRune, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits, "add liquidity units", liquidityUnits)
	if (pool.BalanceRune.IsZero() && !asset.IsVaultAsset()) || pool.BalanceAsset.IsZero() {
		return ErrInternal(err, "pool cannot have zero rune or asset balance")
	}
	if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
		return ErrInternal(err, "fail to save pool")
	}
	if originalUnits.IsZero() && !pool.GetPoolUnits().IsZero() {
		poolEvent := NewEventPool(pool.Asset, pool.Status)
		if err := h.mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
	}

	su.Units = su.Units.Add(liquidityUnits)
	if pool.Status == PoolAvailable {
		if su.AssetDepositValue.IsZero() && su.RuneDepositValue.IsZero() {
			su.RuneDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceRune)
			su.AssetDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceAsset)
		} else {
			su.RuneDepositValue = su.RuneDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceRune))
			su.AssetDepositValue = su.AssetDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceAsset))
		}
	}
	h.mgr.Keeper().SetLiquidityProvider(ctx, su)

	evt := NewEventAddLiquidity(asset, liquidityUnits, su.RuneAddress, pendingRuneAmt, pendingAssetAmt, runeTxID, assetTxID, su.AssetAddress)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
		return ErrInternal(err, "fail to emit add liquidity event")
	}

	// if its the POL is adding, track rune added
	polAddress, err := h.mgr.Keeper().GetModuleAddress(ReserveName)
	if err != nil {
		return err
	}

	if polAddress.Equals(su.RuneAddress) {
		pol, err := h.mgr.Keeper().GetPOL(ctx)
		if err != nil {
			return err
		}
		pol.RuneDeposited = pol.RuneDeposited.Add(pendingRuneAmt)

		if err := h.mgr.Keeper().SetPOL(ctx, pol); err != nil {
			return err
		}

		ctx.Logger().Info("POL deposit", "pool", pool.Asset, "rune", pendingRuneAmt)
		telemetry.IncrCounterWithLabels(
			[]string{"thornode", "pol", "pool", "rune_deposited"},
			telem(pendingRuneAmt),
			[]metrics.Label{telemetry.NewLabel("pool", pool.Asset.String())},
		)
	}
	return nil
}

func (h AddLiquidityHandler) addLiquidityV95(ctx cosmos.Context,
	asset common.Asset,
	addRuneAmount, addAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	requestTxHash common.TxID,
	stage bool,
	constAccessor constants.ConstantValues,
) (err error) {
	ctx.Logger().Info("liquidity provision", "asset", asset, "rune amount", addRuneAmount, "asset amount", addAssetAmount)
	if err := h.validateAddLiquidityMessage(ctx, h.mgr.Keeper(), asset, requestTxHash, runeAddr, assetAddr); err != nil {
		return fmt.Errorf("add liquidity message fail validation: %w", err)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	synthSupply := h.mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	originalUnits := pool.CalcUnits(h.mgr.GetVersion(), synthSupply)

	// if THORNode have no balance, set the default pool status
	if originalUnits.IsZero() {
		defaultPoolStatus := PoolAvailable.String()
		// if the pools is for gas asset on the chain, automatically enable it
		if !pool.Asset.Equals(pool.Asset.GetChain().GetGasAsset()) {
			defaultPoolStatus = constAccessor.GetStringValue(constants.DefaultPoolStatus)
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)
	}

	fetchAddr := runeAddr
	if fetchAddr.IsEmpty() {
		fetchAddr = assetAddr
	}
	su, err := h.mgr.Keeper().GetLiquidityProvider(ctx, asset, fetchAddr)
	if err != nil {
		return ErrInternal(err, "fail to get liquidity provider")
	}

	su.LastAddHeight = ctx.BlockHeight()
	if su.Units.IsZero() {
		if su.PendingTxID.IsEmpty() {
			if su.RuneAddress.IsEmpty() {
				su.RuneAddress = runeAddr
			}
			if su.AssetAddress.IsEmpty() {
				su.AssetAddress = assetAddr
			}
		}

		// ensure input addresses match LP position addresses
		if !runeAddr.Equals(su.RuneAddress) {
			return errAddLiquidityMismatchAddr
		}
		if !assetAddr.Equals(su.AssetAddress) {
			return errAddLiquidityMismatchAddr
		}
	}

	if !assetAddr.IsEmpty() && !su.AssetAddress.Equals(assetAddr) {
		// mismatch of asset addresses from what is known to the address
		// given. Refund it.
		return errAddLiquidityMismatchAddr
	}

	// get tx hashes
	runeTxID := requestTxHash
	assetTxID := requestTxHash
	if addRuneAmount.IsZero() {
		runeTxID = su.PendingTxID
	} else {
		assetTxID = su.PendingTxID
	}

	pendingRuneAmt := su.PendingRune.Add(addRuneAmount)
	pendingAssetAmt := su.PendingAsset.Add(addAssetAmount)

	// if we have an asset address and no asset amount, put the rune pending
	if stage && pendingAssetAmt.IsZero() {
		pool.PendingInboundRune = pool.PendingInboundRune.Add(addRuneAmount)
		su.PendingRune = pendingRuneAmt
		su.PendingTxID = requestTxHash
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound rune", "error", err)
		}

		// add pending liquidity event
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.RuneAddress, addRuneAmount, su.AssetAddress, cosmos.ZeroUint(), requestTxHash, common.TxID(""))
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	// if we have a rune address and no rune asset, put the asset in pending
	if stage && pendingRuneAmt.IsZero() {
		pool.PendingInboundAsset = pool.PendingInboundAsset.Add(addAssetAmount)
		su.PendingAsset = pendingAssetAmt
		su.PendingTxID = requestTxHash
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound asset", "error", err)
		}
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.RuneAddress, cosmos.ZeroUint(), su.AssetAddress, addAssetAmount, common.TxID(""), requestTxHash)
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	pool.PendingInboundRune = common.SafeSub(pool.PendingInboundRune, su.PendingRune)
	pool.PendingInboundAsset = common.SafeSub(pool.PendingInboundAsset, su.PendingAsset)
	su.PendingAsset = cosmos.ZeroUint()
	su.PendingRune = cosmos.ZeroUint()
	su.PendingTxID = ""

	ctx.Logger().Info("pre add liquidity", "pool", pool.Asset, "rune", pool.BalanceRune, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits)
	ctx.Logger().Info("adding liquidity", "rune", addRuneAmount, "asset", addAssetAmount)

	balanceRune := pool.BalanceRune
	balanceAsset := pool.BalanceAsset

	oldPoolUnits := pool.GetPoolUnits()
	newPoolUnits, liquidityUnits, err := calculatePoolUnitsV1(oldPoolUnits, balanceRune, balanceAsset, pendingRuneAmt, pendingAssetAmt)
	if err != nil {
		return ErrInternal(err, "fail to calculate pool unit")
	}
	ctx.Logger().Info("current pool status", "pool units", newPoolUnits, "liquidity units", liquidityUnits)
	poolRune := balanceRune.Add(pendingRuneAmt)
	poolAsset := balanceAsset.Add(pendingAssetAmt)
	pool.LPUnits = pool.LPUnits.Add(liquidityUnits)
	pool.BalanceRune = poolRune
	pool.BalanceAsset = poolAsset
	ctx.Logger().Info("post add liquidity", "pool", pool.Asset, "rune", pool.BalanceRune, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits, "add liquidity units", liquidityUnits)
	if pool.BalanceRune.IsZero() || pool.BalanceAsset.IsZero() {
		return ErrInternal(err, "pool cannot have zero rune or asset balance")
	}
	if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
		return ErrInternal(err, "fail to save pool")
	}
	if originalUnits.IsZero() && !pool.GetPoolUnits().IsZero() {
		poolEvent := NewEventPool(pool.Asset, pool.Status)
		if err := h.mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
	}

	su.Units = su.Units.Add(liquidityUnits)
	if pool.Status == PoolAvailable {
		if su.AssetDepositValue.IsZero() && su.RuneDepositValue.IsZero() {
			su.RuneDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceRune)
			su.AssetDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceAsset)
		} else {
			su.RuneDepositValue = su.RuneDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceRune))
			su.AssetDepositValue = su.AssetDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceAsset))
		}
	}
	h.mgr.Keeper().SetLiquidityProvider(ctx, su)

	evt := NewEventAddLiquidity(asset, liquidityUnits, su.RuneAddress, pendingRuneAmt, pendingAssetAmt, runeTxID, assetTxID, su.AssetAddress)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
		return ErrInternal(err, "fail to emit add liquidity event")
	}

	// if its the POL is adding, track rune added
	polAddress, err := h.mgr.Keeper().GetModuleAddress(ReserveName)
	if err != nil {
		return err
	}

	if polAddress.Equals(su.RuneAddress) {
		pol, err := h.mgr.Keeper().GetPOL(ctx)
		if err != nil {
			return err
		}
		pol.RuneDeposited = pol.RuneDeposited.Add(pendingRuneAmt)

		if err := h.mgr.Keeper().SetPOL(ctx, pol); err != nil {
			return err
		}

		ctx.Logger().Info("POL deposit", "pool", pool.Asset, "rune", pendingRuneAmt)
		telemetry.IncrCounterWithLabels(
			[]string{"thornode", "pol", "pool", "rune_deposited"},
			telem(pendingRuneAmt),
			[]metrics.Label{telemetry.NewLabel("pool", pool.Asset.String())},
		)
	}
	return nil
}

func (h AddLiquidityHandler) addLiquidityV90(ctx cosmos.Context,
	asset common.Asset,
	addRuneAmount, addAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	requestTxHash common.TxID,
	stage bool,
	constAccessor constants.ConstantValues,
) (err error) {
	ctx.Logger().Info("liquidity provision", "asset", asset, "rune amount", addRuneAmount, "asset amount", addAssetAmount)
	if err := h.validateAddLiquidityMessage(ctx, h.mgr.Keeper(), asset, requestTxHash, runeAddr, assetAddr); err != nil {
		return fmt.Errorf("add liquidity message fail validation: %w", err)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	synthSupply := h.mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	originalUnits := pool.CalcUnits(h.mgr.GetVersion(), synthSupply)

	// if THORNode have no balance, set the default pool status
	if originalUnits.IsZero() {
		defaultPoolStatus := PoolAvailable.String()
		// if the pools is for gas asset on the chain, automatically enable it
		if !pool.Asset.Equals(pool.Asset.GetChain().GetGasAsset()) {
			defaultPoolStatus = constAccessor.GetStringValue(constants.DefaultPoolStatus)
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)
	}

	fetchAddr := runeAddr
	if fetchAddr.IsEmpty() {
		fetchAddr = assetAddr
	}
	su, err := h.mgr.Keeper().GetLiquidityProvider(ctx, asset, fetchAddr)
	if err != nil {
		return ErrInternal(err, "fail to get liquidity provider")
	}

	su.LastAddHeight = ctx.BlockHeight()
	if su.Units.IsZero() {
		if su.PendingTxID.IsEmpty() {
			if su.RuneAddress.IsEmpty() {
				su.RuneAddress = runeAddr
			}
			if su.AssetAddress.IsEmpty() {
				su.AssetAddress = assetAddr
			}
		}

		// ensure input addresses match LP position addresses
		if !runeAddr.Equals(su.RuneAddress) {
			return errAddLiquidityMismatchAddr
		}
		if !assetAddr.Equals(su.AssetAddress) {
			return errAddLiquidityMismatchAddr
		}
	}

	if !assetAddr.IsEmpty() && !su.AssetAddress.Equals(assetAddr) {
		// mismatch of asset addresses from what is known to the address
		// given. Refund it.
		return errAddLiquidityMismatchAddr
	}

	// get tx hashes
	runeTxID := requestTxHash
	assetTxID := requestTxHash
	if addRuneAmount.IsZero() {
		runeTxID = su.PendingTxID
	} else {
		assetTxID = su.PendingTxID
	}

	pendingRuneAmt := su.PendingRune.Add(addRuneAmount)
	pendingAssetAmt := su.PendingAsset.Add(addAssetAmount)

	// if we have an asset address and no asset amount, put the rune pending
	if stage && pendingAssetAmt.IsZero() {
		pool.PendingInboundRune = pool.PendingInboundRune.Add(addRuneAmount)
		su.PendingRune = pendingRuneAmt
		su.PendingTxID = requestTxHash
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound rune", "error", err)
		}

		// add pending liquidity event
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.RuneAddress, addRuneAmount, su.AssetAddress, cosmos.ZeroUint(), requestTxHash, common.TxID(""))
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	// if we have a rune address and no rune asset, put the asset in pending
	if stage && pendingRuneAmt.IsZero() {
		pool.PendingInboundAsset = pool.PendingInboundAsset.Add(addAssetAmount)
		su.PendingAsset = pendingAssetAmt
		su.PendingTxID = requestTxHash
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound asset", "error", err)
		}
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.RuneAddress, cosmos.ZeroUint(), su.AssetAddress, addAssetAmount, common.TxID(""), requestTxHash)
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	pool.PendingInboundRune = common.SafeSub(pool.PendingInboundRune, su.PendingRune)
	pool.PendingInboundAsset = common.SafeSub(pool.PendingInboundAsset, su.PendingAsset)
	su.PendingAsset = cosmos.ZeroUint()
	su.PendingRune = cosmos.ZeroUint()
	su.PendingTxID = ""

	ctx.Logger().Info("pre add liquidity", "pool", pool.Asset, "rune", pool.BalanceRune, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits)
	ctx.Logger().Info("adding liquidity", "rune", addRuneAmount, "asset", addAssetAmount)

	balanceRune := pool.BalanceRune
	balanceAsset := pool.BalanceAsset

	oldPoolUnits := pool.GetPoolUnits()
	newPoolUnits, liquidityUnits, err := calculatePoolUnitsV1(oldPoolUnits, balanceRune, balanceAsset, pendingRuneAmt, pendingAssetAmt)
	if err != nil {
		return ErrInternal(err, "fail to calculate pool unit")
	}
	ctx.Logger().Info("current pool status", "pool units", newPoolUnits, "liquidity units", liquidityUnits)
	poolRune := balanceRune.Add(pendingRuneAmt)
	poolAsset := balanceAsset.Add(pendingAssetAmt)
	pool.LPUnits = pool.LPUnits.Add(liquidityUnits)
	pool.BalanceRune = poolRune
	pool.BalanceAsset = poolAsset
	ctx.Logger().Info("post add liquidity", "pool", pool.Asset, "rune", pool.BalanceRune, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits, "add liquidity units", liquidityUnits)
	if pool.BalanceRune.IsZero() || pool.BalanceAsset.IsZero() {
		return ErrInternal(err, "pool cannot have zero rune or asset balance")
	}
	if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
		return ErrInternal(err, "fail to save pool")
	}
	if originalUnits.IsZero() && !pool.GetPoolUnits().IsZero() {
		poolEvent := NewEventPool(pool.Asset, pool.Status)
		if err := h.mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
	}

	su.Units = su.Units.Add(liquidityUnits)
	if pool.Status == PoolAvailable {
		if su.AssetDepositValue.IsZero() && su.RuneDepositValue.IsZero() {
			su.RuneDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceRune)
			su.AssetDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceAsset)
		} else {
			su.RuneDepositValue = su.RuneDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceRune))
			su.AssetDepositValue = su.AssetDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceAsset))
		}
	}
	h.mgr.Keeper().SetLiquidityProvider(ctx, su)

	evt := NewEventAddLiquidity(asset, liquidityUnits, su.RuneAddress, pendingRuneAmt, pendingAssetAmt, runeTxID, assetTxID, su.AssetAddress)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
		return ErrInternal(err, "fail to emit add liquidity event")
	}
	return nil
}

func (h AddLiquidityHandler) addLiquidityV79(ctx cosmos.Context,
	asset common.Asset,
	addRuneAmount, addAssetAmount cosmos.Uint,
	runeAddr, assetAddr common.Address,
	requestTxHash common.TxID,
	stage bool,
	constAccessor constants.ConstantValues,
) error {
	ctx.Logger().Info("liquidity provision", "asset", asset, "rune amount", addRuneAmount, "asset amount", addAssetAmount)
	if err := h.validateAddLiquidityMessage(ctx, h.mgr.Keeper(), asset, requestTxHash, runeAddr, assetAddr); err != nil {
		return fmt.Errorf("add liquidity message fail validation: %w", err)
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool(%s)", asset))
	}
	ver := h.mgr.Keeper().GetLowestActiveVersion(ctx)
	synthSupply := h.mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	originalUnits := pool.CalcUnits(ver, synthSupply)

	// if THORNode have no balance, set the default pool status
	if originalUnits.IsZero() {
		defaultPoolStatus := PoolAvailable.String()
		// if the pools is for gas asset on the chain, automatically enable it
		if !pool.Asset.Equals(pool.Asset.GetChain().GetGasAsset()) {
			defaultPoolStatus = constAccessor.GetStringValue(constants.DefaultPoolStatus)
		}
		pool.Status = GetPoolStatus(defaultPoolStatus)
	}

	fetchAddr := runeAddr
	if fetchAddr.IsEmpty() {
		fetchAddr = assetAddr
	}
	su, err := h.mgr.Keeper().GetLiquidityProvider(ctx, asset, fetchAddr)
	if err != nil {
		return ErrInternal(err, "fail to get liquidity provider")
	}

	su.LastAddHeight = ctx.BlockHeight()
	if su.Units.IsZero() {
		if su.PendingTxID.IsEmpty() {
			if su.RuneAddress.IsEmpty() {
				su.RuneAddress = runeAddr
			}
			if su.AssetAddress.IsEmpty() {
				su.AssetAddress = assetAddr
			}
		}

		// ensure input addresses match LP position addresses
		if !runeAddr.Equals(su.RuneAddress) {
			return errAddLiquidityMismatchAddr
		}
		if !assetAddr.Equals(su.AssetAddress) {
			return errAddLiquidityMismatchAddr
		}
	}

	if !assetAddr.IsEmpty() && !su.AssetAddress.Equals(assetAddr) {
		// mismatch of asset addresses from what is known to the address
		// given. Refund it.
		return errAddLiquidityMismatchAddr
	}

	// get tx hashes
	runeTxID := requestTxHash
	assetTxID := requestTxHash
	if addRuneAmount.IsZero() {
		runeTxID = su.PendingTxID
	} else {
		assetTxID = su.PendingTxID
	}

	pendingRuneAmt := su.PendingRune.Add(addRuneAmount)
	pendingAssetAmt := su.PendingAsset.Add(addAssetAmount)

	// if we have an asset address and no asset amount, put the rune pending
	if stage && pendingAssetAmt.IsZero() {
		pool.PendingInboundRune = pool.PendingInboundRune.Add(addRuneAmount)
		su.PendingRune = pendingRuneAmt
		su.PendingTxID = requestTxHash
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound rune", "error", err)
		}

		// add pending liquidity event
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.RuneAddress, addRuneAmount, su.AssetAddress, cosmos.ZeroUint(), requestTxHash, common.TxID(""))
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	// if we have a rune address and no rune asset, put the asset in pending
	if stage && pendingRuneAmt.IsZero() {
		pool.PendingInboundAsset = pool.PendingInboundAsset.Add(addAssetAmount)
		su.PendingAsset = pendingAssetAmt
		su.PendingTxID = requestTxHash
		h.mgr.Keeper().SetLiquidityProvider(ctx, su)
		if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool pending inbound asset", "error", err)
		}
		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, su.RuneAddress, cosmos.ZeroUint(), su.AssetAddress, addAssetAmount, common.TxID(""), requestTxHash)
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			return ErrInternal(err, "fail to emit partial add liquidity event")
		}
		return nil
	}

	pool.PendingInboundRune = common.SafeSub(pool.PendingInboundRune, su.PendingRune)
	pool.PendingInboundAsset = common.SafeSub(pool.PendingInboundAsset, su.PendingAsset)
	su.PendingAsset = cosmos.ZeroUint()
	su.PendingRune = cosmos.ZeroUint()
	su.PendingTxID = ""

	ctx.Logger().Info("pre add liquidity", "pool", pool.Asset, "rune", pool.BalanceRune, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits)
	ctx.Logger().Info("adding liquidity", "rune", addRuneAmount, "asset", addAssetAmount)

	balanceRune := pool.BalanceRune
	balanceAsset := pool.BalanceAsset

	oldPoolUnits := pool.GetPoolUnits()
	newPoolUnits, liquidityUnits, err := calculatePoolUnitsV1(oldPoolUnits, balanceRune, balanceAsset, pendingRuneAmt, pendingAssetAmt)
	if err != nil {
		return ErrInternal(err, "fail to calculate pool unit")
	}
	ctx.Logger().Info("current pool status", "pool units", newPoolUnits, "liquidity units", liquidityUnits)
	poolRune := balanceRune.Add(pendingRuneAmt)
	poolAsset := balanceAsset.Add(pendingAssetAmt)
	pool.LPUnits = pool.LPUnits.Add(liquidityUnits)
	pool.BalanceRune = poolRune
	pool.BalanceAsset = poolAsset
	ctx.Logger().Info("post add liquidity", "pool", pool.Asset, "rune", pool.BalanceRune, "asset", pool.BalanceAsset, "LP units", pool.LPUnits, "synth units", pool.SynthUnits, "add liquidity units", liquidityUnits)
	if pool.BalanceRune.IsZero() || pool.BalanceAsset.IsZero() {
		return ErrInternal(err, "pool cannot have zero rune or asset balance")
	}
	if err := h.mgr.Keeper().SetPool(ctx, pool); err != nil {
		return ErrInternal(err, "fail to save pool")
	}
	if originalUnits.IsZero() && !pool.GetPoolUnits().IsZero() {
		poolEvent := NewEventPool(pool.Asset, pool.Status)
		if err := h.mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
	}

	su.Units = su.Units.Add(liquidityUnits)
	if pool.Status == PoolAvailable {
		if su.AssetDepositValue.IsZero() && su.RuneDepositValue.IsZero() {
			su.RuneDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceRune)
			su.AssetDepositValue = common.GetSafeShare(su.Units, pool.GetPoolUnits(), pool.BalanceAsset)
		} else {
			su.RuneDepositValue = su.RuneDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceRune))
			su.AssetDepositValue = su.AssetDepositValue.Add(common.GetSafeShare(liquidityUnits, pool.GetPoolUnits(), pool.BalanceAsset))
		}
	}
	h.mgr.Keeper().SetLiquidityProvider(ctx, su)

	evt := NewEventAddLiquidity(asset, liquidityUnits, su.RuneAddress, pendingRuneAmt, pendingAssetAmt, runeTxID, assetTxID, su.AssetAddress)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
		return ErrInternal(err, "fail to emit add liquidity event")
	}
	return nil
}

// getTotalActiveBond
func (h AddLiquidityHandler) getTotalActiveBond(ctx cosmos.Context) (cosmos.Uint, error) {
	nodeAccounts, err := h.mgr.Keeper().ListValidatorsWithBond(ctx)
	if err != nil {
		return cosmos.ZeroUint(), err
	}
	total := cosmos.ZeroUint()
	for _, na := range nodeAccounts {
		if na.Status != NodeActive {
			continue
		}
		total = total.Add(na.Bond)
	}
	return total, nil
}

func (h AddLiquidityHandler) getTotalLiquidityRUNEV1(ctx cosmos.Context) (cosmos.Uint, error) {
	pools, err := h.mgr.Keeper().GetPools(ctx)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get pools from data store: %w", err)
	}
	total := cosmos.ZeroUint()
	for _, p := range pools {
		// ignore suspended pools
		if p.Status == PoolSuspended {
			continue
		}
		if p.Asset.IsVaultAsset() {
			continue
		}
		total = total.Add(p.BalanceRune)
	}
	return total, nil
}

func (h AddLiquidityHandler) validateV99(ctx cosmos.Context, msg MsgAddLiquidity) error {
	if err := msg.ValidateBasicV98(); err != nil {
		ctx.Logger().Error(err.Error())
		return errAddLiquidityFailValidation
	}

	// The Ragnarok key for the TERRA.LUNA pool would be RAGNAROK-TERRA-LUNA .
	k := "RAGNAROK-" + msg.Asset.MimirString()
	v, err := h.mgr.Keeper().GetMimir(ctx, k)
	if err != nil {
		ctx.Logger().Error("fail to get mimir value", "mimir", k, "error", err)
	}
	if v >= 1 {
		return fmt.Errorf("cannot add liquidity to Ragnaroked pool (%s)", msg.Asset.String())
	}

	// Note that GetChain() without GetLayer1Asset() would indicate THORChain for synthetic assets.
	gasAsset := msg.Asset.GetLayer1Asset().GetChain().GetGasAsset()
	// Even if a destination gas asset pool is empty, the first add liquidity has to be symmetrical,
	// and so there is no need to check at this stage for whether the addition is of RUNE or Asset or with needsSwap.
	if !msg.Asset.Equals(gasAsset) {
		gasPool, err := h.mgr.Keeper().GetPool(ctx, gasAsset)
		// Note that for a synthetic asset msg.Asset.Chain (unlike msg.Asset.GetChain())
		// is intentionally used to be the external chain rather than THOR.
		// Any destination asset starting with THOR should be rejected for no THOR.RUNE
		// gas asset pool existing.
		if err != nil {
			return ErrInternal(err, "fail to get gas pool")
		}
		// Note that NewPool from GetPool would return a pool with status;
		// use IsEmpty to check for prior existence.
		if gasPool.IsEmpty() {
			return fmt.Errorf("asset (%s)'s gas asset pool (%s) does not exist yet", msg.Asset.String(), gasAsset.String())
		}
	}

	if msg.Asset.IsDerivedAsset() {
		return fmt.Errorf("asset cannot be a derived asset")
	}

	if msg.Asset.IsVaultAsset() {
		if !msg.Asset.GetLayer1Asset().IsGasAsset() {
			return fmt.Errorf("asset must be a gas asset for the layer1 protocol")
		}
		if !msg.AssetAddress.IsChain(msg.Asset.GetLayer1Asset().GetChain()) {
			return fmt.Errorf("asset address must be layer1 chain")
		}
		if !msg.RuneAmount.IsZero() {
			return fmt.Errorf("cannot deposit rune into a vault")
		}
	}

	if !msg.RuneAddress.IsEmpty() && !msg.RuneAddress.IsChain(common.THORChain) {
		ctx.Logger().Error("rune address must be THORChain")
		return errAddLiquidityFailValidation
	}

	if !msg.AssetAddress.IsEmpty() {
		polAddress, err := h.mgr.Keeper().GetModuleAddress(ReserveName)
		if err != nil {
			return err
		}
		if msg.RuneAddress.Equals(polAddress) {
			return fmt.Errorf("pol lp cannot have asset address")
		}
	}

	// check if swap meets standards
	if h.needsSwap(msg) {
		if !msg.Asset.IsVaultAsset() {
			return fmt.Errorf("swap & add liquidity is only available for synthetic pools")
		}
		if !msg.Asset.GetLayer1Asset().Equals(msg.Tx.Coins[0].Asset) {
			return fmt.Errorf("deposit asset must be the layer1 equivalent for the synthetic asset")
		}
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, "fail to get pool")
	}
	if err := pool.EnsureValidPoolStatus(&msg); err != nil {
		ctx.Logger().Error("fail to check pool status", "error", err)
		return errInvalidPoolStatus
	}

	if isChainHalted(ctx, h.mgr, msg.Asset.Chain) || isLPPaused(ctx, msg.Asset.Chain, h.mgr) {
		return fmt.Errorf("unable to add liquidity while chain has paused LP actions")
	}

	ensureLiquidityNoLargerThanBond := h.mgr.GetConstants().GetBoolValue(constants.StrictBondLiquidityRatio)
	// if the pool is THORChain no need to check economic security
	if msg.Asset.IsVaultAsset() || !ensureLiquidityNoLargerThanBond {
		return nil
	}

	// the following  only applicable for chaosnet
	totalLiquidityRUNE, err := h.getTotalLiquidityRUNE(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get total liquidity RUNE")
	}

	// total liquidity RUNE after current add liquidity
	totalLiquidityRUNE = totalLiquidityRUNE.Add(msg.RuneAmount)
	totalLiquidityRUNE = totalLiquidityRUNE.Add(pool.AssetValueInRune(msg.AssetAmount))
	maximumLiquidityRune, err := h.mgr.Keeper().GetMimir(ctx, constants.MaximumLiquidityRune.String())
	if maximumLiquidityRune < 0 || err != nil {
		maximumLiquidityRune = h.mgr.GetConstants().GetInt64Value(constants.MaximumLiquidityRune)
	}
	if maximumLiquidityRune > 0 {
		if totalLiquidityRUNE.GT(cosmos.NewUint(uint64(maximumLiquidityRune))) {
			return errAddLiquidityRUNEOverLimit
		}
	}

	if !ensureLiquidityNoLargerThanBond {
		return nil
	}
	securityBond, err := h.getEffectiveSecurityBond(ctx)
	if err != nil {
		return ErrInternal(err, "fail to get security bond RUNE")
	}
	if totalLiquidityRUNE.GT(securityBond) {
		ctx.Logger().Info("total liquidity RUNE is more than effective security bond", "rune", totalLiquidityRUNE.String(), "bond", securityBond.String())
		return errAddLiquidityRUNEMoreThanBond
	}

	return nil
}
