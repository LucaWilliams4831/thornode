package utxo

import (
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/x/thorchain"
)

type BitcoinTemporalStorageTestSuite struct{}

var _ = Suite(
	&BitcoinTemporalStorageTestSuite{},
)

func (s *BitcoinTemporalStorageTestSuite) TestNewTemporalStorage(c *C) {
	memStorage := storage.NewMemStorage()
	db, err := leveldb.Open(memStorage, nil)
	c.Assert(err, IsNil)
	dbTemporalStorage, err := NewTemporalStorage(db, 0)
	c.Assert(err, IsNil)
	c.Assert(dbTemporalStorage, NotNil)
	c.Assert(db.Close(), IsNil)
}

func (s *BitcoinTemporalStorageTestSuite) TestTemporalStorage(c *C) {
	memStorage := storage.NewMemStorage()
	db, err := leveldb.Open(memStorage, nil)
	c.Assert(err, IsNil)
	store, err := NewTemporalStorage(db, 0)
	c.Assert(err, IsNil)
	c.Assert(store, NotNil)

	blockMeta := NewBlockMeta("00000000000000d9cba4b81d1f8fb5cecd54e4ec3104763ba937aa7692a86dc5",
		1722479,
		"00000000000000ca7a4633264b9989355e9709f9e9da19506b0f636cc435dc8f")
	c.Assert(store.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)

	key := store.getBlockMetaKey(blockMeta.Height)
	c.Assert(key, Equals, fmt.Sprintf(PrefixBlockMeta+"%d", blockMeta.Height))

	bm, err := store.GetBlockMeta(blockMeta.Height)
	c.Assert(err, IsNil)
	c.Assert(bm, NotNil)

	nbm, err := store.GetBlockMeta(1024)
	c.Assert(err, IsNil)
	c.Assert(nbm, IsNil)
	hash := thorchain.GetRandomTxHash()
	for i := 0; i < 1024; i++ {
		bm := NewBlockMeta(thorchain.GetRandomTxHash().String(), int64(i), thorchain.GetRandomTxHash().String())
		if i == 0 {
			bm.AddSelfTransaction(hash.String())
		}
		c.Assert(store.SaveBlockMeta(bm.Height, bm), IsNil)
	}
	blockMetas, err := store.GetBlockMetas()
	c.Assert(err, IsNil)
	c.Assert(blockMetas, HasLen, 1025)
	c.Assert(store.PruneBlockMeta(1000, func(meta *BlockMeta) bool {
		return !meta.TransactionHashExists(hash.String())
	}), IsNil)
	allBlockMetas, err := store.GetBlockMetas()
	c.Assert(err, IsNil)
	c.Assert(allBlockMetas, HasLen, 26)

	fee, vSize, err := store.GetTransactionFee()
	c.Assert(err, NotNil)
	c.Assert(fee, Equals, 0.0)
	c.Assert(vSize, Equals, int32(0))
	// upsert transaction fee
	c.Assert(store.UpsertTransactionFee(1.0, 1), IsNil)
	fee, vSize, err = store.GetTransactionFee()
	c.Assert(err, IsNil)
	c.Assert(fee, Equals, 1.0)
	c.Assert(vSize, Equals, int32(1))
	c.Assert(db.Close(), IsNil)
}
