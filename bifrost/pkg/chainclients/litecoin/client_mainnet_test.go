//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package litecoin

import (
	"gitlab.com/thorchain/thornode/common"
	. "gopkg.in/check.v1"
)

func (s *LitecoinSuite) TestGetAddress(c *C) {
	pubkey := common.PubKey("thorpub1addwnpepqt7qug8vk9r3saw8n4r803ydj2g3dqwx0mvq5akhnze86fc536xcy2cr8a2")
	addr := s.client.GetAddress(pubkey)
	c.Assert(addr, Equals, "ltc1q2gjc0rnhy4nrxvuklk6ptwkcs9kcr59mursyaz")
}
