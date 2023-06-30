package observer

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"

	"gitlab.com/thorchain/thornode/bifrost/db"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/config"
)

// ObserverStorage save the ondeck tx in item to key value store , in case bifrost restart
type ObserverStorage struct {
	db *leveldb.DB
}

const (
	OnDeckTxKey = "ondeck-tx"
)

// NewObserverStorage create a new instance of LevelDBScannerStorage
func NewObserverStorage(path string, opts config.LevelDBOptions) (*ObserverStorage, error) {
	ldb, err := db.NewLevelDB(path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create observer storage: %w", err)
	}

	return &ObserverStorage{db: ldb}, nil
}

// GetOnDeckTxs retrieve the ondeck tx from key value store
func (s *ObserverStorage) GetOnDeckTxs() ([]types.TxIn, error) {
	buf, err := s.db.Get([]byte(OnDeckTxKey), nil)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("fail to get ondeck tx from key value store: %w", err)
	}
	var result []types.TxIn
	if err := json.Unmarshal(buf, &result); err != nil {
		return nil, fmt.Errorf("fail to unmarshal ondeck tx: %w", err)
	}
	return result, nil
}

// SetOnDeckTxs save the ondeck tx to key value store
func (s *ObserverStorage) SetOnDeckTxs(ondeck []types.TxIn) error {
	buf, err := json.Marshal(ondeck)
	if err != nil {
		return fmt.Errorf("fail to marshal ondeck tx to json: %w", err)
	}
	return s.db.Put([]byte(OnDeckTxKey), buf, nil)
}

func (s *ObserverStorage) Close() error {
	return s.db.Close()
}
