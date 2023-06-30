//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package bitcoincash

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
)

func (s *BitcoinCashSuite) TestGetAddress(c *C) {
	pubkey := common.PubKey("thorpub1addwnpepqt7qug8vk9r3saw8n4r803ydj2g3dqwx0mvq5akhnze86fc536xcy2cr8a2")
	addr := s.client.GetAddress(pubkey)
	c.Assert(addr, Equals, "qpfztpuwwujkvvenjm7mg9d6mzqkmqwshv07z34njm")
}
