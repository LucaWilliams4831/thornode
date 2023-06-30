//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package bitcoincash

import (
	"github.com/gcash/bchd/chaincfg"
	. "gopkg.in/check.v1"
)

func (s *BitcoinCashSignerSuite) TestGetChainCfg(c *C) {
	param := s.client.getChainCfg()
	c.Assert(param, Equals, &chaincfg.MainNetParams)
}
