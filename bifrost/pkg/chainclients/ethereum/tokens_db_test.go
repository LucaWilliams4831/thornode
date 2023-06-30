package ethereum

import (
	"fmt"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
	. "gopkg.in/check.v1"
)

type EthereumTokenMetaTestSuite struct{}

var _ = Suite(
	&EthereumTokenMetaTestSuite{},
)

func (s *EthereumTokenMetaTestSuite) TestNewTokenMeta(c *C) {
	memStorage := storage.NewMemStorage()
	db, err := leveldb.Open(memStorage, nil)
	c.Assert(err, IsNil)
	dbTokenMeta, err := NewLevelDBTokenMeta(db)
	c.Assert(err, IsNil)
	c.Assert(dbTokenMeta, NotNil)
	c.Assert(db.Close(), IsNil)
}

func (s *EthereumTokenMetaTestSuite) TestTokenMeta(c *C) {
	memStorage := storage.NewMemStorage()
	db, err := leveldb.Open(memStorage, nil)
	c.Assert(err, IsNil)
	tokenMeta, err := NewLevelDBTokenMeta(db)
	c.Assert(err, IsNil)
	c.Assert(tokenMeta, NotNil)

	c.Assert(tokenMeta.SaveTokenMeta("TKN", "0xa7d9ddbe1f17865597fbd27ec712455208b6b76d", 18), IsNil)

	key := tokenMeta.getTokenMetaKey("0xa7d9ddbe1f17865597fbd27ec712455208b6b76d")
	c.Assert(key, Equals, fmt.Sprintf(prefixTokenMeta+"%s", strings.ToUpper("0xa7d9ddbe1f17865597fbd27ec712455208b6b76d")))

	tm, err := tokenMeta.GetTokenMeta("0xa7d9ddbe1f17865597fbd27ec712455208b6b76d")
	c.Assert(err, IsNil)
	c.Assert(tm, NotNil)

	ntm, err := tokenMeta.GetTokenMeta("0xa7d9ddbs1f17865597fbd27ec712455208b6b76d")
	c.Assert(err, IsNil)
	c.Assert(ntm.IsEmpty(), Equals, true)

	c.Assert(tokenMeta.SaveTokenMeta("TRN", "0xa7d9ddbs1f17865597fbd27ec712455208b6b76d", 18), IsNil)

	tokens, err := tokenMeta.GetTokens()
	c.Assert(err, IsNil)
	c.Assert(tokens, HasLen, 2)

	// make sure we are not going to set a new token
	c.Assert(tokenMeta.SaveTokenMeta("TRN", "0xA7D9ddbs1f17865597fbd27ec712455208b6b76d", 18), IsNil)
	tokens, err = tokenMeta.GetTokens()
	c.Assert(err, IsNil)
	c.Assert(tokens, HasLen, 2)
	c.Assert(db.Close(), IsNil)
}
