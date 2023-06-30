package bitcoin

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
	ctypes "gitlab.com/thorchain/binance-sdk/common/types"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/utxo"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/cmd"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/config"
	types2 "gitlab.com/thorchain/thornode/x/thorchain/types"
)

type BitcoinSignerSuite struct {
	client *Client
	server *httptest.Server
	bridge thorclient.ThorchainBridge
	cfg    config.BifrostChainConfiguration
	m      *metrics.Metrics
	db     *leveldb.DB
	keys   *thorclient.Keys
}

var _ = Suite(&BitcoinSignerSuite{})

func (s *BitcoinSignerSuite) SetUpSuite(c *C) {
	types2.SetupConfigForTest()
	kb := cKeys.NewInMemory()
	_, _, err := kb.NewMnemonic(bob, cKeys.English, cmd.THORChainHDPath, password, hd.Secp256k1)
	c.Assert(err, IsNil)
	s.keys = thorclient.NewKeysWithKeybase(kb, bob, password)
}

func (s *BitcoinSignerSuite) SetUpTest(c *C) {
	s.m = GetMetricForTest(c)
	s.cfg = config.BifrostChainConfiguration{
		ChainID:     "BTC",
		UserName:    bob,
		Password:    password,
		DisableTLS:  true,
		HTTPostMode: true,
		BlockScanner: config.BifrostBlockScannerConfiguration{
			StartBlockHeight: 1, // avoids querying thorchain for block height
		},
	}
	ctypes.Network = ctypes.TestNetwork

	ns := strconv.Itoa(time.Now().Nanosecond())
	thordir := filepath.Join(os.TempDir(), ns, ".thorcli")
	cfg := config.BifrostClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       "localhost",
		SignerName:      bob,
		SignerPasswd:    password,
		ChainHomeFolder: thordir,
	}

	s.server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.RequestURI == "/thorchain/vaults/tthorpub1addwnpepqwznsrgk2t5vn2cszr6ku6zned6tqxknugzw3vhdcjza284d7djp5rql6vn/signers" { // nolint
			_, err := rw.Write([]byte("[]"))
			c.Assert(err, IsNil)
		} else if strings.HasPrefix(req.RequestURI, "/thorchain/vaults") && strings.HasSuffix(req.RequestURI, "/signers") {
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/tss/keysign_party.json")
		} else if req.RequestURI == "/thorchain/version" {
			httpTestHandler(c, rw, "../../../../test/fixtures/endpoints/version/version.json")
		} else {
			r := struct {
				Method string `json:"method"`
			}{}
			buf, err := io.ReadAll(req.Body)
			c.Assert(err, IsNil)
			if len(buf) == 0 {
				return
			}
			c.Assert(json.Unmarshal(buf, &r), IsNil)
			defer func() {
				c.Assert(req.Body.Close(), IsNil)
			}()
			switch r.Method {
			case "getnetworkinfo":
				httpTestHandler(c, rw, "../../../../test/fixtures/btc/getnetworkinfo.json")
			case "getbestblockhash":
				httpTestHandler(c, rw, "../../../../test/fixtures/btc/getbestblockhash.json")
			case "getblock":
				httpTestHandler(c, rw, "../../../../test/fixtures/btc/block.json")
			case "getrawtransaction":
				httpTestHandler(c, rw, "../../../../test/fixtures/btc/tx.json")
			case "getinfo":
				httpTestHandler(c, rw, "../../../../test/fixtures/btc/getinfo.json")
			case "sendrawtransaction":
				httpTestHandler(c, rw, "../../../../test/fixtures/btc/sendrawtransaction.json")
			case "importaddress":
				httpTestHandler(c, rw, "../../../../test/fixtures/btc/importaddress.json")
			case "createwallet":
				_, err := rw.Write([]byte(`{ "result": null, "error": null, "id": 1 }`))
				c.Assert(err, IsNil)
			case "listunspent":
				body := string(buf)
				if strings.Contains(body, "tb1qleqepvj0d9n7899qj3skd8tw7c7jvh3zlxul70") {
					httpTestHandler(c, rw, "../../../../test/fixtures/btc/listunspent-tss.json")
				} else {
					httpTestHandler(c, rw, "../../../../test/fixtures/btc/listunspent.json")
				}
			}
		}
	}))
	var err error
	s.cfg.RPCHost = s.server.Listener.Addr().String()
	cfg.ChainHost = s.server.Listener.Addr().String()
	s.bridge, err = thorclient.NewThorchainBridge(cfg, s.m, s.keys)
	c.Assert(err, IsNil)
	s.client, err = NewClient(s.keys, s.cfg, nil, s.bridge, s.m)
	c.Assert(err, IsNil)
	storage := storage.NewMemStorage()
	db, err := leveldb.Open(storage, nil)
	c.Assert(err, IsNil)
	s.db = db
	s.client.temporalStorage, err = utxo.NewTemporalStorage(db, 0)
	c.Assert(err, IsNil)
	c.Assert(s.client, NotNil)
}

func (s *BitcoinSignerSuite) TearDownTest(c *C) {
	s.server.Close()
	c.Assert(s.db.Close(), IsNil)
}

func (s *BitcoinSignerSuite) TestGetBTCPrivateKey(c *C) {
	input := "YjQwNGM1ZWM1ODExNmI1ZjBmZTEzNDY0YTkyZTQ2NjI2ZmM1ZGIxMzBlNDE4Y2JjZTk4ZGY4NmZmZTkzMTdjNQ=="
	buf, err := base64.StdEncoding.DecodeString(input)
	c.Assert(err, IsNil)
	c.Assert(buf, NotNil)
	prikeyByte, err := hex.DecodeString(string(buf))
	c.Assert(err, IsNil)
	pk := secp256k1.GenPrivKeyFromSecret(prikeyByte)
	btcPrivateKey, err := getBTCPrivateKey(pk)
	c.Assert(err, IsNil)
	c.Assert(btcPrivateKey, NotNil)
}

func (s *BitcoinSignerSuite) TestSignTx(c *C) {
	txOutItem := stypes.TxOutItem{
		Chain:       common.BNBChain,
		ToAddress:   types2.GetRandomBNBAddress(),
		VaultPubKey: types2.GetRandomPubKey(),
		Coins: common.Coins{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(10000000000)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(1001)),
		},
		InHash:  "",
		OutHash: "",
	}
	// incorrect chain should return an error
	result, _, _, err := s.client.SignTx(txOutItem, 1)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// invalid pubkey should return an error
	txOutItem.Chain = common.BTCChain
	txOutItem.VaultPubKey = common.PubKey("helloworld")
	result, _, _, err = s.client.SignTx(txOutItem, 2)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// invalid to address should return an error
	txOutItem.VaultPubKey = types2.GetRandomPubKey()
	result, _, _, err = s.client.SignTx(txOutItem, 3)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	addr, err := types2.GetRandomPubKey().GetAddress(common.BTCChain)
	c.Assert(err, IsNil)
	txOutItem.ToAddress = addr

	// nothing to sign , because there is not enough UTXO
	result, _, _, err = s.client.SignTx(txOutItem, 4)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
}

func (s *BitcoinSignerSuite) TestSignTxHappyPathWithPrivateKey(c *C) {
	addr, err := types2.GetRandomPubKey().GetAddress(common.BTCChain)
	c.Assert(err, IsNil)
	txOutItem := stypes.TxOutItem{
		Chain:       common.BTCChain,
		ToAddress:   addr,
		VaultPubKey: "tthorpub1addwnpepqw2k68efthm08f0f5akhjs6fk5j2pze4wkwt4fmnymf9yd463puruhh0lyz",
		Coins: common.Coins{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(10)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(1000)),
		},
		InHash:  "",
		OutHash: "",
	}
	txHash := "256222fb25a9950479bb26049a2c00e75b89abbb7f0cf646c623b93e942c4c34"
	c.Assert(err, IsNil)
	blockMeta := utxo.NewBlockMeta("000000000000008a0da55afa8432af3b15c225cc7e04d32f0de912702dd9e2ae",
		100,
		"0000000000000068f0710c510e94bd29aa624745da43e32a1de887387306bfda")
	blockMeta.AddCustomerTransaction(txHash)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)
	priKeyBuf, err := hex.DecodeString("b404c5ec58116b5f0fe13464a92e46626fc5db130e418cbce98df86ffe9317c5")
	c.Assert(err, IsNil)
	pkey, _ := btcec.PrivKeyFromBytes(btcec.S256(), priKeyBuf)
	c.Assert(pkey, NotNil)
	ksw, err := NewKeySignWrapper(pkey, s.client.ksWrapper.tssKeyManager)
	c.Assert(err, IsNil)
	s.client.privateKey = pkey
	s.client.ksWrapper = ksw
	vaultPubKey, err := GetBech32AccountPubKey(pkey)
	c.Assert(err, IsNil)
	txOutItem.VaultPubKey = vaultPubKey
	buf, _, _, err := s.client.SignTx(txOutItem, 1)
	c.Assert(err, IsNil)
	c.Assert(buf, NotNil)
}

func (s *BitcoinSignerSuite) TestSignTxWithoutPredefinedMaxGas(c *C) {
	addr, err := types2.GetRandomPubKey().GetAddress(common.BTCChain)
	c.Assert(err, IsNil)
	txOutItem := stypes.TxOutItem{
		Chain:       common.BTCChain,
		ToAddress:   addr,
		VaultPubKey: "tthorpub1addwnpepqw2k68efthm08f0f5akhjs6fk5j2pze4wkwt4fmnymf9yd463puruhh0lyz",
		Coins: common.Coins{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(10)),
		},
		Memo:    "YGGDRASIL-:101",
		GasRate: 25,
		InHash:  "",
		OutHash: "",
	}
	txHash := "256222fb25a9950479bb26049a2c00e75b89abbb7f0cf646c623b93e942c4c34"
	c.Assert(err, IsNil)
	blockMeta := utxo.NewBlockMeta("000000000000008a0da55afa8432af3b15c225cc7e04d32f0de912702dd9e2ae",
		100,
		"0000000000000068f0710c510e94bd29aa624745da43e32a1de887387306bfda")
	blockMeta.AddCustomerTransaction(txHash)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)
	priKeyBuf, err := hex.DecodeString("b404c5ec58116b5f0fe13464a92e46626fc5db130e418cbce98df86ffe9317c5")
	c.Assert(err, IsNil)
	pkey, _ := btcec.PrivKeyFromBytes(btcec.S256(), priKeyBuf)
	c.Assert(pkey, NotNil)
	ksw, err := NewKeySignWrapper(pkey, s.client.ksWrapper.tssKeyManager)
	c.Assert(err, IsNil)
	s.client.privateKey = pkey
	s.client.ksWrapper = ksw
	vaultPubKey, err := GetBech32AccountPubKey(pkey)
	c.Assert(err, IsNil)
	txOutItem.VaultPubKey = vaultPubKey
	buf, _, _, err := s.client.SignTx(txOutItem, 1)
	c.Assert(err, IsNil)
	c.Assert(buf, NotNil)

	c.Assert(s.client.temporalStorage.UpsertTransactionFee(0.001, 10), IsNil)
	buf, _, _, err = s.client.SignTx(txOutItem, 1)
	c.Assert(err, IsNil)
	c.Assert(buf, NotNil)
}

func (s *BitcoinSignerSuite) TestBroadcastTx(c *C) {
	txOutItem := stypes.TxOutItem{
		Chain:       common.BNBChain,
		ToAddress:   types2.GetRandomBNBAddress(),
		VaultPubKey: types2.GetRandomPubKey(),
		Coins: common.Coins{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(10)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(1)),
		},
		InHash:  "",
		OutHash: "",
	}
	input := []byte("hello world")
	_, err := s.client.BroadcastTx(txOutItem, input)
	c.Assert(err, NotNil)
	input1, err := hex.DecodeString("01000000000103c7d45551ff54354be6711396560348ebbf273b989b542be36645568ed1dbecf10000000000ffffffff951ed70edc0bf2a4b3e1cbfe55d191a72850c5595c381309f69fc084c9af0b540100000000ffffffffc5db14c562b96bfd95f97d74a558a3e3b91841a96e1b09546208c9fb67424f420000000000ffffffff02231710000000000016001417acb08a31369e7666d94664d7e64f0e048220900000000000000000176a1574686f72636861696e3a636f6e736f6c6964617465024730440220756d15a363b78b070b583dfc1a6aba0dd605550407d5d3d92f5e785ef7e42aca02200db19dab144033c9c353481be30469da42c0c0a7580a513f49282bea77d7a29301210223da2ff73fa9b2258d335a4e63a4e7ef88211b8e800588280ed8b51e285ec0ff02483045022100a695f0fece36de02212b10bf6aa2f06dc6ef84ba30cae0c78749deddba1574530220315b490111c830c27e6cb810559c2a37cd00b123de82df79e061df26c8deb14301210223da2ff73fa9b2258d335a4e63a4e7ef88211b8e800588280ed8b51e285ec0ff0247304402207e586439b04985a90a53cf9fc511a53d86acece57b3e5571118562449d4f27ac02206d84f0fba1a68cf55efc8a1c2ec768924479b97ceaf2029ed6941176f004bf8101210223da2ff73fa9b2258d335a4e63a4e7ef88211b8e800588280ed8b51e285ec0ff00000000")
	c.Assert(err, IsNil)
	_, err = s.client.BroadcastTx(txOutItem, input1)
	c.Assert(err, IsNil)
}

func (s *BitcoinSignerSuite) TestIsSelfTransaction(c *C) {
	c.Check(s.client.isSelfTransaction("66d2d6b5eb564972c59e4797683a1225a02515a41119f0a8919381236b63e948"), Equals, false)
	bm := utxo.NewBlockMeta("", 1024, "")
	hash := "66d2d6b5eb564972c59e4797683a1225a02515a41119f0a8919381236b63e948"
	bm.AddSelfTransaction(hash)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(1024, bm), IsNil)
	c.Check(s.client.isSelfTransaction("66d2d6b5eb564972c59e4797683a1225a02515a41119f0a8919381236b63e948"), Equals, true)
}

func (s *BitcoinSignerSuite) TestEstimateTxSize(c *C) {
	size := s.client.estimateTxSize("OUT:2180B871F2DEA2546E1403DBFE9C26B062ABAFFD979CF3A65F2B4D2230105CF1", []btcjson.ListUnspentResult{
		{
			TxID:      "66d2d6b5eb564972c59e4797683a1225a02515a41119f0a8919381236b63e948",
			Vout:      0,
			Spendable: true,
		},
		{
			TxID:      "c5946215d82d5870ba2b1e8f245d8aa1446783975aa3a592cf55589fccbf285f",
			Vout:      0,
			Spendable: true,
		},
	})
	c.Assert(size, Equals, int64(255))
}

func (s *BitcoinSignerSuite) TestSignTxWithAddressPubkey(c *C) {
	txOutItem := stypes.TxOutItem{
		Chain:       common.BTCChain,
		ToAddress:   "04ae1a62fe09c5f51b13905f07f06b99a2f7159b2225f374cd378d71302fa28414e7aab37397f554a7df5f142c21c1b7303b8a0626f1baded5c72a704f7e6cd84c",
		VaultPubKey: "tthorpub1addwnpepqw2k68efthm08f0f5akhjs6fk5j2pze4wkwt4fmnymf9yd463puruhh0lyz",
		Coins: common.Coins{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(10)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(1000)),
		},
		InHash:  "",
		OutHash: "",
	}
	txHash := "256222fb25a9950479bb26049a2c00e75b89abbb7f0cf646c623b93e942c4c34"
	blockMeta := utxo.NewBlockMeta("000000000000008a0da55afa8432af3b15c225cc7e04d32f0de912702dd9e2ae",
		100,
		"0000000000000068f0710c510e94bd29aa624745da43e32a1de887387306bfda")
	blockMeta.AddCustomerTransaction(txHash)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)
	priKeyBuf, err := hex.DecodeString("b404c5ec58116b5f0fe13464a92e46626fc5db130e418cbce98df86ffe9317c5")
	c.Assert(err, IsNil)
	pkey, _ := btcec.PrivKeyFromBytes(btcec.S256(), priKeyBuf)
	c.Assert(pkey, NotNil)
	ksw, err := NewKeySignWrapper(pkey, s.client.ksWrapper.tssKeyManager)
	c.Assert(err, IsNil)
	s.client.privateKey = pkey
	s.client.ksWrapper = ksw
	vaultPubKey, err := GetBech32AccountPubKey(pkey)
	c.Assert(err, IsNil)
	txOutItem.VaultPubKey = vaultPubKey
	// The transaction will not signed, but ignored instead
	buf, _, _, err := s.client.SignTx(txOutItem, 1)
	c.Assert(err, IsNil)
	c.Assert(buf, IsNil)
}

func (s *BitcoinSignerSuite) TestToAddressCanNotRoundTripShouldBlock(c *C) {
	txOutItem := stypes.TxOutItem{
		Chain:       common.BTCChain,
		ToAddress:   "05ae1a62fe09c5f51b13905f07f06b99a2f7159b2225f374cd378d71302fa28414e7aab37397f554a7df5f142c21c1b7303b8a0626f1baded5c72a704f7e6cd84c",
		VaultPubKey: "tthorpub1addwnpepqw2k68efthm08f0f5akhjs6fk5j2pze4wkwt4fmnymf9yd463puruhh0lyz",
		Coins: common.Coins{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(10)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(1000)),
		},
		InHash:  "",
		OutHash: "",
	}
	txHash := "256222fb25a9950479bb26049a2c00e75b89abbb7f0cf646c623b93e942c4c34"
	blockMeta := utxo.NewBlockMeta("000000000000008a0da55afa8432af3b15c225cc7e04d32f0de912702dd9e2ae",
		100,
		"0000000000000068f0710c510e94bd29aa624745da43e32a1de887387306bfda")
	blockMeta.AddCustomerTransaction(txHash)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)
	priKeyBuf, err := hex.DecodeString("b404c5ec58116b5f0fe13464a92e46626fc5db130e418cbce98df86ffe9317c5")
	c.Assert(err, IsNil)
	pkey, _ := btcec.PrivKeyFromBytes(btcec.S256(), priKeyBuf)
	c.Assert(pkey, NotNil)
	ksw, err := NewKeySignWrapper(pkey, s.client.ksWrapper.tssKeyManager)
	c.Assert(err, IsNil)
	s.client.privateKey = pkey
	s.client.ksWrapper = ksw
	vaultPubKey, err := GetBech32AccountPubKey(pkey)
	c.Assert(err, IsNil)
	txOutItem.VaultPubKey = vaultPubKey
	// The transaction will not signed, but ignored instead
	buf, _, _, err := s.client.SignTx(txOutItem, 1)
	c.Assert(err, IsNil)
	c.Assert(buf, IsNil)
}
