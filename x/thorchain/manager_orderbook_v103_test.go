package thorchain

import (
	"fmt"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type OrderBookV103Suite struct{}

var _ = Suite(&OrderBookV103Suite{})

func (s OrderBookV103Suite) TestGetTodoNum(c *C) {
	book := newOrderBookV103(keeper.KVStoreDummy{})

	c.Check(book.getTodoNum(50, 10, 100), Equals, int64(25))     // halves it
	c.Check(book.getTodoNum(11, 10, 100), Equals, int64(5))      // halves it
	c.Check(book.getTodoNum(10, 10, 100), Equals, int64(10))     // does all of them
	c.Check(book.getTodoNum(1, 10, 100), Equals, int64(1))       // does all of them
	c.Check(book.getTodoNum(0, 10, 100), Equals, int64(0))       // does none
	c.Check(book.getTodoNum(10000, 10, 100), Equals, int64(100)) // does max 100
	c.Check(book.getTodoNum(200, 10, 100), Equals, int64(100))   // does max 100
}

func (s OrderBookV103Suite) TestScoreMsgs(c *C) {
	ctx, k := setupKeeperForTest(c)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceRune = cosmos.NewUint(143166 * common.One)
	pool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)
	pool = NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceRune = cosmos.NewUint(73708333 * common.One)
	pool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	book := newOrderBookV103(k)

	// check that we sort by liquidity ok
	msgs := []*MsgSwap{
		NewMsgSwap(common.Tx{
			ID:    common.TxID("5E1DF027321F1FE37CA19B9ECB11C2B4ABEC0D8322199D335D9CE4C39F85F115"),
			Coins: common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(2*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("53C1A22436B385133BDD9157BB365DB7AAC885910D2FA7C9DC3578A04FFD4ADC"),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(50*common.One))},
		}, common.RuneAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("6A470EB9AFE82981979A5EEEED3296E1E325597794BD5BFB3543A372CAF435E5"),
			Coins: common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(1*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("5EE9A7CCC55A3EBAFA0E542388CA1B909B1E3CE96929ED34427B96B7CCE9F8E8"),
			Coins: common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("0FF2A521FB11FFEA4DFE3B7AD4066FF0A33202E652D846F8397EFC447C97A91B"),
			Coins: common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(10*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),

		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000001"),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(150*common.One))},
		}, common.RuneAsset(), GetRandomTHORAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),

		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000002"),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(151*common.One))},
		}, common.RuneAsset(), GetRandomTHORAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
	}

	swaps := make(orderItems, len(msgs))
	for i, msg := range msgs {
		swaps[i] = orderItem{
			msg:  *msg,
			fee:  cosmos.ZeroUint(),
			slip: cosmos.ZeroUint(),
		}
	}
	swaps, err := book.scoreMsgs(ctx, swaps, 10_000)
	c.Assert(err, IsNil)
	swaps = swaps.Sort(ctx)
	c.Check(swaps, HasLen, 7)
	c.Check(swaps[0].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(151*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[1].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(150*common.One)), Equals, true, Commentf("%d", swaps[1].msg.Tx.Coins[0].Amount.Uint64()))
	// 50 BNB is worth more than 100 RUNE
	c.Check(swaps[2].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(50*common.One)), Equals, true, Commentf("%d", swaps[2].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[3].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", swaps[3].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[4].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(10*common.One)), Equals, true, Commentf("%d", swaps[4].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[5].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(2*common.One)), Equals, true, Commentf("%d", swaps[5].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[6].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(1*common.One)), Equals, true, Commentf("%d", swaps[6].msg.Tx.Coins[0].Amount.Uint64()))

	// check that slip is taken into account
	// Do not use GetRandomTxHash for these TxIDs,
	// else items with the same score will have pseudorandom order and sometimes fail unit tests.
	msgs = []*MsgSwap{
		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000003"),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(2*common.One))},
		}, common.RuneAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000004"),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(50*common.One))},
		}, common.RuneAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000005"),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One))},
		}, common.RuneAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000009"),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One))},
		}, common.RuneAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000007"),
			Coins: common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(10*common.One))},
		}, common.RuneAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000008"),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(2*common.One))},
		}, common.RuneAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000006"),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(50*common.One))},
		}, common.RuneAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000010"),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(1*common.One))},
		}, common.RuneAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000013"),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(100*common.One))},
		}, common.RuneAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000012"),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(10*common.One))},
		}, common.RuneAsset(), GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),

		NewMsgSwap(common.Tx{
			ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000011"),
			Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(10*common.One))},
		}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
			"", "", nil,
			MarketOrder,
			GetRandomBech32Addr()),
	}

	swaps = make(orderItems, len(msgs))
	for i, msg := range msgs {
		swaps[i] = orderItem{
			msg:  *msg,
			fee:  cosmos.ZeroUint(),
			slip: cosmos.ZeroUint(),
		}
	}
	swaps, err = book.scoreMsgs(ctx, swaps, 10_000)
	c.Assert(err, IsNil)
	swaps = swaps.Sort(ctx)
	c.Assert(swaps, HasLen, 11)

	c.Check(swaps[0].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(10*common.One)), Equals, true, Commentf("%d", swaps[0].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[0].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)

	c.Check(swaps[1].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", swaps[1].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[1].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)

	c.Check(swaps[2].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(50*common.One)), Equals, true, Commentf("%d", swaps[2].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[2].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)

	c.Check(swaps[3].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", swaps[3].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[3].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)

	c.Check(swaps[4].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(50*common.One)), Equals, true, Commentf("%d", swaps[4].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[4].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)

	c.Check(swaps[5].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(10*common.One)), Equals, true, Commentf("%d", swaps[5].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[5].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)

	c.Check(swaps[6].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(10*common.One)), Equals, true, Commentf("%d", swaps[6].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[6].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)

	c.Check(swaps[7].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(2*common.One)), Equals, true, Commentf("%d", swaps[7].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[7].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)

	c.Check(swaps[8].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(2*common.One)), Equals, true, Commentf("%d", swaps[8].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[8].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)

	c.Check(swaps[9].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(1*common.One)), Equals, true, Commentf("%d", swaps[9].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[9].msg.Tx.Coins[0].Asset.Equals(common.BTCAsset), Equals, true)

	c.Check(swaps[10].msg.Tx.Coins[0].Amount.Equal(cosmos.NewUint(1*common.One)), Equals, true, Commentf("%d", swaps[10].msg.Tx.Coins[0].Amount.Uint64()))
	c.Check(swaps[10].msg.Tx.Coins[0].Asset.Equals(common.BNBAsset), Equals, true)
}

func (s OrderBookV103Suite) TestFetchQueue(c *C) {
	ctx, mgr := setupManagerForTest(c)
	book := newOrderBookV103(mgr.Keeper())

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(2088519094783)
	pool.BalanceRune = cosmos.NewUint(199019591474591)
	pool.Status = PoolAvailable
	c.Check(mgr.Keeper().SetPool(ctx, pool), IsNil)

	pool = NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceAsset = cosmos.NewUint(97645470445)
	pool.BalanceRune = cosmos.NewUint(798072095218642)
	pool.Status = PoolAvailable
	c.Check(mgr.Keeper().SetPool(ctx, pool), IsNil)

	market := NewMsgSwap(common.Tx{
		ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000014"),
		Coins: common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(2*common.One))},
	}, common.BNBAsset, GetRandomBNBAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(),
		"", "", nil,
		MarketOrder,
		GetRandomBech32Addr())
	limit1 := NewMsgSwap(common.Tx{
		ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000015"),
		Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(1*common.One))},
	}, common.BNBAsset, GetRandomBNBAddress(), cosmos.NewUint(80*common.One), common.NoAddress, cosmos.ZeroUint(),
		"", "", nil,
		LimitOrder,
		GetRandomBech32Addr())

	limit2 := NewMsgSwap(common.Tx{
		ID:    common.TxID("0000000000000000000000000000000000000000000000000000000000000016"),
		Coins: common.Coins{common.NewCoin(common.BTCAsset, cosmos.NewUint(1*common.One))},
	}, common.BNBAsset, GetRandomBNBAddress(), cosmos.NewUint(100*common.One), common.NoAddress, cosmos.ZeroUint(),
		"", "", nil,
		LimitOrder,
		GetRandomBech32Addr())

	c.Assert(mgr.Keeper().SetOrderBookItem(ctx, *market), IsNil)
	c.Assert(mgr.Keeper().SetOrderBookItem(ctx, *limit1), IsNil)
	c.Assert(mgr.Keeper().SetOrderBookItem(ctx, *limit2), IsNil)

	c.Assert(mgr.Keeper().SetOrderBookProcessor(ctx, []bool{true, true, true, true, true, true}), IsNil)

	pairs, pools := book.getAssetPairs(ctx)

	items, err := book.FetchQueue(ctx, mgr, pairs, pools)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 2, Commentf("%d", len(items)))
}

func (s OrderBookV103Suite) TestgetAssetPairs(c *C) {
	ctx, k := setupKeeperForTest(c)

	book := newOrderBookV103(k)

	pool := NewPool()
	pool.Asset = common.BTCAsset
	c.Assert(k.SetPool(ctx, pool), IsNil)
	pool.Asset = common.BNBAsset
	c.Assert(k.SetPool(ctx, pool), IsNil)

	pairs, pools := book.getAssetPairs(ctx)
	c.Check(pools, HasLen, 2)
	c.Check(pairs, HasLen, len(pools)*(len(pools)+1))
}

func (s OrderBookV103Suite) TestTradePairsTodo(c *C) {
	pairs := tradePairs{
		{common.RuneAsset(), common.BNBAsset},
		{common.BNBAsset, common.RuneAsset()},
		{common.RuneAsset(), common.BTCAsset},
		{common.BTCAsset, common.RuneAsset()},
		{common.BNBAsset, common.BTCAsset},
		{common.BTCAsset, common.BNBAsset},
	}

	// RUNE --> BNB
	todo := make(tradePairs, 0)
	todo = todo.findMatchingTrades(genTradePair(common.RuneAsset(), common.BNBAsset), pairs)
	c.Check(todo, HasLen, 2, Commentf("%d", len(todo)))
	c.Check(todo[0].Equals(genTradePair(common.BNBAsset, common.RuneAsset())), Equals, true, Commentf("%s", todo[0]))
	c.Check(todo[1].Equals(genTradePair(common.BNBAsset, common.BTCAsset)), Equals, true, Commentf("%s", todo[1]))

	// ensure we don't duplicate
	todo = todo.findMatchingTrades(genTradePair(common.RuneAsset(), common.BNBAsset), pairs)
	c.Check(todo, HasLen, 2, Commentf("%d", len(todo)))

	// BTC --> RUNE
	todo = make(tradePairs, 0)
	todo = todo.findMatchingTrades(genTradePair(common.BTCAsset, common.RuneAsset()), pairs)
	c.Check(todo, HasLen, 2, Commentf("%d", len(todo)))
	c.Check(todo[0].Equals(genTradePair(common.RuneAsset(), common.BTCAsset)), Equals, true, Commentf("%s", todo[0]))
	c.Check(todo[1].Equals(genTradePair(common.BNBAsset, common.BTCAsset)), Equals, true, Commentf("%s", todo[1]))

	// BTC --> BNB
	todo = make(tradePairs, 0)
	todo = todo.findMatchingTrades(genTradePair(common.BTCAsset, common.BNBAsset), pairs)
	c.Check(todo, HasLen, 3, Commentf("%d", len(todo)))
	c.Check(todo[0].Equals(genTradePair(common.BNBAsset, common.RuneAsset())), Equals, true, Commentf("%s", todo[0]))
	c.Check(todo[1].Equals(genTradePair(common.RuneAsset(), common.BTCAsset)), Equals, true, Commentf("%s", todo[1]))
	c.Check(todo[2].Equals(genTradePair(common.BNBAsset, common.BTCAsset)), Equals, true, Commentf("%s", todo[2]))
}

func (s OrderBookV103Suite) TestConvertProc(c *C) {
	_, k := setupKeeperForTest(c)

	pairs := tradePairs{
		{common.RuneAsset(), common.BNBAsset},
		{common.BNBAsset, common.RuneAsset()},
		{common.RuneAsset(), common.BTCAsset},
		{common.BTCAsset, common.RuneAsset()},
		{common.BNBAsset, common.BTCAsset},
		{common.BTCAsset, common.BNBAsset},
	}

	book := newOrderBookV103(k)

	result, ok := book.convertProcToAssetArrays(nil, pairs)
	c.Assert(result, HasLen, 0)
	c.Assert(ok, Equals, false)
	result, ok = book.convertProcToAssetArrays([]bool{false, false, false, false, false, false}, pairs)
	c.Assert(result, HasLen, 0)
	c.Assert(ok, Equals, true)

	testcases := []tradePairs{
		{},
		{pairs[0]},
		{pairs[1]},
		{pairs[2]},
		{pairs[0], pairs[1]},
		{pairs[0], pairs[2]},
		{pairs[1], pairs[2]},
		{pairs[0], pairs[1], pairs[2]},
	}
	for _, test := range testcases {
		proc := book.convertAssetArraysToProc(test, pairs)
		result, ok = book.convertProcToAssetArrays(proc, pairs)
		c.Assert(result, DeepEquals, test)
		c.Assert(ok, Equals, true)
	}

	proc := book.convertAssetArraysToProc(tradePairs{pairs[0], genTradePair(common.BNBAsset, common.ETHAsset)}, pairs)
	result, ok = book.convertProcToAssetArrays(proc, pairs)
	c.Assert(result, DeepEquals, tradePairs{pairs[0]})
	c.Assert(ok, Equals, true)

	proc = book.convertAssetArraysToProc(tradePairs{pairs[0], pairs[1], pairs[1], pairs[1], pairs[1], pairs[1], pairs[0]}, pairs)
	result, ok = book.convertProcToAssetArrays(proc, pairs)
	c.Assert(result, DeepEquals, tradePairs{pairs[0], pairs[1]})
	c.Assert(ok, Equals, true)

	result, ok = book.convertProcToAssetArrays([]bool{true}, pairs)
	c.Assert(result, DeepEquals, tradePairs{})
	c.Assert(ok, Equals, false)
}

func (s OrderBookV103Suite) TestEndBlock(c *C) {
	ctx, mgr := setupManagerForTest(c)
	mgr.txOutStore = NewTxStoreDummy()
	book := newOrderBookV103(mgr.Keeper())

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(2088519094783)
	pool.BalanceRune = cosmos.NewUint(199019591474591)
	pool.Status = PoolAvailable
	c.Check(mgr.Keeper().SetPool(ctx, pool), IsNil)

	pool = NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceAsset = cosmos.NewUint(97645470445)
	pool.BalanceRune = cosmos.NewUint(798072095218642)
	pool.Status = PoolAvailable
	c.Check(mgr.Keeper().SetPool(ctx, pool), IsNil)

	affilAddr := GetRandomTHORAddress()

	tx := GetRandomTx()
	bnbAddr := GetRandomBNBAddress()
	tx.Memo = fmt.Sprintf("swap:BNB.BNB:%s", bnbAddr)
	tx.Coins = common.NewCoins(common.NewCoin(common.RuneAsset(), cosmos.NewUint(2*common.One)))
	market := NewMsgSwap(
		tx, common.BNBAsset, bnbAddr, cosmos.ZeroUint(),
		affilAddr, cosmos.NewUint(1_000),
		"", "", nil,
		MarketOrder,
		GetRandomBech32Addr())

	tx = GetRandomTx()
	tx.Memo = fmt.Sprintf("swap:BNB.BNB:%s", bnbAddr)
	tx.Coins = common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(1*common.One)))
	limit1 := NewMsgSwap(
		tx, common.BNBAsset, bnbAddr, cosmos.NewUint(856815149),
		affilAddr, cosmos.NewUint(1_000),
		"", "", nil,
		LimitOrder,
		GetRandomBech32Addr())

	c.Assert(mgr.Keeper().SetOrderBookItem(ctx, *market), IsNil)
	c.Assert(mgr.Keeper().SetOrderBookItem(ctx, *limit1), IsNil)

	c.Assert(mgr.Keeper().SetOrderBookProcessor(ctx, []bool{true, true, true, true, true, true}), IsNil)

	err := book.EndBlock(ctx, mgr)
	c.Assert(err, IsNil)

	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 2) // two outbounds are rune, which doesn't show up in the outbound items list

	proc, err := mgr.Keeper().GetOrderBookProcessor(ctx)
	c.Assert(err, IsNil)
	c.Check(proc, DeepEquals, []bool{false, true, true, true, false, false}, Commentf("%+v", proc))
}
