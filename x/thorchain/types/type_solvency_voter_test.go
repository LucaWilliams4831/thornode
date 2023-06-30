package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	. "gopkg.in/check.v1"
)

type TypeSolvencyVoterTestSuite struct{}

var _ = Suite(&TypeSolvencyVoterTestSuite{})

func (s *TypeSolvencyVoterTestSuite) TestSolvencyVoter(c *C) {
	signer := GetRandomBech32Addr()
	msg, err := NewMsgSolvency(common.BTCChain, GetRandomPubKey(), common.NewCoins(
		common.NewCoin(common.BTCAsset, cosmos.NewUint(1024)),
	), 1024,
		signer)
	c.Assert(err, IsNil)
	voter := NewSolvencyVoter(msg.Id, msg.Chain, msg.PubKey, msg.Coins, msg.Height, msg.Signer)
	c.Assert(voter.Empty(), Equals, false)
	c.Assert(voter.String() != "", Equals, true)
	addr := GetRandomBech32Addr()
	c.Check(voter.HasSigned(addr), Equals, false)
	c.Assert(voter.Sign(addr), Equals, true)
	c.Assert(voter.Signers, HasLen, 2)
	c.Assert(voter.HasSigned(addr), Equals, true)
	c.Assert(voter.Sign(addr), Equals, false)
	c.Assert(voter.Signers, HasLen, 2)
	c.Assert(voter.HasConsensus(nil), Equals, false)
	nas := NodeAccounts{
		NodeAccount{NodeAddress: addr, Status: NodeStatus_Active},
		NodeAccount{NodeAddress: msg.Signer, Status: NodeStatus_Active},
	}
	c.Assert(voter.HasConsensus(nas), Equals, true)
}
