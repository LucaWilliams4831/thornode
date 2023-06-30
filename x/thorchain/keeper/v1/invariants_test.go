package keeperv1

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type InvariantsSuite struct{}

var _ = Suite(&InvariantsSuite{})

func (s *InvariantsSuite) TestAsgardInvariant(c *C) {
	ctx, k := setupKeeperForTest(c)

	// empty the starting balance of asgard
	runeBal := k.GetRuneBalanceOfModule(ctx, AsgardName)
	coins := common.NewCoins(common.NewCoin(common.RuneAsset(), runeBal))
	c.Assert(k.SendFromModuleToModule(ctx, AsgardName, ReserveName, coins), IsNil)

	pool := NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceRune = cosmos.NewUint(1000)
	pool.PendingInboundRune = cosmos.NewUint(100)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	// derived asset pools are not included in expectations
	pool = NewPool()
	pool.Asset = common.BTCAsset.GetDerivedAsset()
	pool.BalanceRune = cosmos.NewUint(666)
	pool.PendingInboundRune = cosmos.NewUint(777)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	// savers pools are not included in expectations
	pool = NewPool()
	pool.Asset = common.BTCAsset.GetSyntheticAsset()
	pool.BalanceRune = cosmos.NewUint(666)
	pool.PendingInboundRune = cosmos.NewUint(777)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	swapMsg := MsgSwap{
		Tx: GetRandomTx(),
	}
	swapMsg.Tx.Coins = common.NewCoins(common.NewCoin(common.RuneAsset(), cosmos.NewUint(2000)))
	c.Assert(k.SetSwapQueueItem(ctx, swapMsg, 0), IsNil)

	// synth swaps are ignored
	swapMsg.Tx.Coins = common.NewCoins(common.NewCoin(common.BTCAsset.GetSyntheticAsset(), cosmos.NewUint(666)))
	c.Assert(k.SetSwapQueueItem(ctx, swapMsg, 1), IsNil)

	// layer1 swaps are ignored
	swapMsg.Tx.Coins = common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(777)))
	c.Assert(k.SetSwapQueueItem(ctx, swapMsg, 2), IsNil)

	invariant := AsgardInvariant(k)

	msg, broken := invariant(ctx)
	c.Assert(broken, Equals, true)
	c.Assert(msg, Equals, "insolvent: 666btc/btc\ninsolvent: 3100rune\n")

	// send the expected amount to asgard
	expCoins := common.NewCoins(
		common.NewCoin(common.BTCAsset.GetSyntheticAsset(), cosmos.NewUint(666)),
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(3100)),
	)
	for _, coin := range expCoins {
		c.Assert(k.MintToModule(ctx, ModuleName, coin), IsNil)
	}
	c.Assert(k.SendFromModuleToModule(ctx, ModuleName, AsgardName, expCoins), IsNil)

	msg, broken = invariant(ctx)
	c.Assert(broken, Equals, false)
	c.Assert(msg, Equals, "")

	// send a little more to make asgard oversolvent
	extraCoins := common.NewCoins(common.NewCoin(common.RuneAsset(), cosmos.NewUint(1)))
	c.Assert(k.SendFromModuleToModule(ctx, ReserveName, AsgardName, extraCoins), IsNil)

	msg, broken = invariant(ctx)
	c.Assert(broken, Equals, true)
	c.Assert(msg, Equals, "oversolvent: 1rune\n")
}

func (s *InvariantsSuite) TestBondInvariant(c *C) {
	ctx, k := setupKeeperForTest(c)

	node := GetRandomValidatorNode(NodeActive)
	node.Bond = cosmos.NewUint(1000)
	c.Assert(k.SetNodeAccount(ctx, node), IsNil)

	node = GetRandomValidatorNode(NodeActive)
	node.Bond = cosmos.NewUint(100)
	c.Assert(k.SetNodeAccount(ctx, node), IsNil)

	network := NewNetwork()
	network.BondRewardRune = cosmos.NewUint(2000)
	c.Assert(k.SetNetwork(ctx, network), IsNil)

	invariant := BondInvariant(k)

	msg, broken := invariant(ctx)
	c.Assert(broken, Equals, true)
	c.Assert(msg, Equals, "insolvent: 3100rune")

	expRune := common.NewCoin(common.RuneAsset(), cosmos.NewUint(3100))
	c.Assert(k.MintToModule(ctx, ModuleName, expRune), IsNil)
	c.Assert(k.SendFromModuleToModule(ctx, ModuleName, BondName, common.NewCoins(expRune)), IsNil)

	msg, broken = invariant(ctx)
	c.Assert(broken, Equals, false)
	c.Assert(msg, Equals, "")

	// send more to make bond oversolvent
	c.Assert(k.MintToModule(ctx, ModuleName, expRune), IsNil)
	c.Assert(k.SendFromModuleToModule(ctx, ModuleName, BondName, common.NewCoins(expRune)), IsNil)

	msg, broken = invariant(ctx)
	c.Assert(broken, Equals, true)
	c.Assert(msg, Equals, "oversolvent: 3100rune")
}

func (s *InvariantsSuite) TestTHORChainInvariant(c *C) {
	ctx, k := setupKeeperForTest(c)

	invariant := THORChainInvariant(k)

	// should pass since it has no coins
	msg, broken := invariant(ctx)
	c.Assert(broken, Equals, false)
	c.Assert(msg, Equals, "")

	// send some coins to make it oversolvent
	coins := common.NewCoins(common.NewCoin(common.RuneAsset(), cosmos.NewUint(1)))
	c.Assert(k.SendFromModuleToModule(ctx, AsgardName, ModuleName, coins), IsNil)

	msg, broken = invariant(ctx)
	c.Assert(broken, Equals, true)
	c.Assert(msg, Equals, "oversolvent: 1rune\n")
}
