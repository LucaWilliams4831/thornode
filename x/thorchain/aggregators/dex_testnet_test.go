package aggregators

import (
	"github.com/blang/semver"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
)

type DexAggregatorTestnetSuite struct{}

var _ = Suite(&DexAggregatorTestnetSuite{})

func (s *DexAggregatorTestnetSuite) TestDexAggregators(c *C) {
	ver := semver.MustParse("9999.0.0")

	// happy path
	addr, err := FetchDexAggregator(ver, common.ETHChain, "3848")
	c.Assert(err, IsNil)
	c.Check(addr, Equals, "0x69800327b38A4CeF30367Dec3f64c2f2386f3848")

	// unhappy path
	_, err = FetchDexAggregator(ver, common.BTCChain, "156")
	c.Assert(err, NotNil)
	_, err = FetchDexAggregator(ver, common.ETHChain, "foobar")
	c.Assert(err, NotNil)
}
