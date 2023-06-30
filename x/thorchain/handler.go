package thorchain

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/x/auth/legacy/legacytx"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/common/tokenlist"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// MsgHandler is an interface expect all handler to implement
type MsgHandler interface {
	Run(ctx cosmos.Context, msg cosmos.Msg) (*cosmos.Result, error)
}

// NewExternalHandler returns a handler for "thorchain" type messages.
func NewExternalHandler(mgr Manager) cosmos.Handler {
	return func(ctx cosmos.Context, msg cosmos.Msg) (_ *cosmos.Result, err error) {
		// recover
		if mgr.GetVersion().GTE(semver.MustParse("1.106.0")) {
			defer func() {
				if r := recover(); r != nil {
					// print stack
					stack := make([]byte, 1024)
					length := runtime.Stack(stack, true)
					ctx.Logger().Error("panic", "msg", msg)
					fmt.Println(string(stack[:length]))
					err = fmt.Errorf("panic: %v", r)
				}
			}()
		}

		ctx = ctx.WithEventManager(cosmos.NewEventManager())
		if mgr.GetVersion().LT(semver.MustParse("1.90.0")) {
			_ = mgr.Keeper().GetLowestActiveVersion(ctx) // TODO: remove me on hard fork
		}
		handlerMap := getHandlerMapping(mgr)
		legacyMsg, ok := msg.(legacytx.LegacyMsg)
		if !ok {
			return nil, cosmos.ErrUnknownRequest("unknown message type")
		}
		h, ok := handlerMap[legacyMsg.Type()]
		if !ok {
			errMsg := fmt.Sprintf("Unrecognized thorchain Msg type: %v", legacyMsg.Type())
			return nil, cosmos.ErrUnknownRequest(errMsg)
		}
		result, err := h.Run(ctx, msg)
		if err != nil {
			return nil, err
		}
		if result == nil {
			result = &cosmos.Result{}
		}
		if len(ctx.EventManager().Events()) > 0 {
			result.Events = ctx.EventManager().ABCIEvents()
		}
		return result, nil
	}
}

func getHandlerMapping(mgr Manager) map[string]MsgHandler {
	return getHandlerMappingV65(mgr)
}

func getHandlerMappingV65(mgr Manager) map[string]MsgHandler {
	// New arch handlers
	m := make(map[string]MsgHandler)

	// consensus handlers
	m[MsgTssPool{}.Type()] = NewTssHandler(mgr)
	m[MsgObservedTxIn{}.Type()] = NewObservedTxInHandler(mgr)
	m[MsgObservedTxOut{}.Type()] = NewObservedTxOutHandler(mgr)
	m[MsgTssKeysignFail{}.Type()] = NewTssKeysignHandler(mgr)
	m[MsgErrataTx{}.Type()] = NewErrataTxHandler(mgr)
	m[MsgBan{}.Type()] = NewBanHandler(mgr)
	m[MsgNetworkFee{}.Type()] = NewNetworkFeeHandler(mgr)
	m[MsgSolvency{}.Type()] = NewSolvencyHandler(mgr)

	// cli handlers (non-consensus)
	m[MsgMimir{}.Type()] = NewMimirHandler(mgr)
	m[MsgSetNodeKeys{}.Type()] = NewSetNodeKeysHandler(mgr)
	m[MsgSetVersion{}.Type()] = NewVersionHandler(mgr)
	m[MsgSetIPAddress{}.Type()] = NewIPAddressHandler(mgr)
	m[MsgNodePauseChain{}.Type()] = NewNodePauseChainHandler(mgr)

	// native handlers (non-consensus)
	m[MsgSend{}.Type()] = NewSendHandler(mgr)
	m[MsgDeposit{}.Type()] = NewDepositHandler(mgr)
	return m
}

// NewInternalHandler returns a handler for "thorchain" internal type messages.
func NewInternalHandler(mgr Manager) cosmos.Handler {
	return func(ctx cosmos.Context, msg cosmos.Msg) (*cosmos.Result, error) {
		version := mgr.GetVersion()
		if version.LT(semver.MustParse("1.90.0")) {
			version = mgr.Keeper().GetLowestActiveVersion(ctx) // TODO remove me on hard fork
		}
		handlerMap := getInternalHandlerMapping(mgr)
		legacyMsg, ok := msg.(legacytx.LegacyMsg)
		if !ok {
			return nil, cosmos.ErrUnknownRequest("invalid message type")
		}
		h, ok := handlerMap[legacyMsg.Type()]
		if !ok {
			errMsg := fmt.Sprintf("Unrecognized thorchain Msg type: %v", legacyMsg.Type())
			return nil, cosmos.ErrUnknownRequest(errMsg)
		}
		if version.GTE(semver.MustParse("1.88.1")) {
			// CacheContext() returns a context which caches all changes and only forwards
			// to the underlying context when commit() is called. Call commit() only when
			// the handler succeeds, otherwise return error and the changes will be discarded.
			// On commit, cached events also have to be explicitly emitted.
			cacheCtx, commit := ctx.CacheContext()
			res, err := h.Run(cacheCtx, msg)
			if err == nil {
				// Success, commit the cached changes and events
				commit()
				ctx.EventManager().EmitEvents(cacheCtx.EventManager().Events())
			}
			return res, err
		}
		return h.Run(ctx, msg)
	}
}

func getInternalHandlerMapping(mgr Manager) map[string]MsgHandler {
	// New arch handlers
	m := make(map[string]MsgHandler)
	m[MsgOutboundTx{}.Type()] = NewOutboundTxHandler(mgr)
	m[MsgYggdrasil{}.Type()] = NewYggdrasilHandler(mgr)
	m[MsgSwap{}.Type()] = NewSwapHandler(mgr)
	m[MsgReserveContributor{}.Type()] = NewReserveContributorHandler(mgr)
	m[MsgBond{}.Type()] = NewBondHandler(mgr)
	m[MsgUnBond{}.Type()] = NewUnBondHandler(mgr)
	m[MsgLeave{}.Type()] = NewLeaveHandler(mgr)
	m[MsgDonate{}.Type()] = NewDonateHandler(mgr)
	m[MsgWithdrawLiquidity{}.Type()] = NewWithdrawLiquidityHandler(mgr)
	m[MsgAddLiquidity{}.Type()] = NewAddLiquidityHandler(mgr)
	m[MsgRefundTx{}.Type()] = NewRefundHandler(mgr)
	m[MsgMigrate{}.Type()] = NewMigrateHandler(mgr)
	m[MsgRagnarok{}.Type()] = NewRagnarokHandler(mgr)
	m[MsgSwitch{}.Type()] = NewSwitchHandler(mgr)
	m[MsgNoOp{}.Type()] = NewNoOpHandler(mgr)
	m[MsgConsolidate{}.Type()] = NewConsolidateHandler(mgr)
	m[MsgManageTHORName{}.Type()] = NewManageTHORNameHandler(mgr)
	m[MsgLoanOpen{}.Type()] = NewLoanOpenHandler(mgr)
	m[MsgLoanRepayment{}.Type()] = NewLoanRepaymentHandler(mgr)
	return m
}

func getMsgSwapFromMemo(memo SwapMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	if memo.Destination.IsEmpty() {
		memo.Destination = tx.Tx.FromAddress
	}
	return NewMsgSwap(tx.Tx, memo.GetAsset(), memo.Destination, memo.SlipLimit, memo.AffiliateAddress, memo.AffiliateBasisPoints, memo.GetDexAggregator(), memo.GetDexTargetAddress(), memo.GetDexTargetLimit(), memo.GetOrderType(), signer), nil
}

func getMsgWithdrawFromMemo(memo WithdrawLiquidityMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	withdrawAmount := cosmos.NewUint(MaxWithdrawBasisPoints)
	if !memo.GetAmount().IsZero() {
		withdrawAmount = memo.GetAmount()
	}
	return NewMsgWithdrawLiquidity(tx.Tx, tx.Tx.FromAddress, withdrawAmount, memo.GetAsset(), memo.GetWithdrawalAsset(), signer), nil
}

func getMsgAddLiquidityFromMemo(ctx cosmos.Context, memo AddLiquidityMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	// Extract the Rune amount and the asset amount from the transaction. At least one of them must be
	// nonzero. If THORNode saw two types of coins, one of them must be the asset coin.
	runeCoin := tx.Tx.Coins.GetCoin(common.RuneAsset())
	assetCoin := tx.Tx.Coins.GetCoin(memo.GetAsset())

	var runeAddr common.Address
	var assetAddr common.Address
	if tx.Tx.Chain.Equals(common.THORChain) {
		runeAddr = tx.Tx.FromAddress
		assetAddr = memo.GetDestination()
	} else {
		runeAddr = memo.GetDestination()
		assetAddr = tx.Tx.FromAddress
	}
	// in case we are providing native rune and another native asset
	if memo.GetAsset().Chain.Equals(common.THORChain) {
		assetAddr = runeAddr
	}

	return NewMsgAddLiquidity(tx.Tx, memo.GetAsset(), runeCoin.Amount, assetCoin.Amount, runeAddr, assetAddr, memo.AffiliateAddress, memo.AffiliateBasisPoints, signer), nil
}

func getMsgDonateFromMemo(memo DonateMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	runeCoin := tx.Tx.Coins.GetCoin(common.RuneAsset())
	assetCoin := tx.Tx.Coins.GetCoin(memo.GetAsset())
	return NewMsgDonate(tx.Tx, memo.GetAsset(), runeCoin.Amount, assetCoin.Amount, signer), nil
}

func getMsgRefundFromMemo(memo RefundMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgRefundTx(tx, memo.GetTxID(), signer), nil
}

func getMsgOutboundFromMemo(memo OutboundMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgOutboundTx(tx, memo.GetTxID(), signer), nil
}

func getMsgMigrateFromMemo(memo MigrateMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgMigrate(tx, memo.GetBlockHeight(), signer), nil
}

func getMsgRagnarokFromMemo(memo RagnarokMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgRagnarok(tx, memo.GetBlockHeight(), signer), nil
}

func getMsgLeaveFromMemo(memo LeaveMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgLeave(tx.Tx, memo.GetAccAddress(), signer), nil
}

func getMsgLoanOpenFromMemo(memo LoanOpenMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgLoanOpen(tx.Tx.FromAddress, tx.Tx.Coins[0].Asset, tx.Tx.Coins[0].Amount, memo.TargetAddress, memo.TargetAsset, memo.GetMinOut(), memo.GetAffiliateAddress(), memo.GetAffiliateBasisPoints(), memo.GetDexAggregator(), memo.GetDexTargetAddress(), memo.DexTargetLimit, signer), nil
}

func getMsgLoanRepaymentFromMemo(memo LoanRepaymentMemo, from common.Address, coin common.Coin, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgLoanRepayment(memo.Owner, memo.Asset, memo.MinOut, from, coin, signer), nil
}

func getMsgBondFromMemo(memo BondMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	coin := tx.Tx.Coins.GetCoin(common.RuneAsset())
	return NewMsgBond(tx.Tx, memo.GetAccAddress(), coin.Amount, tx.Tx.FromAddress, memo.BondProviderAddress, signer, memo.NodeOperatorFee), nil
}

func getMsgUnbondFromMemo(memo UnbondMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	return NewMsgUnBond(tx.Tx, memo.GetAccAddress(), memo.GetAmount(), tx.Tx.FromAddress, memo.BondProviderAddress, signer), nil
}

func getMsgManageTHORNameFromMemo(memo ManageTHORNameMemo, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	if len(tx.Tx.Coins) == 0 {
		return nil, fmt.Errorf("transaction must have rune in it")
	}
	return NewMsgManageTHORName(memo.Name, memo.Chain, memo.Address, tx.Tx.Coins[0], memo.Expire, memo.PreferredAsset, memo.Owner, signer), nil
}

func processOneTxIn(ctx cosmos.Context, version semver.Version, keeper keeper.Keeper, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
	switch {
	case version.GTE(semver.MustParse("1.107.0")):
		return processOneTxInV107(ctx, keeper, tx, signer)
	case version.GTE(semver.MustParse("0.63.0")):
		return processOneTxInV63(ctx, keeper, tx, signer)
	}
	return nil, errBadVersion
}

func processOneTxInV107(ctx cosmos.Context, keeper keeper.Keeper, tx ObservedTx, signer cosmos.AccAddress) (cosmos.Msg, error) {
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
	case LoanOpenMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		newMsg, err = getMsgLoanOpenFromMemo(m, tx, signer)
	case LoanRepaymentMemo:
		m.Asset = fuzzyAssetMatch(ctx, keeper, m.Asset)
		from := common.NoAddress
		if keeper.GetVersion().GTE(semver.MustParse("1.110.0")) {
			from = tx.Tx.FromAddress
		}
		newMsg, err = getMsgLoanRepaymentFromMemo(m, from, tx.Tx.Coins[0], signer)
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

func fuzzyAssetMatch(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset) common.Asset {
	version := keeper.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.103.0")):
		return fuzzyAssetMatchV103(ctx, keeper, asset)
	case version.GTE(semver.MustParse("1.83.0")):
		return fuzzyAssetMatchV83(ctx, keeper, asset)
	default:
		return fuzzyAssetMatchV1(ctx, keeper, asset)
	}
}

func fuzzyAssetMatchV103(ctx cosmos.Context, keeper keeper.Keeper, origAsset common.Asset) common.Asset {
	asset := origAsset.GetLayer1Asset()
	// if it's already an exact match with successfully-added liquidity, return it immediately
	pool, err := keeper.GetPool(ctx, asset)
	if err != nil {
		return origAsset
	}
	// Only check BalanceRune after checking the error so that no panic if there were an error.
	if !pool.BalanceRune.IsZero() {
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
		// Use LTE rather than LT so this function can only return origAsset or a match,
		// never NewPool()'s empty Asset.
		if winner.BalanceRune.LTE(pool.BalanceRune) {
			winner = pool
		}
	}

	winner.Asset.Synth = origAsset.Synth

	return winner.Asset
}

func externalAssetMatch(version semver.Version, chain common.Chain, hint string) string {
	switch {
	case version.GTE(semver.MustParse("1.95.0")):
		return externalAssetMatchV95(version, chain, hint)
	case version.GTE(semver.MustParse("1.93.0")):
		return externalAssetMatchV93(version, chain, hint)
	default:
		return hint
	}
}

func externalAssetMatchV95(version semver.Version, chain common.Chain, hint string) string {
	if len(hint) == 0 {
		return hint
	}
	if chain.IsEVM() {
		// find all potential matches
		matches := []string{}
		for _, token := range tokenlist.GetEVMTokenList(chain, version).Tokens {
			if strings.HasSuffix(strings.ToLower(token.Address), strings.ToLower(hint)) {
				matches = append(matches, token.Address)
				if len(matches) > 1 {
					break
				}
			}
		}
		// if we only have one match, lets go with it, otherwise leave the
		// user's input alone. It may still work, if it doesn't, should get the
		// gas asset instead of the erc20 desired.
		if len(matches) == 1 {
			return matches[0]
		}

		return hint
	}
	return hint
}

func externalAssetMatchV93(version semver.Version, chain common.Chain, hint string) string {
	if len(hint) == 0 {
		return hint
	}
	switch chain {
	case common.ETHChain:
		// find all potential matches
		matches := []string{}
		for _, token := range tokenlist.GetETHTokenList(version).Tokens {
			if strings.HasSuffix(strings.ToLower(token.Address), strings.ToLower(hint)) {
				matches = append(matches, token.Address)
				if len(matches) > 1 {
					break
				}
			}
		}

		// if we only have one match, lets go with it, otherwise leave the
		// user's input alone. It may still work, if it doesn't, should get the
		// gas asset instead of the erc20 desired.
		if len(matches) == 1 {
			return matches[0]
		}

		return hint
	default:
		return hint
	}
}
