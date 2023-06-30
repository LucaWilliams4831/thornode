package thorchain

import (
	"os"

	"gitlab.com/thorchain/thornode/common/cosmos"
	. "gopkg.in/check.v1"
)

type PreTHORNameTestSuite struct{}

var _ = Suite(&PreTHORNameTestSuite{})

func (s *PreTHORNameTestSuite) TestLoadingJson(c *C) {
	// use the mainnet preregister thornames for test
	var err error
	preregisterTHORNames, err = os.ReadFile("preregister_thornames.json")
	c.Assert(err, IsNil)

	ctx, _ := setupKeeperForTest(c)
	config := cosmos.GetConfig()
	config.SetBech32PrefixForAccount("thor", "thorpub")
	names, err := getPreRegisterTHORNames(ctx, 100)
	c.Assert(err, IsNil)
	c.Check(names, HasLen, 9195, Commentf("%d", len(names)))
}
