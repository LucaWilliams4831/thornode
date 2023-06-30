//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package bitcoin

import (
	"github.com/btcsuite/btcd/chaincfg"
	. "gopkg.in/check.v1"
)

func (s *BitcoinSignerSuite) TestGetChainCfg(c *C) {
	param := s.client.getChainCfg()
	c.Assert(param, Equals, &chaincfg.MainNetParams)
}
