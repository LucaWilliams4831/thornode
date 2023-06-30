package keeperv1

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

func (k KVStore) setRagnarokWithdrawPosition(ctx cosmos.Context, key string, record RagnarokWithdrawPosition) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getRagnarokWithdrawPosition(ctx cosmos.Context, key string, record *RagnarokWithdrawPosition) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, record); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	return true, nil
}

// RagnarokInProgress return true only when Ragnarok is happening, when Ragnarok block height is not 0
func (k KVStore) RagnarokInProgress(ctx cosmos.Context) bool {
	height, err := k.GetRagnarokBlockHeight(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get ragnarok block height", "error", err)
		return true
	}
	return height > 0
}

// getRagnarokValue - fetches the ragnarok value at given prefix
func (k KVStore) getRagnarokValue(ctx cosmos.Context, prefix types.DbPrefix) (int64, error) {
	record := int64(0)
	_, err := k.getInt64(ctx, k.GetKey(ctx, prefix, ""), &record)
	return record, err
}

// GetRagnarokBlockHeight get ragnarok block height from key value store
func (k KVStore) GetRagnarokBlockHeight(ctx cosmos.Context) (int64, error) {
	return k.getRagnarokValue(ctx, prefixRagnarokHeight)
}

// SetRagnarokBlockHeight save ragnarok block height to key value store, once it get set , it means ragnarok started
func (k KVStore) SetRagnarokBlockHeight(ctx cosmos.Context, height int64) {
	k.setInt64(ctx, k.GetKey(ctx, prefixRagnarokHeight, ""), height)
}

// GetRagnarokNth when ragnarok get triggered , THORNode will use a few rounds to refund all assets
// this method return which round it is in
func (k KVStore) GetRagnarokNth(ctx cosmos.Context) (int64, error) {
	return k.getRagnarokValue(ctx, prefixRagnarokNth)
}

// SetRagnarokNth save the round number into key value store
func (k KVStore) SetRagnarokNth(ctx cosmos.Context, nth int64) {
	k.setInt64(ctx, k.GetKey(ctx, prefixRagnarokNth, ""), nth)
}

// GetRagnarokPending get ragnarok pending state from key value store
func (k KVStore) GetRagnarokPending(ctx cosmos.Context) (int64, error) {
	return k.getRagnarokValue(ctx, prefixRagnarokPending)
}

// SetRagnarokPending save ragnarok pending to key value store
func (k KVStore) SetRagnarokPending(ctx cosmos.Context, pending int64) {
	k.setInt64(ctx, k.GetKey(ctx, prefixRagnarokPending, ""), pending)
}

// GetRagnarokWithdrawPosition get ragnarok withdrawing position
func (k KVStore) GetRagnarokWithdrawPosition(ctx cosmos.Context) (RagnarokWithdrawPosition, error) {
	record := RagnarokWithdrawPosition{}
	_, err := k.getRagnarokWithdrawPosition(ctx, k.GetKey(ctx, prefixRagnarokPosition, ""), &record)
	return record, err
}

// SetRagnarokWithdrawPosition set ragnarok withdraw position
func (k KVStore) SetRagnarokWithdrawPosition(ctx cosmos.Context, position RagnarokWithdrawPosition) {
	k.setRagnarokWithdrawPosition(ctx, k.GetKey(ctx, prefixRagnarokPosition, ""), position)
}

// SetPoolRagnarokStart set pool ragnarok start block height
func (k KVStore) SetPoolRagnarokStart(ctx cosmos.Context, asset common.Asset) {
	k.setInt64(ctx, k.GetKey(ctx, prefixRagnarokPoolHeight, asset.String()), ctx.BlockHeight())
}

// GetPoolRagnarokStart get pool ragnarok start block height
func (k KVStore) GetPoolRagnarokStart(ctx cosmos.Context, asset common.Asset) (int64, error) {
	record := int64(0)
	_, err := k.getInt64(ctx, k.GetKey(ctx, prefixRagnarokPoolHeight, asset.String()), &record)
	return record, err
}
