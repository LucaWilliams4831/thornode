package ethereum

import (
	"encoding/json"
	"fmt"

	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/ethereum/types"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
)

type EthereumBlockMetaAccessorTestSuite struct{}

var _ = Suite(
	&EthereumBlockMetaAccessorTestSuite{},
)

func CreateBlock(height int) (*etypes.Header, error) {
	strHeight := fmt.Sprintf("%x", height)
	blockJson := `{
		"parentHash":"0x8b535592eb3192017a527bbf8e3596da86b3abea51d6257898b2ced9d3a83826",
		"difficulty": "0x31962a3fc82b",
		"extraData": "0x4477617266506f6f6c",
		"gasLimit": "0x47c3d8",
		"gasUsed": "0x0",
		"hash": "0x78bfef68fccd4507f9f4804ba5c65eb2f928ea45b3383ade88aaa720f1209cba",
		"logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		"miner": "0x2a65aca4d5fc5b5c859090a6c34d164135398226",
		"nonce": "0xa5e8fb780cc2cd5e",
		"number": "0x` + strHeight + `",
		"receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
		"sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		"size": "0x20e",
		"stateRoot": "0xdc6ed0a382e50edfedb6bd296892690eb97eb3fc88fd55088d5ea753c48253dc",
		"timestamp": "0x579f4981",
		"totalDifficulty": "0x25cff06a0d96f4bee",
		"transactionsRoot": "0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b"
	}`
	var header *etypes.Header
	if err := json.Unmarshal([]byte(blockJson), &header); err != nil {
		return nil, err
	}
	return header, nil
}

func (s *EthereumBlockMetaAccessorTestSuite) TestNewBlockMetaAccessor(c *C) {
	memStorage := storage.NewMemStorage()
	db, err := leveldb.Open(memStorage, nil)
	c.Assert(err, IsNil)
	dbBlockMetaAccessor, err := NewLevelDBBlockMetaAccessor(db)
	c.Assert(err, IsNil)
	c.Assert(dbBlockMetaAccessor, NotNil)
	c.Assert(db.Close(), IsNil)
}

func (s *EthereumBlockMetaAccessorTestSuite) TestBlockMetaAccessor(c *C) {
	memStorage := storage.NewMemStorage()
	db, err := leveldb.Open(memStorage, nil)
	c.Assert(err, IsNil)
	blockMetaAccessor, err := NewLevelDBBlockMetaAccessor(db)
	c.Assert(err, IsNil)
	c.Assert(blockMetaAccessor, NotNil)

	block, err := CreateBlock(1722479)
	c.Assert(err, IsNil)
	blockMeta := types.NewBlockMeta(block, stypes.TxIn{TxArray: []stypes.TxInItem{{Tx: "0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b"}}})
	c.Assert(blockMetaAccessor.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)

	key := blockMetaAccessor.getBlockMetaKey(blockMeta.Height)
	c.Assert(key, Equals, fmt.Sprintf(PrefixBlockMeta+"%d", blockMeta.Height))

	bm, err := blockMetaAccessor.GetBlockMeta(blockMeta.Height)
	c.Assert(err, IsNil)
	c.Assert(bm, NotNil)

	nbm, err := blockMetaAccessor.GetBlockMeta(1024)
	c.Assert(err, IsNil)
	c.Assert(nbm, IsNil)

	for i := 0; i < 1024; i++ {
		block, err = CreateBlock(i)
		c.Assert(err, IsNil)
		bm := types.NewBlockMeta(block, stypes.TxIn{TxArray: []stypes.TxInItem{{Tx: "0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b"}}})
		c.Assert(blockMetaAccessor.SaveBlockMeta(bm.Height, bm), IsNil)
	}
	blockMetas, err := blockMetaAccessor.GetBlockMetas()
	c.Assert(err, IsNil)
	c.Assert(blockMetas, HasLen, 1025)
	c.Assert(blockMetaAccessor.PruneBlockMeta(1000), IsNil)
	allBlockMetas, err := blockMetaAccessor.GetBlockMetas()
	c.Assert(err, IsNil)
	c.Assert(allBlockMetas, HasLen, 25)
	c.Assert(db.Close(), IsNil)
}
