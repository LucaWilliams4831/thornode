package ethereum

import (
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/ethereum/types"
	"gitlab.com/thorchain/thornode/bifrost/pubkeymanager"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/cmd"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/config"
	"gitlab.com/thorchain/thornode/x/thorchain"
)

type BlockScannerTestSuite struct {
	m      *metrics.Metrics
	bridge thorclient.ThorchainBridge
	keys   *thorclient.Keys
}

var _ = Suite(&BlockScannerTestSuite{})

func (s *BlockScannerTestSuite) SetUpSuite(c *C) {
	thorchain.SetupConfigForTest()
	s.m = GetMetricForTest(c)
	c.Assert(s.m, NotNil)
	cfg := config.BifrostClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       "localhost",
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: "",
	}

	kb := cKeys.NewInMemory()
	_, _, err := kb.NewMnemonic(cfg.SignerName, cKeys.English, cmd.THORChainHDPath, cfg.SignerPasswd, hd.Secp256k1)
	c.Assert(err, IsNil)
	thorKeys := thorclient.NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd)
	c.Assert(err, IsNil)
	s.keys = thorKeys
	s.bridge, err = thorclient.NewThorchainBridge(cfg, s.m, thorKeys)
	c.Assert(err, IsNil)
}

func getConfigForTest(rpcHost string) config.BifrostBlockScannerConfiguration {
	return config.BifrostBlockScannerConfiguration{
		RPCHost:                    rpcHost,
		StartBlockHeight:           1, // avoids querying thorchain for block height
		BlockScanProcessors:        1,
		HTTPRequestTimeout:         time.Second,
		HTTPRequestReadTimeout:     time.Second * 30,
		HTTPRequestWriteTimeout:    time.Second * 30,
		MaxHTTPRequestRetry:        3,
		BlockHeightDiscoverBackoff: time.Second,
		BlockRetryInterval:         time.Second,
		Concurrency:                1,
		GasCacheBlocks:             40,
	}
}

func (s *BlockScannerTestSuite) TestNewBlockScanner(c *C) {
	storage, err := blockscanner.NewBlockScannerStorage("", config.LevelDBOptions{})
	c.Assert(err, IsNil)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		c.Assert(err, IsNil)
		type RPCRequest struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      interface{}     `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		var rpcRequest RPCRequest
		err = json.Unmarshal(body, &rpcRequest)
		c.Assert(err, IsNil)
		if rpcRequest.Method == "eth_chainId" {
			_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x539"}`))
			c.Assert(err, IsNil)
		}
		if rpcRequest.Method == "eth_gasPrice" {
			_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
			c.Assert(err, IsNil)
		}
	}))
	ethClient, err := ethclient.Dial(server.URL)
	c.Assert(err, IsNil)
	pubKeyManager, err := pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	c.Assert(err, IsNil)
	solvencyReporter := func(height int64) error {
		return nil
	}
	bs, err := NewETHScanner(getConfigForTest(""), nil, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, s.m, pubKeyManager, solvencyReporter, nil)
	c.Assert(err, NotNil)
	c.Assert(bs, IsNil)

	bs, err = NewETHScanner(getConfigForTest("127.0.0.1"), storage, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, nil, pubKeyManager, solvencyReporter, nil)
	c.Assert(err, NotNil)
	c.Assert(bs, IsNil)

	bs, err = NewETHScanner(getConfigForTest("127.0.0.1"), storage, big.NewInt(int64(types.Mainnet)), nil, s.bridge, s.m, pubKeyManager, solvencyReporter, nil)
	c.Assert(err, NotNil)
	c.Assert(bs, IsNil)

	bs, err = NewETHScanner(getConfigForTest("127.0.0.1"), storage, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, s.m, nil, solvencyReporter, nil)
	c.Assert(err, NotNil)
	c.Assert(bs, IsNil)

	bs, err = NewETHScanner(getConfigForTest("127.0.0.1"), storage, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, s.m, pubKeyManager, solvencyReporter, nil)
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)
}

func (s *BlockScannerTestSuite) TestProcessBlock(c *C) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch {
		case req.RequestURI == thorclient.PubKeysEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/pubKeys.json")
		case req.RequestURI == thorclient.InboundAddressesEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/inbound_addresses/inbound_addresses.json")
		case req.RequestURI == thorclient.AsgardVault:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/asgard.json")
		case strings.HasPrefix(req.RequestURI, thorclient.NodeAccountEndpoint):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/nodeaccount/template.json")
		case strings.HasPrefix(req.RequestURI, thorclient.LastBlockEndpoint):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/lastblock/bnb.json")
		case strings.HasPrefix(req.RequestURI, thorclient.AuthAccountEndpoint):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/auth/accounts/template.json")
		default:
			body, err := io.ReadAll(req.Body)
			c.Assert(err, IsNil)
			defer func() {
				c.Assert(req.Body.Close(), IsNil)
			}()
			type RPCRequest struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      interface{}     `json:"id"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params"`
			}
			var rpcRequest RPCRequest
			err = json.Unmarshal(body, &rpcRequest)
			if err != nil {
				return
			}
			if rpcRequest.Method == "eth_chainId" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_gasPrice" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x3b9aca00"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_getTransactionReceipt" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0xe8c5","logsBloom":"0x00000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000800000000000000000000000800000000000000000001000000000000800000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000004000000000000000000000000000000000400000000000000000000000000000020000020000000000000000000000000000000000000000000000000000000000000","logs":[{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0xef519b7eb82aaf6ac376a6df2d793843ebfd593de5f1a0601d3cc6ab49ebb395","0x00000000000000000000000058e99c9c4a20f5f054c737389fdd51d7ed9c7d2a","0x0000000000000000000000000000000000000000000000000000000000000000"],"data":"0x0000000000000000000000000000000000000000000000004563918244f40000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000384144443a4554482e4554483a7474686f72313678786e30636164727575773661327177707633356176306d6568727976647a7a6a7a3361660000000000000000","blockNumber":"0x22","transactionHash":"0xa132791c8f868ac84bcffc0c2c8076f35c0b8fa1f7358428917892f0edddc550","transactionIndex":"0x0","blockHash":"0x2383a22acdbe27d3c7c56a0452ae5e7edfbebeabe3a9a047c87716dafc8fa9d0","logIndex":"0x0","removed":false}],"transactionHash":"0xa132791c8f868ac84bcffc0c2c8076f35c0b8fa1f7358428917892f0edddc550","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0xe8c5","blockHash":"0x2383a22acdbe27d3c7c56a0452ae5e7edfbebeabe3a9a047c87716dafc8fa9d0","blockNumber":"0x22","transactionIndex":"0x0"}}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_call" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x52554e45"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_getBlockByNumber" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"difficulty":"0x2","extraData":"0xd88301091a846765746888676f312e31352e36856c696e757800000000000000e86d9af8b427b780cd1e6f7cabd2f9231ccac25d313ed475351ed64ac19f21491461ed1fae732d3bbf73a5866112aec23b0ca436185685b9baee4f477a950f9400","gasLimit":"0x9e0f54","gasUsed":"0xabd3","hash":"0xb273789207ce61a1ec0314fdb88efe6c6b554a9505a97ff3dff05aa691e220ac","logsBloom":"0x00010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000040000000000000000010000200020000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000000000000000040000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000010000000000000000000000000000000000000000000000020000000000000","miner":"0x0000000000000000000000000000000000000000","mixHash":"0x0000000000000000000000000000000000000000000000000000000000000000","nonce":"0x0000000000000000","number":"0x6b","parentHash":"0xf18470c54efec284fb5ad57c0ee4afe2774d61393bd5224ac5484b39a0a07556","receiptsRoot":"0x794a74d56ec50769a1400f7ae0887061b0ec3ea6702589a0b45b9102df2c9954","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0x30a","stateRoot":"0x1c84090d7f5dc8137d6762e3d4babe10b30bf61fa827618346ae1ba8600a9629","timestamp":"0x6008f03a","totalDifficulty":"0xd7","transactions":[{"blockHash":"0xb273789207ce61a1ec0314fdb88efe6c6b554a9505a97ff3dff05aa691e220ac","blockNumber":"0x6b","from":"0xfabb9cc6ec839b1214bb11c53377a56a6ed81762","gas":"0x23273","gasPrice":"0x1","hash":"0x501d0b7fc8fcdff367280dc8b0c077f6beb9e324ad9550e2c0e34a2fa8e99aed","input":"0x095ea7b3000000000000000000000000e65e9d372f8cacc7b6dfcd4af6507851ed31bb4400000000000000000000000000000000000000000000000000000000ee6b2800","nonce":"0x1","to":"0x40bcd4db8889a8bf0b1391d0c819dcd9627f9d0a","transactionIndex":"0x0","value":"0x0","v":"0xa95","r":"0x614fa842510a4293d25ce4799a01a3d3cfeada4b79d7157c755bb4872984fff","s":"0x351e831427ca7e2f1b5f45b5101cc1d170d6fd8e7129378c8d55a6a436f403dc"}],"transactionsRoot":"0x4247bb112edbe20ee8cf406864b335f4a3aa215f65ea686c9820f056c637aca6","uncles":[]}}`))
				c.Assert(err, IsNil)
			}
		}
	}))
	ethClient, err := ethclient.Dial(server.URL)
	c.Assert(err, IsNil)
	c.Assert(ethClient, NotNil)
	storage, err := blockscanner.NewBlockScannerStorage("", config.LevelDBOptions{})
	c.Assert(err, IsNil)
	u, err := url.Parse(server.URL)
	c.Assert(err, IsNil)
	bridge, err := thorclient.NewThorchainBridge(config.BifrostClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       u.Host,
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: "",
	}, s.m, s.keys)
	c.Assert(err, IsNil)
	pubKeyMgr, err := pubkeymanager.NewPubKeyManager(bridge, s.m)
	c.Assert(err, IsNil)
	c.Assert(pubKeyMgr.Start(), IsNil)
	defer func() {
		c.Assert(pubKeyMgr.Stop(), IsNil)
	}()

	bs, err := NewETHScanner(getConfigForTest(server.URL), storage, big.NewInt(1337), ethClient, bridge, s.m, pubKeyMgr, func(height int64) error {
		return nil
	}, nil)
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)
	whitelistSmartContractAddress = append(whitelistSmartContractAddress, "0x40bcd4dB8889a8Bf0b1391d0c819dcd9627f9d0a")
	txIn, err := bs.FetchTxs(int64(1), int64(1))
	c.Assert(err, IsNil)
	c.Check(len(txIn.TxArray), Equals, 1)
}

func httpTestHandler(c *C, rw http.ResponseWriter, fixture string) {
	var content []byte
	var err error

	switch fixture {
	case "500":
		rw.WriteHeader(http.StatusInternalServerError)
	default:
		content, err = os.ReadFile(fixture)
		if err != nil {
			c.Fatal(err)
		}
	}

	rw.Header().Set("Content-Type", "application/json")
	if _, err := rw.Write(content); err != nil {
		c.Fatal(err)
	}
}

func (s *BlockScannerTestSuite) TestFromTxToTxIn(c *C) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch {
		case req.RequestURI == thorclient.PubKeysEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/pubKeys.json")
		case req.RequestURI == thorclient.InboundAddressesEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/inbound_addresses/inbound_addresses.json")
		case req.RequestURI == thorclient.AsgardVault:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/asgard.json")
		case strings.HasPrefix(req.RequestURI, thorclient.NodeAccountEndpoint):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/nodeaccount/template.json")
		default:
			body, err := io.ReadAll(req.Body)
			c.Assert(err, IsNil)
			type RPCRequest struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      interface{}     `json:"id"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params"`
			}
			var rpcRequest RPCRequest
			err = json.Unmarshal(body, &rpcRequest)
			if err != nil {
				return
			}
			if rpcRequest.Method == "eth_chainId" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_gasPrice" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_call" {
				if string(rpcRequest.Params) == `[{"data":"0x95d89b41","from":"0x0000000000000000000000000000000000000000","to":"0x3b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483"},"latest"]` ||
					string(rpcRequest.Params) == `[{"data":"0x95d89b41","from":"0x0000000000000000000000000000000000000000","to":"0x40bcd4db8889a8bf0b1391d0c819dcd9627f9d0a"},"latest"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000003544b4e0000000000000000000000000000000000000000000000000000000000"}`))
					c.Assert(err, IsNil)
					return
				} else if string(rpcRequest.Params) == `[{"data":"0x313ce567","from":"0x0000000000000000000000000000000000000000","to":"0x3b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483"},"latest"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
					c.Assert(err, IsNil)
					return
				}
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x52554e45"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_getTransactionReceipt" {
				switch string(rpcRequest.Params) {
				case `["0xa132791c8f868ac84bcffc0c2c8076f35c0b8fa1f7358428917892f0edddc550"]`:
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0xe8c5","logsBloom":"0x00000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000800000000000000000000000800000000000000000001000000000000800000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000004000000000000000000000000000000000400000000000000000000000000000020000020000000000000000000000000000000000000000000000000000000000000","logs":[{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0xef519b7eb82aaf6ac376a6df2d793843ebfd593de5f1a0601d3cc6ab49ebb395","0x00000000000000000000000058e99c9c4a20f5f054c737389fdd51d7ed9c7d2a","0x0000000000000000000000000000000000000000000000000000000000000000"],"data":"0x0000000000000000000000000000000000000000000000004563918244f40000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000384144443a4554482e4554483a7474686f72313678786e30636164727575773661327177707633356176306d6568727976647a7a6a7a3361660000000000000000","blockNumber":"0x22","transactionHash":"0xa132791c8f868ac84bcffc0c2c8076f35c0b8fa1f7358428917892f0edddc550","transactionIndex":"0x0","blockHash":"0x2383a22acdbe27d3c7c56a0452ae5e7edfbebeabe3a9a047c87716dafc8fa9d0","logIndex":"0x0","removed":false}],"transactionHash":"0xa132791c8f868ac84bcffc0c2c8076f35c0b8fa1f7358428917892f0edddc550","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0xe8c5","blockHash":"0x2383a22acdbe27d3c7c56a0452ae5e7edfbebeabe3a9a047c87716dafc8fa9d0","blockNumber":"0x22","transactionIndex":"0x0"}}`))
					c.Assert(err, IsNil)
					return
				case `["0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd"]`:
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0x13d20","logsBloom":"0x00000000000000000002000020000000000000000000000000000000000000000000000000004000000000000000000000000800000000000000000000000000000000000000000000000008000000000000000000000000000000000000000010000200000000000000000000000000000000000000000000000810000000000000000001010000000000800000000000000000000000000000000000040000000000000000002000000000000000000000000000003400000000000004000000000002000000000000000000000400000000000000000000000000000000000020000000000000000000002000000000000000010000800000000000000000","logs":[{"address":"0x3b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x0000000000000000000000003fd2d4ce97b082d4bce3f9fee2a3d60668d2f473","0x000000000000000000000000e65e9d372f8cacc7b6dfcd4af6507851ed31bb44"],"data":"0x0000000000000000000000000000000000000000000000004563918244f40000","blockNumber":"0x20","transactionHash":"0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd","transactionIndex":"0x0","blockHash":"0xe2ac172ea4c9b390adff7b21a4fe134251e60ba1d31a1acc0fb0d3bad350e34f","logIndex":"0x0","removed":false},{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0xef519b7eb82aaf6ac376a6df2d793843ebfd593de5f1a0601d3cc6ab49ebb395","0x00000000000000000000000058e99c9c4a20f5f054c737389fdd51d7ed9c7d2a","0x0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483"],"data":"0x0000000000000000000000000000000000000000000000004563918244f40000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000634144443a4554482e544b4e2d3078336237464134646432316336663942413363613337353231374541443743416239443662463438333a7474686f72313678786e30636164727575773661327177707633356176306d6568727976647a7a6a7a3361660000000000000000000000000000000000000000000000000000000000","blockNumber":"0x20","transactionHash":"0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd","transactionIndex":"0x0","blockHash":"0xe2ac172ea4c9b390adff7b21a4fe134251e60ba1d31a1acc0fb0d3bad350e34f","logIndex":"0x1","removed":false}],"transactionHash":"0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0x13d20","blockHash":"0xe2ac172ea4c9b390adff7b21a4fe134251e60ba1d31a1acc0fb0d3bad350e34f","blockNumber":"0x20","transactionIndex":"0x0"}}`))
					c.Assert(err, IsNil)
					return
				case `["0x4b8845b0d99c13bae6716b3c422cdb61aa141c0db04cfb18bcc031b76471595b"]`:
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0xecc1","logsBloom":"0x00000000000000000002010000000000000000000000000000000000000000000000000000000000000000000000000400000100000000000000002000000000000000000000000000000000000000000000000000040000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000100000004000000000000000000000000000000000000000000000000000000000000000000080000000000020000000000000000000000000000000000000000000000000000","logs":[{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0xa9cd03aa3c1b4515114539cd53d22085129d495cb9e9f9af77864526240f1bf7","0x0000000000000000000000005dcd69c5a0e2a6ccf7416c1c259063b88668a5ca","0x0000000000000000000000008d8bba78a27881294b34c82fb5978596e2df66dd"],"data":"0x0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000031f2ffcfc1f7c00000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000444f55543a4332323337423935393946434332443337323434383644414641363042413139343036353030393244333135353144383538343536314236303042434246343300000000000000000000000000000000000000000000000000000000","blockNumber":"0x60","transactionHash":"0x4b8845b0d99c13bae6716b3c422cdb61aa141c0db04cfb18bcc031b76471595b","transactionIndex":"0x0","blockHash":"0x8a60816fdc9649f754994aae1cb3ca952d14274d38fea797decc47c0c7a29188","logIndex":"0x0","removed":false}],"transactionHash":"0x4b8845b0d99c13bae6716b3c422cdb61aa141c0db04cfb18bcc031b76471595b","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0xecc1","blockHash":"0x8a60816fdc9649f754994aae1cb3ca952d14274d38fea797decc47c0c7a29188","blockNumber":"0x60","transactionIndex":"0x0"}}`))
					c.Assert(err, IsNil)
					return
				case `["0xe8d7b5ff2e2f3ae814dfd422444196a72349e03a761eda5452fcc244291fc599"]`:
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0x9a91","logsBloom":"0x00000000000000000002000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004000000000000000008000000000000000000000000000000000000000002000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000001000000000000004000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000010000800000000000000000","logs":[{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0x05b90458f953d3fcb2d7fb25616a2fddeca749d0c47cc5c9832d0266b5346eea","0x0000000000000000000000003fd2d4ce97b082d4bce3f9fee2a3d60668d2f473","0x0000000000000000000000009f4aab49a9cd8fc54dcb3701846f608a6f2c44da"],"data":"0x0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf48300000000000000000000000000000000000000000000000011572680468e44000000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000f59474744524153494c2b3a313032340000000000000000000000000000000000","blockNumber":"0x1a6","transactionHash":"0xe8d7b5ff2e2f3ae814dfd422444196a72349e03a761eda5452fcc244291fc599","transactionIndex":"0x0","blockHash":"0x39b72c414a032e8172f871c94e2382065c3e848ae69bb68f60114cb5b8fa7868","logIndex":"0x0","removed":false}],"transactionHash":"0xe8d7b5ff2e2f3ae814dfd422444196a72349e03a761eda5452fcc244291fc599","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0x9a91","blockHash":"0x39b72c414a032e8172f871c94e2382065c3e848ae69bb68f60114cb5b8fa7868","blockNumber":"0x1a6","transactionIndex":"0x0"}}`))
					c.Assert(err, IsNil)
					return
				case `["0x4b19cce0afd29141931f2c35e8805ab596c6467d19ddbde6268b606c8b258106"]`:
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"root":"0x","status":"0x1","cumulativeGasUsed":"0x13d20","logsBloom":"0x00000000000000000002000020000000000000000000000000000000000000000000000000004000000000000000000000000800000000000000000000000000000000000000000000000008000000000000000000000000000000000000000010000200000000000000000000000000000000000000000000000810000000000000000001010000000000800000000000000000000000000000000000040000000000000000002000000000000000000000000000003400000000000004000000000002000000000000000000000400000000000000000000000000000000000020000000000000000000002000000000000000010000800000000000000000","logs":[{"address":"0x3b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x0000000000000000000000003fd2d4ce97b082d4bce3f9fee2a3d60668d2f473","0x000000000000000000000000e65e9d372f8cacc7b6dfcd4af6507851ed31bb44"],"data":"0x0000000000000000000000000000000000000000000000004563918244f40000","blockNumber":"0x20","transactionHash":"0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd","transactionIndex":"0x0","blockHash":"0xe2ac172ea4c9b390adff7b21a4fe134251e60ba1d31a1acc0fb0d3bad350e34f","logIndex":"0x0","removed":false},{"address":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","topics":["0xef519b7eb82aaf6ac376a6df2d793843ebfd593de5f1a0601d3cc6ab49ebb395","0x00000000000000000000000058e99c9c4a20f5f054c737389fdd51d7ed9c7d2a","0x0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483"],"data":"0x0000000000000000000000000000000000000000000000004563918244f40000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000634144443a4554482e544b4e2d3078336237464134646432316336663942413363613337353231374541443743416239443662463438333a7474686f72313678786e30636164727575773661327177707633356176306d6568727976647a7a6a7a3361660000000000000000000000000000000000000000000000000000000000","blockNumber":"0x20","transactionHash":"0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd","transactionIndex":"0x0","blockHash":"0xe2ac172ea4c9b390adff7b21a4fe134251e60ba1d31a1acc0fb0d3bad350e34f","logIndex":"0x1","removed":false}],"transactionHash":"0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd","contractAddress":"0x0000000000000000000000000000000000000000","gasUsed":"0x13d20","blockHash":"0xe2ac172ea4c9b390adff7b21a4fe134251e60ba1d31a1acc0fb0d3bad350e34f","blockNumber":"0x20","transactionIndex":"0x0"}}`))
					c.Assert(err, IsNil)
					return
				}
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{
				"transactionHash":"0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
				"transactionIndex":"0x0",
				"blockNumber":"0x1",
				"blockHash":"0x78bfef68fccd4507f9f4804ba5c65eb2f928ea45b3383ade88aaa720f1209cba",
				"cumulativeGasUsed":"0xc350",
				"gasUsed":"0x4dc",
				"logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
				"logs":[],
				"status":"0x1"
			}}`))
				c.Assert(err, IsNil)
			}
		}
	}))
	ethClient, err := ethclient.Dial(server.URL)
	c.Assert(err, IsNil)
	c.Assert(ethClient, NotNil)
	storage, err := blockscanner.NewBlockScannerStorage("", config.LevelDBOptions{})
	c.Assert(err, IsNil)
	c.Assert(storage, NotNil)
	u, err := url.Parse(server.URL)
	c.Assert(err, IsNil)

	cfg := config.BifrostClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       u.Host,
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: "",
	}
	bridge, err := thorclient.NewThorchainBridge(cfg, s.m, s.keys)
	c.Assert(err, IsNil)
	c.Assert(bridge, NotNil)
	pkeyMgr, err := pubkeymanager.NewPubKeyManager(bridge, s.m)
	c.Assert(pkeyMgr.Start(), IsNil)
	defer func() {
		c.Assert(pkeyMgr.Stop(), IsNil)
	}()
	c.Assert(err, IsNil)
	bs, err := NewETHScanner(getConfigForTest(server.URL), storage, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, s.m, pkeyMgr, func(height int64) error {
		return nil
	}, nil)
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)

	// send directly to ETH address
	encodedTx := `{
		"blockHash":"0x1d59ff54b1eb26b013ce3cb5fc9dab3705b415a67127a003c3e61eb445bb8df2",
		"blockNumber":"0x5daf3b",
		"from":"0xa7d9ddbe1f17865597fbd27ec712455208b6b76d",
		"gas":"0xc350",
		"gasPrice":"0x4a817c800",
		"hash":"0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
		"input":"0x68656c6c6f21",
		"nonce":"0x15",
		"to":"0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb",
		"transactionIndex":"0x41",
		"value":"0xf3dbb76162000",
		"v":"0x25",
		"r":"0x1b5e176d927f8e9ab405058b2d2457392da3e20f328b16ddabcebc33eaac5fea",
		"s":"0x4ba69724e8f69de52f0125ad8b3c5c2cef33019bac3249e2c0a2192766d1721c"
	}`
	tx := etypes.NewTransaction(0, common.HexToAddress(ethToken), nil, 0, nil, nil)
	err = tx.UnmarshalJSON([]byte(encodedTx))
	c.Assert(err, IsNil)

	txInItem, err := bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Check(txInItem.Sender, Equals, "0xa7d9ddbe1f17865597fbd27ec712455208b6b76d")
	c.Check(txInItem.To, Equals, "0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb")
	c.Check(len(txInItem.Coins), Equals, 1)

	c.Check(txInItem.Coins[0].Asset.String(), Equals, "ETH.ETH")
	c.Check(
		txInItem.Coins[0].Amount.Equal(cosmos.NewUint(429000)),
		Equals,
		true,
	)
	c.Check(
		txInItem.Gas[0].Amount.Equal(cosmos.NewUint(100000)),
		Equals,
		true,
	)

	bs, err = NewETHScanner(getConfigForTest(server.URL), storage, big.NewInt(1337), ethClient, s.bridge, s.m, pkeyMgr, func(height int64) error {
		return nil
	}, nil)
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)
	// smart contract - deposit
	encodedTx = `{"nonce":"0x4","gasPrice":"0x1","gas":"0x177b8","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","value":"0x0","input":"0x1fece7b400000000000000000000000058e99c9c4a20f5f054c737389fdd51d7ed9c7d2a0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf4830000000000000000000000000000000000000000000000004563918244f40000000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000634144443a4554482e544b4e2d3078336237464134646432316336663942413363613337353231374541443743416239443662463438333a7474686f72313678786e30636164727575773661327177707633356176306d6568727976647a7a6a7a3361660000000000000000000000000000000000000000000000000000000000","v":"0xa95","r":"0x8a82b49901d67748c6840d7417d7307a40e6093579f6f73f7222cb52622f92cd","s":"0x21a1097c02306b177a0ca1a6e9f9599a8c4bab9926893493e966253c436977fd","hash":"0x817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd"}`
	tx = etypes.NewTransaction(0, common.HexToAddress(ethToken), nil, 0, nil, nil)
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473")
	c.Assert(txInItem.To, Equals, "0x58e99C9c4a20f5F054C737389FdD51D7eD9c7d2a")
	c.Assert(txInItem.Memo, Equals, "ADD:ETH.TKN-0x3b7FA4dd21c6f9BA3ca375217EAD7CAb9D6bF483:tthor16xxn0cadruuw6a2qwpv35av0mehryvdzzjz3af")
	c.Assert(txInItem.Tx, Equals, "817665ed5d08f6bcc47e409c147187fe0450201152ea1c80c85edf103d623acd")
	c.Assert(txInItem.Coins[0].Asset.String(), Equals, "ETH.TKN-0X3B7FA4DD21C6F9BA3CA375217EAD7CAB9D6BF483")
	c.Assert(txInItem.Coins[0].Amount.Equal(cosmos.NewUint(500000000)), Equals, true)

	bs, err = NewETHScanner(getConfigForTest(server.URL), storage, big.NewInt(1337), ethClient, s.bridge, s.m, pkeyMgr, func(height int64) error {
		return nil
	}, nil)
	// whitelist the address for test
	whitelistSmartContractAddress = append(whitelistSmartContractAddress,
		"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44",
		"0x81a392e6a757d58a7eb6781a775a3449da3b9df5")
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)
	// smart contract - deposit via smart contract (transaction to != router)
	encodedTx = `{"nonce":"0x4","gasPrice":"0x1","gas":"0x177b8","to":"0x81a392e6a757d58a7eb6781a775a3449da3b9df5","value":"0x0","input":"0x1fece7b400000000000000000000000058e99c9c4a20f5f054c737389fdd51d7ed9c7d2a0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf4830000000000000000000000000000000000000000000000004563918244f40000000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000634144443a4554482e544b4e2d3078336237464134646432316336663942413363613337353231374541443743416239443662463438333a7474686f72313678786e30636164727575773661327177707633356176306d6568727976647a7a6a7a3361660000000000000000000000000000000000000000000000000000000000","v":"0xa95","r":"0x8a82b49901d67748c6840d7417d7307a40e6093579f6f73f7222cb52622f92cd","s":"0x21a1097c02306b177a0ca1a6e9f9599a8c4bab9926893493e966253c436977fd","hash":"0x94ac3936bf227f830e21f9f852bec127086024f327d41862455b3d5f101d18c5"}`
	tx = etypes.NewTransaction(0, common.HexToAddress(ethToken), nil, 0, nil, nil)
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x26355f70ede2642c609d1d4894d608232bf1fd8c")
	c.Assert(txInItem.To, Equals, "0x58e99C9c4a20f5F054C737389FdD51D7eD9c7d2a")
	c.Assert(txInItem.Memo, Equals, "ADD:ETH.TKN-0x3b7FA4dd21c6f9BA3ca375217EAD7CAb9D6bF483:tthor16xxn0cadruuw6a2qwpv35av0mehryvdzzjz3af")
	c.Assert(txInItem.Tx, Equals, "4b19cce0afd29141931f2c35e8805ab596c6467d19ddbde6268b606c8b258106")
	c.Assert(txInItem.Coins[0].Asset.String(), Equals, "ETH.TKN-0X3B7FA4DD21C6F9BA3CA375217EAD7CAB9D6BF483")
	c.Assert(txInItem.Coins[0].Amount.Equal(cosmos.NewUint(500000000)), Equals, true)

	// smart contract - depositETH
	encodedTx = `{"nonce":"0x5","gasPrice":"0x1","gas":"0xe8c5","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","value":"0x4563918244f40000","input":"0x1fece7b400000000000000000000000058e99c9c4a20f5f054c737389fdd51d7ed9c7d2a00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000384144443a4554482e4554483a7474686f72313678786e30636164727575773661327177707633356176306d6568727976647a7a6a7a3361660000000000000000","v":"0xa96","r":"0x46b81d77656e26b199438349244593b9f3131224acfc39a7e0c09e2cd08dc1d8","s":"0x36427688c3ffef46b9c99fd2b0f8e191b85dae908f9d76116a878317398382ad","hash":"0xa132791c8f868ac84bcffc0c2c8076f35c0b8fa1f7358428917892f0edddc550"}`
	tx = &etypes.Transaction{}
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473")
	c.Assert(txInItem.To, Equals, "0x58e99C9c4a20f5F054C737389FdD51D7eD9c7d2a")
	c.Assert(txInItem.Memo, Equals, "ADD:ETH.ETH:tthor16xxn0cadruuw6a2qwpv35av0mehryvdzzjz3af")
	c.Assert(txInItem.Tx, Equals, "a132791c8f868ac84bcffc0c2c8076f35c0b8fa1f7358428917892f0edddc550")
	c.Assert(txInItem.Coins[0].Asset.String(), Equals, "ETH.ETH")
	c.Assert(txInItem.Coins[0].Amount.Equal(cosmos.NewUint(500000000)), Equals, true)

	// smart contract - transferOut
	encodedTx = `{"nonce":"0x0","gasPrice":"0x2540be400","gas":"0xecc1","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","value":"0x31f2ffcfc1f7c00","input":"0x574da7170000000000000000000000008d8bba78a27881294b34c82fb5978596e2df66dd0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000031d13d4898b6000000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000444f55543a4332323337423935393946434332443337323434383644414641363042413139343036353030393244333135353144383538343536314236303042434246343300000000000000000000000000000000000000000000000000000000","v":"0xa96","r":"0xb27f9fff5cc936d5918aa557c9c4df559e3e4f6c4ac5b0b79d43c4e3bdcb91e","s":"0x1417cedea6a9b879bd24d547b29c05d214100bfc586a32a1c24de3a090528f62","hash":"0x4b8845b0d99c13bae6716b3c422cdb61aa141c0db04cfb18bcc031b76471595b"}`
	tx = &etypes.Transaction{}
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x5dcd69c5a0e2a6ccf7416c1c259063b88668a5ca")
	c.Assert(txInItem.To, Equals, "0x8d8Bba78A27881294b34c82Fb5978596e2DF66dD")
	c.Assert(txInItem.Memo, Equals, "OUT:C2237B9599FCC2D3724486DAFA60BA1940650092D31551D8584561B600BCBF43")
	c.Assert(txInItem.Tx, Equals, "4b8845b0d99c13bae6716b3c422cdb61aa141c0db04cfb18bcc031b76471595b")
	c.Assert(txInItem.Coins[0].Asset.String(), Equals, "ETH.ETH")
	c.Assert(txInItem.Coins[0].Amount.Equal(cosmos.NewUint(22495127)), Equals, true)

	// smart contract - allowance
	encodedTx = `{"nonce":"0xb","gasPrice":"0x1","gas":"0xd529","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44","value":"0x0","input":"0x1b738b32000000000000000000000000e65e9d372f8cacc7b6dfcd4af6507851ed31bb440000000000000000000000009f4aab49a9cd8fc54dcb3701846f608a6f2c44da0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483000000000000000000000000000000000000000000000000ad67810426efff1800000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000","v":"0xa96","r":"0x967771b4ec53f895b6f6a2e8b4febbfd04fba079b5f1ab3c6476d9d612cc23d5","s":"0x2cc999ea73cd67cac387a0c5fa49cf6eeab8de1b4602ad376f788a3b700b97fa","hash":"0xe8d7b5ff2e2f3ae814dfd422444196a72349e03a761eda5452fcc244291fc599"}`
	tx = &etypes.Transaction{}
	c.Assert(tx.UnmarshalJSON([]byte(encodedTx)), IsNil)
	txInItem, err = bs.fromTxToTxIn(tx)
	c.Assert(err, IsNil)
	c.Assert(txInItem, NotNil)
	c.Assert(txInItem.Sender, Equals, "0x3fd2d4ce97b082d4bce3f9fee2a3d60668d2f473")
	c.Assert(txInItem.To, Equals, "0x9F4AaB49A9cd8FC54Dcb3701846f608a6f2C44dA")
	c.Assert(txInItem.Memo, Equals, "YGGDRASIL+:1024")
	c.Assert(txInItem.Tx, Equals, "e8d7b5ff2e2f3ae814dfd422444196a72349e03a761eda5452fcc244291fc599")
	c.Assert(txInItem.Coins[0].Asset.String(), Equals, "ETH.TKN-0X3B7FA4DD21C6F9BA3CA375217EAD7CAB9D6BF483")
	c.Logf("======> %+v \n", txInItem)
	c.Assert(txInItem.Coins[0].Amount.Equal(cosmos.NewUint(124950975)), Equals, true)
}

func (s *BlockScannerTestSuite) TestProcessReOrg(c *C) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch {
		case req.RequestURI == thorclient.PubKeysEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/pubKeys.json")
		case req.RequestURI == thorclient.InboundAddressesEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/inbound_addresses/inbound_addresses.json")
		case req.RequestURI == thorclient.AsgardVault:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/asgard.json")
		case strings.HasPrefix(req.RequestURI, thorclient.NodeAccountEndpoint):
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/nodeaccount/template.json")
		default:
			body, err := io.ReadAll(req.Body)
			c.Assert(err, IsNil)
			type RPCRequest struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      interface{}     `json:"id"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params"`
			}
			var rpcRequest RPCRequest
			err = json.Unmarshal(body, &rpcRequest)
			c.Assert(err, IsNil)
			if rpcRequest.Method == "eth_chainId" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_gasPrice" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_getTransactionReceipt" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32700,"message":"Not found tx"},"id": null}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_call" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x52554e45"}`))
				c.Assert(err, IsNil)
			}
			if rpcRequest.Method == "eth_getBlockByNumber" {
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{
				"parentHash":"0x8b535592eb3192017a527bbf8e3596da86b3abea51d6257898b2ced9d3a83826",
				"difficulty": "0x31962a3fc82b",
				"extraData": "0x4477617266506f6f6c",
				"gasLimit": "0x47c3d8",
				"gasUsed": "0x0",
				"hash": "0x78bfef68fccd4507f9f4804ba5c65eb2f928ea45b3383ade88aaa720f1209cba",
				"logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
				"miner": "0x2a65aca4d5fc5b5c859090a6c34d164135398226",
				"nonce": "0xa5e8fb780cc2cd5e",
				"number": "0x0",
				"receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
				"sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
				"size": "0x20e",
				"stateRoot": "0xdc6ed0a382e50edfedb6bd296892690eb97eb3fc88fd55088d5ea753c48253dc",
				"timestamp": "0x579f4981",
				"totalDifficulty": "0x25cff06a0d96f4bee",
				"transactions": [{
					"blockHash":"0x78bfef68fccd4507f9f4804ba5c65eb2f928ea45b3383ade88aaa720f1209cba",
					"blockNumber":"0x1",
					"from":"0xa7d9ddbe1f17865597fbd27ec712455208b6b76d",
					"gas":"0xc350",
					"gasPrice":"0x4a817c800",
					"hash":"0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
					"input":"0x68656c6c6f21",
					"nonce":"0x15",
					"to":"0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb",
					"transactionIndex":"0x0",
					"value":"0xf3dbb76162000",
					"v":"0x25",
					"r":"0x1b5e176d927f8e9ab405058b2d2457392da3e20f328b16ddabcebc33eaac5fea",
					"s":"0x4ba69724e8f69de52f0125ad8b3c5c2cef33019bac3249e2c0a2192766d1721c"
				}],
				"transactionsRoot": "0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
				"uncles": [
			]}}`))
				c.Assert(err, IsNil)
			}
		}
	}))
	ethClient, err := ethclient.Dial(server.URL)
	c.Assert(err, IsNil)
	c.Assert(ethClient, NotNil)
	storage, err := blockscanner.NewBlockScannerStorage("", config.LevelDBOptions{})
	c.Assert(err, IsNil)
	bridge, err := thorclient.NewThorchainBridge(config.BifrostClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       server.Listener.Addr().String(),
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: "",
	}, s.m, s.keys)
	c.Assert(err, IsNil)
	c.Assert(bridge, NotNil)
	pkeyMgr, err := pubkeymanager.NewPubKeyManager(bridge, s.m)
	c.Assert(err, IsNil)
	c.Assert(pkeyMgr.Start(), IsNil)
	defer func() {
		c.Assert(pkeyMgr.Stop(), IsNil)
	}()
	bs, err := NewETHScanner(getConfigForTest(server.URL), storage, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, s.m, pkeyMgr, func(height int64) error {
		return nil
	}, nil)
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)
	block, err := CreateBlock(0)
	c.Assert(err, IsNil)
	c.Assert(block, NotNil)
	blockNew, err := CreateBlock(1)
	c.Assert(err, IsNil)
	c.Assert(blockNew, NotNil)
	blockMeta := types.NewBlockMeta(block, stypes.TxIn{TxArray: []stypes.TxInItem{{Tx: "0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b"}}})
	blockMeta.Transactions = append(blockMeta.Transactions, types.TransactionMeta{
		Hash:        "0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
		BlockHeight: block.Number.Int64(),
	})
	// add one UTXO which will trigger the re-org process next
	c.Assert(bs.blockMetaAccessor.SaveBlockMeta(0, blockMeta), IsNil)
	bs.globalErrataQueue = make(chan stypes.ErrataBlock, 1)
	reorgedBlocks, err := bs.processReorg(blockNew)
	c.Assert(err, IsNil)
	c.Assert(reorgedBlocks, IsNil)
	// make sure there is errata block in the queue
	c.Assert(bs.globalErrataQueue, HasLen, 1)
	blockMeta, err = bs.blockMetaAccessor.GetBlockMeta(0)
	c.Assert(err, IsNil)
	c.Assert(blockMeta, NotNil)
}

// -------------------------------------------------------------------------------------
// GasPriceV2
// -------------------------------------------------------------------------------------

func (s *BlockScannerTestSuite) TestGasPriceV2(c *C) {
	storage, err := blockscanner.NewBlockScannerStorage("", config.LevelDBOptions{})
	c.Assert(err, IsNil)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		c.Assert(err, IsNil)
		type RPCRequest struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      interface{}     `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		var rpcRequest RPCRequest
		err = json.Unmarshal(body, &rpcRequest)
		c.Assert(err, IsNil)
		if rpcRequest.Method == "eth_chainId" {
			_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x539"}`))
			c.Assert(err, IsNil)
		}
		if rpcRequest.Method == "eth_gasPrice" {
			_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
			c.Assert(err, IsNil)
		}
	}))
	ethClient, err := ethclient.Dial(server.URL)
	c.Assert(err, IsNil)
	pubKeyManager, err := pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	c.Assert(err, IsNil)
	solvencyReporter := func(height int64) error {
		return nil
	}
	conf := getConfigForTest("127.0.0.1")
	bs, err := NewETHScanner(conf, storage, big.NewInt(int64(types.Mainnet)), ethClient, s.bridge, s.m, pubKeyManager, solvencyReporter, nil)
	c.Assert(err, IsNil)
	c.Assert(bs, NotNil)

	// almost fill gas cache
	for i := 0; i < 39; i++ {
		bs.updateGasPriceV2([]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4)})
	}

	// empty blocks should not count
	bs.updateGasPriceV2([]*big.Int{})
	c.Assert(len(bs.gasCache), Equals, 39)
	c.Assert(bs.gasPrice.Cmp(big.NewInt(initialGasPrice)), Equals, 0)

	// now we should get the average of the 25th percentile gas (2)
	bs.updateGasPriceV2([]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4)})
	c.Assert(len(bs.gasCache), Equals, 40)
	c.Assert(bs.gasPrice.Uint64(), Equals, big.NewInt(2).Uint64())

	// add 20 more blocks with 2x the 25th percentile and we should get 6 (3 + 3x stddev)
	for i := 0; i < 20; i++ {
		bs.updateGasPriceV2([]*big.Int{big.NewInt(2), big.NewInt(4), big.NewInt(6), big.NewInt(8)})
	}
	c.Assert(len(bs.gasCache), Equals, 40)
	c.Assert(bs.gasPrice.Uint64(), Equals, big.NewInt(6).Uint64())

	// add 20 more blocks with 2x the 25th percentile and we should get 4
	for i := 0; i < 20; i++ {
		bs.updateGasPriceV2([]*big.Int{big.NewInt(2), big.NewInt(4), big.NewInt(6), big.NewInt(8)})
	}
	c.Assert(len(bs.gasCache), Equals, 40)
	c.Assert(bs.gasPrice.Uint64(), Equals, big.NewInt(4).Uint64())

	// add 20 more blocks with 2x the 25th percentile and we should get 12 (6 + 3x stddev)
	for i := 0; i < 20; i++ {
		bs.updateGasPriceV2([]*big.Int{big.NewInt(4), big.NewInt(8), big.NewInt(12), big.NewInt(16)})
	}
	c.Assert(len(bs.gasCache), Equals, 40)
	c.Assert(bs.gasPrice.Uint64(), Equals, big.NewInt(12).Uint64())
}
