package keeperv1

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStore) setErrataTxVoter(ctx cosmos.Context, key string, record ErrataTxVoter) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getErrataTxVoter(ctx cosmos.Context, key string, record *ErrataTxVoter) (bool, error) {
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

// SetErrataTxVoter - save a errata voter object
func (k KVStore) SetErrataTxVoter(ctx cosmos.Context, errata ErrataTxVoter) {
	k.setErrataTxVoter(ctx, k.GetKey(ctx, prefixErrataTx, errata.String()), errata)
}

// GetErrataTxVoterIterator iterate errata tx voter
func (k KVStore) GetErrataTxVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixErrataTx)
}

// GetErrataTxVoter - gets information of errata tx voter
func (k KVStore) GetErrataTxVoter(ctx cosmos.Context, txID common.TxID, chain common.Chain) (ErrataTxVoter, error) {
	record := NewErrataTxVoter(txID, chain)
	_, err := k.getErrataTxVoter(ctx, k.GetKey(ctx, prefixErrataTx, record.String()), &record)
	return record, err
}
