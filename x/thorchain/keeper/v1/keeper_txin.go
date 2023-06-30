package keeperv1

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

// SetObservedTxInVoter - save a txin voter object
func (k KVStore) SetObservedTxInVoter(ctx cosmos.Context, tx ObservedTxVoter) {
	k.setObservedTxVoter(ctx, prefixObservedTxIn, tx)
}

// SetObservedTxOutVoter - save a txout voter object
func (k KVStore) SetObservedTxOutVoter(ctx cosmos.Context, tx ObservedTxVoter) {
	k.setObservedTxVoter(ctx, prefixObservedTxOut, tx)
}

func (k KVStore) setObservedTxVoter(ctx cosmos.Context, prefix types.DbPrefix, tx ObservedTxVoter) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefix, tx.String())
	buf := k.cdc.MustMarshal(&tx)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

// GetObservedTxInVoterIterator iterate tx in voters
func (k KVStore) GetObservedTxInVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getObservedTxVoterIterator(ctx, prefixObservedTxIn)
}

// GetObservedTxOutVoterIterator iterate tx out voters
func (k KVStore) GetObservedTxOutVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getObservedTxVoterIterator(ctx, prefixObservedTxOut)
}

func (k KVStore) getObservedTxVoterIterator(ctx cosmos.Context, prefix types.DbPrefix) cosmos.Iterator {
	return k.getIterator(ctx, prefix)
}

// GetObservedTxInVoter - gets information of an observed inbound tx based on the txid
func (k KVStore) GetObservedTxInVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error) {
	return k.getObservedTxVoter(ctx, prefixObservedTxIn, hash)
}

// GetObservedTxOutVoter - gets information of an observed outbound tx based on the txid
func (k KVStore) GetObservedTxOutVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error) {
	return k.getObservedTxVoter(ctx, prefixObservedTxOut, hash)
}

func (k KVStore) getObservedTxVoter(ctx cosmos.Context, prefix types.DbPrefix, hash common.TxID) (ObservedTxVoter, error) {
	record := ObservedTxVoter{TxID: hash}
	key := k.GetKey(ctx, prefix, hash.String())
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return record, nil
	}

	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, &record); err != nil {
		return record, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	return record, nil
}

func (k KVStore) SetObservedLink(ctx cosmos.Context, inhash, outhash common.TxID) {
	k.setObservedLink(ctx, inhash, outhash)
}

func (k KVStore) setObservedLink(ctx cosmos.Context, inhash, outhash common.TxID) {
	key := k.GetKey(ctx, prefixObservedLink, inhash.String())
	record := make([]string, 0)
	_, _ = k.getStrings(ctx, key, &record)
	for _, s := range record {
		if s == outhash.String() {
			return
		}
	}

	record = append(record, outhash.String())
	k.setStrings(ctx, key, record)
}

func (k KVStore) GetObservedLink(ctx cosmos.Context, inhash common.TxID) []common.TxID {
	hashes := make([]common.TxID, 0)
	strs := make([]string, 0)

	key := k.GetKey(ctx, prefixObservedLink, inhash.String())
	_, _ = k.getStrings(ctx, key, &strs)

	for _, s := range strs {
		hash, err := common.NewTxID(s)
		if err != nil {
			ctx.Logger().Error("failed to parse TXID", "txid", s, "error", err)
			continue
		}
		hashes = append(hashes, hash)
	}
	return hashes
}
