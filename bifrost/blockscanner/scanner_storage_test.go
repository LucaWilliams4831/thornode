package blockscanner

import (
	"gitlab.com/thorchain/thornode/config"
	. "gopkg.in/check.v1"
)

type BlockScannerStorageSuite struct{}

var _ = Suite(&BlockScannerStorageSuite{})

func (s *BlockScannerStorageSuite) TestScannerSetup(c *C) {
	tmpdir := "/tmp/scanner_storage"
	scanner, err := NewBlockScannerStorage(tmpdir, config.LevelDBOptions{})
	c.Assert(err, IsNil)
	c.Assert(scanner, NotNil)

	// in memory storage
	scanner, err = NewBlockScannerStorage("", config.LevelDBOptions{})
	c.Assert(err, IsNil)
	c.Assert(scanner, NotNil)
}
