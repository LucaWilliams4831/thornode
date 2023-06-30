package types

import (
	"encoding/json"
	"sort"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type NodeAccountSuite struct{}

var _ = Suite(&NodeAccountSuite{})

func (NodeAccountSuite) TestNodeAccount(c *C) {
	addr := GetRandomBech32Addr()
	c.Check(addr.Empty(), Equals, false)
	bepConsPubKey := GetRandomBech32ConsensusPubKey()
	nodeAddress := GetRandomBech32Addr()
	bondAddr := GetRandomBNBAddress()
	pubKeys := common.PubKeySet{
		Secp256k1: GetRandomPubKey(),
		Ed25519:   GetRandomPubKey(),
	}

	na := NewNodeAccount(nodeAddress, NodeStatus_Active, pubKeys, bepConsPubKey, cosmos.NewUint(common.One), bondAddr, 1)
	result, err := json.MarshalIndent(na, "", "	")
	c.Assert(err, IsNil)
	c.Log(string(result))
	c.Assert(na.IsEmpty(), Equals, false)
	c.Assert(na.Valid(), IsNil)
	c.Assert(na.Bond.Uint64(), Equals, uint64(common.One))
	nas := NodeAccounts{
		na,
	}
	c.Assert(nas.IsNodeKeys(addr), Equals, false)
	c.Assert(nas.IsNodeKeys(nodeAddress), Equals, true)
	naEmpty := NewNodeAccount(cosmos.AccAddress{}, NodeStatus_Active, pubKeys, bepConsPubKey, cosmos.NewUint(common.One), bondAddr, 1)
	c.Assert(naEmpty.Valid(), NotNil)
	c.Assert(naEmpty.IsEmpty(), Equals, true)
	invalidBondAddr := NewNodeAccount(cosmos.AccAddress{}, NodeStatus_Active, pubKeys, bepConsPubKey, cosmos.NewUint(common.One), "", 1)
	c.Assert(invalidBondAddr.Valid(), NotNil)

	na1 := NewNodeAccount(nodeAddress, NodeStatus_Active, pubKeys, bepConsPubKey, cosmos.NewUint(common.One), common.NoAddress, 1)
	c.Check(na1.Valid(), NotNil)

	na2 := NewNodeAccount(nodeAddress, NodeStatus_Unknown, pubKeys, bepConsPubKey, cosmos.NewUint(common.One), bondAddr, 1)
	c.Check(na2.Valid(), NotNil)

	na3 := NewNodeAccount(nodeAddress, NodeStatus_Active, pubKeys, bepConsPubKey, cosmos.NewUint(common.One), bondAddr, 1)
	c.Check(na3.Equals(na), Equals, true)
	c.Check(na3.Equals(na1), Equals, false)
}

func (NodeAccountSuite) TestNodeAccountsSort(c *C) {
	var accounts NodeAccounts
	for {
		na := GetRandomValidatorNode(NodeStatus_Active)
		dup := false
		for _, node := range accounts {
			if na.NodeAddress.Equals(node.NodeAddress) {
				dup = true
			}
		}
		if dup {
			continue
		}
		accounts = append(accounts, na)
		if len(accounts) == 10 {
			break
		}
	}

	sort.Sort(accounts)

	for i, na := range accounts {
		if i == 0 {
			continue
		}
		if na.NodeAddress.String() < accounts[i].NodeAddress.String() {
			c.Errorf("%s should be before %s", na.NodeAddress, accounts[i].NodeAddress)
		}
	}
	c.Check(accounts.IsEmpty(), Equals, false)
	c.Check(accounts.Contains(accounts[0]), Equals, true)
	var emptyNodeAccounts NodeAccounts
	c.Check(emptyNodeAccounts.Contains(accounts[0]), Equals, false)
	c.Check(emptyNodeAccounts.IsEmpty(), Equals, true)
}

func (NodeAccountSuite) TestNodeAccountUpdateStatusAndSort(c *C) {
	var accounts NodeAccounts
	for i := 0; i < 10; i++ {
		na := GetRandomValidatorNode(NodeStatus_Active)
		accounts = append(accounts, na)
	}
	isSorted := sort.SliceIsSorted(accounts, func(i, j int) bool {
		return accounts[i].StatusSince < accounts[j].StatusSince
	})
	c.Assert(isSorted, Equals, true)
}

func (NodeAccountSuite) TestTryAddSignerPubKey(c *C) {
	na := NewNodeAccount(GetRandomBech32Addr(), NodeStatus_Active, GetRandomPubKeySet(), GetRandomBech32ConsensusPubKey(), cosmos.NewUint(100*common.One), GetRandomBNBAddress(), 1)
	pk := GetRandomPubKey()
	emptyPK := common.EmptyPubKey
	// make sure it get added
	na.TryAddSignerPubKey(pk)
	c.Assert(na.SignerMembership, NotNil)
	c.Assert(na.SignerMembership, HasLen, 1)
	na.TryAddSignerPubKey(emptyPK)
	c.Assert(na.SignerMembership, HasLen, 1)

	// add the same key again should be a noop
	na.TryAddSignerPubKey(pk)
	c.Assert(len(na.SignerMembership), Equals, 1)
	na.TryRemoveSignerPubKey(emptyPK)
	c.Assert(len(na.SignerMembership), Equals, 1)

	na.TryRemoveSignerPubKey(pk)
	c.Assert(na.SignerMembership, HasLen, 0)
}

func (s *NodeAccountSuite) TestCalcNodeRewards(c *C) {
	na := NodeAccount{
		ActiveBlockHeight: 30,
	}
	blocks := na.CalcBondUnits(50, 2)
	c.Check(blocks.Uint64(), Equals, uint64(18))

	na = NodeAccount{
		ActiveBlockHeight: 30,
	}
	blocks = na.CalcBondUnits(50, 100000)
	c.Check(blocks.Uint64(), Equals, uint64(0))

	na = NodeAccount{
		ActiveBlockHeight: 100,
	}
	blocks = na.CalcBondUnits(50, 0)
	c.Check(blocks.Uint64(), Equals, uint64(0))

	na = NodeAccount{
		ActiveBlockHeight: 30,
	}
	blocks = na.CalcBondUnits(-50, 0)
	c.Check(blocks.Uint64(), Equals, uint64(0))

	na = NodeAccount{
		ActiveBlockHeight: -100,
	}
	blocks = na.CalcBondUnits(50, 0)
	c.Check(blocks.Uint64(), Equals, uint64(0), Commentf("%d", blocks.Uint64()))
}

func (s *NodeAccountSuite) TestBondProvider(c *C) {
	provider := BondProvider{}
	c.Assert(provider.IsEmpty(), Equals, true)
	provider = NewBondProvider(GetRandomBech32Addr())
	c.Assert(provider.IsEmpty(), Equals, false)
}

func (s *NodeAccountSuite) TestBondProviders(c *C) {
	acc1 := GetRandomBech32Addr()
	acc2 := GetRandomBech32Addr()
	acc3 := GetRandomBech32Addr()
	p1 := NewBondProvider(acc1)
	p2 := NewBondProvider(acc2)
	p3 := NewBondProvider(acc3)

	bp := NewBondProviders(acc1)
	bp.NodeOperatorFee = cosmos.NewUint(2000)
	bp.Providers = []BondProvider{p1, p2, p3}

	// Provider hasn't bonded yet
	c.Assert(bp.HasProviderBonded(acc1), Equals, false)

	bp.Bond(cosmos.NewUint(300000), acc1)
	// Provider still hasn't bonded
	c.Assert(bp.HasProviderBonded(acc1), Equals, false)
	bp.Bond(cosmos.NewUint(100000), acc2)
	// Provider has bonded
	c.Assert(bp.HasProviderBonded(acc1), Equals, true)
	bp.Bond(cosmos.NewUint(50000), acc3)

	c.Assert(bp.Has(acc1), Equals, true)
	c.Assert(bp.Get(acc1).Bond.Uint64(), Equals, uint64(300000))
	bp.Unbond(cosmos.NewUint(100000), acc1)
	c.Assert(bp.Get(acc1).Bond.Uint64(), Equals, uint64(200000))
	bp.Remove(acc3) // unsuccessful remove, due to bond still being there
	c.Assert(bp.Get(acc3).Bond.Uint64(), Equals, uint64(50000))
	bp.Unbond(cosmos.NewUint(100000), acc3)
	bp.Remove(acc3) // unsuccessful remove, due to bond still being there
	c.Assert(bp.Has(acc3), Equals, false)

	// testing adjust
	acc1 = GetRandomBech32Addr()
	acc2 = GetRandomBech32Addr()
	acc3 = GetRandomBech32Addr()
	p1 = NewBondProvider(acc1)
	p2 = NewBondProvider(acc2)
	p3 = NewBondProvider(acc3)

	bp = NewBondProviders(acc1)
	bp.NodeOperatorFee = cosmos.NewUint(2000)
	bp.Providers = []BondProvider{p1, p2, p3}
	bp.Bond(cosmos.NewUint(300000), acc1)
	bp.Bond(cosmos.NewUint(100000), acc2)
	bp.Bond(cosmos.NewUint(50000), acc3)

	version := GetCurrentVersion()
	bp.Adjust(version, cosmos.NewUint(500000))
	c.Check(bp.Get(acc1).Bond.Uint64(), Equals, uint64(336667))
	c.Check(bp.Get(acc2).Bond.Uint64(), Equals, uint64(108889))
	c.Check(bp.Get(acc3).Bond.Uint64(), Equals, uint64(54444))

	// no node operator fee
	bp = NewBondProviders(acc1)
	bp.NodeOperatorFee = cosmos.NewUint(0)
	bp.Providers = []BondProvider{p1, p2, p3}
	bp.Bond(cosmos.NewUint(300000), acc1)
	bp.Bond(cosmos.NewUint(100000), acc2)
	bp.Bond(cosmos.NewUint(50000), acc3)

	bp.Adjust(version, cosmos.NewUint(500000))
	c.Check(bp.Get(acc1).Bond.Uint64(), Equals, uint64(333333))
	c.Check(bp.Get(acc2).Bond.Uint64(), Equals, uint64(111111))
	c.Check(bp.Get(acc3).Bond.Uint64(), Equals, uint64(55556))

	// Using as an example node thor12qwtrq4njj2s29gq56jun43dvxalejaksptqqn's
	// state at the end of block 4707078.
	bp.NodeOperatorFee = cosmos.NewUint(500)
	bp.Providers = []BondProvider{p1}
	bp.Providers[0].Bond = cosmos.ZeroUint()
	nodeBond := cosmos.NewUint(75773500000000)
	bp.Adjust(GetCurrentVersion(), nodeBond)
	c.Assert(bp.Providers[0].Bond.Equal(nodeBond), Equals, true)
	// The sole provider's bond equals the node bond after adjustment.

	// TODO remove this comparison paragraph after hard fork
	//
	// Using as an example node thor12qwtrq4njj2s29gq56jun43dvxalejaksptqqn's
	// state at the end of block 4707078.
	bp.NodeOperatorFee = cosmos.NewUint(500)
	bp.Providers = []BondProvider{p1}
	bp.Providers[0].Bond = cosmos.ZeroUint()
	nodeBond = cosmos.NewUint(75773500000000)
	bp.AdjustV1(nodeBond)
	c.Assert(bp.Providers[0].Bond.Equal(nodeBond), Equals, false)
	// The sole provider's bond doesn't equal the node bond after adjustment.
	c.Assert(bp.Providers[0].Bond.String(), Equals, "3788675000000")
	// This is .tqqn's sole provider's displayed bond at the end of block 4707078.
}
