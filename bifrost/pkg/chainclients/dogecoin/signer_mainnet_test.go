//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package dogecoin

import (
	"github.com/eager7/dogd/chaincfg"
	. "gopkg.in/check.v1"
)

func (s *DogecoinSignerSuite) TestGetChainCfg(c *C) {
	param := s.client.getChainCfg()
	c.Assert(param, Equals, &chaincfg.MainNetParams)
}
