package keeperv1

import (
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

// SetLastSignedHeight save last signed height into kv store
func (k KVStore) SetLastSignedHeight(ctx cosmos.Context, height int64) error {
	lastHeight, _ := k.GetLastSignedHeight(ctx)
	if lastHeight > height {
		// it is very possible we try to update last sign height to a smaller value , asgard outbound takes time , however yggdrasil tx will be much faster
		// simply ignore it
		ctx.Logger().Info("block height can't go backward ", "last height", lastHeight, "current height", height)
		return nil
	}
	k.setInt64(ctx, k.GetKey(ctx, prefixLastSignedHeight, ""), height)
	return nil
}

// GetLastSignedHeight get last signed height from key value store
func (k KVStore) GetLastSignedHeight(ctx cosmos.Context) (int64, error) {
	var record int64
	_, err := k.getInt64(ctx, k.GetKey(ctx, prefixLastSignedHeight, ""), &record)
	return record, err
}

// SetLastChainHeight save last chain height
func (k KVStore) SetLastChainHeight(ctx cosmos.Context, chain common.Chain, height int64) error {
	lastHeight, _ := k.GetLastChainHeight(ctx, chain)
	if lastHeight > height {
		err := fmt.Errorf("last block height %d is larger than %d, block height can't go backward ", lastHeight, height)
		return dbError(ctx, "", err)
	}
	k.setInt64(ctx, k.GetKey(ctx, prefixLastChainHeight, chain.String()), height)
	return nil
}

// ForceSetLastChainHeight force sets the last chain height.
func (k KVStore) ForceSetLastChainHeight(ctx cosmos.Context, chain common.Chain, height int64) {
	k.setInt64(ctx, k.GetKey(ctx, prefixLastChainHeight, chain.String()), height)
}

// GetLastChainHeight get last chain height
func (k KVStore) GetLastChainHeight(ctx cosmos.Context, chain common.Chain) (int64, error) {
	var record int64
	_, err := k.getInt64(ctx, k.GetKey(ctx, prefixLastChainHeight, chain.String()), &record)
	return record, err
}

// GetLastChainHeights get the iterator for last chain height
func (k KVStore) GetLastChainHeights(ctx cosmos.Context) (map[common.Chain]int64, error) {
	iter := k.getIterator(ctx, prefixLastChainHeight)
	result := make(map[common.Chain]int64)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		key := string(iter.Key())
		c := strings.TrimPrefix(key, string(prefixLastChainHeight+"/"))
		chain, err := common.NewChain(c)
		if err != nil {
			return nil, fmt.Errorf("fail to parse chain: %w", err)
		}
		value := ProtoInt64{}
		k.cdc.MustUnmarshal(iter.Value(), &value)
		result[chain] = value.Value
	}
	return result, nil
}

// SetLastObserveHeight save the last observe height into key value store
func (k KVStore) SetLastObserveHeight(ctx cosmos.Context, chain common.Chain, address cosmos.AccAddress, height int64) error {
	var lastHeight int64
	key := k.GetKey(ctx, prefixLastObserveHeight, address.String()) + "/" + chain.String()
	exist, err := k.getInt64(ctx, key, &lastHeight)
	if err != nil {
		ctx.Logger().Error("fail to get last observe height", "error", err)
	}
	// if the last height is already larger then current height , then just bail out
	if exist && lastHeight > height {
		return nil
	}

	k.setInt64(ctx, key, height)
	return nil
}

// ForceSetLastObserveHeight force sets the observe height.
func (k KVStore) ForceSetLastObserveHeight(ctx cosmos.Context, chain common.Chain, address cosmos.AccAddress, height int64) {
	key := k.GetKey(ctx, prefixLastObserveHeight, address.String()) + "/" + chain.String()
	k.setInt64(ctx, key, height)
}

// GetLastObserveHeight retrieve last observe height of a given node account from key value store
func (k KVStore) GetLastObserveHeight(ctx cosmos.Context, address cosmos.AccAddress) (map[common.Chain]int64, error) {
	prefixKey := k.GetKey(ctx, prefixLastObserveHeight, address.String()) + "/"
	iter := k.getIterator(ctx, types.DbPrefix(prefixKey))
	result := make(map[common.Chain]int64)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		key := string(iter.Key())
		c := strings.TrimPrefix(key, prefixKey)
		chain, err := common.NewChain(c)
		if err != nil {
			return nil, fmt.Errorf("fail to parse chain: %w", err)
		}
		value := ProtoInt64{}
		k.cdc.MustUnmarshal(iter.Value(), &value)
		result[chain] = value.Value
	}
	return result, nil
}
