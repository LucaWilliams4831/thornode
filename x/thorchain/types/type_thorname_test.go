package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
)

type THORNameSuite struct{}

var _ = Suite(&THORNameSuite{})

func (THORNameSuite) TestTHORName(c *C) {
	// happy path
	n := NewTHORName("iamthewalrus", 0, []THORNameAlias{{Chain: common.THORChain, Address: GetRandomTHORAddress()}})
	c.Check(n.Valid(), IsNil)

	// unhappy path
	n1 := NewTHORName("", 0, []THORNameAlias{{Chain: common.BNBChain, Address: GetRandomTHORAddress()}})
	c.Check(n1.Valid(), NotNil)
	n2 := NewTHORName("hello", 0, []THORNameAlias{{Chain: common.EmptyChain, Address: GetRandomTHORAddress()}})
	c.Check(n2.Valid(), NotNil)
	n3 := NewTHORName("hello", 0, []THORNameAlias{{Chain: common.THORChain, Address: common.Address("")}})
	c.Check(n3.Valid(), NotNil)
}
