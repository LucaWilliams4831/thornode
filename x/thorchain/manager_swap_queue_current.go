package thorchain

import (
	"sort"
	"strconv"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type swapItem struct {
	index int
	msg   MsgSwap
	fee   cosmos.Uint
	slip  cosmos.Uint
}
type swapItems []swapItem

func (items swapItems) Sort() swapItems {
	// sort by liquidity fee , descending
	byFee := items
	sort.SliceStable(byFee, func(i, j int) bool {
		return byFee[i].fee.GT(byFee[j].fee)
	})

	// sort by slip fee , descending
	bySlip := items
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

	// This sorted appears to sort twice, but actually the first sort informs
	// the second. If we have multiple swaps with the same score, it will use
	// the ID sort to deterministically sort within the same score

	// sort by ID, first
	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].msg.Tx.ID.String() < scores[j].msg.Tx.ID.String()
	})

	// sort by score, second
	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].score < scores[j].score
	})

	// sort our items by score
	sorted := make(swapItems, len(items))
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

// SwapQueueV104 is going to manage the swaps queue
type SwapQueueV104 struct {
	k keeper.Keeper
}

// newSwapQueueV104 create a new vault manager
func newSwapQueueV104(k keeper.Keeper) *SwapQueueV104 {
	return &SwapQueueV104{k: k}
}

// FetchQueue - grabs all swap queue items from the kvstore and returns them
func (vm *SwapQueueV104) FetchQueue(ctx cosmos.Context) (swapItems, error) { // nolint
	items := make(swapItems, 0)
	iterator := vm.k.GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := vm.k.Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			ctx.Logger().Error("fail to fetch swap msg from queue", "error", err)
			continue
		}

		ss := strings.Split(string(iterator.Key()), "-")
		i, err := strconv.Atoi(ss[len(ss)-1])
		if err != nil {
			ctx.Logger().Error("fail to parse swap queue msg index", "key", iterator.Key(), "error", err)
			continue
		}

		items = append(items, swapItem{
			msg:   msg,
			index: i,
			fee:   cosmos.ZeroUint(),
			slip:  cosmos.ZeroUint(),
		})
	}

	return items, nil
}

// EndBlock trigger the real swap to be processed
func (vm *SwapQueueV104) EndBlock(ctx cosmos.Context, mgr Manager) error {
	handler := NewInternalHandler(mgr)

	minSwapsPerBlock, err := vm.k.GetMimir(ctx, constants.MinSwapsPerBlock.String())
	if minSwapsPerBlock < 0 || err != nil {
		minSwapsPerBlock = mgr.GetConstants().GetInt64Value(constants.MinSwapsPerBlock)
	}
	maxSwapsPerBlock, err := vm.k.GetMimir(ctx, constants.MaxSwapsPerBlock.String())
	if maxSwapsPerBlock < 0 || err != nil {
		maxSwapsPerBlock = mgr.GetConstants().GetInt64Value(constants.MaxSwapsPerBlock)
	}
	synthVirtualDepthMult, err := vm.k.GetMimir(ctx, constants.VirtualMultSynthsBasisPoints.String())
	if synthVirtualDepthMult < 1 || err != nil {
		synthVirtualDepthMult = mgr.GetConstants().GetInt64Value(constants.VirtualMultSynthsBasisPoints)
	}

	swaps, err := vm.FetchQueue(ctx)
	if err != nil {
		ctx.Logger().Error("fail to fetch swap queue from store", "error", err)
		return err
	}
	swaps, err = vm.scoreMsgs(ctx, swaps, synthVirtualDepthMult)
	if err != nil {
		ctx.Logger().Error("fail to fetch swap items", "error", err)
		// continue, don't exit, just do them out of order (instead of not at all)
	}
	swaps = swaps.Sort()

	for i := int64(0); i < vm.getTodoNum(int64(len(swaps)), minSwapsPerBlock, maxSwapsPerBlock); i++ {
		pick := swaps[i]
		_, err := handler(ctx, &pick.msg)
		if err != nil {
			ctx.Logger().Error("fail to swap", "msg", pick.msg.Tx.String(), "error", err)

			var refundErr error

			// Get the full ObservedTx from the TxID, for the vault ObservedPubKey to first try to refund from.
			voter, voterErr := mgr.Keeper().GetObservedTxInVoter(ctx, pick.msg.Tx.ID)
			if voterErr == nil && !voter.Tx.IsEmpty() {
				refundErr = refundTx(ctx, ObservedTx{Tx: pick.msg.Tx, ObservedPubKey: voter.Tx.ObservedPubKey}, mgr, CodeSwapFail, err.Error(), "")
			} else {
				// If the full ObservedTx could not be retrieved, proceed with just the MsgSwap's Tx (no ObservedPubKey).
				ctx.Logger().Error("fail to get non-empty observed tx", "error", voterErr)
				refundErr = refundTx(ctx, ObservedTx{Tx: pick.msg.Tx}, mgr, CodeSwapFail, err.Error(), "")
			}

			if nil != refundErr {
				ctx.Logger().Error("fail to refund swap", "error", err)
			}
		}
		vm.k.RemoveSwapQueueItem(ctx, pick.msg.Tx.ID, pick.index)
	}
	return nil
}

// getTodoNum - determine how many swaps to do.
func (vm *SwapQueueV104) getTodoNum(queueLen, minSwapsPerBlock, maxSwapsPerBlock int64) int64 {
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
// swapItem list
func (vm *SwapQueueV104) scoreMsgs(ctx cosmos.Context, items swapItems, synthVirtualDepthMult int64) (swapItems, error) {
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
				pools[a], err = vm.k.GetPool(ctx, a.GetLayer1Asset())
				if err != nil {
					ctx.Logger().Error("fail to get pool", "pool", a, "error", err)
					continue
				}
			}
		}

		nonRuneAsset := sourceAsset
		if nonRuneAsset.IsRune() {
			nonRuneAsset = targetAsset
		}
		pool := pools[nonRuneAsset]
		if pool.IsEmpty() || pool.BalanceRune.IsZero() || pool.BalanceAsset.IsZero() {
			continue
		}
		// synths may be redeemed on unavailable pools, score them
		if !pool.IsAvailable() && !sourceAsset.IsSyntheticAsset() {
			continue
		}
		virtualDepthMult := int64(10_000)
		if nonRuneAsset.IsSyntheticAsset() {
			virtualDepthMult = synthVirtualDepthMult
		}
		vm.getLiquidityFeeAndSlip(ctx, pool, item.msg.Tx.Coins[0], &items[i], virtualDepthMult)

		if sourceAsset.IsRune() || targetAsset.IsRune() {
			// single swap , stop here
			continue
		}
		// double swap , thus need to convert source coin to RUNE and calculate fee and slip again
		runeCoin := common.NewCoin(common.RuneAsset(), pool.AssetValueInRune(item.msg.Tx.Coins[0].Amount))
		nonRuneAsset = targetAsset
		pool = pools[nonRuneAsset]
		if pool.IsEmpty() || !pool.IsAvailable() || pool.BalanceRune.IsZero() || pool.BalanceAsset.IsZero() {
			continue
		}
		virtualDepthMult = int64(10_000)
		if targetAsset.IsSyntheticAsset() {
			virtualDepthMult = synthVirtualDepthMult
		}
		vm.getLiquidityFeeAndSlip(ctx, pool, runeCoin, &items[i], virtualDepthMult)
	}

	return items, nil
}

// getLiquidityFeeAndSlip calculate liquidity fee and slip, fee is in RUNE
func (vm *SwapQueueV104) getLiquidityFeeAndSlip(ctx cosmos.Context, pool Pool, sourceCoin common.Coin, item *swapItem, virtualDepthMult int64) {
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

	swapper, err := GetSwapper(vm.k.GetVersion())
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
