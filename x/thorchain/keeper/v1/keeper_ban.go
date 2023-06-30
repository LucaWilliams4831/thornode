package keeperv1

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStore) setBanVoter(ctx cosmos.Context, key string, record BanVoter) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getBanVoter(ctx cosmos.Context, key string, record *BanVoter) (bool, error) {
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

// SetBanVoter - save a ban voter object
func (k KVStore) SetBanVoter(ctx cosmos.Context, ban BanVoter) {
	k.setBanVoter(ctx, k.GetKey(ctx, prefixBanVoter, ban.String()), ban)
}

// GetBanVoter - gets information of ban voter
func (k KVStore) GetBanVoter(ctx cosmos.Context, addr cosmos.AccAddress) (BanVoter, error) {
	record := NewBanVoter(addr)
	_, err := k.getBanVoter(ctx, k.GetKey(ctx, prefixBanVoter, record.String()), &record)
	return record, err
}

// GetBanVoterIterator - get an iterator for ban voter
func (k KVStore) GetBanVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixBanVoter)
}
