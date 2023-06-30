package observer

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/prometheus/client_golang/prometheus/testutil"
	ctypes "gitlab.com/thorchain/binance-sdk/common/types"
	txType "gitlab.com/thorchain/binance-sdk/types/tx"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/binance"
	"gitlab.com/thorchain/thornode/bifrost/pubkeymanager"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/cmd"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/config"
	"gitlab.com/thorchain/thornode/x/thorchain"
	types2 "gitlab.com/thorchain/thornode/x/thorchain/types"
)

func TestPackage(t *testing.T) { TestingT(t) }

type ObserverSuite struct {
	m        *metrics.Metrics
	thordir  string
	thorKeys *thorclient.Keys
	bridge   thorclient.ThorchainBridge
	b        *binance.Binance
}

var _ = Suite(&ObserverSuite{})

const binanceNodeInfo = `{"node_info":{"protocol_version":{"p2p":7,"block":10,"app":0},"id":"7bbe02b44f45fb8f73981c13bb21b19b30e2658d","listen_addr":"10.201.42.4:27146","network":"Binance-Chain-Ganges","version":"0.31.5","channels":"3640202122233038","moniker":"Kita","other":{"tx_index":"on","rpc_address":"tcp://0.0.0.0:27147"}},"sync_info":{"latest_block_hash":"BFADEA1DC558D23CB80564AA3C08C863929E4CC93E43C4925D96219114489DC0","latest_app_hash":"1115D879135E2492A947CF3EB9FE055B9813581084EFE3686A6466C2EC12DB7A","latest_block_height":0,"latest_block_time":"2019-08-25T00:54:02.906908056Z","catching_up":false},"validator_info":{"address":"E0DD72609CC106210D1AA13936CB67B93A0AEE21","pub_key":[4,34,67,57,104,143,1,46,100,157,228,142,36,24,128,9,46,170,143,106,160,244,241,75,252,249,224,199,105,23,192,182],"voting_power":100000000000}}`

var status = fmt.Sprintf(`{ "jsonrpc": "2.0", "id": "", "result": %s}`, binanceNodeInfo)

const accountInfo string = `{
  "jsonrpc": "2.0",
  "id": "",
  "result": {
    "response": {
      "value": "S9xMJwr/CAoUgT5JOfFWeyGXBP/CrU31i94BCHkSDAoHMDAwLTBFMRCiUhIOCgdBQUEtRUI4EJCFogQSEQoIQUdSSS1CRDIQouubj/8CEg4KCEFMSVMtOTVCEIXFPRIRCgdBTk4tNDU3EICQprf5pQISEgoIQVRPTS0yMEMQgIDpg7HeFhIOCgdBVlQtQjc0EIqg/h4SDQoHQkMxLTNBMRCQv28SDQoDQk5CELLzuMXDvhASEQoHQk5OLTQxMRCAkKa3+aUCEhAKCUJUQy5CLTkxOBDwqf41EhIKCUJUTUdMLUM3MhDxx52H+gUSEQoHQ05OLTIxMBCAkKa3+aUCEhUKCkNPU01PUy01ODcQ8Ybm677a6FgSDwoIQ09USS1EMTMQyK7iBBINCgdEQzEtNEI4EJC/bxIRCghEVUlULTMxQxDU+fGWwwMSDgoHRURVLUREMBCM+9lCEg8KB0ZSSS1ENUYQyaiJ9SkSDgoHSUFBLUM4MRDk18AEEg4KB0lCQi04REUQ5NfABBIOCgdJQ0MtNkVGEOTXwAQSDgoHSURELTUxNhDk18AEEg4KB0lFRS1EQ0EQ5NfABBIOCgdJRkYtODA0EOTXwAQSDgoHSUdHLTAxMxDk18AEEg4KB0lISC1ENEUQ5NfABBIOCgdJSUktMjVDEOTXwAQSDgoHSUpKLTY1RRDk18AEEhIKCktPR0U0OC0zNUQQgMivoCUSDQoHTEMxLTdGQxCQv28SDwoHTENRLUFDNRDO5ZyDIhIQCgdNRkgtOUI1ENb6yYbSJBIKCghOQVNDLTEzNxINCgdOQzEtMjc5EJC/bxINCgdOQzItMjQ5EO6TVhIPCgdPQ0ItQjk1EIDIr6AlEhAKB1BJQy1GNDAQouubj/8CEg4KB1BQQy0wMEEQtLDpYRIRCgdRQlgtQUY1EICi/KevmgESDQoHUkJULUNCNxCFxT0SDQoHUkMxLTk0MxCQv28SDQoHUkMxLUExRRCQv28SDQoHUkMxLUY0ORCQv28SDgoHU1ZDLUExNBCi99oIEg0KB1RDMS1GNDMQkL9vEg8KB1RFRC1ERjIQwP3LzgUSEwoIVEVTVC0wNzUQgICE/qbe4RESEAoIVEVTVC01OTkQgJzNymQSEwoIVEVTVC03OEYQgICE/qbe4RESEwoIVEVTVC1EM0YQgICE/qbe4RESDgoHVEZBLTNCNBD8590CEg8KB1RHVC05RkMQ7KCu73sSDgoHVFNULUQ1NxCAhK9fEg4KB1RTVy02RkQQgMLXLxIPCgdVQ1gtQ0M4EIHPg5sFEg8KB1VETy02MzgQwYbx4xISEwoKVVNEVC5CLUI3QxDsxNuFhQQSEAoJV1dXNzYtQThGEJC+mQISDgoHWFNYLTA3MhC1o/AEEg4KB1lMQy1EOEIQ5aq0ZBIPCgdaQ0ItQjM2EIDkl9ASEg4KCVpFQlJBLTE2RBDoBxIOCgdaWlotMjFFEPTl1QYaJuta6YchAhOb3ZXecsIqwqKw+HhTscyi6K35xYpKaJx10yYwE0QaINLlGCh3"
    }
  }
}`

const accountInfoWithMemoFlag string = `
{
  "jsonrpc": "2.0",
  "id": "",
  "result": {
    "response": {
      "value": "S9xMJwpDChQmefkgNkKwoDDRn/aGeQyZIQ0TKhom61rphyEDAZiSiqsU7aoacPPMYg+1FOYq1cFAi/52RJ5moAaoWLcg9VEoASgB"
    }
  }
}`

func (s *ObserverSuite) NewMockBinanceInstance(c *C, jsonData string) {
	var err error
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		c.Logf("requestUri:%s", req.RequestURI)
		if strings.EqualFold(req.RequestURI, "/abci_query?path=%22%2Faccount%2Ftbnb1yeuljgpkg2c2qvx3nlmgv7gvnyss6ye2u8rasf%22") { // nolint
			_, err := rw.Write([]byte(accountInfoWithMemoFlag))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/abci_query?") {
			if _, err := rw.Write([]byte(accountInfo)); err != nil {
				c.Error(err)
			}
		} else if strings.HasPrefix(req.RequestURI, "/status") {
			if _, err := rw.Write([]byte(status)); err != nil {
				c.Error(err)
			}
		} else if req.RequestURI == "/abci_info" {
			_, err := rw.Write([]byte(`{ "jsonrpc": "2.0", "id": "", "result": { "response": { "data": "BNBChain", "last_block_height": "0", "last_block_app_hash": "pwx4TJjXu3yaF6dNfLQ9F4nwAhjIqmzE8fNa+RXwAzQ=" } } }`))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/block") {
			_, err := rw.Write([]byte(`{ "jsonrpc": "2.0", "id": "", "result": { "block": { "header": { "height": "1" } } } }`))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/tx_search") {
			_, err := rw.Write([]byte(jsonData))
			c.Assert(err, IsNil)
		}
	}))

	blockHeightDiscoverBackoff, _ := time.ParseDuration("1s")
	blockRetryInterval, _ := time.ParseDuration("10s")
	httpRequestTimeout, _ := time.ParseDuration("30s")
	s.b, err = binance.NewBinance(s.thorKeys, config.BifrostChainConfiguration{RPCHost: server.URL, BlockScanner: config.BifrostBlockScannerConfiguration{
		RPCHost:                    server.URL,
		BlockScanProcessors:        1,
		BlockHeightDiscoverBackoff: blockHeightDiscoverBackoff,
		BlockRetryInterval:         blockRetryInterval,
		ChainID:                    common.BNBChain,
		HTTPRequestTimeout:         httpRequestTimeout,
		HTTPRequestReadTimeout:     httpRequestTimeout,
		HTTPRequestWriteTimeout:    httpRequestTimeout,
		MaxHTTPRequestRetry:        10,
		StartBlockHeight:           1, // avoids querying thorchain for block height
		EnforceBlockHeight:         true,
	}}, nil, s.bridge, s.m)
	c.Assert(err, IsNil)
	c.Assert(s.b, NotNil)
}

func (s *ObserverSuite) SetUpSuite(c *C) {
	var err error
	s.m, err = metrics.NewMetrics(config.BifrostMetricsConfiguration{
		Enabled:      false,
		ListenPort:   9000,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		Chains:       common.Chains{common.BNBChain},
	})
	c.Assert(s.m, NotNil)
	c.Assert(err, IsNil)

	ns := strconv.Itoa(time.Now().Nanosecond())
	types2.SetupConfigForTest()
	ctypes.Network = ctypes.TestNetwork
	c.Assert(os.Setenv("NET", "testnet"), IsNil)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.RequestURI, "/errata") { // nolint
			_, err := rw.Write([]byte(`{ "jsonrpc": "2.0", "id": "E7FDA9DE4D0AD37D823813CB5BC0D6E69AB0D41BB666B65B965D12D24A3AE83C", "result": { "height": "1", "txhash": "AAAA000000000000000000000000000000000000000000000000000000000000", "logs": [{"success": "true", "log": ""}] } }`))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, thorclient.MimirEndpoint) {
			buf, err := os.ReadFile("../../test/fixtures/endpoints/mimir/mimir.json")
			c.Assert(err, IsNil)
			_, err = rw.Write(buf)
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/thorchain/lastblock/BNB") {
			_, err := rw.Write([]byte(`[{ "chain": "BNB", "lastobservedin": 0, "lastsignedout": 0, "thorchain": 0 }]`))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/thorchain/lastblock") {
			_, err := rw.Write([]byte(`[{ "chain": "THORChain", "lastobservedin": 0, "lastsignedout": 0, "thorchain": 0 }]`))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/auth/accounts/") {
			_, err := rw.Write([]byte(`{ "jsonrpc": "2.0", "id": "", "result": { "height": "0", "result": { "value": { "account_number": "0", "sequence": "0" } } } |`))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/thorchain/vaults/pubkeys") {
			_, err := rw.Write([]byte(`{ "jsonrpc": "2.0", "id": "", "result": { "asgard": ["tthorpub1addwnpepq2jgpsw2lalzuk7sgtmyakj7l6890f5cfpwjyfp8k4y4t7cw2vk8v2ch5uz"], "yggdrasil": ["tthorpub1addwnpepqdqvd4r84lq9m54m5kk9sf4k6kdgavvch723pcgadulxd6ey9u70k6zq8qe"] } }`))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/thorchain/keysign") {
			_, err := rw.Write([]byte(`{"chains":{}}`))
			c.Assert(err, IsNil)
		} else if strings.HasSuffix(req.RequestURI, "/signers") {
			_, err := rw.Write([]byte(`[
  "tthorpub1addwnpepqflvfv08t6qt95lmttd6wpf3ss8wx63e9vf6fvyuj2yy6nnyna576rfzjks",
  "tthorpub1addwnpepq2flfr96skc5lkwdv0n5xjsnhmuju20x3zndgu42zd8dtkrud9m2vajhww6",
  "tthorpub1addwnpepqwhnus6xs4208d4ynm05lv493amz3fexfjfx4vptntedd7k0ajlcunlfxxs"
]`))
			c.Assert(err, IsNil)

		} else if req.RequestURI == "/thorchain/thorname/all-good" {
			_, err := rw.Write([]byte(`{
				"name": "all-good",
				"expire_block_height": 10000,
				"owner": "tthor1tdfqy34uptx207scymqsy4k5uzfmry5sf7z3dw",
				"aliases": [
					{"chain": "BNB", "address": "tbnb1yycn4mh6ffwpjf584t8lpp7c27ghu03gpvqkfjw"}
				]
			}`))
			c.Assert(err, IsNil)
		} else if req.RequestURI == "/thorchain/thorname/unknown" {
			_, err := rw.Write([]byte(`{"error": "internal"}`))
			c.Assert(err, IsNil)
		} else if req.RequestURI == "/thorchain/thorname/bnb-memo" {
			_, err := rw.Write([]byte(`{
				"name": "bnb-memo",
				"expire_block_height": 10000,
				"owner": "tthor1tdfqy34uptx207scymqsy4k5uzfmry5sf7z3dw",
				"aliases": [
					{"chain": "BNB", "address": "tbnb1yeuljgpkg2c2qvx3nlmgv7gvnyss6ye2u8rasf"}
				]
			}`))
			c.Assert(err, IsNil)

		} else if strings.HasPrefix(req.RequestURI, "/") {
			_, err := rw.Write([]byte(`{ "jsonrpc": "2.0", "id": 0, "result": { "height": "1", "hash": "E7FDA9DE4D0AD37D823813CB5BC0D6E69AB0D41BB666B65B965D12D24A3AE83C", "logs": [{"success": "true", "log": ""}] } }`))
			c.Assert(err, IsNil)
		} else {
			c.Errorf("invalid server query: %s", req.RequestURI)
		}
	}))

	s.thordir = filepath.Join(os.TempDir(), ns, ".thorcli")
	cfg := config.BifrostClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       server.Listener.Addr().String(),
		ChainRPC:        server.Listener.Addr().String(),
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: s.thordir,
	}

	kb := cKeys.NewInMemory()
	_, _, err = kb.NewMnemonic(cfg.SignerName, cKeys.English, cmd.THORChainHDPath, cfg.SignerPasswd, hd.Secp256k1)
	c.Assert(err, IsNil)
	s.thorKeys = thorclient.NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd)

	c.Assert(s.thorKeys, NotNil)
	s.bridge, err = thorclient.NewThorchainBridge(cfg, s.m, s.thorKeys)
	c.Assert(s.bridge, NotNil)
	c.Assert(err, IsNil)
	priv, err := s.thorKeys.GetPrivateKey()
	c.Assert(err, IsNil)
	tmp, err := codec.ToTmPubKeyInterface(priv.PubKey())
	c.Assert(err, IsNil)
	pk, err := common.NewPubKeyFromCrypto(tmp)
	c.Assert(err, IsNil)
	txOut := getTxOutFromJSONInput(`{ "height": 0, "tx_array": [ { "vault_pub_key":"", "to_address": "tbnb186nvjtqk4kkea3f8a30xh4vqtkrlu2rm9xgly3", "memo": "migrate", "coin":  { "asset": "BNB", "amount": "194765912" }  } ]}`, c)
	txOut.TxArray[0].VaultPubKey = pk
	out := txOut.TxArray[0].TxOutItem()

	s.NewMockBinanceInstance(c, "")

	r, _, _, err := s.b.SignTx(out, 1440)
	c.Assert(err, IsNil)
	c.Assert(r, NotNil)
	buf, err := hex.DecodeString(string(r))
	c.Assert(err, IsNil)
	var t txType.StdTx
	err = txType.Cdc.UnmarshalBinaryLengthPrefixed(buf, &t)
	c.Assert(err, IsNil)
	bin, _ := txType.Cdc.MarshalBinaryLengthPrefixed(t)
	encodedTx := base64.StdEncoding.EncodeToString(bin)
	jsonData := `{ "jsonrpc": "2.0", "id": "", "result": { "txs": [ { "hash": "10C4E872A5DC842BE72AC8DE9C6A13F97DF6D345336F01B87EBA998F5A3BC36D", "height": "1", "tx": "` + encodedTx + `" } ], "total_count": "1" } }`

	s.NewMockBinanceInstance(c, jsonData)
}

func (s *ObserverSuite) TearDownSuite(c *C) {
	c.Assert(os.Unsetenv("NET"), IsNil)

	if err := os.RemoveAll(s.thordir); err != nil {
		c.Error(err)
	}

	if err := os.RemoveAll("observer/observer_data"); err != nil {
		c.Error(err)
	}

	if err := os.RemoveAll("observer/var"); err != nil {
		c.Error(err)
	}
}

func (s *ObserverSuite) TestProcess(c *C) {
	pubkeyMgr, err := pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	c.Assert(err, IsNil)
	obs, err := NewObserver(pubkeyMgr, map[common.Chain]chainclients.ChainClient{common.BNBChain: s.b}, s.bridge, s.m, "", metrics.NewTssKeysignMetricMgr())
	c.Assert(obs, NotNil)
	c.Assert(err, IsNil)
	err = obs.Start()
	c.Assert(err, IsNil)
	time.Sleep(time.Second * 2)
	metric, err := s.m.GetCounterVec(metrics.ObserverError).GetMetricWithLabelValues("fail_to_send_to_thorchain", "1")
	c.Assert(err, IsNil)
	c.Check(int(testutil.ToFloat64(metric)), Equals, 0)

	err = obs.Stop()
	c.Assert(err, IsNil)
}

func getTxOutFromJSONInput(input string, c *C) types.TxOut {
	var txOut types.TxOut
	err := json.Unmarshal([]byte(input), &txOut)
	c.Check(err, IsNil)
	return txOut
}

func (s *ObserverSuite) TestErrataTx(c *C) {
	pubkeyMgr, err := pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	c.Assert(err, IsNil)
	obs, err := NewObserver(pubkeyMgr, nil, s.bridge, s.m, "", metrics.NewTssKeysignMetricMgr())
	c.Assert(obs, NotNil)
	c.Assert(err, IsNil)
	c.Assert(obs.sendErrataTxToThorchain(25, thorchain.GetRandomTxHash(), common.BNBChain), IsNil)
}

func (s *ObserverSuite) TestFilterMemoFlag(c *C) {
	pubkeyMgr, err := pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	c.Assert(err, IsNil)
	obs, err := NewObserver(pubkeyMgr, map[common.Chain]chainclients.ChainClient{
		common.BNBChain: s.b,
	}, s.bridge, s.m, "", metrics.NewTssKeysignMetricMgr())
	c.Assert(obs, NotNil)
	c.Assert(err, IsNil)

	// swap destination
	result := obs.filterBinanceMemoFlag(common.BNBChain, []types.TxInItem{
		{
			BlockHeight: 1024,
			Tx:          "tx1",
			Memo:        "swap:BNB.RUNE-67C:tbnb1yeuljgpkg2c2qvx3nlmgv7gvnyss6ye2u8rasf",
			Sender:      thorchain.GetRandomBNBAddress().String(),
			To:          thorchain.GetRandomBNBAddress().String(),
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
			},
			Gas:                 nil,
			ObservedVaultPubKey: thorchain.GetRandomPubKey(),
		},
	})
	c.Assert(result, HasLen, 0)

	// sender has memo flag
	result = obs.filterBinanceMemoFlag(common.BNBChain, []types.TxInItem{
		{
			BlockHeight: 1024,
			Tx:          "tx1",
			Memo:        "swap:BNB.RUNE-67C:" + thorchain.GetRandomBNBAddress().String(),
			Sender:      "tbnb1yeuljgpkg2c2qvx3nlmgv7gvnyss6ye2u8rasf",
			To:          thorchain.GetRandomBNBAddress().String(),
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
			},
			Gas:                 nil,
			ObservedVaultPubKey: thorchain.GetRandomPubKey(),
		},
	})
	c.Assert(result, HasLen, 0)

	// swap from BTC chain to BNB chain
	result = obs.filterBinanceMemoFlag(common.BTCChain, []types.TxInItem{
		{
			BlockHeight: 1024,
			Tx:          "tx1",
			Memo:        "swap:BNB.RUNE-67C:tbnb1yeuljgpkg2c2qvx3nlmgv7gvnyss6ye2u8rasf",
			Sender:      thorchain.GetRandomBNBAddress().String(),
			To:          thorchain.GetRandomBNBAddress().String(),
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
			},
			Gas:                 nil,
			ObservedVaultPubKey: thorchain.GetRandomPubKey(),
		},
	})
	c.Assert(result, HasLen, 0)

	// normal swap
	result = obs.filterBinanceMemoFlag(common.BTCChain, []types.TxInItem{
		{
			BlockHeight: 1024,
			Tx:          "tx1",
			Memo:        "swap:BNB.RUNE-67C",
			Sender:      thorchain.GetRandomBNBAddress().String(),
			To:          thorchain.GetRandomBNBAddress().String(),
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
			},
			Gas:                 nil,
			ObservedVaultPubKey: thorchain.GetRandomPubKey(),
		},
	})
	c.Assert(result, HasLen, 1)

	// thorname resolves to a good bnb address, should pass filter
	result = obs.filterBinanceMemoFlag(common.BNBChain, []types.TxInItem{
		{
			BlockHeight: 1024,
			Tx:          "tx1",
			Memo:        "swap:BNB.RUNE-67C:all-good",
			Sender:      thorchain.GetRandomBNBAddress().String(),
			To:          thorchain.GetRandomBNBAddress().String(),
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
			},
			Gas:                 nil,
			ObservedVaultPubKey: thorchain.GetRandomPubKey(),
		},
	})
	c.Assert(result, HasLen, 1)

	// thorname is not known, should pass filter
	result = obs.filterBinanceMemoFlag(common.BNBChain, []types.TxInItem{
		{
			BlockHeight: 1024,
			Tx:          "tx1",
			Memo:        "swap:BNB.RUNE-67C:unknown",
			Sender:      thorchain.GetRandomBNBAddress().String(),
			To:          thorchain.GetRandomBNBAddress().String(),
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
			},
			Gas:                 nil,
			ObservedVaultPubKey: thorchain.GetRandomPubKey(),
		},
	})
	c.Assert(result, HasLen, 1)

	// thorname resolves to a bnb addr that requires a memo, should be filtered
	result = obs.filterBinanceMemoFlag(common.BNBChain, []types.TxInItem{
		{
			BlockHeight: 1024,
			Tx:          "tx1",
			Memo:        "swap:BNB.RUNE-67C:bnb-memo",
			Sender:      thorchain.GetRandomBNBAddress().String(),
			To:          thorchain.GetRandomBNBAddress().String(),
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
			},
			Gas:                 nil,
			ObservedVaultPubKey: thorchain.GetRandomPubKey(),
		},
	})
	c.Assert(result, HasLen, 0)

	// thorname resolves to a bnb addr that requires a memo, should be filtered
	result = obs.filterBinanceMemoFlag(common.BNBChain, []types.TxInItem{
		{
			BlockHeight: 1024,
			Tx:          "tx1",
			Memo:        "ADD:BNB.BNB:bnb-memo",
			Sender:      thorchain.GetRandomBNBAddress().String(),
			To:          thorchain.GetRandomBNBAddress().String(),
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
			},
			Gas:                 nil,
			ObservedVaultPubKey: thorchain.GetRandomPubKey(),
		},
	})
	c.Assert(result, HasLen, 0)

	// when there is no binance client , the check will be ignored
	obs, err = NewObserver(pubkeyMgr, nil, s.bridge, s.m, "", metrics.NewTssKeysignMetricMgr())
	c.Assert(obs, NotNil)
	c.Assert(err, IsNil)
	result = obs.filterBinanceMemoFlag(common.BNBChain, []types.TxInItem{
		{
			BlockHeight: 1024,
			Tx:          "tx1",
			Memo:        "swap:BNB.RUNE-67C:tbnb1yeuljgpkg2c2qvx3nlmgv7gvnyss6ye2u8rasf",
			Sender:      thorchain.GetRandomBNBAddress().String(),
			To:          thorchain.GetRandomBNBAddress().String(),
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
			},
			Gas:                 nil,
			ObservedVaultPubKey: thorchain.GetRandomPubKey(),
		},
	})
	c.Assert(result, HasLen, 1)
}

func (s *ObserverSuite) TestGetSaversMemo(c *C) {
	pubkeyMgr, err := pubkeymanager.NewPubKeyManager(s.bridge, s.m)
	c.Assert(err, IsNil)
	obs, err := NewObserver(pubkeyMgr, map[common.Chain]chainclients.ChainClient{
		common.BNBChain: s.b,
	}, s.bridge, s.m, "", metrics.NewTssKeysignMetricMgr())
	c.Assert(obs, NotNil)
	c.Assert(err, IsNil)

	busd, err := common.NewAsset("BNB.BUSD-BD1")
	c.Assert(err, IsNil)

	bnbSaversTx := types.TxInItem{
		BlockHeight: 1024,
		Tx:          "tx1",
		Memo:        "",
		Sender:      thorchain.GetRandomBNBAddress().String(),
		To:          thorchain.GetRandomBNBAddress().String(),
		Coins: common.Coins{
			common.NewCoin(busd, cosmos.NewUint(1024)),
		},
		Gas:                 nil,
		ObservedVaultPubKey: thorchain.GetRandomPubKey(),
	}

	// memo should be withdraw 1024 basis points
	memo := obs.getSaversMemo(common.BNBChain, bnbSaversTx)
	c.Assert(memo, Equals, "-:BNB/BUSD-BD1:1024")

	// memo should be withdraw 1000 basis points
	bnbSaversTx.Coins = common.NewCoins(common.NewCoin(common.BNBAsset, cosmos.NewUint(1000)))
	memo = obs.getSaversMemo(common.BNBChain, bnbSaversTx)
	c.Assert(memo, Equals, "-:BNB/BNB:1000")

	// memo should be add
	bnbSaversTx.Coins = common.NewCoins(common.NewCoin(common.BNBAsset, cosmos.NewUint(20_000)))
	memo = obs.getSaversMemo(common.BNBChain, bnbSaversTx)
	c.Assert(memo, Equals, "+:BNB/BNB")

	btcSaversTx := types.TxInItem{
		BlockHeight: 1024,
		Tx:          "tx1",
		Memo:        "",
		Sender:      thorchain.GetRandomBNBAddress().String(),
		To:          thorchain.GetRandomBNBAddress().String(),
		Coins: common.Coins{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(1000)),
		},
		Gas:                 nil,
		ObservedVaultPubKey: thorchain.GetRandomPubKey(),
	}

	// memo should be empty, amount not above dust threshold
	memo = obs.getSaversMemo(common.BTCChain, btcSaversTx)
	c.Assert(memo, Equals, "")

	// memo should still be empty, amount is at the dust thresold, but you can't withdraw 0 basis points
	btcSaversTx.Coins = common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(10_000)))
	memo = obs.getSaversMemo(common.BTCChain, btcSaversTx)
	c.Assert(memo, Equals, "")

	// memo should be withdraw 500 basis points
	btcSaversTx.Coins = common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(10_500)))
	memo = obs.getSaversMemo(common.BTCChain, btcSaversTx)
	c.Assert(memo, Equals, "-:BTC/BTC:500")

	// memo should be withdraw 10_000 basis points
	btcSaversTx.Coins = common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(20_000)))
	memo = obs.getSaversMemo(common.BTCChain, btcSaversTx)
	c.Assert(memo, Equals, "-:BTC/BTC:10000")

	// memo should be add
	btcSaversTx.Coins = common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(40_000)))
	memo = obs.getSaversMemo(common.BTCChain, btcSaversTx)
	c.Assert(memo, Equals, "+:BTC/BTC")
}
