package keeperv1

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
	. "gopkg.in/check.v1"
)

type KeeperOrderBookSuite struct{}

var _ = Suite(&KeeperOrderBookSuite{})

func (s *KeeperOrderBookSuite) TestKeeperOrderBook(c *C) {
	ctx, k := setupKeeperForTest(c)

	// not found
	_, err := k.GetOrderBookItem(ctx, GetRandomTxHash())
	c.Assert(err, NotNil)

	msg1 := MsgSwap{
		Tx:          GetRandomTx(),
		TradeTarget: cosmos.NewUint(10 * common.One),
		OrderType:   types.OrderType_limit,
	}
	msg2 := MsgSwap{
		Tx:          GetRandomTx(),
		TradeTarget: cosmos.NewUint(10 * common.One),
		OrderType:   types.OrderType_limit,
	}

	c.Assert(k.SetOrderBookItem(ctx, msg1), IsNil)
	c.Assert(k.SetOrderBookItem(ctx, msg2), IsNil)
	msg3, err := k.GetOrderBookItem(ctx, msg1.Tx.ID)
	c.Assert(err, IsNil)
	c.Check(msg3.Tx.ID.Equals(msg1.Tx.ID), Equals, true)

	c.Check(k.HasOrderBookItem(ctx, msg1.Tx.ID), Equals, true)
	ok, err := k.HasOrderBookIndex(ctx, msg1)
	c.Assert(err, IsNil)
	c.Check(ok, Equals, true)

	iter := k.GetOrderBookItemIterator(ctx)
	for ; iter.Valid(); iter.Next() {
		var m MsgSwap
		k.Cdc().MustUnmarshal(iter.Value(), &m)
		c.Check(m.Tx.ID.Equals(msg1.Tx.ID) || m.Tx.ID.Equals(msg2.Tx.ID), Equals, true)
	}
	iter.Close()

	iter = k.GetOrderBookIndexIterator(ctx, msg1.OrderType, msg1.Tx.Coins[0].Asset, msg1.TargetAsset)
	for ; iter.Valid(); iter.Next() {
		hashes := make([]string, 0)
		ok, err := k.getStrings(ctx, string(iter.Key()), &hashes)
		c.Assert(err, IsNil)
		c.Check(ok, Equals, true)
		c.Check(hashes, HasLen, 2)
		c.Check(hashes[0], Equals, msg1.Tx.ID.String())
		c.Check(hashes[1], Equals, msg2.Tx.ID.String())
	}
	iter.Close()

	// test remove
	c.Assert(k.RemoveOrderBookItem(ctx, msg1.Tx.ID), IsNil)
	_, err = k.GetOrderBookItem(ctx, msg1.Tx.ID)
	c.Check(err, NotNil)
	c.Check(k.HasOrderBookItem(ctx, msg1.Tx.ID), Equals, false)
	ok, err = k.HasOrderBookIndex(ctx, msg1)
	c.Assert(err, IsNil)
	c.Check(ok, Equals, false)
}

func (s *KeeperOrderBookSuite) TestGetOrderBookIndexKey(c *C) {
	ctx, k := setupKeeperForTest(c)
	msg := MsgSwap{
		OrderType: types.OrderType_limit,
		Tx: common.Tx{
			Coins: common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(10000))),
		},
		TargetAsset: common.RuneAsset(),
		TradeTarget: cosmos.NewUint(1239585),
	}
	c.Check(k.getOrderBookIndexKey(ctx, msg), Equals, "olim//BTC.BTC>THOR.RUNE/000000000000806721/")
}

func (s *KeeperOrderBookSuite) TestRewriteRatio(c *C) {
	c.Check(rewriteRatio(3, "5"), Equals, "005")    // smaller
	c.Check(rewriteRatio(3, "5000"), Equals, "500") // larger
	c.Check(rewriteRatio(3, "500"), Equals, "500")  // just right
}

func (s *KeeperOrderBookSuite) TestRemoveSlice(c *C) {
	c.Check(removeString([]string{"foo", "bar", "baz"}, 0), DeepEquals, []string{"baz", "bar"})
	c.Check(removeString([]string{"foo", "bar", "baz"}, 1), DeepEquals, []string{"foo", "baz"})
	c.Check(removeString([]string{"foo", "bar", "baz"}, 2), DeepEquals, []string{"foo", "bar"})
	c.Check(removeString([]string{"foo", "bar", "baz"}, 3), DeepEquals, []string{"foo", "bar", "baz"})
	c.Check(removeString([]string{"foo", "bar", "baz"}, -1), DeepEquals, []string{"foo", "bar", "baz"})
}
