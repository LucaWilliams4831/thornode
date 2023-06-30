package tokenlist

import (
	"gitlab.com/thorchain/thornode/constants"
	. "gopkg.in/check.v1"
)

type ETHTokenListSuite struct{}

var _ = Suite(&ETHTokenListSuite{})

func (s ETHTokenListSuite) TestLoad(c *C) {
	tokens := GetETHTokenList(constants.SWVersion)
	c.Check(tokens.Name, Equals, "Testnet Token List")
	c.Check(len(tokens.Tokens) > 0, Equals, true)
}
