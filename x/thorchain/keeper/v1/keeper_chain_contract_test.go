package keeperv1

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
)

type KeeperChainContractSuite struct{}

var _ = Suite(&KeeperChainContractSuite{})

func (s *KeeperChainContractSuite) TestChainContractVoter(c *C) {
	ctx, k := setupKeeperForTest(c)
	chain := common.ETHChain
	addr := GetRandomBNBAddress()
	cc := NewChainContract(chain, addr)
	k.SetChainContract(ctx, cc)
	cc, err := k.GetChainContract(ctx, chain)
	c.Assert(err, IsNil)
	c.Check(cc.Chain.Equals(chain), Equals, true)
	c.Check(cc.Router.Equals(addr), Equals, true)

	cc1, err := k.GetChainContract(ctx, common.BTCChain)
	c.Check(err, IsNil)
	c.Check(cc1.IsEmpty(), Equals, true)

	iter := k.GetChainContractIterator(ctx)
	c.Check(iter, NotNil)
	iter.Close()
}
