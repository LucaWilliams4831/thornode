package thorclient

import (
	"net/http"
	"net/http/httptest"
	"strings"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/config"
)

type BlockHeightSuite struct {
	server  *httptest.Server
	bridge  *thorchainBridge
	cfg     config.BifrostClientConfiguration
	fixture string
}

var _ = Suite(&BlockHeightSuite{})

func (s *BlockHeightSuite) SetUpSuite(c *C) {
	s.server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.RequestURI, LastBlockEndpoint) {
			httpTestHandler(c, rw, s.fixture)
		}
	}))

	cfg, _, kb := SetupThorchainForTest(c)
	s.cfg = cfg
	s.cfg.ChainHost = s.server.Listener.Addr().String()
	var err error
	bridge, err := NewThorchainBridge(s.cfg, GetMetricForTest(c), NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd))
	var ok bool
	s.bridge, ok = bridge.(*thorchainBridge)
	c.Assert(ok, Equals, true)
	s.bridge.httpClient.RetryMax = 1
	c.Assert(err, IsNil)
	c.Assert(s.bridge, NotNil)
}

func (s *BlockHeightSuite) TearDownSuite(c *C) {
	s.server.Close()
}

func (s *BlockHeightSuite) TestGetBlockHeight(c *C) {
	s.fixture = "../../test/fixtures/endpoints/lastblock/bnb.json"
	height, err := s.bridge.GetBlockHeight()
	c.Assert(err, IsNil)
	c.Assert(height, NotNil)
	c.Assert(height, Equals, int64(4))
}

func (s *BlockHeightSuite) TestGetLastObservedInHeight(c *C) {
	s.fixture = "../../test/fixtures/endpoints/lastblock/bnb.json"
	height, err := s.bridge.GetLastObservedInHeight(common.BNBChain)
	c.Assert(err, IsNil)
	c.Assert(height, NotNil)
	c.Assert(height, Equals, int64(52875358))

	s.fixture = "../../test/fixtures/endpoints/lastblock/btc.json"
	height, err = s.bridge.GetLastObservedInHeight(common.BTCChain)
	c.Assert(err, IsNil)
	c.Assert(height, NotNil)
	c.Assert(height, Equals, int64(17))

	s.fixture = "../../test/fixtures/endpoints/lastblock/eth.json"
	height, err = s.bridge.GetLastObservedInHeight(common.ETHChain)
	c.Assert(err, IsNil)
	c.Assert(height, NotNil)
	c.Assert(height, Equals, int64(12345))
}

func (s *BlockHeightSuite) TestGetLastSignedHeight(c *C) {
	s.fixture = "../../test/fixtures/endpoints/lastblock/bnb.json"
	height, err := s.bridge.GetLastSignedOutHeight(common.BNBChain)
	c.Assert(err, IsNil)
	c.Assert(height, NotNil)
	c.Assert(height, Equals, int64(2))
}
