package gaia

import (
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	btypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/cmd"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/config"
	. "gopkg.in/check.v1"
)

func TestPackage(t *testing.T) { TestingT(t) }

type CosmosTestSuite struct {
	thordir  string
	thorKeys *thorclient.Keys
	bridge   thorclient.ThorchainBridge
	m        *metrics.Metrics
}

var _ = Suite(&CosmosTestSuite{})

var m *metrics.Metrics

func GetMetricForTest(c *C) *metrics.Metrics {
	if m == nil {
		var err error
		m, err = metrics.NewMetrics(config.BifrostMetricsConfiguration{
			Enabled:      false,
			ListenPort:   9000,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
			Chains:       common.Chains{common.GAIAChain},
		})
		c.Assert(m, NotNil)
		c.Assert(err, IsNil)
	}
	return m
}

func (s *CosmosTestSuite) SetUpSuite(c *C) {
	cosmosSDKConfg := cosmos.GetConfig()
	cosmosSDKConfg.SetBech32PrefixForAccount("sthor", "sthorpub")

	s.m = GetMetricForTest(c)
	c.Assert(s.m, NotNil)
	ns := strconv.Itoa(time.Now().Nanosecond())
	c.Assert(os.Setenv("NET", "stagenet"), IsNil)

	s.thordir = filepath.Join(os.TempDir(), ns, ".thorcli")
	cfg := config.BifrostClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       "localhost",
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: s.thordir,
	}

	kb := cKeys.NewInMemory()
	_, _, err := kb.NewMnemonic(cfg.SignerName, cKeys.English, cmd.THORChainHDPath, cfg.SignerPasswd, hd.Secp256k1)
	c.Assert(err, IsNil)
	s.thorKeys = thorclient.NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd)
	c.Assert(err, IsNil)
	s.bridge, err = thorclient.NewThorchainBridge(cfg, s.m, s.thorKeys)
	c.Assert(err, IsNil)
}

func (s *CosmosTestSuite) TearDownSuite(c *C) {
	c.Assert(os.Unsetenv("NET"), IsNil)
	if err := os.RemoveAll(s.thordir); err != nil {
		c.Error(err)
	}
}

func (s *CosmosTestSuite) TestGetAddress(c *C) {
	mockBankServiceClient := NewMockBankServiceClient()
	mockAccountServiceClient := NewMockAccountServiceClient()

	cc := CosmosClient{
		cfg:           config.BifrostChainConfiguration{ChainID: common.GAIAChain},
		bankClient:    mockBankServiceClient,
		accountClient: mockAccountServiceClient,
	}

	addr := "cosmos10tjz4ave7znpctgd2rfu6v2r6zkeup2dlmqtuz"
	atom, _ := common.NewAsset("GAIA.ATOM")
	expectedCoins := common.NewCoins(
		common.NewCoin(atom, cosmos.NewUint(496694100)),
	)

	acc, err := cc.GetAccountByAddress(addr, big.NewInt(0))
	c.Assert(err, IsNil)
	c.Check(acc.AccountNumber, Equals, int64(3530305))
	c.Check(acc.Sequence, Equals, int64(3))
	c.Check(acc.Coins.Equals(expectedCoins), Equals, true)

	pk := common.PubKey("sthorpub1addwnpepqf72ur2e8zk8r5augtrly40cuy94f7e663zh798tyms6pu2k8qdswf4es66")
	acc, err = cc.GetAccount(pk, big.NewInt(0))
	c.Assert(err, IsNil)
	c.Check(acc.AccountNumber, Equals, int64(3530305))
	c.Check(acc.Sequence, Equals, int64(3))
	c.Check(acc.Coins.Equals(expectedCoins), Equals, true)

	resultAddr := cc.GetAddress(pk)

	c.Logf(resultAddr)
	c.Check(addr, Equals, resultAddr)
}

func (s *CosmosTestSuite) TestProcessOutboundTx(c *C) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
	}))

	client, err := NewCosmosClient(s.thorKeys, config.BifrostChainConfiguration{
		ChainID: common.GAIAChain,
		RPCHost: server.URL,
		BlockScanner: config.BifrostBlockScannerConfiguration{
			RPCHost:          server.URL,
			StartBlockHeight: 1, // avoids querying thorchain for block height
		},
	}, nil, s.bridge, s.m)
	c.Assert(err, IsNil)

	vaultPubKey, err := common.NewPubKey("sthorpub1addwnpepqda0q2avvxnferqasee42lu5492jlc4zvf6u264famvg9dywgq2kz0zaecw")
	c.Assert(err, IsNil)
	outAsset, err := common.NewAsset("GAIA.ATOM")
	c.Assert(err, IsNil)
	toAddress, err := common.NewAddress("cosmos10tjz4ave7znpctgd2rfu6v2r6zkeup2dlmqtuz")
	c.Assert(err, IsNil)
	txOut := stypes.TxOutItem{
		Chain:       common.GAIAChain,
		ToAddress:   toAddress,
		VaultPubKey: vaultPubKey,
		Coins:       common.Coins{common.NewCoin(outAsset, cosmos.NewUint(24528352))},
		Memo:        "memo",
		MaxGas:      common.Gas{common.NewCoin(outAsset, cosmos.NewUint(235824))},
		GasRate:     750000,
		InHash:      "hash",
	}

	msg, err := client.processOutboundTx(txOut, 1)
	c.Assert(err, IsNil)

	expectedAmount := int64(245283)
	expectedDenom := "uatom"
	c.Check(msg.Amount[0].Amount.Int64(), Equals, expectedAmount)
	c.Check(msg.Amount[0].Denom, Equals, expectedDenom)
	c.Logf(msg.FromAddress)
	c.Check(msg.FromAddress, Equals, "cosmos126kpfewtlc7agqjrwdl2wfg0txkphsawus338n")
	c.Check(msg.ToAddress, Equals, toAddress.String())
}

func (s *CosmosTestSuite) TestSign(c *C) {
	priv, err := s.thorKeys.GetPrivateKey()
	c.Assert(err, IsNil)

	temp, err := cryptocodec.ToTmPubKeyInterface(priv.PubKey())
	c.Assert(err, IsNil)

	pk, err := common.NewPubKeyFromCrypto(temp)
	c.Assert(err, IsNil)

	localKm := &keyManager{
		privKey: priv,
		addr:    types.AccAddress(priv.PubKey().Address()),
		pubkey:  pk,
	}

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	interfaceRegistry.RegisterImplementations((*types.Msg)(nil), &btypes.MsgSend{})
	marshaler := codec.NewProtoCodec(interfaceRegistry)

	clientConfig := config.BifrostChainConfiguration{ChainID: common.GAIAChain}
	scannerConfig := config.BifrostBlockScannerConfiguration{ChainID: common.GAIAChain}
	txConfig := tx.NewTxConfig(marshaler, []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})

	mockTmServiceClient := NewMockTmServiceClient()
	mockAccountServiceClient := NewMockAccountServiceClient()
	mockBankServiceClient := NewMockBankServiceClient()

	client := CosmosClient{
		cfg:             clientConfig,
		txConfig:        txConfig,
		cosmosScanner:   &CosmosBlockScanner{cfg: scannerConfig, tmService: mockTmServiceClient},
		bankClient:      mockBankServiceClient,
		accountClient:   mockAccountServiceClient,
		chainID:         "columbus-5",
		localKeyManager: localKm,
		accts:           NewCosmosMetaDataStore(),
	}

	vaultPubKey, err := common.NewPubKey(pk.String())
	c.Assert(err, IsNil)
	outAsset, err := common.NewAsset("GAIA.ATOM")
	c.Assert(err, IsNil)
	toAddress, err := common.NewAddress("cosmos10tjz4ave7znpctgd2rfu6v2r6zkeup2dlmqtuz")
	c.Assert(err, IsNil)
	txOut := stypes.TxOutItem{
		Chain:       common.GAIAChain,
		ToAddress:   toAddress,
		VaultPubKey: vaultPubKey,
		Coins:       common.Coins{common.NewCoin(outAsset, cosmos.NewUint(24528352))},
		Memo:        "memo",
		MaxGas:      common.Gas{common.NewCoin(outAsset, cosmos.NewUint(235824))},
		GasRate:     750000,
		InHash:      "hash",
	}

	msg, err := client.processOutboundTx(txOut, 1)
	c.Assert(err, IsNil)

	meta := client.accts.Get(pk)
	c.Check(meta.AccountNumber, Equals, int64(0))
	c.Check(meta.SeqNumber, Equals, int64(0))

	gas := types.NewCoins(types.NewCoin("uatom", types.NewInt(100)))

	txb, err := buildUnsigned(
		txConfig,
		msg,
		vaultPubKey,
		"memo",
		gas,
		uint64(meta.AccountNumber),
		uint64(meta.SeqNumber),
	)
	c.Assert(err, IsNil)

	c.Check(txb.GetTx().GetFee().IsEqual(gas), Equals, true)
	c.Check(txb.GetTx().GetMemo(), Equals, "memo")
	pks, err := txb.GetTx().GetPubKeys()
	c.Assert(err, IsNil)

	c.Check(pks[0].Address().String(), Equals, priv.PubKey().Address().String())

	// Ensure the signature is present but tranasaction has not been signed yet
	sigs, err := txb.GetTx().GetSignaturesV2()
	c.Assert(err, IsNil)
	c.Check(sigs[0].PubKey.String(), Equals, priv.PubKey().String())
	sigData, ok := sigs[0].Data.(*signingtypes.SingleSignatureData)
	c.Check(ok, Equals, true)
	c.Check(sigData.SignMode, Equals, signingtypes.SignMode_SIGN_MODE_DIRECT)
	c.Check(len(sigData.Signature), Equals, 0)

	// Sign the message
	_, err = client.signMsg(
		txb,
		vaultPubKey,
		uint64(meta.AccountNumber),
		uint64(meta.SeqNumber),
	)
	c.Assert(err, IsNil)
}
