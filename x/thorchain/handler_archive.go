package thorchain

import (
	"strings"

	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

func processOneTxInV63(ctx cosmos.Context, keeper keeper.Keeper, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	memo, err := ParseMemoWithTHORNames(ctx, keeper, tx.Tx.Memo)
	if err != nil {
		ctx.Logger().Error("fail to parse memo", "error", err)
		return nil, err
	}
	// THORNode should not have one tx across chain, if it is cross chain it should be separate tx
	var newMsg cosmos.Msg
	// interpret the memo and initialize a corresponding msg event
	switch m := memo.(type) {
	case AddLiquidityMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgAddLiquidityFromMemo(ctx, m, tx, signer)
	case WithdrawLiquidityMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgWithdrawFromMemo(m, tx, signer)
	case SwapMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		m.DexTargetAddress = externalAssetMatch(keeper.GetVersion(), m.Asset.GetChain(), m.DexTargetAddress)
		newMsg, err = getMsgSwapFromMemo(m, tx, signer)
	case DonateMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgDonateFromMemo(m, tx, signer)
	case RefundMemo:
		newMsg, err = getMsgRefundFromMemo(m, tx, signer)
	case OutboundMemo:
		newMsg, err = getMsgOutboundFromMemo(m, tx, signer)
	case MigrateMemo:
		newMsg, err = getMsgMigrateFromMemo(m, tx, signer)
	case BondMemo:
		newMsg, err = getMsgBondFromMemo(m, tx, signer)
	case UnbondMemo:
		newMsg, err = getMsgUnbondFromMemo(m, tx, signer)
	case RagnarokMemo:
		newMsg, err = getMsgRagnarokFromMemo(m, tx, signer)
	case LeaveMemo:
		newMsg, err = getMsgLeaveFromMemo(m, tx, signer)
	case YggdrasilFundMemo:
		newMsg = NewMsgYggdrasil(tx.Tx, tx.ObservedPubKey, m.GetBlockHeight(), true, tx.Tx.Coins, signer)
	case YggdrasilReturnMemo:
		newMsg = NewMsgYggdrasil(tx.Tx, tx.ObservedPubKey, m.GetBlockHeight(), false, tx.Tx.Coins, signer)
	case ReserveMemo:
		res := NewReserveContributor(tx.Tx.FromAddress, tx.Tx.Coins.GetCoin(common.RuneAsset()).Amount)
		newMsg = NewMsgReserveContributor(tx.Tx, res, signer)
	case SwitchMemo:
		newMsg = NewMsgSwitch(tx.Tx, memo.GetDestination(), signer)
	case NoOpMemo:
		newMsg = NewMsgNoOp(tx, signer, m.Action)
	case ConsolidateMemo:
		newMsg = NewMsgConsolidate(tx, signer)
	case ManageTHORNameMemo:
		newMsg, err = getMsgManageTHORNameFromMemo(m, tx, signer)
	default:
		return nil, errInvalidMemo
	}

	if err != nil {
		return newMsg, err
	}
	// MsgAddLiquidity & MsgSwap has a new version of validateBasic
	switch m := newMsg.(type) {
	case *MsgAddLiquidity:
		switch {
		case keeper.GetVersion().GTE(semver.MustParse("1.98.0")):
			return newMsg, m.ValidateBasicV98()
		case keeper.GetVersion().GTE(semver.MustParse("1.93.0")):
			return newMsg, m.ValidateBasicV93()
		default:
			return newMsg, m.ValidateBasicV63()
		}
	case *MsgSwap:
		return newMsg, m.ValidateBasicV63()
	}
	return newMsg, newMsg.ValidateBasic()
}

func fuzzyAssetMatchV83(ctx cosmos.Context, keeper keeper.Keeper, origAsset common.Asset) common.Asset {
	asset := origAsset.GetLayer1Asset()
	// if its already an exact match, return it immediately
	if keeper.PoolExist(ctx, asset.GetLayer1Asset()) {
		return origAsset
	}

	matches := make(Pools, 0)

	iterator := keeper.GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := keeper.Cdc().Unmarshal(iterator.Value(), &pool); err != nil {
			ctx.Logger().Error("fail to fetch pool", "asset", asset, "err", err)
			continue
		}

		// check chain match
		if !asset.Chain.Equals(pool.Asset.Chain) {
			continue
		}

		// check ticker match
		if !asset.Ticker.Equals(pool.Asset.Ticker) {
			continue
		}

		// check symbol
		parts := strings.Split(asset.Symbol.String(), "-")
		// check if no symbol given (ie "USDT" or "USDT-")
		if len(parts) < 2 || strings.EqualFold(parts[1], "") {
			matches = append(matches, pool)
			continue
		}

		if strings.HasSuffix(strings.ToLower(pool.Asset.Symbol.String()), strings.ToLower(parts[1])) {
			matches = append(matches, pool)
			continue
		}
	}

	// if we found no matches, return the argument given
	if len(matches) == 0 {
		return origAsset
	}

	// find the deepest pool
	winner := NewPool()
	for _, pool := range matches {
		if winner.BalanceRune.LT(pool.BalanceRune) {
			winner = pool
		}
	}

	winner.Asset.Synth = origAsset.Synth

	return winner.Asset
}

func fuzzyAssetMatchV1(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset) common.Asset {
	// if its already an exact match, return it immediately
	if keeper.PoolExist(ctx, asset.GetLayer1Asset()) {
		return asset
	}

	matches := make(Pools, 0)

	iterator := keeper.GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := keeper.Cdc().Unmarshal(iterator.Value(), &pool); err != nil {
			ctx.Logger().Error("fail to fetch pool", "asset", asset, "err", err)
			continue
		}

		// check chain match
		if !asset.Chain.Equals(pool.Asset.Chain) {
			continue
		}

		// check ticker match
		if !asset.Ticker.Equals(pool.Asset.Ticker) {
			continue
		}

		// check symbol
		parts := strings.Split(asset.Symbol.String(), "-")
		// check if no symbol given (ie "USDT" or "USDT-")
		if len(parts) < 2 || strings.EqualFold(parts[1], "") {
			matches = append(matches, pool)
			continue
		}

		if strings.HasSuffix(strings.ToLower(pool.Asset.Symbol.String()), strings.ToLower(parts[1])) {
			matches = append(matches, pool)
			continue
		}
	}

	// if we found no matches, return the argument given
	if len(matches) == 0 {
		return asset
	}

	// find the deepest pool
	winner := NewPool()
	for _, pool := range matches {
		if winner.BalanceRune.LT(pool.BalanceRune) {
			winner = pool
		}
	}

	return winner.Asset
}
