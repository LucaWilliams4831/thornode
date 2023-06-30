package types

import (
	"testing"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func TestPackage(t *testing.T) { TestingT(t) }

var (
	bnbSingleTxFee = cosmos.NewUint(37500)
	bnbMultiTxFee  = cosmos.NewUint(30000)
)

// Gas Fees
var BNBGasFeeSingleton = common.Gas{
	{Asset: common.BNBAsset, Amount: bnbSingleTxFee},
}

var BNBGasFeeMulti = common.Gas{
	{Asset: common.BNBAsset, Amount: bnbMultiTxFee},
}

type TypesSuite struct{}

var _ = Suite(&TypesSuite{})

func (s TypesSuite) TestHasSuperMajority(c *C) {
	// happy path
	c.Check(HasSuperMajority(3, 4), Equals, true)
	c.Check(HasSuperMajority(2, 3), Equals, true)
	c.Check(HasSuperMajority(4, 4), Equals, true)
	c.Check(HasSuperMajority(1, 1), Equals, true)
	c.Check(HasSuperMajority(67, 100), Equals, true)

	// unhappy path
	c.Check(HasSuperMajority(2, 4), Equals, false)
	c.Check(HasSuperMajority(9, 4), Equals, false)
	c.Check(HasSuperMajority(-9, 4), Equals, false)
	c.Check(HasSuperMajority(9, -4), Equals, false)
	c.Check(HasSuperMajority(0, 0), Equals, false)
	c.Check(HasSuperMajority(3, 0), Equals, false)
	c.Check(HasSuperMajority(8, 15), Equals, false)
}

func (TypesSuite) TestHasSimpleMajority(c *C) {
	c.Check(HasSimpleMajority(3, 4), Equals, true)
	c.Check(HasSimpleMajority(2, 3), Equals, true)
	c.Check(HasSimpleMajority(1, 2), Equals, true)
	c.Check(HasSimpleMajority(1, 3), Equals, false)
	c.Check(HasSimpleMajority(2, 4), Equals, true)
	c.Check(HasSimpleMajority(100000, 3000000), Equals, false)
}

func (TypesSuite) TestHasMinority(c *C) {
	c.Check(HasMinority(3, 4), Equals, true)
	c.Check(HasMinority(2, 3), Equals, true)
	c.Check(HasMinority(1, 2), Equals, true)
	c.Check(HasMinority(1, 3), Equals, true)
	c.Check(HasMinority(2, 4), Equals, true)
	c.Check(HasMinority(1, 4), Equals, false)
	c.Check(HasMinority(100000, 3000000), Equals, false)
}

func (TypesSuite) TestGetThreshold(c *C) {
	_, err := GetThreshold(-2)
	c.Assert(err, NotNil)
	output, err := GetThreshold(4)
	c.Assert(err, IsNil)
	c.Assert(output, Equals, 3)
	output, err = GetThreshold(9)
	c.Assert(err, IsNil)
	c.Assert(output, Equals, 6)
	output, err = GetThreshold(10)
	c.Assert(err, IsNil)
	c.Assert(output, Equals, 7)
	output, err = GetThreshold(99)
	c.Assert(err, IsNil)
	c.Assert(output, Equals, 66)
}

func EnsureMsgBasicCorrect(m cosmos.Msg, c *C) {
	signers := m.GetSigners()
	c.Check(signers, NotNil)
	c.Check(len(signers), Equals, 1)
	c.Check(m.ValidateBasic(), IsNil)
}
