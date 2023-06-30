package ethereum

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pubkeymanager"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/cmd"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/config"
	types2 "gitlab.com/thorchain/thornode/x/thorchain/types"
	. "gopkg.in/check.v1"
)

type UnstuckTestSuite struct {
	thorKeys *thorclient.Keys
	bridge   thorclient.ThorchainBridge
	m        *metrics.Metrics
	server   *httptest.Server
}

var _ = Suite(&UnstuckTestSuite{})

func (s *UnstuckTestSuite) SetUpTest(c *C) {
	s.m = GetMetricForTest(c)
	c.Assert(s.m, NotNil)
	types2.SetupConfigForTest()
	c.Assert(os.Setenv("NET", "testnet"), IsNil)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.RequestURI {
		case thorclient.PubKeysEndpoint:
			priKey, _ := s.thorKeys.GetPrivateKey()
			tm, _ := codec.ToTmPubKeyInterface(priKey.PubKey())
			pk, err := common.NewPubKeyFromCrypto(tm)
			c.Assert(err, IsNil)
			content, err := os.ReadFile("../../../../test/fixtures/endpoints/vaults/pubKeys.json")
			c.Assert(err, IsNil)
			var pubKeysVault types2.QueryVaultsPubKeys
			c.Assert(json.Unmarshal(content, &pubKeysVault), IsNil)
			pubKeysVault.Yggdrasil = append(pubKeysVault.Yggdrasil, types2.QueryVaultPubKeyContract{
				PubKey: pk,
				Routers: []types2.ChainContract{
					{
						Chain:  common.ETHChain,
						Router: "0xE65e9d372F8cAcc7b6dfcd4af6507851Ed31bb44",
					},
				},
			})
			buf, err := json.MarshalIndent(pubKeysVault, "", "	")
			c.Assert(err, IsNil)
			_, err = rw.Write(buf)
			c.Assert(err, IsNil)
		case thorclient.LastBlockEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/eth/last_block_height.json")
		case thorclient.InboundAddressesEndpoint:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/inbound_addresses/inbound_addresses.json")
		case thorclient.AsgardVault:
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/vaults/asgard.json")
		case thorclient.NodeAccountEndpoint:
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
			fmt.Println("rpc request:", rpcRequest.Method)
			switch rpcRequest.Method {
			case "eth_chainId":
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0xf"}`))
				c.Assert(err, IsNil)
				return
			case "eth_getTransactionByHash":
				var hashes []string
				c.Assert(json.Unmarshal(rpcRequest.Params, &hashes), IsNil)
				if hashes[0] == "0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b" {
					_, err := rw.Write([]byte(`{
    "jsonrpc": "2.0",
    "id": 1,
    "result": {
        "nonce": "0x2",
        "gasPrice": "0x1",
        "gas": "0x13990",
        "to": "0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44",
        "value": "0x22b1c8c1227a00000",
        "input": "0x1fece7b4000000000000000000000000f6da288748ec4c77642f6c5543717539b3ae001b00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000045345454400000000000000000000000000000000000000000000000000000000",
        "v": "0xa96",
        "r": "0x4fed375d064158c79dd0ee1e35cbfbe6e19ed7c0005763ca1edc10121124d1fd",
        "s": "0x56d194669c9188176ed87e96b4bd2e2b3869cdb959c1153a557e3d8d8d48c12c",
        "hash": "0x81604fe8c8df8b5e32daafa00acd06ec97281ed3056ab368cf57e2dcacd7e2d1"
    }
}`))
					c.Assert(err, IsNil)
				} else if hashes[0] == "0x96395fbdb39e33293999dc1a0a3b87c8a9e51185e177760d1482c2155bb35b87" {
					_, err := rw.Write([]byte(`{
    "jsonrpc": "2.0",
    "id": 1,
    "result": {
        "blockHash": "0x96395fbdb39e33293999dc1a0a3b87c8a9e51185e177760d1482c2155bb35b87",
        "blockNumber": "0x32",
        "from": "0xfabb9cc6ec839b1214bb11c53377a56a6ed81762",
        "gas": "0x26fca",
        "gasPrice": "0x1",
        "hash": "0xc416a0332b4346f8090818981d1b2bf491d67b22cfec44ed8ec9a897b3631db2",
        "input": "0x1fece7b40000000000000000000000008d8f3199e684c76f25eeb9c0ce922d15bf72dfa200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000384144443a4554482e4554483a7474686f7231777a3738716d726b706c726468793337747730746e766e30746b6d35707164367a64703235370000000000000000",
        "nonce": "0x0",
        "to": "0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44",
        "transactionIndex": "0x0",
        "value": "0x58d15e176280000",
        "v": "0xa96",
        "r": "0x3c5e5945cadf1429bdb3e45a74003d86d62dd4091d9664f0c8163a951fe6f10e",
        "s": "0x1b86c85a84b0a76ea5d75ff08ea667f7f1e1883245b70deb4d4c347de3585ece"
    }
}`))
					c.Assert(err, IsNil)
				}
				return
			case "eth_gasPrice":
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
				c.Assert(err, IsNil)
				return
			case "eth_sendRawTransaction":
				_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b"}`))
				c.Assert(err, IsNil)
				return
			}
			if rpcRequest.Method == "eth_call" {
				if string(rpcRequest.Params) == `[{"data":"0x03b6a6730000000000000000000000009f4aab49a9cd8fc54dcb3701846f608a6f2c44da0000000000000000000000003b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483","from":"0x9f4aab49a9cd8fc54dcb3701846f608a6f2c44da","to":"0xe65e9d372f8cacc7b6dfcd4af6507851ed31bb44"},"latest"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
					c.Assert(err, IsNil)
				} else if string(rpcRequest.Params) == `[{"data":"0x95d89b41","from":"0x0000000000000000000000000000000000000000","to":"0x3b7fa4dd21c6f9ba3ca375217ead7cab9d6bf483"},"latest"]` {
					_, err := rw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000003544b4e0000000000000000000000000000000000000000000000000000000000"}`))
					c.Assert(err, IsNil)
				}
			}
		}
	}))
	s.server = server
	cfg := config.BifrostClientConfiguration{
		ChainID:      "thorchain",
		ChainHost:    server.Listener.Addr().String(),
		SignerName:   "bob",
		SignerPasswd: "password",
	}

	kb := cKeys.NewInMemory()
	_, _, err := kb.NewMnemonic(cfg.SignerName, cKeys.English, cmd.THORChainHDPath, cfg.SignerPasswd, hd.Secp256k1)
	c.Assert(err, IsNil)
	s.thorKeys = thorclient.NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd)
	s.bridge, err = thorclient.NewThorchainBridge(cfg, s.m, s.thorKeys)
	c.Assert(err, IsNil)
}

func (s *UnstuckTestSuite) TearDownTest(c *C) {
	c.Assert(os.Unsetenv("NET"), IsNil)
}

func (s *UnstuckTestSuite) TestUnstuckProcess(c *C) {
	pubkeyMgr, err := pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	c.Assert(err, IsNil)
	poolMgr := thorclient.NewPoolMgr(s.bridge)
	e, err := NewClient(s.thorKeys, config.BifrostChainConfiguration{
		RPCHost: "http://" + s.server.Listener.Addr().String(),
		BlockScanner: config.BifrostBlockScannerConfiguration{
			StartBlockHeight:   1, // avoids querying thorchain for block height
			HTTPRequestTimeout: time.Second * 10,
		},
	}, nil, s.bridge, s.m, pubkeyMgr, poolMgr)
	c.Assert(err, IsNil)
	c.Assert(e, NotNil)
	c.Assert(pubkeyMgr.Start(), IsNil)
	defer func() { c.Assert(pubkeyMgr.Stop(), IsNil) }()
	pubkey := e.kw.GetPubKey().String()

	txID1 := types2.GetRandomTxHash().String()
	txID2 := types2.GetRandomTxHash().String()
	// add some thing here
	c.Assert(e.ethScanner.blockMetaAccessor.AddSignedTxItem(SignedTxItem{
		Hash:        txID1,
		Height:      1022,
		VaultPubKey: pubkey,
	}), IsNil)
	c.Assert(e.ethScanner.blockMetaAccessor.AddSignedTxItem(SignedTxItem{
		Hash:        txID2,
		Height:      1024,
		VaultPubKey: pubkey,
	}), IsNil)
	// this should not do anything , because because all the tx has not been
	e.unstuckAction()
	items, err := e.ethScanner.blockMetaAccessor.GetSignedTxItems()
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 2)
	c.Assert(e.ethScanner.blockMetaAccessor.RemoveSignedTxItem(txID1), IsNil)
	c.Assert(e.ethScanner.blockMetaAccessor.RemoveSignedTxItem(txID2), IsNil)
	c.Assert(e.ethScanner.blockMetaAccessor.AddSignedTxItem(SignedTxItem{
		Hash:        "0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
		Height:      800,
		VaultPubKey: pubkey,
	}), IsNil)
	c.Assert(e.ethScanner.blockMetaAccessor.AddSignedTxItem(SignedTxItem{
		Hash:        "0x96395fbdb39e33293999dc1a0a3b87c8a9e51185e177760d1482c2155bb35b87",
		Height:      800,
		VaultPubKey: pubkey,
	}), IsNil)
	// this should try to check 0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b
	e.unstuckAction()
	items, err = e.ethScanner.blockMetaAccessor.GetSignedTxItems()
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)
}
