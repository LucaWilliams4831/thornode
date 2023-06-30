package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type TypeTssKeysignFailTestSuite struct{}

var _ = Suite(&TypeTssKeysignFailTestSuite{})

func (s *TypeTssKeysignFailTestSuite) TestVoter(c *C) {
	nodes := []Node{
		{Pubkey: GetRandomPubKey().String()},
		{Pubkey: GetRandomPubKey().String()},
		{Pubkey: GetRandomPubKey().String()},
	}
	b := Blame{BlameNodes: nodes, FailReason: "fail to keysign"}
	m, err := NewMsgTssKeysignFail(1, b, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, GetRandomBech32Addr(), GetRandomPubKey())
	c.Assert(err, IsNil)
	tss := NewTssKeysignFailVoter(m.ID, 1)
	c.Check(tss.Empty(), Equals, false)
	c.Check(tss.String(), Equals, tss.ID)

	addr := GetRandomBech32Addr()
	c.Check(tss.HasSigned(addr), Equals, false)
	tss.Sign(addr)
	c.Check(tss.Signers, HasLen, 1)
	c.Check(tss.HasSigned(addr), Equals, true)
	tss.Sign(addr) // ensure signing twice doesn't duplicate
	c.Check(tss.Signers, HasLen, 1)

	c.Check(tss.HasConsensus(nil), Equals, false)
	nas := NodeAccounts{
		NodeAccount{NodeAddress: addr, Status: NodeStatus_Active},
	}
	c.Check(tss.HasConsensus(nas), Equals, true)
}
