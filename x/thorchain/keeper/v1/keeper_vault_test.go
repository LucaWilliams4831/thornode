package keeperv1

import (
	"github.com/cosmos/cosmos-sdk/types"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperVaultSuite struct{}

var _ = Suite(&KeeperVaultSuite{})

func (s *KeeperVaultSuite) TestVault(c *C) {
	ctx, k := setupKeeperForTest(c)
	existVault, err := k.HasValidVaultPools(ctx)
	c.Check(err, IsNil)
	c.Check(existVault, Equals, false)

	pubKey := GetRandomPubKey()
	yggdrasil := NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, pubKey, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	err = k.SetVault(ctx, yggdrasil)
	c.Assert(err, IsNil)
	c.Assert(k.VaultExists(ctx, pubKey), Equals, true)
	pubKey1 := GetRandomPubKey()
	yggdrasil1 := NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, pubKey1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	yggdrasil1.PendingTxBlockHeights = []int64{35}
	yggdrasil1.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, types.NewUint(100)),
	}
	c.Assert(k.SetVault(ctx, yggdrasil1), IsNil)
	ygg, err := k.GetVault(ctx, pubKey1)
	c.Assert(err, IsNil)
	c.Assert(ygg.IsEmpty(), Equals, false)
	c.Assert(ygg.PendingTxBlockHeights, HasLen, 1)
	c.Assert(ygg.PendingTxBlockHeights[0], Equals, int64(35))
	hasYgg, err := k.HasValidVaultPools(ctx)
	c.Assert(err, IsNil)
	c.Assert(hasYgg, Equals, true)

	asgards, err := k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	c.Assert(err, IsNil)
	c.Assert(asgards, HasLen, 0)
	pubKey = GetRandomPubKey()
	asgard := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, pubKey, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	c.Assert(k.SetVault(ctx, asgard), IsNil)
	asgard2 := NewVault(ctx.BlockHeight(), InactiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	c.Assert(k.SetVault(ctx, asgard2), IsNil)
	asgards, err = k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	c.Assert(err, IsNil)
	c.Assert(asgards, HasLen, 1)
	c.Check(asgards[0].PubKey.Equals(pubKey), Equals, true)

	c.Assert(k.DeleteVault(ctx, pubKey), IsNil)
	c.Assert(k.DeleteVault(ctx, pubKey), IsNil) // second time should also not error
	asgards, err = k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	c.Assert(err, IsNil)
	c.Assert(asgards, HasLen, 0)

	vault1 := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	vault1.AddFunds(common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*100)),
	})
	c.Check(k.SetVault(ctx, vault1), IsNil)
	c.Check(k.DeleteVault(ctx, vault1.PubKey), NotNil)
}

func (s *KeeperVaultSuite) TestVaultSorBySecurity(c *C) {
	ctx, k := setupKeeperForTest(c)

	// Add node accounts
	na1 := GetRandomValidatorNode(NodeActive)
	na1.Bond = cosmos.NewUint(100 * common.One)
	c.Assert(k.SetNodeAccount(ctx, na1), IsNil)
	na2 := GetRandomValidatorNode(NodeActive)
	na2.Bond = cosmos.NewUint(200 * common.One)
	c.Assert(k.SetNodeAccount(ctx, na2), IsNil)
	na3 := GetRandomValidatorNode(NodeActive)
	na3.Bond = cosmos.NewUint(300 * common.One)
	c.Assert(k.SetNodeAccount(ctx, na3), IsNil)
	na4 := GetRandomValidatorNode(NodeActive)
	na4.Bond = cosmos.NewUint(400 * common.One)
	c.Assert(k.SetNodeAccount(ctx, na4), IsNil)
	na5 := GetRandomValidatorNode(NodeActive)
	na5.Bond = cosmos.NewUint(500 * common.One)
	c.Assert(k.SetNodeAccount(ctx, na5), IsNil)
	na6 := GetRandomValidatorNode(NodeActive)
	na6.Bond = cosmos.NewUint(600 * common.One)
	c.Assert(k.SetNodeAccount(ctx, na6), IsNil)

	// Create Pools
	pool1 := NewPool()
	pool1.Asset = common.BNBAsset
	pool1.BalanceRune = cosmos.NewUint(common.One * 100)
	pool1.BalanceAsset = cosmos.NewUint(common.One * 100)
	c.Assert(k.SetPool(ctx, pool1), IsNil)

	// Create three vaults
	vault1 := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	vault1.AddFunds(common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(common.One*200)),
	})
	vault1.Membership = []string{
		na1.PubKeySet.Secp256k1.String(),
		na6.PubKeySet.Secp256k1.String(),
	}
	c.Check(k.SetVault(ctx, vault1), IsNil)

	vault2 := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	vault2.AddFunds(common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*1000)),
	})
	vault2.Membership = []string{
		na2.PubKeySet.Secp256k1.String(),
		na5.PubKeySet.Secp256k1.String(),
	}
	c.Check(k.SetVault(ctx, vault2), IsNil)

	vault3 := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	vault3.AddFunds(common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*100)),
	})
	vault3.Membership = []string{
		na3.PubKeySet.Secp256k1.String(),
		na4.PubKeySet.Secp256k1.String(),
	}
	c.Check(k.SetVault(ctx, vault3), IsNil)

	// test that we sort appropriately
	sorted := k.SortBySecurity(ctx, Vaults{vault1, vault2, vault3}, 300)
	c.Assert(sorted, HasLen, 3)
	c.Assert(sorted[0].PubKey.Equals(vault2.PubKey), Equals, true)
	c.Assert(sorted[1].PubKey.Equals(vault3.PubKey), Equals, true)
	c.Assert(sorted[2].PubKey.Equals(vault1.PubKey), Equals, true)
}
