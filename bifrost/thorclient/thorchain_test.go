package thorclient

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	ckeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/cmd"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/config"
	stypes "gitlab.com/thorchain/thornode/x/thorchain/types"
)

func TestPackage(t *testing.T) { TestingT(t) }

type ThorchainSuite struct {
	server             *httptest.Server
	cfg                config.BifrostClientConfiguration
	bridge             *thorchainBridge
	authAccountFixture string
	nodeAccountFixture string
}

var _ = Suite(&ThorchainSuite{})

func (s *ThorchainSuite) SetUpTest(c *C) {
	cfg2 := cosmos.GetConfig()
	cfg2.SetBech32PrefixForAccount(cmd.Bech32PrefixAccAddr, cmd.Bech32PrefixAccAddr)
	cfg, _, kb := SetupThorchainForTest(c)
	s.cfg = cfg
	s.server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasPrefix(req.RequestURI, AuthAccountEndpoint):
			httpTestHandler(c, rw, s.authAccountFixture)
		case strings.HasPrefix(req.RequestURI, NodeAccountEndpoint):
			httpTestHandler(c, rw, s.nodeAccountFixture)
		case strings.HasPrefix(req.RequestURI, LastBlockEndpoint):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/lastblock/bnb.json")
		case strings.HasPrefix(req.RequestURI, StatusEndpoint):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/status/status.json")
		case strings.HasPrefix(req.RequestURI, KeysignEndpoint):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/keysign/template.json")
		case strings.HasPrefix(req.RequestURI, "/thorchain/vaults") && strings.HasSuffix(req.RequestURI, "/signers"):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/tss/keysign_party.json")
		case strings.HasPrefix(req.RequestURI, AsgardVault):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/vaults/asgard.json")
		case strings.HasPrefix(req.RequestURI, PubKeysEndpoint):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/vaults/pubKeys.json")
		case strings.EqualFold(req.RequestURI, BroadcastTxsEndpoint):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/txs/success.json")
		case strings.HasPrefix(req.RequestURI, ThorchainConstants):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/constants/constants.json")
		case strings.HasPrefix(req.RequestURI, RagnarokEndpoint):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/ragnarok/ragnarok.json")
		case strings.HasPrefix(req.RequestURI, ChainVersionEndpoint):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/version/version.json")
		case strings.HasPrefix(req.RequestURI, MimirEndpoint):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/mimir/mimir.json")
		case strings.HasPrefix(req.RequestURI, InboundAddressesEndpoint):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/inbound_addresses/inbound_addresses.json")
		case strings.HasPrefix(req.RequestURI, "/thorchain/thorname/"):
			httpTestHandler(c, rw, "../../test/fixtures/endpoints/thorname/thorname.json")
		}
	}))
	s.cfg.ChainHost = s.server.Listener.Addr().String()
	s.cfg.ChainRPC = s.server.Listener.Addr().String()

	var err error
	bridge, err := NewThorchainBridge(s.cfg, GetMetricForTest(c), NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd))
	var ok bool
	s.bridge, ok = bridge.(*thorchainBridge)
	c.Assert(ok, Equals, true)
	s.bridge.httpClient.RetryMax = 1 // fail fast
	c.Assert(err, IsNil)
	c.Assert(s.bridge, NotNil)
}

func (s *ThorchainSuite) TearDownTest(c *C) {
	s.server.Close()
}

func (s *ThorchainSuite) TestGetThorChainURL(c *C) {
	uri := s.bridge.getThorChainURL("")
	c.Assert(uri, Equals, "http://"+s.server.Listener.Addr().String())
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

func (s *ThorchainSuite) TestGet(c *C) {
	buf, status, err := s.bridge.getWithPath("")
	c.Check(status, Equals, http.StatusOK)
	c.Assert(err, IsNil)
	c.Assert(buf, NotNil)
}

func (s *ThorchainSuite) TestGetObservationStdTx_OutboundShouldHaveNotConfirmationCounting(c *C) {
	pk := stypes.GetRandomPubKey()
	vaultAddr, err := pk.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	tx := stypes.NewObservedTx(
		common.Tx{
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(123400000)),
			},
			Memo:        "This is my memo!",
			FromAddress: vaultAddr,
			ToAddress:   "bnb1ntqj0v0sv62ut0ehxt7jqh7lenfrd3hmfws0aq",
		},
		1,
		pk,
		100,
	)

	signedMsg, err := s.bridge.GetObservationsStdTx(stypes.ObservedTxs{tx})
	c.Assert(signedMsg, NotNil)
	c.Assert(err, IsNil)
	m, ok := signedMsg[0].(*stypes.MsgObservedTxOut)
	c.Assert(ok, Equals, true)
	c.Assert(m.Txs[0].FinaliseHeight == m.Txs[0].BlockHeight, Equals, true)
}

func (s *ThorchainSuite) TestSign(c *C) {
	pk := stypes.GetRandomPubKey()
	vaultAddr, err := pk.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	tx := stypes.NewObservedTx(
		common.Tx{
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(123400000)),
			},
			Memo:        "This is my memo!",
			FromAddress: vaultAddr,
			ToAddress:   common.Address("bnb1ntqj0v0sv62ut0ehxt7jqh7lenfrd3hmfws0aq"),
		},
		1,
		pk,
		1,
	)

	signedMsg, err := s.bridge.GetObservationsStdTx(stypes.ObservedTxs{tx})
	c.Log(err)
	c.Assert(signedMsg, NotNil)
	c.Assert(err, IsNil)
}

func (ThorchainSuite) TestNewThorchainBridge(c *C) {
	testFunc := func(cfg config.BifrostClientConfiguration, errChecker, sbChecker Checker) {
		kb := ckeys.NewInMemory()
		_, _, err := kb.NewMnemonic(cfg.SignerName, cKeys.English, cmd.THORChainHDPath, cfg.SignerPasswd, hd.Secp256k1)
		c.Assert(err, IsNil)
		sb, err := NewThorchainBridge(cfg, m, NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd))
		c.Assert(err, errChecker)
		c.Assert(sb, sbChecker)
	}
	testFunc(config.BifrostClientConfiguration{
		ChainID:         "",
		ChainHost:       "localhost",
		ChainHomeFolder: "~/.thorcli",
		SignerName:      "signer",
		SignerPasswd:    "signerpassword",
	}, NotNil, IsNil)
	testFunc(config.BifrostClientConfiguration{
		ChainID:         "chainid",
		ChainHost:       "",
		ChainHomeFolder: "~/.thorcli",
		SignerName:      "signer",
		SignerPasswd:    "signerpassword",
	}, NotNil, IsNil)
}

func (s *ThorchainSuite) TestGetAccountNumberAndSequenceNumber_Success(c *C) {
	s.nodeAccountFixture = "../../test/fixtures/endpoints/nodeaccount/template.json"
	s.authAccountFixture = "../../test/fixtures/endpoints/auth/accounts/template.json"
	accNumber, sequence, err := s.bridge.getAccountNumberAndSequenceNumber()
	c.Assert(err, IsNil)
	c.Assert(accNumber, Equals, uint64(3))
	c.Assert(sequence, Equals, uint64(5))
}

func (s *ThorchainSuite) TestGetAccountNumberAndSequenceNumber_Fail(c *C) {
	s.nodeAccountFixture = "../../test/fixtures/endpoints/nodeaccount/template.json"
	s.authAccountFixture = ""
	accNumber, sequence, err := s.bridge.getAccountNumberAndSequenceNumber()
	c.Assert(err, NotNil)
	c.Assert(accNumber, Equals, uint64(0))
	c.Assert(sequence, Equals, uint64(0))
}

func (s *ThorchainSuite) TestGetAccountNumberAndSequenceNumber_Fail_500(c *C) {
	s.nodeAccountFixture = "../../test/fixtures/endpoints/nodeaccount/template.json"
	s.authAccountFixture = "500"
	accNumber, sequence, err := s.bridge.getAccountNumberAndSequenceNumber()
	c.Assert(err, NotNil)
	c.Assert(accNumber, Equals, uint64(0))
	c.Assert(sequence, Equals, uint64(0))
}

func (s *ThorchainSuite) TestGetAccountNumberAndSequenceNumber_Fail_Unmarshal(c *C) {
	s.nodeAccountFixture = "../../test/fixtures/endpoints/nodeaccount/template.json"
	s.authAccountFixture = "../../test/fixtures/endpoints/auth/accounts/malformed.json"
	accNumber, sequence, err := s.bridge.getAccountNumberAndSequenceNumber()
	c.Assert(err, NotNil)
	c.Assert(true, Equals, strings.HasPrefix(err.Error(), "failed to unmarshal account resp"))
	c.Assert(accNumber, Equals, uint64(0))
	c.Assert(sequence, Equals, uint64(0))
}

func (s *ThorchainSuite) TestEnsureNodeWhitelisted_Success(c *C) {
	s.authAccountFixture = "../../test/fixtures/endpoints/auth/accounts/template.json"
	s.nodeAccountFixture = "../../test/fixtures/endpoints/nodeaccount/template.json"
	err := s.bridge.EnsureNodeWhitelisted()
	c.Assert(err, IsNil)
}

func (s *ThorchainSuite) TestEnsureNodeWhitelisted_Fail(c *C) {
	s.authAccountFixture = "../../test/fixtures/endpoints/auth/accounts/template.json"
	s.nodeAccountFixture = "../../test/fixtures/endpoints/nodeaccount/disabled.json"
	err := s.bridge.EnsureNodeWhitelisted()
	c.Assert(err, NotNil)
}

func (s *ThorchainSuite) TestGetKeysignParty(c *C) {
	pubKey := stypes.GetRandomPubKey()
	pubKeys, err := s.bridge.GetKeysignParty(pubKey)
	c.Assert(err, IsNil)
	c.Assert(pubKeys, HasLen, 3)
}

func (s *ThorchainSuite) TestIsCatchingUp(c *C) {
	ok, err := s.bridge.IsCatchingUp()
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, false)
}

func (s *ThorchainSuite) TestGetAsgards(c *C) {
	vaults, err := s.bridge.GetAsgards()
	c.Assert(err, IsNil)
	c.Assert(vaults, NotNil)
}

func (s *ThorchainSuite) TestGetPubKeys(c *C) {
	pks, err := s.bridge.GetPubKeys()
	c.Assert(err, IsNil)
	c.Assert(pks, HasLen, 6)
}

func (s *ThorchainSuite) TestPostNetworkFee(c *C) {
	s.authAccountFixture = "../../test/fixtures/endpoints/auth/accounts/template.json"
	txid, err := s.bridge.PostNetworkFee(1024, common.BNBChain, 100, 100)
	c.Assert(err, IsNil)
	c.Assert(txid.IsEmpty(), Equals, false)
}

func (s *ThorchainSuite) TestGetConstants(c *C) {
	result, err := s.bridge.GetConstants()
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
}

func (s *ThorchainSuite) TestGetRagnarok(c *C) {
	result, err := s.bridge.RagnarokInProgress()
	c.Assert(err, IsNil)
	c.Assert(result, Equals, false)
}

func (s *ThorchainSuite) TestGetThorchainVersion(c *C) {
	result, err := s.bridge.GetThorchainVersion()
	c.Assert(err, IsNil)
	c.Assert(result.EQ(semver.MustParse("0.11.0")), Equals, true)
}

func (s *ThorchainSuite) TestGetMimir(c *C) {
	result, err := s.bridge.GetMimir("HaltBNBChain")
	c.Assert(err, IsNil)
	c.Assert(result, Equals, int64(10))
}

func (s *ThorchainSuite) TestGetContractAddress(c *C) {
	result, err := s.bridge.GetContractAddress()
	c.Assert(err, IsNil)
	c.Assert(result[0].Contracts[common.ETHChain].String(), Equals, "0xE65e9d372F8cAcc7b6dfcd4af6507851Ed31bb44")
}

func (s *ThorchainSuite) TestTHORName(c *C) {
	result, err := s.bridge.GetTHORName("test1")
	c.Assert(err, IsNil)
	c.Assert(result.Name, Equals, "test1")
	c.Assert(result.ExpireBlockHeight, Equals, int64(10000))
	c.Assert(result.Aliases, HasLen, 1)
	c.Assert(result.Aliases[0].Chain, Equals, common.THORChain)
	c.Assert(result.Aliases[0].Address, Equals, common.Address("tthor1tdfqy34uptx207scymqsy4k5uzfmry5sf7z3dw"))
}
