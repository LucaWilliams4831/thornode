package keeperv1

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStore) setSolvencyVoter(ctx cosmos.Context, key string, record SolvencyVoter) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getSolvencyVoter(ctx cosmos.Context, key string, record *SolvencyVoter) (bool, error) {
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

// SetSolvencyVoter - save a solvency voter object
func (k KVStore) SetSolvencyVoter(ctx cosmos.Context, solvencyVoter SolvencyVoter) {
	key := fmt.Sprintf("%s-%s", solvencyVoter.Chain, solvencyVoter.Id)
	k.setSolvencyVoter(ctx, k.GetKey(ctx, prefixSolvencyVoter, key), solvencyVoter)
}

// GetSolvencyVoter - gets information of solvency voter
func (k KVStore) GetSolvencyVoter(ctx cosmos.Context, txID common.TxID, chain common.Chain) (SolvencyVoter, error) {
	key := fmt.Sprintf("%s-%s", chain, txID)
	var solvencyVoter SolvencyVoter
	_, err := k.getSolvencyVoter(ctx, k.GetKey(ctx, prefixSolvencyVoter, key), &solvencyVoter)
	return solvencyVoter, err
}
