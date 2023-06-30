package blockscanner

import (
	"fmt"
	"io"

	"github.com/syndtr/goleveldb/leveldb"

	"gitlab.com/thorchain/thornode/bifrost/db"
	"gitlab.com/thorchain/thornode/config"
)

// ScannerStorage define the method need to be used by scanner
type ScannerStorage interface {
	GetScanPos() (int64, error)
	SetScanPos(block int64) error
	SetBlockScanStatus(block Block, status BlockScanStatus) error
	RemoveBlockStatus(block int64) error
	GetBlocksForRetry(failedOnly bool) ([]Block, error)
	GetInternalDb() *leveldb.DB
	io.Closer
}

// BlockScannerStorage
type BlockScannerStorage struct {
	*LevelDBScannerStorage
	db *leveldb.DB
}

func NewBlockScannerStorage(levelDbFolder string, opts config.LevelDBOptions) (*BlockScannerStorage, error) {
	ldb, err := db.NewLevelDB(levelDbFolder, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create level db: %w", err)
	}

	levelDbStorage, err := NewLevelDBScannerStorage(ldb)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}
	return &BlockScannerStorage{
		LevelDBScannerStorage: levelDbStorage,
		db:                    ldb,
	}, nil
}

func (s *BlockScannerStorage) GetInternalDb() *leveldb.DB {
	return s.db
}
