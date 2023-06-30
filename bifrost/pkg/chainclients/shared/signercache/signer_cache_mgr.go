package signercache

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/syndtr/goleveldb/leveldb"
)

// StorageAccessor define the necessary methods to access the key value store
type StorageAccessor interface {
	SetSigned(hash string) error
	HasSigned(hash string) bool
	RemoveSigned(transactionHash string) error
	SetTransactionHashMap(txOutItemHash, transactionHash string) error
}

// CacheManager maintain a store of the transaction that signer already signed
type CacheManager struct {
	logger          zerolog.Logger
	storageAccessor StorageAccessor
}

// NewSignerCacheManager create a new instance of CacheManager
func NewSignerCacheManager(db *leveldb.DB) (*CacheManager, error) {
	if db == nil {
		return nil, fmt.Errorf("db parameter is nil")
	}
	cacheStore := NewCacheStore(db)
	return &CacheManager{
		logger:          log.With().Str("module", "SignerCacheManager").Logger(),
		storageAccessor: cacheStore,
	}, nil
}

// SetSigned mark a tx out item has been signed
func (cm *CacheManager) SetSigned(txOutItemHash, transactionHash string) error {
	if err := cm.storageAccessor.SetSigned(txOutItemHash); err != nil {
		cm.logger.Err(err).
			Str("txout_hash", txOutItemHash).
			Str("transaction_hash", transactionHash).
			Msg("fail to set signed cache")
		return fmt.Errorf("fail to set signed cache %w", err)
	}
	return cm.storageAccessor.SetTransactionHashMap(txOutItemHash, transactionHash)
}

// HasSigned check whether the given tx out item has been signed before
func (cm *CacheManager) HasSigned(txOutItemHash string) bool {
	return cm.storageAccessor.HasSigned(txOutItemHash)
}

// RemoveSigned remove the given transaction hash related tx out item cache
// Usually this means the given transaction failed, so need to remove it from cache , so it can retry
func (cm *CacheManager) RemoveSigned(transactionHash string) {
	if err := cm.storageAccessor.RemoveSigned(transactionHash); err != nil {
		cm.logger.Err(err).Msgf("fail to remove signed transaction hash: %s", transactionHash)
	}
}
