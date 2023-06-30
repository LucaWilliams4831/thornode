package keeperv1

import (
	"fmt"
	"strconv"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStore) setKeygenBlock(ctx cosmos.Context, key string, record KeygenBlock) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getKeygenBlock(ctx cosmos.Context, key string, record *KeygenBlock) (bool, error) {
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

// SetKeygenBlock save the KeygenBlock to kv store
func (k KVStore) SetKeygenBlock(ctx cosmos.Context, keygen KeygenBlock) {
	k.setKeygenBlock(ctx, k.GetKey(ctx, prefixKeygen, strconv.FormatInt(keygen.Height, 10)), keygen)
}

// GetKeygenBlockIterator return an iterator
func (k KVStore) GetKeygenBlockIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixKeygen)
}

// GetKeygenBlock from a given height
func (k KVStore) GetKeygenBlock(ctx cosmos.Context, height int64) (KeygenBlock, error) {
	record := NewKeygenBlock(height)
	_, err := k.getKeygenBlock(ctx, k.GetKey(ctx, prefixKeygen, strconv.FormatInt(height, 10)), &record)
	return record, err
}
