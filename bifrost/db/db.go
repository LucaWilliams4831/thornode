package db

import (
	"fmt"

	log "github.com/rs/zerolog/log"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"

	"gitlab.com/thorchain/thornode/config"
)

func NewLevelDB(path string, opts config.LevelDBOptions) (*leveldb.DB, error) {
	// if path is empty, use in memory db
	if path == "" {
		storage := storage.NewMemStorage()
		return leveldb.Open(storage, nil)
	}

	// open the database (or create)
	db, err := leveldb.OpenFile(path, opts.Options())
	if err != nil {
		return nil, fmt.Errorf("failed to open level db %s: %w", path, err)
	}

	// compact the database if configured
	if opts.CompactOnInit {
		log.Info().Str("path", path).Msg("compacting leveldb...")
		err = db.CompactRange(util.Range{})
		if err != nil {
			return nil, fmt.Errorf("failed to compact level db %s: %w", path, err)
		}
		log.Info().Str("path", path).Msg("leveldb compacted")
	}

	return db, nil
}
