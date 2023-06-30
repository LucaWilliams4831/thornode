package thorchain

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"

	"github.com/jinzhu/copier"
)

type orderItem struct {
	index int
	msg   MsgSwap
	fee   cosmos.Uint
	slip  cosmos.Uint
}

type orderItems []orderItem

func (items orderItems) HasItem(hash common.TxID) bool {
	for _, item := range items {
		if item.msg.Tx.ID.Equals(hash) {
			return true
		}
	}
	return false
}

type tradePair struct {
	source common.Asset
	target common.Asset
}

type tradePairs []tradePair

func genTradePair(s, t common.Asset) tradePair {
	return tradePair{
		source: s,
		target: t,
	}
}

func (pair tradePair) String() string {
	return fmt.Sprintf("%s>%s", pair.source, pair.target)
}

func (pair tradePair) HasRune() bool {
	return pair.source.IsNativeRune() || pair.target.IsNativeRune()
}

func (pair tradePair) Equals(p tradePair) bool {
	return pair.source.Equals(p.source) && pair.target.Equals(p.target)
}

// given a trade pair, find the trading pairs that are the reverse of this
// trade pair. This helps us build a list of trading pairs/order books to check
// for limit orders later
func (p tradePairs) findMatchingTrades(trade tradePair, pairs tradePairs) tradePairs {
	var comp func(pair tradePair) bool
	switch {
	case trade.source.IsNativeRune():
		comp = func(pair tradePair) bool { return pair.source.Equals(trade.target) }
	case trade.target.IsNativeRune():
		comp = func(pair tradePair) bool { return pair.target.Equals(trade.source) }
	default:
		comp = func(pair tradePair) bool { return pair.source.Equals(trade.target) || pair.target.Equals(trade.source) }
	}
	for _, pair := range pairs {
		if comp(pair) {
			// check for duplicates
			exists := false
			for _, p2 := range p {
				if p2.Equals(pair) {
					exists = true
					break
				}
			}
			if !exists {
				p = append(p, pair)
			}
		}
	}
	return p
}

func (items orderItems) Sort(ctx cosmos.Context) orderItems {
	// sort by liquidity fee , descending

	byFee := make(orderItems, len(items))
	if err := copier.Copy(&byFee, &items); err != nil {
		ctx.Logger().Error("fail copy items by fee", "error", err)
	}
	sort.SliceStable(byFee, func(i, j int) bool {
		return byFee[i].fee.GT(byFee[j].fee)
	})

	// sort by slip fee , descending
	bySlip := make(orderItems, len(items))
	if err := copier.Copy(&bySlip, &items); err != nil {
		ctx.Logger().Error("fail copy items by slip", "error", err)
	}
	sort.SliceStable(bySlip, func(i, j int) bool {
		return bySlip[i].slip.GT(bySlip[j].slip)
	})

	type score struct {
		msg   MsgSwap
		score int
		index int
	}

	// add liquidity fee score
	scores := make([]score, len(items))
	for i, item := range byFee {
		scores[i] = score{
			msg:   item.msg,
			score: i,
			index: item.index,
		}
	}

	// add slip score
	for i, item := range bySlip {
		for j, score := range scores {
			if score.msg.Tx.ID.Equals(item.msg.Tx.ID) && score.index == item.index {
				scores[j].score += i
				break
			}
		}
	}

	// sorting by two attribute to ensure there is no abiguity here, since
	// multiple items can have the same score (but not the same tx hash)
	sort.SliceStable(scores, func(i, j int) bool {
		switch {
		case scores[i].score < scores[j].score:
			return true
		case scores[i].score == scores[j].score:
			return scores[i].msg.Tx.ID.String() < scores[j].msg.Tx.ID.String()
		default:
			return false
		}
	})

	// sort our items by score
	sorted := make(orderItems, len(items))
	for i, score := range scores {
		for _, item := range items {
			if item.msg.Tx.ID.Equals(score.msg.Tx.ID) && score.index == item.index {
				sorted[i] = item
				break
			}
		}
	}

	return sorted
}

// OrderBookV104 is going to manage the swaps queue
type OrderBookV104 struct {
	k           keeper.Keeper
	limitOrders orderItems
}

// newOrderBookV104 create a new vault manager
func newOrderBookV104(k keeper.Keeper) *OrderBookV104 {
	return &OrderBookV104{k: k, limitOrders: make(orderItems, 0)}
}

// FetchQueue - grabs all swap queue items from the kvstore and returns them
func (ob *OrderBookV104) FetchQueue(ctx cosmos.Context, mgr Manager, pairs tradePairs, pools Pools) (orderItems, error) { // nolint
	items := make(orderItems, 0)

	// if the network is doing a pool cycle, no swaps/orders are executed this
	// block. This is because the change of active pools can cause the
	// mechanism to index/encode the selected pools/trading pairs that need to
	// be checked (proc).
	poolCycle := mgr.Keeper().GetConfigInt64(ctx, constants.PoolCycle)
	if ctx.BlockHeight()%poolCycle == 0 {
		return nil, nil
	}

	proc, err := ob.k.GetOrderBookProcessor(ctx)
	if err != nil {
		return nil, err
	}

	todo, ok := ob.convertProcToAssetArrays(proc, pairs)
	if !ok {
		// number of pools has changed from the previous block. Skip processing
		// swaps/orders for this block. This is due to our total pair list (aka
		// reference table) changing underneath our feet.
		return nil, nil
	}

	// get market orders
	hashes, err := ob.k.GetOrderBookIndex(ctx, MsgSwap{OrderType: MarketOrder})
	if err != nil {
		return nil, err
	}
	for _, hash := range hashes {
		msg, err := ob.k.GetOrderBookItem(ctx, hash)
		if err != nil {
			ctx.Logger().Error("fail to fetch order book item", "error", err)
			continue
		}

		items = append(items, orderItem{
			msg:   msg,
			index: 0,
			fee:   cosmos.ZeroUint(),
			slip:  cosmos.ZeroUint(),
		})
	}

	for _, pair := range todo {
		newItems, done := ob.discoverLimitOrders(ctx, pair, pools)
		items = append(items, newItems...)
		if done {
			break
		}
	}

	return items, nil
}

func (ob *OrderBookV104) discoverLimitOrders(ctx cosmos.Context, pair tradePair, pools Pools) (orderItems, bool) {
	items := make(orderItems, 0)
	done := false

	iter := ob.k.GetOrderBookIndexIterator(ctx, LimitOrder, pair.source, pair.target)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		ratio, err := ob.parseRatioFromKey(string(iter.Key()))
		if err != nil {
			ctx.Logger().Error("fail to parse ratio", "key", string(iter.Key()), "error", err)
			continue
		}

		// if a fee-less swap doesn't meet the ratio requirement, then we
		// can be assured that all order book items in this index and every
		// index there after will not be met.
		if ok := ob.checkFeelessSwap(pools, pair, ratio); !ok {
			done = true
			break
		}

		record := make([]string, 0)
		value := ProtoStrings{Value: record}
		if err := ob.k.Cdc().Unmarshal(iter.Value(), &value); err != nil {
			ctx.Logger().Error("fail to fetch indexed txn hashes", "error", err)
			continue
		}

		for i, rec := range value.Value {
			hash, err := common.NewTxID(rec)
			if err != nil {
				ctx.Logger().Error("fail to parse tx hash", "error", err)
				continue
			}
			msg, err := ob.k.GetOrderBookItem(ctx, hash)
			if err != nil {
				ctx.Logger().Error("fail to fetch msg swap", "error", err)
				continue
			}

			// do a swap, including swap fees and outbound fees. If this passes attempt the swap.
			if ok := ob.checkWithFeeSwap(ctx, pools, msg); !ok {
				continue
			}

			items = append(items, orderItem{
				msg:   msg,
				index: i,
				fee:   cosmos.ZeroUint(),
				slip:  cosmos.ZeroUint(),
			})
		}
	}
	return items, done
}

func (ob *OrderBookV104) checkFeelessSwap(pools Pools, pair tradePair, indexRatio uint64) bool {
	var ratio cosmos.Uint
	switch {
	case !pair.HasRune():
		sourcePool, ok := pools.Get(pair.source.GetLayer1Asset())
		if !ok {
			return false
		}
		targetPool, ok := pools.Get(pair.target.GetLayer1Asset())
		if !ok {
			return false
		}
		one := cosmos.NewUint(common.One)
		runeAmt := common.GetSafeShare(one, sourcePool.BalanceAsset, sourcePool.BalanceRune)
		emit := common.GetSafeShare(runeAmt, targetPool.BalanceRune, targetPool.BalanceAsset)
		ratio = ob.getRatio(one, emit)
	case pair.source.IsNativeRune():
		pool, ok := pools.Get(pair.target.GetLayer1Asset())
		if !ok {
			return false
		}
		ratio = ob.getRatio(pool.BalanceRune, pool.BalanceAsset)
	case pair.target.IsNativeRune():
		pool, ok := pools.Get(pair.source.GetLayer1Asset())
		if !ok {
			return false
		}
		ratio = ob.getRatio(pool.BalanceAsset, pool.BalanceRune)
	}
	return cosmos.NewUint(indexRatio).GT(ratio)
}

func (ob *OrderBookV104) checkWithFeeSwap(ctx cosmos.Context, pools Pools, msg MsgSwap) bool {
	swapper, err := GetSwapper(ob.k.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to load swapper", "error", err)
		swapper = newSwapperV92()
	}

	// account for affiliate fee
	source := msg.Tx.Coins[0]
	if !msg.AffiliateBasisPoints.IsZero() {
		maxBasisPoints := cosmos.NewUint(10_000)
		source.Amount = common.GetSafeShare(common.SafeSub(maxBasisPoints, msg.AffiliateBasisPoints), maxBasisPoints, source.Amount)
	}

	target := common.NewCoin(msg.TargetAsset, msg.TradeTarget)
	var emit cosmos.Uint
	switch {
	case !source.Asset.IsNativeRune() && !target.Asset.IsNativeRune():
		sourcePool, ok := pools.Get(source.Asset.GetLayer1Asset())
		if !ok {
			return false
		}
		targetPool, ok := pools.Get(target.Asset.GetLayer1Asset())
		if !ok {
			return false
		}
		emit = swapper.CalcAssetEmission(sourcePool.BalanceAsset, source.Amount, sourcePool.BalanceRune)
		emit = swapper.CalcAssetEmission(targetPool.BalanceRune, emit, targetPool.BalanceAsset)
	case source.Asset.IsNativeRune():
		pool, ok := pools.Get(target.Asset.GetLayer1Asset())
		if !ok {
			return false
		}
		emit = swapper.CalcAssetEmission(pool.BalanceRune, source.Amount, pool.BalanceAsset)
	case target.Asset.IsNativeRune():
		pool, ok := pools.Get(source.Asset.GetLayer1Asset())
		if !ok {
			return false
		}
		emit = swapper.CalcAssetEmission(pool.BalanceAsset, source.Amount, pool.BalanceRune)
	}

	// txout manager has fees as well, that might fail the swap. That is NOT
	// accounted for here, because its prob more work computationally than its
	// worth to check (?).

	return emit.GT(target.Amount)
}

func (ob *OrderBookV104) getRatio(input, output cosmos.Uint) cosmos.Uint {
	if output.IsZero() {
		return cosmos.ZeroUint()
	}
	return input.MulUint64(1e8).Quo(output)
}

// converts a proc, cosmos.Uint, into a series of selected pairs from the pairs
// input (ie asset pairs that need to be check for executable order)
func (ob *OrderBookV104) convertProcToAssetArrays(proc []bool, pairs tradePairs) (tradePairs, bool) {
	result := make(tradePairs, 0)
	if len(proc) != len(pairs) {
		return result, false
	}
	for i, b := range proc {
		if len(pairs)-1 < i {
			break // pairs length < bin length
		}
		if b {
			result = append(result, pairs[i])
		}
	}
	return result, true
}

// converts a list of selected pairs from a list of total pairs, to be represented as a uint64
func (ob *OrderBookV104) convertAssetArraysToProc(toProc, pairs tradePairs) []bool {
	builder := make([]bool, len(pairs))
	for i, pair := range pairs {
		builder[i] = false
		for _, p := range toProc {
			if pair.Equals(p) {
				builder[i] = true
				break
			}
		}
	}
	return builder
}

// getAssetPairs - fetches a list of strings that represents directional trading pairs
func (ob *OrderBookV104) getAssetPairs(ctx cosmos.Context) (tradePairs, Pools) {
	result := make(tradePairs, 0)
	var pools Pools

	assets := []common.Asset{common.RuneAsset()}
	iterator := ob.k.GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		err := ob.k.Cdc().Unmarshal(iterator.Value(), &pool)
		if err != nil {
			ctx.Logger().Error("fail to unmarshal pool", "error", err)
			continue
		}
		if pool.Status != PoolAvailable {
			continue
		}
		if pool.Asset.IsSyntheticAsset() {
			continue
		}
		assets = append(assets, pool.Asset)
		pools = append(pools, pool)
	}

	for _, a1 := range assets {
		for _, a2 := range assets {
			if a1.Equals(a2) {
				continue
			}
			result = append(result, genTradePair(a1, a2))
		}
	}

	return result, pools
}

func (ob *OrderBookV104) AddOrderBookItem(ctx cosmos.Context, msg MsgSwap) error {
	if err := ob.k.SetOrderBookItem(ctx, msg); err != nil {
		ctx.Logger().Error("fail to add order book item", "error", err)
		return err
	}
	if msg.OrderType == LimitOrder {
		ob.limitOrders = append(ob.limitOrders, orderItem{
			msg:   msg,
			index: 0,
			fee:   cosmos.ZeroUint(),
			slip:  cosmos.ZeroUint(),
		})
	}
	return nil
}

// EndBlock trigger the real swap to be processed
func (ob *OrderBookV104) EndBlock(ctx cosmos.Context, mgr Manager) error {
	handler := NewInternalHandler(mgr)

	minSwapsPerBlock, err := ob.k.GetMimir(ctx, constants.MinSwapsPerBlock.String())
	if minSwapsPerBlock < 0 || err != nil {
		minSwapsPerBlock = mgr.GetConstants().GetInt64Value(constants.MinSwapsPerBlock)
	}
	maxSwapsPerBlock, err := ob.k.GetMimir(ctx, constants.MaxSwapsPerBlock.String())
	if maxSwapsPerBlock < 0 || err != nil {
		maxSwapsPerBlock = mgr.GetConstants().GetInt64Value(constants.MaxSwapsPerBlock)
	}
	synthVirtualDepthMult, err := ob.k.GetMimir(ctx, constants.VirtualMultSynthsBasisPoints.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = mgr.GetConstants().GetInt64Value(constants.VirtualMultSynthsBasisPoints)
	}

	todo := make(tradePairs, 0)
	pairs, pools := ob.getAssetPairs(ctx)

	swaps, err := ob.FetchQueue(ctx, mgr, pairs, pools)
	if err != nil {
		ctx.Logger().Error("fail to fetch swap queue from store", "error", err)
		return err
	}

	// pull new limit orders added this block (if not already added)
	for _, item := range ob.limitOrders {
		if !swaps.HasItem(item.msg.Tx.ID) {
			swaps = append(swaps, item)
		}
	}
	ob.limitOrders = make(orderItems, 0)

	swaps, err = ob.scoreMsgs(ctx, swaps, synthVirtualDepthMult)
	if err != nil {
		ctx.Logger().Error("fail to fetch swap items", "error", err)
		// continue, don't exit, just do them out of order (instead of not at all)
	}
	swaps = swaps.Sort(ctx)

	refund := func(msg MsgSwap, err error) {
		ctx.Logger().Error("fail to execute order", "msg", msg.Tx.String(), "error", err)

		var refundErr error

		// Get the full ObservedTx from the TxID, for the vault ObservedPubKey to first try to refund from.
		voter, voterErr := mgr.Keeper().GetObservedTxInVoter(ctx, msg.Tx.ID)
		if voterErr == nil && !voter.Tx.IsEmpty() {
			refundErr = refundTx(ctx, ObservedTx{Tx: msg.Tx, ObservedPubKey: voter.Tx.ObservedPubKey}, mgr, CodeSwapFail, err.Error(), "")
		} else {
			// If the full ObservedTx could not be retrieved, proceed with just the MsgSwap's Tx (no ObservedPubKey).
			ctx.Logger().Error("fail to get non-empty observed tx", "error", voterErr)
			refundErr = refundTx(ctx, ObservedTx{Tx: msg.Tx}, mgr, CodeSwapFail, err.Error(), "")
		}

		if nil != refundErr {
			ctx.Logger().Error("fail to refund swap", "error", err)
		}
	}

	for i := int64(0); i < ob.getTodoNum(int64(len(swaps)), minSwapsPerBlock, maxSwapsPerBlock); i++ {
		pick := swaps[i]
		var msg, affiliateSwap MsgSwap
		if err := copier.Copy(&msg, &pick.msg); err != nil {
			ctx.Logger().Error("fail copy msg", "msg", msg.Tx.String(), "error", err)
			continue
		}
		if !msg.AffiliateBasisPoints.IsZero() && msg.AffiliateAddress.IsChain(common.THORChain) {
			affiliateAmt := common.GetSafeShare(
				msg.AffiliateBasisPoints,
				cosmos.NewUint(10000),
				msg.Tx.Coins[0].Amount,
			)
			msg.Tx.Coins[0].Amount = common.SafeSub(msg.Tx.Coins[0].Amount, affiliateAmt)

			affiliateSwap = *NewMsgSwap(
				msg.Tx,
				common.RuneAsset(),
				msg.AffiliateAddress,
				cosmos.ZeroUint(),
				common.NoAddress,
				cosmos.ZeroUint(),
				"",
				"", nil,
				MarketOrder,
				msg.Signer,
			)
			if affiliateSwap.Tx.Coins[0].Amount.GTE(affiliateAmt) {
				affiliateSwap.Tx.Coins[0].Amount = affiliateAmt
			}
		}

		// make the primary swap
		_, err := handler(ctx, &msg)
		if err != nil {
			switch pick.msg.OrderType {
			case MarketOrder:
				refund(pick.msg, err)
			case LimitOrder:
				// if swap fails due to not enough outbound amounts, don't
				// remove the order book item and try again later
				if strings.Contains(err.Error(), "less than price limit") || strings.Contains(err.Error(), "outbound amount does not meet requirements") {
					continue
				}
				refund(pick.msg, err)
			default:
				// non-supported order book item, refund
				refund(pick.msg, err)
			}
		} else {
			todo = todo.findMatchingTrades(genTradePair(msg.Tx.Coins[0].Asset, msg.TargetAsset), pairs)
			if !affiliateSwap.Tx.IsEmpty() {
				// if asset sent in is native rune, no need
				if affiliateSwap.Tx.Coins[0].Asset.IsNativeRune() {
					toAddress, err := msg.AffiliateAddress.AccAddress()
					if err != nil {
						ctx.Logger().Error("fail to convert address into AccAddress", "msg", msg.AffiliateAddress, "error", err)
						continue
					}
					// since native transaction fee has been charged to inbound from address, thus for affiliated fee , the network doesn't need to charge it again
					coin := common.NewCoin(common.RuneAsset(), affiliateSwap.Tx.Coins[0].Amount)
					sdkErr := mgr.Keeper().SendFromModuleToAccount(ctx, AsgardName, toAddress, common.NewCoins(coin))
					if sdkErr != nil {
						ctx.Logger().Error("fail to send native asset to affiliate", "msg", msg.AffiliateAddress, "error", err, "asset", coin.Asset)
					}
				} else {
					// make the affiliate fee swap
					_, err := handler(ctx, &affiliateSwap)
					if err != nil {
						ctx.Logger().Error("fail to execute affiliate swap", "msg", affiliateSwap.Tx.String(), "error", err)
					}
				}
			}
		}
		if err := ob.k.RemoveOrderBookItem(ctx, pick.msg.Tx.ID); err != nil {
			ctx.Logger().Error("fail to remove order book item", "msg", pick.msg.Tx.String(), "error", err)
		}
	}

	if err := ob.k.SetOrderBookProcessor(ctx, ob.convertAssetArraysToProc(todo, pairs)); err != nil {
		ctx.Logger().Error("fail to set book processor", "error", err)
	}

	return nil
}

// getTodoNum - determine how many swaps to do.
func (ob *OrderBookV104) getTodoNum(queueLen, minSwapsPerBlock, maxSwapsPerBlock int64) int64 {
	// Do half the length of the queue. Unless...
	//	1. The queue length is greater than maxSwapsPerBlock
	//  2. The queue legnth is less than minSwapsPerBlock
	todo := queueLen / 2
	if minSwapsPerBlock >= queueLen {
		todo = queueLen
	}
	if maxSwapsPerBlock < todo {
		todo = maxSwapsPerBlock
	}
	return todo
}

// scoreMsgs - this takes a list of MsgSwap, and converts them to a scored
// orderItem list
func (ob *OrderBookV104) scoreMsgs(ctx cosmos.Context, items orderItems, synthVirtualDepthMult int64) (orderItems, error) {
	pools := make(map[common.Asset]Pool)

	for i, item := range items {
		// the asset customer send
		sourceAsset := item.msg.Tx.Coins[0].Asset
		// the asset customer want
		targetAsset := item.msg.TargetAsset

		for _, a := range []common.Asset{sourceAsset, targetAsset} {
			if a.IsRune() {
				continue
			}

			if _, ok := pools[a]; !ok {
				var err error
				pools[a], err = ob.k.GetPool(ctx, a)
				if err != nil {
					ctx.Logger().Error("fail to get pool", "pool", a, "error", err)
					continue
				}
			}
		}

		poolAsset := sourceAsset
		if poolAsset.IsRune() {
			poolAsset = targetAsset
		}
		pool := pools[poolAsset]
		if pool.IsEmpty() || !pool.IsAvailable() || pool.BalanceRune.IsZero() || pool.BalanceAsset.IsZero() {
			continue
		}
		virtualDepthMult := int64(10_000)
		if poolAsset.IsSyntheticAsset() {
			virtualDepthMult = synthVirtualDepthMult
		}
		ob.getLiquidityFeeAndSlip(ctx, pool, item.msg.Tx.Coins[0], &items[i], virtualDepthMult)

		if sourceAsset.IsRune() || targetAsset.IsRune() {
			// single swap , stop here
			continue
		}
		// double swap , thus need to convert source coin to RUNE and calculate fee and slip again
		runeCoin := common.NewCoin(common.RuneAsset(), pool.AssetValueInRune(item.msg.Tx.Coins[0].Amount))
		poolAsset = targetAsset
		pool = pools[poolAsset]
		if pool.IsEmpty() || !pool.IsAvailable() || pool.BalanceRune.IsZero() || pool.BalanceAsset.IsZero() {
			continue
		}
		virtualDepthMult = int64(10_000)
		if targetAsset.IsSyntheticAsset() {
			virtualDepthMult = synthVirtualDepthMult
		}
		ob.getLiquidityFeeAndSlip(ctx, pool, runeCoin, &items[i], virtualDepthMult)
	}

	return items, nil
}

// getLiquidityFeeAndSlip calculate liquidity fee and slip, fee is in RUNE
func (ob *OrderBookV104) getLiquidityFeeAndSlip(ctx cosmos.Context, pool Pool, sourceCoin common.Coin, item *orderItem, virtualDepthMult int64) {
	// Get our X, x, Y values
	var X, x, Y cosmos.Uint
	x = sourceCoin.Amount
	if sourceCoin.Asset.IsRune() {
		X = pool.BalanceRune
		Y = pool.BalanceAsset
	} else {
		Y = pool.BalanceRune
		X = pool.BalanceAsset
	}

	X = common.GetUncappedShare(cosmos.NewUint(uint64(virtualDepthMult)), cosmos.NewUint(10_000), X)
	Y = common.GetUncappedShare(cosmos.NewUint(uint64(virtualDepthMult)), cosmos.NewUint(10_000), Y)

	swapper, err := GetSwapper(ob.k.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to fetch swapper", "error", err)
		swapper = newSwapperV92()
	}
	fee := swapper.CalcLiquidityFee(X, x, Y)
	if sourceCoin.Asset.IsRune() {
		fee = pool.AssetValueInRune(fee)
	}
	slip := swapper.CalcSwapSlip(X, x)
	item.fee = item.fee.Add(fee)
	item.slip = item.slip.Add(slip)
}

func (ob *OrderBookV104) parseRatioFromKey(key string) (uint64, error) {
	parts := strings.Split(key, "/")
	if len(parts) < 5 {
		return 0, fmt.Errorf("invalid key format")
	}
	return strconv.ParseUint(parts[len(parts)-2], 10, 64)
}
