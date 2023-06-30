package types

import (
	"gitlab.com/thorchain/thornode/common"
	. "gopkg.in/check.v1"
)

type TypeLoanSuite struct{}

var _ = Suite(&TypeLoanSuite{})

func (mas *TypeLoanSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (TypeLoanSuite) TestLoan(c *C) {
	addr := common.Address("bnb1xw3mrgcvfmcrxc3uec3hyn2v3f56pvz569tf7c")
	loan := NewLoan(addr, common.BNBAsset, 25)

	c.Check(loan.Key(), Equals, "BNB.BNB/bnb1xw3mrgcvfmcrxc3uec3hyn2v3f56pvz569tf7c")

	// happy path
	c.Check(loan.Valid(), IsNil)

	// bad last height
	loan.LastOpenHeight = 0
	c.Check(loan.Valid(), NotNil)
	loan.LastOpenHeight = -5
	c.Check(loan.Valid(), NotNil)

	// bad owner
	loan.LastOpenHeight = 25
	loan.Owner = common.NoAddress
	c.Check(loan.Valid(), NotNil)

	// bad asset
	loan.Owner = addr
	loan.Asset = common.EmptyAsset
	c.Check(loan.Valid(), NotNil)
}
