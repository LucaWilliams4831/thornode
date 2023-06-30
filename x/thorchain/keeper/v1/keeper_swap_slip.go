package keeperv1

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// AddToSwapSlip - add swap slip to block
func (k KVStore) AddToSwapSlip(ctx cosmos.Context, asset common.Asset, amt cosmos.Int) error {
	currentHeight := ctx.BlockHeight()

	poolSlip, err := k.GetPoolSwapSlip(ctx, currentHeight, asset)
	if err != nil {
		return err
	}

	poolSlip = poolSlip.Add(amt)

	// update pool slip
	k.setInt64(ctx, k.GetKey(ctx, prefixPoolSwapSlip, fmt.Sprintf("%d-%s", currentHeight, asset.String())), poolSlip.Int64())
	return nil
}

func (k KVStore) DeletePoolSwapSlip(ctx cosmos.Context, height int64, asset common.Asset) {
	key := k.GetKey(ctx, prefixPoolSwapSlip, fmt.Sprintf("%d-%s", height, asset.String()))
	k.del(ctx, key)
}

func (k KVStore) getSwapSlip(ctx cosmos.Context, key string) (cosmos.Int, error) {
	var record int64
	_, err := k.getInt64(ctx, key, &record)
	return cosmos.NewInt(record), err
}

// GetPoolSwapSlip - total of slip in each block per pool
func (k KVStore) GetPoolSwapSlip(ctx cosmos.Context, height int64, asset common.Asset) (cosmos.Int, error) {
	key := k.GetKey(ctx, prefixPoolSwapSlip, fmt.Sprintf("%d-%s", height, asset.String()))
	return k.getSwapSlip(ctx, key)
}

// RollupSwapSlip - sums the amount of slip in a given pool in the last targetCount blocks
func (k KVStore) RollupSwapSlip(ctx cosmos.Context, targetCount int64, asset common.Asset) (cosmos.Int, error) {
	var currCount int64
	currCountKey := k.GetKey(ctx, prefixPoolSwapSlip, fmt.Sprintf("rollup-count/%s", asset.String()))
	_, err := k.getInt64(ctx, currCountKey, &currCount)
	if err != nil {
		return cosmos.ZeroInt(), err
	}

	var currRollup int64
	currRollupKey := k.GetKey(ctx, prefixPoolSwapSlip, fmt.Sprintf("rollup/%s", asset.String()))
	_, err = k.getInt64(ctx, currRollupKey, &currRollup)
	if err != nil {
		return cosmos.ZeroInt(), err
	}

	reset := func(err error) (cosmos.Int, error) {
		if err != nil {
			ctx.Logger().Error("resetting pool swap slip rollup", "asset", asset.String(), "err", err)
		}
		k.setInt64(ctx, currCountKey, 0)
		k.setInt64(ctx, currRollupKey, 0)
		return cosmos.ZeroInt(), err
	}

	if currCount > targetCount {
		// we need to reset, likely the target count was changed to a lower
		// number than it was before
		ctx.Logger().Info("resetting pool swap rollup", "asset", asset.String())
		return reset(nil)
	}

	// add the swap slip from the previous block to the rollup
	prevBlockSlip, err := k.GetPoolSwapSlip(ctx, ctx.BlockHeight()-1, asset)
	if err != nil {
		return reset(err)
	}
	currRollup += prevBlockSlip.Int64()
	currCount++

	if currCount > targetCount {
		// remove the oldest swap slip block from the count
		oldBlockSlip, err := k.GetPoolSwapSlip(ctx, ctx.BlockHeight()-targetCount, asset)
		if err != nil {
			return reset(err)
		}
		currRollup -= oldBlockSlip.Int64()
		currCount--
		k.DeletePoolSwapSlip(ctx, ctx.BlockHeight()-targetCount, asset)
	}

	k.setInt64(ctx, currCountKey, currCount)
	k.setInt64(ctx, currRollupKey, currRollup)

	return cosmos.NewInt(currRollup), nil
}
