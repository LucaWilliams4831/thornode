package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
)

type ChainContractTestSuite struct{}

var _ = Suite(&ChainContractTestSuite{})

func (ChainContractTestSuite) TestChainContract_basic(c *C) {
	chainContract := NewChainContract(common.ETHChain, common.Address("0xE65e9d372F8cAcc7b6dfcd4af6507851Ed31bb44"))
	c.Assert(chainContract.IsEmpty(), Equals, false)
	c.Assert(chainContract.String(), Equals, "ETH-0xE65e9d372F8cAcc7b6dfcd4af6507851Ed31bb44")
	emptyChainContract := ChainContract{}
	c.Assert(emptyChainContract.IsEmpty(), Equals, true)
}
