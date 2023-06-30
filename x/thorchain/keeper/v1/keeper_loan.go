package keeperv1

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

func (k KVStore) setLoan(ctx cosmos.Context, key string, record Loan) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getLoan(ctx cosmos.Context, key string, record *Loan) (bool, error) {
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

// GetLoanIterator iterate loans
func (k KVStore) GetLoanIterator(ctx cosmos.Context, asset common.Asset) cosmos.Iterator {
	key := k.GetKey(ctx, prefixLoan, asset.String())
	return k.getIterator(ctx, types.DbPrefix(key))
}

// GetLoan retrieve loan from the data store
func (k KVStore) GetLoan(ctx cosmos.Context, asset common.Asset, addr common.Address) (Loan, error) {
	record := NewLoan(addr, asset, 0)
	_, err := k.getLoan(ctx, k.GetKey(ctx, prefixLoan, record.Key()), &record)
	return record, err
}

// SetLoan save the loan to kv store
func (k KVStore) SetLoan(ctx cosmos.Context, lp Loan) {
	k.setLoan(ctx, k.GetKey(ctx, prefixLoan, lp.Key()), lp)
}

// RemoveLoan remove the loan to kv store
func (k KVStore) RemoveLoan(ctx cosmos.Context, lp Loan) {
	k.del(ctx, k.GetKey(ctx, prefixLoan, lp.Key()))
}

func (k KVStore) SetTotalCollateral(ctx cosmos.Context, asset common.Asset, amt cosmos.Uint) {
	key := k.GetKey(ctx, prefixLoanTotalCollateral, asset.String())
	k.setUint64(ctx, key, amt.Uint64())
}

func (k KVStore) GetTotalCollateral(ctx cosmos.Context, asset common.Asset) (cosmos.Uint, error) {
	var record uint64
	key := k.GetKey(ctx, prefixLoanTotalCollateral, asset.String())
	_, err := k.getUint64(ctx, key, &record)
	return cosmos.NewUint(record), err
}
