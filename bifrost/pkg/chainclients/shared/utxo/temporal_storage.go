package utxo

import (
	"encoding/json"
	"fmt"

	lru "github.com/hashicorp/golang-lru"
	"github.com/rs/zerolog/log"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// -------------------------------------------------------------------------------------
// Config
// -------------------------------------------------------------------------------------

const (
	// TransactionFeeKey is the LevelDB key used to store the transaction fee of the most
	// recent signed transaction.
	// TODO: trailing "-" should be removed, will this cause issues with active bifrosts?
	TransactionFeeKey = "transactionfee-"

	// PrefixBlockMeta is the LevelDB key prefix used for storing BlockMeta. The height of
	// the block is appended for the final key.
	PrefixBlockMeta = "blockmeta-"

	// PrefixMempool is the LevelDB key prefix used for storing transactions in the
	// mempool. The hash of the transaction is appended for the final key.
	PrefixMempool = "mempool-"

	// PrefixObservedTx is the LevelDB key prefix used for storing observed transactions.
	// The hash of the transaction is appended for the final key.
	PrefixObservedTx = "observed-"
)

// -------------------------------------------------------------------------------------
// Types
// -------------------------------------------------------------------------------------

// PruneBlockMetaFunc defines a function type that is used to prune blocks from temporal
// storage. The function should return true if the block is eligible for pruning.
type PruneBlockMetaFunc func(meta *BlockMeta) bool

// TransactionFee represents the transaction fee on a UTXO chain.
type TransactionFee struct {
	// Fee is the transaction fee in the chain asset.
	Fee float64 `json:"fee"`

	// VSize is the estimated vbytes of the transaction. On chains with no concept of
	// transaction weight, this is simply the estimated byte size of the transaction.
	VSize int32 `json:"v_size"`
}

// -------------------------------------------------------------------------------------
// TemporalStorage
// -------------------------------------------------------------------------------------

// TemporalStorage provides persistent storage of block and transaction data over a
// window of recent time. This is used to track transactions we have sent, during re-org
// processing, and to ensure duplicate observations are not posted to Thorchain
// which could result in bond slash.
type TemporalStorage struct {
	db               *leveldb.DB
	mempoolTxIDCache *lru.Cache
}

func NewTemporalStorage(db *leveldb.DB, txidCacheSize int) (*TemporalStorage, error) {
	t := &TemporalStorage{db: db}

	if txidCacheSize > 0 {
		var err error
		t.mempoolTxIDCache, err = lru.New(txidCacheSize)
		if err != nil {
			log.Error().Err(err).Msg("failed to create mempool txid cache")
		}
	}

	return t, nil
}

// GetBlockMeta returns the BlockMeta at the provided height. Note that if the BlockMeta
// for the requested height is not found, we will return nil with nil error.
func (t *TemporalStorage) GetBlockMeta(height int64) (*BlockMeta, error) {
	key := t.getBlockMetaKey(height)
	exist, err := t.db.Has([]byte(key), nil)
	if err != nil {
		return nil, fmt.Errorf("fail to check whether block meta(%s) exist: %w", key, err)
	}
	if !exist {
		return nil, nil
	}
	v, err := t.db.Get([]byte(key), nil)
	if err != nil {
		return nil, fmt.Errorf("fail to get block meta(%s) from storage: %w", key, err)
	}
	var blockMeta BlockMeta
	if err := json.Unmarshal(v, &blockMeta); err != nil {
		return nil, fmt.Errorf("fail to unmarshal block meta from json: %w", err)
	}
	return &blockMeta, nil
}

// SaveBlockMeta will store the provided BlockMeta at the provided height.
func (t *TemporalStorage) SaveBlockMeta(height int64, blockMeta *BlockMeta) error {
	key := t.getBlockMetaKey(height)
	buf, err := json.Marshal(blockMeta)
	if err != nil {
		return fmt.Errorf("fail to marshal block meta to json: %w", err)
	}
	return t.db.Put([]byte(key), buf, nil)
}

// GetBlockMetas returns all the block metas in storage.
func (t *TemporalStorage) GetBlockMetas() ([]*BlockMeta, error) {
	blockMetas := make([]*BlockMeta, 0)
	iterator := t.db.NewIterator(util.BytesPrefix([]byte(PrefixBlockMeta)), nil)
	defer iterator.Release()
	for ; iterator.Next(); iterator.Valid() {
		buf := iterator.Value()
		if len(buf) == 0 {
			continue
		}
		var blockMeta BlockMeta
		if err := json.Unmarshal(buf, &blockMeta); err != nil {
			return nil, fmt.Errorf("fail to unmarshal block meta: %w", err)
		}
		blockMetas = append(blockMetas, &blockMeta)
	}
	return blockMetas, nil
}

// PruneBlockMeta removes all BlockMetas that are older than the provided block height
// and pass the provided filter function. Consumers should provide a function for the
// filter to ensure there are no transactions in the mempool corresponding to the block.
func (t *TemporalStorage) PruneBlockMeta(height int64, filter PruneBlockMetaFunc) error {
	iterator := t.db.NewIterator(util.BytesPrefix([]byte(PrefixBlockMeta)), nil)
	defer iterator.Release()
	targetToDelete := make([]string, 0)
	for ; iterator.Next(); iterator.Valid() {
		buf := iterator.Value()
		if len(buf) == 0 {
			continue
		}
		var blockMeta BlockMeta
		if err := json.Unmarshal(buf, &blockMeta); err != nil {
			return fmt.Errorf("fail to unmarshal block meta: %w", err)
		}

		if blockMeta.Height < height {
			if filter != nil && !filter(&blockMeta) {
				continue
			}
			targetToDelete = append(targetToDelete, t.getBlockMetaKey(blockMeta.Height))
		}
	}

	for _, key := range targetToDelete {
		if err := t.db.Delete([]byte(key), nil); err != nil {
			return fmt.Errorf("fail to delete block meta with key(%s) from storage: %w", key, err)
		}
	}
	return nil
}

// UpsertTransactionFee sets the latest transaction fee, overwriting any existing value.
func (t *TemporalStorage) UpsertTransactionFee(fee float64, vSize int32) error {
	transactionFee := TransactionFee{
		Fee:   fee,
		VSize: vSize,
	}
	buf, err := json.Marshal(transactionFee)
	if err != nil {
		return fmt.Errorf("fail to marshal transaction fee struct to json: %w", err)
	}
	return t.db.Put([]byte(TransactionFeeKey), buf, nil)
}

// GetTransactionFee returns the last transaction fee written to storage.
func (t *TemporalStorage) GetTransactionFee() (float64, int32, error) {
	buf, err := t.db.Get([]byte(TransactionFeeKey), nil)
	if err != nil {
		return 0.0, 0, fmt.Errorf("fail to get transaction fee from storage: %w", err)
	}
	var transactionFee TransactionFee
	if err := json.Unmarshal(buf, &transactionFee); err != nil {
		return 0.0, 0, fmt.Errorf("fail to unmarshal transaction fee: %w", err)
	}
	return transactionFee.Fee, transactionFee.VSize, nil
}

// TrackMempoolTx attempts to track the provided mempool txid. Returns true if the txid
// was successfully added, and false if the txid was already tracked or an error
// occurred during write.
func (t *TemporalStorage) TrackMempoolTx(txid string) (bool, error) {
	key := t.getMemPoolKey(txid)

	// first check the in memory id cache
	if t.mempoolTxIDCache != nil && t.mempoolTxIDCache.Contains(key) {
		return false, nil
	}

	exist, err := t.db.Has([]byte(key), nil)
	if err != nil {
		return exist, err
	}
	if exist {
		// update cache with existence
		if t.mempoolTxIDCache != nil {
			t.mempoolTxIDCache.Add(key, nil)
		}

		return false, nil
	}
	err = t.db.Put([]byte(key), []byte(txid), nil)

	// if successful, add to cache
	if err == nil && t.mempoolTxIDCache != nil {
		t.mempoolTxIDCache.Add(key, nil)
	}

	return true, err
}

// UntrackMempoolTx untracks the provided mempool txid.
func (t *TemporalStorage) UntrackMempoolTx(txid string) error {
	key := t.getMemPoolKey(txid)
	err := t.db.Delete([]byte(key), nil)

	// if successful, remove from cache
	if err == nil && t.mempoolTxIDCache != nil {
		t.mempoolTxIDCache.Remove(key)
	}

	return err
}

// TrackObservedTx attempts to track the provided observed txid. Returns
// true if the txid was successfully added, and false if the txid was already tracked or
// an error occurred during write.
func (t *TemporalStorage) TrackObservedTx(txid string) (bool, error) {
	key := t.getObservedTxKey(txid)
	exist, err := t.db.Has([]byte(key), nil)
	if err != nil {
		return exist, err
	}
	if exist {
		return false, nil
	}
	err = t.db.Put([]byte(key), []byte(txid), nil)
	return true, err
}

// UntrackObservedTx untracks the provided observed txid.
func (t *TemporalStorage) UntrackObservedTx(txid string) error {
	key := t.getObservedTxKey(txid)
	return t.db.Delete([]byte(key), nil)
}

// ------------------------------ internal ------------------------------

func (t *TemporalStorage) getBlockMetaKey(height int64) string {
	return fmt.Sprintf(PrefixBlockMeta+"%d", height)
}

func (t *TemporalStorage) getMemPoolKey(txid string) string {
	return PrefixMempool + txid
}

func (t *TemporalStorage) getObservedTxKey(txid string) string {
	return PrefixObservedTx + txid
}
