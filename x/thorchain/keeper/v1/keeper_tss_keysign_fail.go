package keeperv1

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStore) setTssKeysignFailVoter(ctx cosmos.Context, key string, record TssKeysignFailVoter) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getTssKeysignFailVoter(ctx cosmos.Context, key string, record *TssKeysignFailVoter) (bool, error) {
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

// SetTssKeysignFailVoter - save a tss keysign fail voter object
func (k KVStore) SetTssKeysignFailVoter(ctx cosmos.Context, tss TssKeysignFailVoter) {
	k.setTssKeysignFailVoter(ctx, k.GetKey(ctx, prefixTssKeysignFailure, tss.String()), tss)
}

// GetTssKeysignFailVoterIterator iterate tx in voters
func (k KVStore) GetTssKeysignFailVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixTssKeysignFailure)
}

// GetTssKeysignFailVoter - gets information of a tss keysign failure voter object
func (k KVStore) GetTssKeysignFailVoter(ctx cosmos.Context, id string) (TssKeysignFailVoter, error) {
	record := TssKeysignFailVoter{ID: id}
	_, err := k.getTssKeysignFailVoter(ctx, k.GetKey(ctx, prefixTssKeysignFailure, id), &record)
	return record, err
}
