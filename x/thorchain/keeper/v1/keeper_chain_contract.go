package keeperv1

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStore) setChainContract(ctx cosmos.Context, key string, record ChainContract) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getChainContract(ctx cosmos.Context, key string, record *ChainContract) (bool, error) {
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

// SetChainContract - save chain contract address
func (k KVStore) SetChainContract(ctx cosmos.Context, cc ChainContract) {
	k.setChainContract(ctx, k.GetKey(ctx, prefixChainContract, cc.Chain.String()), cc)
}

// GetChainContract - gets chain contract
func (k KVStore) GetChainContract(ctx cosmos.Context, chain common.Chain) (ChainContract, error) {
	var record ChainContract
	_, err := k.getChainContract(ctx, k.GetKey(ctx, prefixChainContract, chain.String()), &record)
	return record, err
}

// GetChainContractIterator - get an iterator for chain contract
func (k KVStore) GetChainContractIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixChainContract)
}

// GetChainContracts return a list of chain contracts , which match the requested chains
func (k KVStore) GetChainContracts(ctx cosmos.Context, chains common.Chains) []ChainContract {
	contracts := make([]ChainContract, 0, len(chains))
	for _, item := range chains {
		cc, err := k.GetChainContract(ctx, item)
		if err != nil {
			ctx.Logger().Error("fail to get chain contract", "err", err, "chain", item.String())
			continue
		}
		if cc.IsEmpty() {
			continue
		}
		contracts = append(contracts, cc)
	}
	return contracts
}
