package gaia

import (
	"sync"

	"gitlab.com/thorchain/thornode/common"
)

type CosmosMetadata struct {
	AccountNumber int64
	SeqNumber     int64
	BlockHeight   int64
}

type CosmosMetaDataStore struct {
	lock  *sync.Mutex
	accts map[common.PubKey]CosmosMetadata
}

func NewCosmosMetaDataStore() *CosmosMetaDataStore {
	return &CosmosMetaDataStore{
		lock:  &sync.Mutex{},
		accts: make(map[common.PubKey]CosmosMetadata),
	}
}

func (b *CosmosMetaDataStore) Get(pk common.PubKey) CosmosMetadata {
	b.lock.Lock()
	defer b.lock.Unlock()
	if val, ok := b.accts[pk]; ok {
		return val
	}
	return CosmosMetadata{}
}

func (b *CosmosMetaDataStore) GetByAccount(acct int64) CosmosMetadata {
	b.lock.Lock()
	defer b.lock.Unlock()
	for _, meta := range b.accts {
		if meta.AccountNumber == acct {
			return meta
		}
	}
	return CosmosMetadata{}
}

func (b *CosmosMetaDataStore) Set(pk common.PubKey, meta CosmosMetadata) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.accts[pk] = meta
}

func (b *CosmosMetaDataStore) SeqInc(pk common.PubKey) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if meta, ok := b.accts[pk]; ok {
		meta.SeqNumber++
		b.accts[pk] = meta
	}
}
