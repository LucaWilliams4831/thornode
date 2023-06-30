package ethereum

import (
	"encoding/json"
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"

	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/ethereum/types"
)

// PrefixTxStorage declares prefix to use in leveldb to avoid conflicts
const (
	PrefixBlockMeta    = `eth-blockmeta-`
	PrefixSignedTxItem = `signed-txitem-`
)

// LevelDBBlockMetaAccessor struct
type LevelDBBlockMetaAccessor struct {
	db *leveldb.DB
}

// NewLevelDBBlockMetaAccessor creates a new level db backed BlockMeta accessor
func NewLevelDBBlockMetaAccessor(db *leveldb.DB) (*LevelDBBlockMetaAccessor, error) {
	return &LevelDBBlockMetaAccessor{db: db}, nil
}

func (t *LevelDBBlockMetaAccessor) getBlockMetaKey(height int64) string {
	return fmt.Sprintf("%s%d", PrefixBlockMeta, height)
}

// GetBlockMeta at given block height ,  when the requested block meta doesn't exist , it will return nil , thus caller need to double check it
func (t *LevelDBBlockMetaAccessor) GetBlockMeta(height int64) (*types.BlockMeta, error) {
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
	var blockMeta types.BlockMeta
	if err := json.Unmarshal(v, &blockMeta); err != nil {
		return nil, fmt.Errorf("fail to unmarshal block meta from json: %w", err)
	}
	return &blockMeta, nil
}

// SaveBlockMeta persistent the given BlockMeta into storage
func (t *LevelDBBlockMetaAccessor) SaveBlockMeta(height int64, blockMeta *types.BlockMeta) error {
	key := t.getBlockMetaKey(height)
	buf, err := json.Marshal(blockMeta)
	if err != nil {
		return fmt.Errorf("fail to marshal block meta to json: %w", err)
	}
	return t.db.Put([]byte(key), buf, nil)
}

// GetBlockMetas returns all the block metas in storage
// The chain client will Prune block metas every time it finished scan a block , so at maximum it will keep BlockCacheSize blocks
// thus it should not grow out of control
func (t *LevelDBBlockMetaAccessor) GetBlockMetas() ([]*types.BlockMeta, error) {
	blockMetas := make([]*types.BlockMeta, 0)
	iterator := t.db.NewIterator(util.BytesPrefix([]byte(PrefixBlockMeta)), nil)
	defer iterator.Release()
	for iterator.Next() {
		buf := iterator.Value()
		if len(buf) == 0 {
			continue
		}
		var blockMeta types.BlockMeta
		if err := json.Unmarshal(buf, &blockMeta); err != nil {
			return nil, fmt.Errorf("fail to unmarshal block meta: %w", err)
		}
		blockMetas = append(blockMetas, &blockMeta)
	}
	return blockMetas, nil
}

// PruneBlockMeta remove all block meta that is older than the given block height
// with exception, if there are unspent transaction output in it , then the block meta will not be removed
func (t *LevelDBBlockMetaAccessor) PruneBlockMeta(height int64) error {
	iterator := t.db.NewIterator(util.BytesPrefix([]byte(PrefixBlockMeta)), nil)
	defer iterator.Release()
	targetToDelete := make([]string, 0)
	for iterator.Next() {
		buf := iterator.Value()
		if len(buf) == 0 {
			continue
		}
		var blockMeta types.BlockMeta
		if err := json.Unmarshal(buf, &blockMeta); err != nil {
			return fmt.Errorf("fail to unmarshal block meta: %w", err)
		}
		if blockMeta.Height < height {
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

func (t *LevelDBBlockMetaAccessor) getSignedTxItemKey(hash string) string {
	return PrefixSignedTxItem + hash
}

// AddSignedTxItem add a signed tx item to key value store
func (t *LevelDBBlockMetaAccessor) AddSignedTxItem(item SignedTxItem) error {
	key := t.getSignedTxItemKey(item.Hash)
	buf, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("fail to marshal signed tx item to json: %w", err)
	}
	return t.db.Put([]byte(key), buf, nil)
}

// RemoveSignedTxItem remove a signed item from key value store
func (t *LevelDBBlockMetaAccessor) RemoveSignedTxItem(hash string) error {
	key := t.getSignedTxItemKey(hash)
	return t.db.Delete([]byte(key), nil)
}

// GetSignedTxItems get all the signed tx items that in the key value store
func (t *LevelDBBlockMetaAccessor) GetSignedTxItems() ([]SignedTxItem, error) {
	txItems := make([]SignedTxItem, 0)
	iterator := t.db.NewIterator(util.BytesPrefix([]byte(PrefixSignedTxItem)), nil)
	defer iterator.Release()
	for iterator.Next() {
		buf := iterator.Value()
		if len(buf) == 0 {
			continue
		}
		var txItem SignedTxItem
		if err := json.Unmarshal(buf, &txItem); err != nil {
			return nil, fmt.Errorf("fail to unmarshal sign tx items: %w", err)
		}
		txItems = append(txItems, txItem)
	}
	return txItems, nil
}
