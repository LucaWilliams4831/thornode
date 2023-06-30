package utxo

import (
	. "gopkg.in/check.v1"
)

type BlockMetaTestSuite struct{}

var _ = Suite(&BlockMetaTestSuite{})

func (b *BlockMetaTestSuite) TestBlockMeta(c *C) {
	blockMeta := NewBlockMeta("00000000000000d9cba4b81d1f8fb5cecd54e4ec3104763ba937aa7692a86dc5",
		1722479,
		"00000000000000ca7a4633264b9989355e9709f9e9da19506b0f636cc435dc8f")
	c.Assert(blockMeta, NotNil)

	txID := "31f8699ce9028e9cd37f8a6d58a79e614a96e3fdd0f58be5fc36d2d95484716f"
	blockMeta.AddCustomerTransaction(txID)
	c.Assert(blockMeta.CustomerTransactions, HasLen, 1)
	blockMeta.RemoveCustomerTransaction(txID)
	c.Assert(blockMeta.CustomerTransactions, HasLen, 0)

	txID = "9a7cd2192b78aaf4adfe6781ae5b12ba90fe5e1b509a593196b4103bef607330"
	blockMeta.AddSelfTransaction(txID)
	c.Assert(blockMeta.SelfTransactions, HasLen, 1)
	blockMeta.AddCustomerTransaction(txID)
	c.Assert(blockMeta.CustomerTransactions, HasLen, 0)
	c.Assert(blockMeta.TransactionHashExists(txID), Equals, true)
}
