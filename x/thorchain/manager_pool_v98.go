package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type PoolMgrV98 struct{}

func newPoolMgrV98() *PoolMgrV98 {
	return &PoolMgrV98{}
}

// EndBlock cycle pools if required and if ragnarok is not in progress
func (pm *PoolMgrV98) EndBlock(ctx cosmos.Context, mgr Manager) error {
	poolCycle, err := mgr.Keeper().GetMimir(ctx, constants.PoolCycle.String())
	if poolCycle < 0 || err != nil {
		poolCycle = mgr.GetConstants().GetInt64Value(constants.PoolCycle)
	}
	// Enable a pool every poolCycle
	if ctx.BlockHeight()%poolCycle == 0 && !mgr.Keeper().RagnarokInProgress(ctx) {
		maxAvailablePools, err := mgr.Keeper().GetMimir(ctx, constants.MaxAvailablePools.String())
		if maxAvailablePools < 0 || err != nil {
			maxAvailablePools = mgr.GetConstants().GetInt64Value(constants.MaxAvailablePools)
		}
		minRunePoolDepth, err := mgr.Keeper().GetMimir(ctx, constants.MinRunePoolDepth.String())
		if minRunePoolDepth < 0 || err != nil {
			minRunePoolDepth = mgr.GetConstants().GetInt64Value(constants.MinRunePoolDepth)
		}
		stagedPoolCost, err := mgr.Keeper().GetMimir(ctx, constants.StagedPoolCost.String())
		if stagedPoolCost < 0 || err != nil {
			stagedPoolCost = mgr.GetConstants().GetInt64Value(constants.StagedPoolCost)
		}
		if err := pm.cyclePools(ctx, maxAvailablePools, minRunePoolDepth, stagedPoolCost, mgr); err != nil {
			ctx.Logger().Error("Unable to enable a pool", "error", err)
		}
	}
	return nil
}

// cyclePools update the set of Available and Staged pools
// Available non-gas pools not meeting the fee quota since last cycle, or not
// meeting availability requirements, are demoted to Staged.
// Staged pools are charged a fee and those with with zero rune depth and
// non-zero asset depth are removed along with their liquidity providers, and
// remaining assets are abandoned.
// The valid Staged pool with the highest rune depth is promoted to Available.
// If there are more than the maximum available pools, the Available pool with
// with the lowest rune depth is demoted to Staged
func (pm *PoolMgrV98) cyclePools(ctx cosmos.Context, maxAvailablePools, minRunePoolDepth, stagedPoolCost int64, mgr Manager) error {
	var availblePoolCount int64
	onDeck := NewPool()        // currently staged pool that could get promoted
	choppingBlock := NewPool() // currently available pool that is on the chopping block to being demoted
	minRuneDepth := cosmos.NewUint(uint64(minRunePoolDepth))
	minPoolLiquidityFee := fetchConfigInt64(ctx, mgr, constants.MinimumPoolLiquidityFee)
	// quick func to check the validity of a pool
	validPool := func(pool Pool) bool {
		if pool.BalanceAsset.IsZero() || pool.BalanceRune.IsZero() || pool.BalanceRune.LT(minRuneDepth) {
			return false
		}
		return true
	}

	// quick func to save a pool status and emit event
	setPool := func(pool Pool) error {
		poolEvt := NewEventPool(pool.Asset, pool.Status)
		if err := mgr.EventMgr().EmitEvent(ctx, poolEvt); err != nil {
			return fmt.Errorf("fail to emit pool event: %w", err)
		}

		switch pool.Status {
		case PoolAvailable:
			ctx.Logger().Info("New available pool", "pool", pool.Asset)
		case PoolStaged:
			ctx.Logger().Info("Pool demoted to staged status", "pool", pool.Asset)
		}
		pool.StatusSince = ctx.BlockHeight()
		return mgr.Keeper().SetPool(ctx, pool)
	}

	iterator := mgr.Keeper().GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &pool); err != nil {
			return err
		}
		// skip all cycling on saver pools
		if pool.Asset.IsSyntheticAsset() {
			continue
		}
		if pool.Asset.IsGasAsset() {
			continue
		}
		switch pool.Status {
		case PoolAvailable:
			// any available pools that have no asset, no rune, or less than
			// min rune, moves back to staged status
			if validPool(pool) &&
				pm.poolMeetTradingVolumeCriteria(ctx, mgr, pool, cosmos.NewUint(uint64(minPoolLiquidityFee))) {
				availblePoolCount += 1
			} else {
				pool.Status = PoolStaged
				if err := setPool(pool); err != nil {
					return err
				}
			}
			// reset the pool rolling liquidity fee to zero
			mgr.Keeper().ResetRollingPoolLiquidityFee(ctx, pool.Asset)
			if pool.BalanceRune.LT(choppingBlock.BalanceRune) || choppingBlock.IsEmpty() {
				// omit pools that are gas assets from being on the chopping
				// block, removing these pool requires a chain ragnarok, and
				// cannot be handled individually
				choppingBlock = pool
			}
		case PoolStaged:
			// deduct staged pool rune fee
			fee := cosmos.NewUint(uint64(stagedPoolCost))
			if fee.GT(pool.BalanceRune) {
				fee = pool.BalanceRune
			}
			if !fee.IsZero() {
				pool.BalanceRune = common.SafeSub(pool.BalanceRune, fee)
				if err := mgr.Keeper().SetPool(ctx, pool); err != nil {
					ctx.Logger().Error("fail to save pool", "pool", pool.Asset, "err", err)
				}

				if err := mgr.Keeper().AddPoolFeeToReserve(ctx, fee); err != nil {
					ctx.Logger().Error("fail to add rune to reserve", "from pool", pool.Asset, "err", err)
				}

				emitPoolBalanceChangedEvent(ctx,
					NewPoolMod(pool.Asset, fee, false, cosmos.ZeroUint(), false),
					"pool stage cost",
					mgr)
			}
			// check if the rune balance is zero, and asset balance IS NOT
			// zero. This is because we don't want to abandon a pool that is in
			// the process of being created (race condition). We can safely
			// assume, if a pool has asset, but no rune, it should be
			// abandoned.
			if pool.BalanceRune.IsZero() && !pool.BalanceAsset.IsZero() {
				// the staged pool no longer has any rune, abandon the pool
				// and liquidity provider, and burn the asset (via zero'ing
				// the vaults for the asset, and churning away from the
				// tokens)
				ctx.Logger().Info("burning pool", "pool", pool.Asset)

				// remove LPs
				pm.removeLiquidityProviders(ctx, pool.Asset, mgr)

				// delete the pool
				mgr.Keeper().RemovePool(ctx, pool.Asset)

				poolEvent := NewEventPool(pool.Asset, PoolSuspended)
				if err := mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
					ctx.Logger().Error("fail to emit pool event", "error", err)
				}
				// remove asset from Vault
				pm.removeAssetFromVault(ctx, pool.Asset, mgr)

			} else if validPool(pool) && onDeck.BalanceRune.LT(pool.BalanceRune) {
				onDeck = pool
			}
		}
	}

	if availblePoolCount >= maxAvailablePools {
		// if we've hit our max available pools, and the onDeck pool is less
		// than the chopping block pool, then we do make no changes, by
		// resetting the variables
		if onDeck.BalanceRune.LTE(choppingBlock.BalanceRune) {
			onDeck = NewPool()        // reset
			choppingBlock = NewPool() // reset
		}
	} else {
		// since we haven't hit the max number of available pools, there is no
		// available pool on the chopping block
		choppingBlock = NewPool() // reset
	}

	if !onDeck.IsEmpty() {
		onDeck.Status = PoolAvailable
		if err := setPool(onDeck); err != nil {
			return err
		}
	}

	if !choppingBlock.IsEmpty() {
		choppingBlock.Status = PoolStaged
		if err := setPool(choppingBlock); err != nil {
			return err
		}
	}

	return nil
}

// poolMeetTradingVolumeCriteria check if pool generated the minimum amount of fees since last cycle
func (pm *PoolMgrV98) poolMeetTradingVolumeCriteria(ctx cosmos.Context, mgr Manager, pool Pool, minPoolLiquidityFee cosmos.Uint) bool {
	if minPoolLiquidityFee.IsZero() {
		return true
	}
	blockPoolLiquidityFee, err := mgr.Keeper().GetRollingPoolLiquidityFee(ctx, pool.Asset)
	if err != nil {
		ctx.Logger().Error("fail to get rolling pool liquidity from key value store", "error", err)
		// when we failed to get rolling liquidity fee from key value store for some reason, return true here
		// thus the pool will not be demoted
		return true
	}
	return cosmos.NewUint(blockPoolLiquidityFee).GTE(minPoolLiquidityFee)
}

// removeAssetFromVault set asset balance to zero for all vaults holding the asset
func (pm *PoolMgrV98) removeAssetFromVault(ctx cosmos.Context, asset common.Asset, mgr Manager) {
	// zero vaults with the pool asset
	vaultIter := mgr.Keeper().GetVaultIterator(ctx)
	defer vaultIter.Close()
	for ; vaultIter.Valid(); vaultIter.Next() {
		var vault Vault
		if err := mgr.Keeper().Cdc().Unmarshal(vaultIter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
			continue
		}
		if vault.HasAsset(asset) {
			for i, coin := range vault.Coins {
				if asset.Equals(coin.Asset) {
					vault.Coins[i].Amount = cosmos.ZeroUint()
					if err := mgr.Keeper().SetVault(ctx, vault); err != nil {
						ctx.Logger().Error("fail to save vault", "error", err)
					}
					break
				}
			}
		}
	}
}

// removeLiquidityProviders remove all lps for the given asset pool
func (pm *PoolMgrV98) removeLiquidityProviders(ctx cosmos.Context, asset common.Asset, mgr Manager) {
	iterator := mgr.Keeper().GetLiquidityProviderIterator(ctx, asset)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var lp LiquidityProvider
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &lp); err != nil {
			ctx.Logger().Error("fail to unmarshal liquidity provider", "error", err)
			continue
		}
		withdrawEvt := NewEventWithdraw(
			asset,
			lp.Units,
			int64(0),
			cosmos.ZeroDec(),
			common.Tx{FromAddress: lp.GetAddress()},
			cosmos.ZeroUint(),
			cosmos.ZeroUint(),
			cosmos.ZeroUint(),
		)
		if err := mgr.EventMgr().EmitEvent(ctx, withdrawEvt); err != nil {
			ctx.Logger().Error("fail to emit pool withdraw event", "error", err)
		}
		mgr.Keeper().RemoveLiquidityProvider(ctx, lp)
	}
}
