package pubkeymanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/config"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

func Test(t *testing.T) { TestingT(t) }

type PubKeyMgrSuite struct{}

var _ = Suite(&PubKeyMgrSuite{})

func (s *PubKeyMgrSuite) TestPubkeyMgr(c *C) {
	pk1 := types.GetRandomPubKey()
	pk2 := types.GetRandomPubKey()
	pk3 := types.GetRandomPubKey()
	pk4 := types.GetRandomPubKey()

	pubkeyMgr, err := NewPubKeyManager(nil, nil)
	c.Assert(err, IsNil)
	c.Check(pubkeyMgr.HasPubKey(pk1), Equals, false)
	pubkeyMgr.AddPubKey(pk1, true)
	c.Check(pubkeyMgr.HasPubKey(pk1), Equals, true)
	c.Check(pubkeyMgr.pubkeys[0].PubKey.Equals(pk1), Equals, true)
	c.Check(pubkeyMgr.pubkeys[0].Signer, Equals, true)

	pubkeyMgr.AddPubKey(pk2, false)
	c.Check(pubkeyMgr.HasPubKey(pk2), Equals, true)
	c.Check(pubkeyMgr.pubkeys[1].PubKey.Equals(pk2), Equals, true)
	c.Check(pubkeyMgr.pubkeys[1].Signer, Equals, false)

	pks := pubkeyMgr.GetPubKeys()
	c.Assert(pks, HasLen, 2)

	pks = pubkeyMgr.GetSignPubKeys()
	c.Assert(pks, HasLen, 1)
	c.Check(pks[0].Equals(pk1), Equals, true)

	// remove a pubkey
	pubkeyMgr.RemovePubKey(pk2)
	c.Check(pubkeyMgr.HasPubKey(pk2), Equals, false)

	addr, err := pk1.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	ok, _ := pubkeyMgr.IsValidPoolAddress(addr.String(), common.BNBChain)
	c.Assert(ok, Equals, true)

	addr, err = pk3.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	pubkeyMgr.AddNodePubKey(pk4)
	c.Check(pubkeyMgr.GetNodePubKey().String(), Equals, pk4.String())
	ok, _ = pubkeyMgr.IsValidPoolAddress(addr.String(), common.BNBChain)
	c.Assert(ok, Equals, false)
}

func (s *PubKeyMgrSuite) TestFetchKeys(c *C) {
	pk1 := types.GetRandomPubKey()
	pk2 := types.GetRandomPubKey()
	pk3 := types.GetRandomPubKey()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Logf("================>:%s", r.RequestURI)
		switch r.RequestURI {
		case "/thorchain/vaults/pubkeys":
			var result types.QueryVaultsPubKeys
			result.Asgard = append(result.Asgard, types.QueryVaultPubKeyContract{
				PubKey: pk1,
				Routers: []types.ChainContract{
					{
						Chain:  common.ETHChain,
						Router: "0xE65e9d372F8cAcc7b6dfcd4af6507851Ed31bb44",
					},
				},
			})
			buf, err := json.MarshalIndent(result, "", "	")
			c.Assert(err, IsNil)
			if _, err := w.Write(buf); err != nil {
				c.Error(err)
			}
		case "/thorchain/vaults/asgard":
			if _, err := w.Write([]byte(fmt.Sprintf(`[{"membership":["%s"]}]`, pk3))); err != nil {
				c.Error(err)
			}
		}
	}))

	cfg := config.BifrostClientConfiguration{
		ChainID:   "thorchain",
		ChainHost: server.URL[7:],
	}
	bridge, err := thorclient.NewThorchainBridge(cfg, nil, nil)
	c.Assert(err, IsNil)
	pubkeyMgr, err := NewPubKeyManager(bridge, nil)
	c.Assert(err, IsNil)
	hasCallbackFired := false
	callBack := func(pk common.PubKey) error {
		hasCallbackFired = true
		return nil
	}
	pubkeyMgr.RegisterCallback(callBack)
	pubkeyMgr.AddPubKey(pk2, false)
	c.Check(hasCallbackFired, Equals, true)
	// add a key that is the node account, ensure it doesn't get removed
	pubkeyMgr.pubkeys = append(pubkeyMgr.pubkeys, pubKeyInfo{
		PubKey:      pk3,
		Signer:      true,
		NodeAccount: true,
		Contracts:   map[common.Chain]common.Address{},
	})
	c.Check(len(pubkeyMgr.GetPubKeys()), Equals, 2)
	err = pubkeyMgr.Start()
	c.Assert(err, IsNil)
	pubkeyMgr.fetchPubKeys(true)
	pubKeys := pubkeyMgr.GetPubKeys()
	c.Check(len(pubKeys), Equals, 3)
	c.Check(pubKeys[0].Equals(pk1), Equals, false)
	c.Check(pubKeys[1].Equals(pk3), Equals, true)
	c.Check(pubkeyMgr.pubkeys[1].Signer, Equals, true)
	err = pubkeyMgr.Stop()
	c.Assert(err, IsNil)
}
