package thorchain

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	abci "github.com/tendermint/tendermint/abci/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/log"
	openapi "gitlab.com/thorchain/thornode/openapi/gen"
	mem "gitlab.com/thorchain/thornode/x/thorchain/memo"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

// -------------------------------------------------------------------------------------
// Config
// -------------------------------------------------------------------------------------

const (
	fromAssetParam            = "from_asset"
	toAssetParam              = "to_asset"
	targetAssetParam          = "target_asset"
	loanAssetParam            = "loan_asset"
	assetParam                = "asset"
	fromAddressParam          = "from_address"
	addressParam              = "address"
	loanOwnerParam            = "loan_owner"
	withdrawBasisPointsParam  = "withdraw_bps"
	amountParam               = "amount"
	destinationParam          = "destination"
	toleranceBasisPointsParam = "tolerance_bps"
	affiliateParam            = "affiliate"
	affiliateBpsParam         = "affiliate_bps"
	minOutParam               = "min_out"

	quoteWarning    = "Do not cache this response. Do not send funds after the expiry."
	quoteExpiration = 15 * time.Minute
)

var nullLogger = &log.TendermintLogWrapper{Logger: zerolog.New(io.Discard)}

// -------------------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------------------

func quoteErrorResponse(err error) ([]byte, error) {
	return json.Marshal(map[string]string{"error": err.Error()})
}

func quoteParseParams(data []byte) (params url.Values, err error) {
	// parse the query parameters
	u, err := url.ParseRequestURI(string(data))
	if err != nil {
		return nil, fmt.Errorf("bad params: %w", err)
	}

	// error if parameters were not provided
	if len(u.Query()) == 0 {
		return nil, fmt.Errorf("no parameters provided")
	}

	return u.Query(), nil
}

func quoteParseAddress(ctx cosmos.Context, mgr *Mgrs, addrString string, chain common.Chain) (common.Address, error) {
	if addrString == "" {
		return common.NoAddress, nil
	}

	// attempt to parse a raw address
	addr, err := common.NewAddress(addrString)
	if err == nil {
		return addr, nil
	}

	// attempt to lookup a thorname address
	name, err := mgr.Keeper().GetTHORName(ctx, addrString)
	if err != nil {
		return common.NoAddress, fmt.Errorf("unable to parse address: %w", err)
	}

	// find the address for the correct chain
	for _, alias := range name.Aliases {
		if alias.Chain.Equals(chain) {
			return alias.Address, nil
		}
	}

	return common.NoAddress, fmt.Errorf("no thorname alias for chain %s", chain)
}

func quoteHandleAffiliate(ctx cosmos.Context, mgr *Mgrs, params url.Values, amount sdk.Uint) (affiliate common.Address, memo string, bps, newAmount sdk.Uint, err error) {
	// parse affiliate
	memo = "" // do not resolve thorname for the memo
	if len(params[affiliateParam]) > 0 {
		affiliate, err = quoteParseAddress(ctx, mgr, params[affiliateParam][0], common.THORChain)
		if err != nil {
			err = fmt.Errorf("bad affiliate address: %w", err)
			return
		}
		memo = params[affiliateParam][0]
	}

	// parse affiliate fee
	bps = sdk.NewUint(0)
	if len(params[affiliateBpsParam]) > 0 {
		bps, err = sdk.ParseUint(params[affiliateBpsParam][0])
		if err != nil {
			err = fmt.Errorf("bad affiliate fee: %w", err)
			return
		}
	}

	// verify affiliate fee
	if bps.GT(sdk.NewUint(10000)) {
		err = fmt.Errorf("affiliate fee must be less than 10000 bps")
		return
	}

	// compute the new swap amount if an affiliate fee will be taken first
	if affiliate != common.NoAddress && !bps.IsZero() {
		// affiliate fee modifies amount at observation before the swap
		amount = common.GetSafeShare(
			cosmos.NewUint(10000).Sub(bps),
			cosmos.NewUint(10000),
			amount,
		)
	}

	return affiliate, memo, bps, amount, nil
}

func hasPrefixMatch(prefix string, values []string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func quoteReverseFuzzyAsset(ctx cosmos.Context, mgr *Mgrs, asset common.Asset) (common.Asset, error) {
	// get all pools
	pools, err := mgr.Keeper().GetPools(ctx)
	if err != nil {
		return asset, fmt.Errorf("failed to get pools: %w", err)
	}

	// get all other assets
	assets := []string{}
	for _, p := range pools {
		if p.IsAvailable() && !p.IsEmpty() && !p.Asset.Equals(asset) {
			assets = append(assets, p.Asset.String())
		}
	}

	// find the shortest unique prefix of the memo asset
	as := asset.String()
	for i := 1; i < len(as); i++ {
		if !hasPrefixMatch(as[:i], assets) {
			return common.NewAsset(as[:i])
		}
	}

	return asset, nil
}

func quoteSimulateSwap(ctx cosmos.Context, mgr *Mgrs, amount sdk.Uint, msg *MsgSwap) (res *openapi.QuoteSwapResponse, emitAmount, outboundFeeAmount sdk.Uint, err error) {
	// if the generated memo is too long for the source chain send error
	maxMemoLength := msg.Tx.Coins[0].Asset.Chain.MaxMemoLength()
	if maxMemoLength > 0 && len(msg.Tx.Memo) > maxMemoLength {
		return nil, sdk.ZeroUint(), sdk.ZeroUint(), fmt.Errorf("generated memo too long for source chain")
	}

	// use the first active node account as the signer
	nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, sdk.ZeroUint(), sdk.ZeroUint(), fmt.Errorf("no active node accounts: %w", err)
	}
	msg.Signer = nodeAccounts[0].NodeAddress

	// simulate the swap
	events, err := simulateInternal(ctx, mgr, msg)
	if err != nil {
		return nil, sdk.ZeroUint(), sdk.ZeroUint(), err
	}

	// extract events
	var swaps []map[string]string
	var fee map[string]string
	for _, e := range events {
		switch e.Type {
		case "swap":
			swaps = append(swaps, eventMap(e))
		case "fee":
			fee = eventMap(e)
		}
	}
	finalSwap := swaps[len(swaps)-1]

	// parse outbound fee from event
	outboundFeeCoin, err := common.ParseCoin(fee["coins"])
	if err != nil {
		return nil, sdk.ZeroUint(), sdk.ZeroUint(), fmt.Errorf("unable to parse outbound fee coin: %w", err)
	}
	outboundFeeAmount = outboundFeeCoin.Amount

	// parse outbound amount from event
	emitCoin, err := common.ParseCoin(finalSwap["emit_asset"])
	if err != nil {
		return nil, sdk.ZeroUint(), sdk.ZeroUint(), fmt.Errorf("unable to parse emit coin: %w", err)
	}
	emitAmount = emitCoin.Amount

	// approximate the affiliate fee in the target asset
	affiliateFee := sdk.ZeroUint()
	if msg.AffiliateAddress != common.NoAddress && !msg.AffiliateBasisPoints.IsZero() {
		affiliateFee = common.GetUncappedShare(msg.AffiliateBasisPoints, cosmos.NewUint(10_000), amount)
		affiliateFee = affiliateFee.Mul(emitAmount).Quo(msg.Tx.Coins[0].Amount)

		// undo the approximate slip fee since the affiliate fee is taken first
		factor := sdk.NewUint(10_000)
		for _, s := range swaps {
			factor.Add(sdk.NewUintFromString(s["swap_slip"]))
		}
		affiliateFee = affiliateFee.Mul(factor).Quo(sdk.NewUint(10_000))
	}

	// sum the slip fees
	slippageBps := sdk.ZeroUint()
	for _, s := range swaps {
		slippageBps = slippageBps.Add(sdk.NewUintFromString(s["swap_slip"]))
	}

	// build response from simulation result events
	return &openapi.QuoteSwapResponse{
		ExpectedAmountOut: emitAmount.String(),
		Fees: openapi.QuoteFees{
			Asset:     msg.TargetAsset.String(),
			Affiliate: wrapString(affiliateFee.String()),
			Outbound:  "0", // set by the caller if non-zero
		},
		SlippageBps: slippageBps.BigInt().Int64(),
	}, emitAmount, outboundFeeAmount, nil
}

func quoteInboundInfo(ctx cosmos.Context, mgr *Mgrs, amount sdk.Uint, chain common.Chain) (address, router common.Address, confirmations int64, err error) {
	// If inbound chain is THORChain there is no inbound address
	if chain.IsTHORChain() {
		address = common.NoAddress
		router = common.NoAddress
	} else {
		// get the most secure vault for inbound
		active, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			return common.NoAddress, common.NoAddress, 0, err
		}
		constAccessor := mgr.GetConstants()
		signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
		vault := mgr.Keeper().GetMostSecure(ctx, active, signingTransactionPeriod)
		address, err = vault.PubKey.GetAddress(chain)
		if err != nil {
			return common.NoAddress, common.NoAddress, 0, err
		}

		router = common.NoAddress
		if chain.IsEVM() {
			router = vault.GetContract(chain).Router
		}
	}

	// estimate the inbound confirmation count blocks: ceil(amount/coinbase)
	if chain.DefaultCoinbase() > 0 {
		coinbase := cosmos.NewUint(uint64(chain.DefaultCoinbase()) * common.One)
		confirmations = amount.Quo(coinbase).BigInt().Int64()
		if !amount.Mod(coinbase).IsZero() {
			confirmations++
		}
	}

	return address, router, confirmations, nil
}

func quoteOutboundInfo(ctx cosmos.Context, mgr *Mgrs, coin common.Coin) (int64, error) {
	toi := TxOutItem{
		Memo: "OUT:-",
		Coin: coin,
	}
	outboundHeight, err := mgr.txOutStore.CalcTxOutHeight(ctx, mgr.GetVersion(), toi)
	if err != nil {
		return 0, err
	}
	return outboundHeight - ctx.BlockHeight(), nil
}

// -------------------------------------------------------------------------------------
// Swap
// -------------------------------------------------------------------------------------

// calculateMinSwapAmount returns the recommended minimum swap amount
// The recommended min swap amount is:
// - MAX(outbound_fee(src_chain), outbound_fee(dest_chain)) * 4 (priced in the inbound asset)
//
// The reason the base value is the MAX of the outbound fees of each chain is because if the swap is refunded
// the input amount will need to cover the outbound fee of the source chain.
// A 4x buffer is applied because outbound fees can spike quickly, meaning the original input amount could be less than the new
// outbound fee. If this happens and the swap is refunded, the refund will fail, and the user will lose the entire input amount.
func calculateMinSwapAmount(ctx cosmos.Context, mgr *Mgrs, fromAsset, toAsset common.Asset) (cosmos.Uint, error) {
	srcOutboundFee := mgr.GasMgr().GetFee(ctx, fromAsset.GetChain(), fromAsset)
	destOutboundFee := mgr.GasMgr().GetFee(ctx, toAsset.GetChain(), toAsset)

	if fromAsset.GetChain().IsTHORChain() && toAsset.GetChain().IsTHORChain() {
		// If this is a purely THORChain swap, no need to give a 4x buffer since outbound fees do not change
		// 2x buffer should suffice
		return srcOutboundFee.Mul(cosmos.NewUint(2)), nil
	}

	srcPool, err := mgr.Keeper().GetPool(ctx, fromAsset.GetLayer1Asset())
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get pool for asset %s", fromAsset)
	}

	destPool, err := mgr.Keeper().GetPool(ctx, toAsset.GetLayer1Asset())
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get pool for asset %s", toAsset)
	}

	// Convert destination chain outbound fee to input asset
	destInSrcAsset := destOutboundFee
	if !toAsset.IsNativeRune() {
		destInSrcAsset = destPool.AssetValueInRune(destOutboundFee)
	}

	if !fromAsset.IsNativeRune() {
		destInSrcAsset = srcPool.RuneValueInAsset(destInSrcAsset)
	}

	minSwapAmount := srcOutboundFee
	if destInSrcAsset.GT(srcOutboundFee) {
		minSwapAmount = destInSrcAsset
	}

	return minSwapAmount.Mul(cosmos.NewUint(4)), nil
}

func queryQuoteSwap(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// extract parameters
	params, err := quoteParseParams(req.Data)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// validate required parameters
	for _, p := range []string{fromAssetParam, toAssetParam, amountParam} {
		if len(params[p]) == 0 {
			return quoteErrorResponse(fmt.Errorf("missing required parameter %s", p))
		}
	}

	// parse assets
	fromAsset, err := common.NewAsset(params[fromAssetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad from asset: %w", err))
	}
	toAsset, err := common.NewAsset(params[toAssetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad to asset: %w", err))
	}

	// parse amount
	amount, err := cosmos.ParseUint(params[amountParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad amount: %w", err))
	}

	// if from asset is a synth, transfer asset to asgard module
	if fromAsset.IsSyntheticAsset() {
		if len(params[fromAddressParam]) == 0 {
			return quoteErrorResponse(fmt.Errorf("missing required parameter %s", fromAddressParam))
		}
		fromAddress, err := quoteParseAddress(ctx, mgr, params[fromAddressParam][0], fromAsset.Chain)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad from address: %w", err))
		}

		fromAccAddress, err := fromAddress.AccAddress()
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to get account address: %w", err))
		}

		synthCoins := cosmos.NewCoins(cosmos.NewCoin(string(fromAsset.Symbol), sdk.NewInt(int64(amount.Uint64()))))

		// If from_address doesn't have enough synth balance just mint required coins to Asgard so swap can be simulated.
		// Otherwise, just send synth coins to Asgard from from_address
		if !mgr.Keeper().HasCoins(ctx, fromAccAddress, synthCoins) {
			err = mgr.Keeper().MintToModule(ctx, ModuleName, common.NewCoin(fromAsset, amount))
			if err != nil {
				return quoteErrorResponse(fmt.Errorf("failed to mint coins to module: %w", err))
			}

			err = mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(common.NewCoin(fromAsset, amount)))
			if err != nil {
				return quoteErrorResponse(fmt.Errorf("failed to send coins to asgard: %w", err))
			}
		} else {
			err = mgr.Keeper().SendFromAccountToModule(ctx, fromAccAddress, AsgardName, common.NewCoins(common.NewCoin(fromAsset, amount)))
			if err != nil {
				return quoteErrorResponse(fmt.Errorf("failed to send from account to module: %w", err))
			}
		}
	}

	// parse affiliate
	affiliate, affiliateMemo, affiliateBps, swapAmount, err := quoteHandleAffiliate(ctx, mgr, params, amount)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// parse destination address or generate a random one
	sendMemo := true
	var destination common.Address
	if len(params[destinationParam]) > 0 {
		destination, err = quoteParseAddress(ctx, mgr, params[destinationParam][0], toAsset.Chain)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad destination address: %w", err))
		}

	} else {
		chain := common.THORChain
		if !toAsset.IsSyntheticAsset() {
			chain = toAsset.Chain
		}
		destination, err = types.GetRandomPubKey().GetAddress(chain)
		if err != nil {
			return nil, fmt.Errorf("failed to generate address: %w", err)
		}
		sendMemo = false // do not send memo if destination was random
	}

	// parse tolerance basis points
	limit := sdk.ZeroUint()
	if len(params[toleranceBasisPointsParam]) > 0 {
		// validate tolerance basis points
		toleranceBasisPoints, err := sdk.ParseUint(params[toleranceBasisPointsParam][0])
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad tolerance basis points: %w", err))
		}
		if toleranceBasisPoints.GT(sdk.NewUint(10000)) {
			return quoteErrorResponse(fmt.Errorf("tolerance basis points must be less than 10000"))
		}

		// convert to a limit of target asset amount assuming zero fees and slip
		feelessEmit := swapAmount

		// When one asset is RUNE, no conversion is necessary,
		// and empty fields would cause a divide-by-zero error; skip it.
		if !fromAsset.IsRune() {
			// get from asset pool
			fromPool, err := mgr.Keeper().GetPool(ctx, fromAsset.GetLayer1Asset())
			if err != nil {
				return quoteErrorResponse(fmt.Errorf("failed to get pool: %w", err))
			}

			// ensure pool exists
			if fromPool.IsEmpty() {
				return quoteErrorResponse(fmt.Errorf("pool does not exist"))
			}

			feelessEmit = feelessEmit.Mul(fromPool.BalanceRune).Quo(fromPool.BalanceAsset)
		}
		if !toAsset.IsRune() {
			// get to asset pool
			toPool, err := mgr.Keeper().GetPool(ctx, toAsset.GetLayer1Asset())
			if err != nil {
				return quoteErrorResponse(fmt.Errorf("failed to get pool: %w", err))
			}

			// ensure pool exists
			if toPool.IsEmpty() {
				return quoteErrorResponse(fmt.Errorf("pool does not exist"))
			}

			feelessEmit = feelessEmit.Mul(toPool.BalanceAsset).Quo(toPool.BalanceRune)
		}
		limit = feelessEmit.MulUint64(10000 - toleranceBasisPoints.Uint64()).QuoUint64(10000)
	}

	// create the memo
	memo := &SwapMemo{
		MemoBase: mem.MemoBase{
			TxType: TxSwap,
			Asset:  toAsset,
		},
		Destination:          destination,
		SlipLimit:            limit,
		AffiliateAddress:     common.Address(affiliateMemo),
		AffiliateBasisPoints: affiliateBps,
	}

	// if from asset chain has memo length restrictions use a prefix
	if fromAsset.Chain.MaxMemoLength() > 0 && len(memo.String()) > fromAsset.Chain.MaxMemoLength() {
		memo.Asset, err = quoteReverseFuzzyAsset(ctx, mgr, toAsset)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to reverse fuzzy asset: %w", err))
		}
	}

	// create the swap message
	msg := &types.MsgSwap{
		Tx: common.Tx{
			ID:          common.BlankTxID,
			Chain:       fromAsset.Chain,
			FromAddress: common.NoopAddress,
			ToAddress:   common.NoopAddress,
			Coins: []common.Coin{
				{
					Asset:  fromAsset,
					Amount: swapAmount,
				},
			},
			Gas: []common.Coin{{
				Asset:  common.RuneAsset(),
				Amount: sdk.NewUint(1),
			}},
			Memo: memo.String(),
		},
		TargetAsset:          toAsset,
		TradeTarget:          limit,
		Destination:          destination,
		AffiliateAddress:     affiliate,
		AffiliateBasisPoints: affiliateBps,
	}

	// simulate the swap
	res, emitAmount, outboundFeeAmount, err := quoteSimulateSwap(ctx, mgr, amount, msg)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to simulate swap: %w", err))
	}

	// check invariant
	if emitAmount.LT(outboundFeeAmount) {
		return quoteErrorResponse(fmt.Errorf("invariant broken: emit %s less than outbound fee %s", emitAmount, outboundFeeAmount))
	}

	// the amount out will deduct the outbound fee
	res.ExpectedAmountOut = emitAmount.Sub(outboundFeeAmount).String()
	res.Fees.Outbound = outboundFeeAmount.String()

	// estimate the inbound info
	inboundAddress, routerAddress, inboundConfirmations, err := quoteInboundInfo(ctx, mgr, amount, fromAsset.GetChain())
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.InboundAddress = wrapString(inboundAddress.String())
	if inboundConfirmations > 0 {
		res.InboundConfirmationBlocks = wrapInt64(inboundConfirmations)
		res.InboundConfirmationSeconds = wrapInt64(inboundConfirmations * msg.Tx.Chain.ApproximateBlockMilliseconds() / 1000)
	}

	// estimate the outbound info
	outboundDelay, err := quoteOutboundInfo(ctx, mgr, common.Coin{Asset: toAsset, Amount: emitAmount})
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.OutboundDelayBlocks = outboundDelay
	res.OutboundDelaySeconds = outboundDelay * common.THORChain.ApproximateBlockMilliseconds() / 1000

	// send memo if the destination was provided
	if sendMemo {
		res.Memo = wrapString(memo.String())
	}

	// set info fields
	if fromAsset.Chain.IsEVM() {
		res.Router = wrapString(routerAddress.String())
	}
	if !fromAsset.Chain.DustThreshold().IsZero() {
		res.DustThreshold = wrapString(fromAsset.Chain.DustThreshold().String())
	}

	res.Notes = fromAsset.GetChain().InboundNotes()
	res.Warning = quoteWarning
	res.Expiry = time.Now().Add(quoteExpiration).Unix()
	minSwapAmount, err := calculateMinSwapAmount(ctx, mgr, fromAsset, toAsset)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("Failed to calculate min amount in: %s", err.Error()))
	}
	res.RecommendedMinAmountIn = wrapString(minSwapAmount.String())

	return json.MarshalIndent(res, "", "  ")
}

// -------------------------------------------------------------------------------------
// Saver Deposit
// -------------------------------------------------------------------------------------

func queryQuoteSaverDeposit(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// extract parameters
	params, err := quoteParseParams(req.Data)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// validate required parameters
	for _, p := range []string{assetParam, amountParam} {
		if len(params[p]) == 0 {
			return quoteErrorResponse(fmt.Errorf("missing required parameter %s", p))
		}
	}

	// parse asset
	asset, err := common.NewAsset(params[assetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad asset: %w", err))
	}

	// parse amount
	amount, err := cosmos.ParseUint(params[amountParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad amount: %w", err))
	}

	// parse affiliate
	affiliate, affiliateMemo, affiliateBps, depositAmount, err := quoteHandleAffiliate(ctx, mgr, params, amount)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// create the memo
	memo := &SwapMemo{
		MemoBase: mem.MemoBase{
			TxType: TxSwap, // swap and add uses swap handler
			Asset:  asset.GetSyntheticAsset(),
		},
		SlipLimit:            sdk.ZeroUint(),
		AffiliateAddress:     common.Address(affiliateMemo),
		AffiliateBasisPoints: affiliateBps,
	}

	// use random destination address
	destination, err := types.GetRandomPubKey().GetAddress(common.THORChain)
	if err != nil {
		return nil, fmt.Errorf("failed to generate address: %w", err)
	}

	// create the message
	msg := &types.MsgSwap{
		Tx: common.Tx{
			ID:          common.BlankTxID,
			Chain:       asset.Chain,
			FromAddress: common.NoopAddress,
			ToAddress:   common.NoopAddress,
			Coins: []common.Coin{
				{
					Asset:  asset,
					Amount: depositAmount,
				},
			},
			Gas: []common.Coin{
				{
					Asset:  common.RuneAsset(),
					Amount: sdk.NewUint(1),
				},
			},
			Memo: memo.String(),
		},
		TargetAsset:          asset.GetSyntheticAsset(),
		TradeTarget:          sdk.ZeroUint(),
		AffiliateAddress:     affiliate,
		AffiliateBasisPoints: affiliateBps,
		Destination:          destination,
	}

	// get the swap result
	swapRes, _, _, err := quoteSimulateSwap(ctx, mgr, amount, msg)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to simulate swap: %w", err))
	}

	// generate deposit memo
	depositMemoComponents := []string{
		"+",
		asset.GetSyntheticAsset().String(),
		"",
		affiliateMemo,
		affiliateBps.String(),
	}
	depositMemo := strings.Join(depositMemoComponents[:2], ":")
	if affiliate != common.NoAddress && !affiliateBps.IsZero() {
		depositMemo = strings.Join(depositMemoComponents, ":")
	}

	// use the swap result info to generate the deposit quote
	res := &openapi.QuoteSaverDepositResponse{
		// TODO: deprecate ExpectedAmountOut in future version
		ExpectedAmountOut:          wrapString(swapRes.ExpectedAmountOut),
		ExpectedAmountDeposit:      swapRes.ExpectedAmountOut,
		Fees:                       swapRes.Fees,
		SlippageBps:                swapRes.SlippageBps,
		InboundConfirmationBlocks:  swapRes.InboundConfirmationBlocks,
		InboundConfirmationSeconds: swapRes.InboundConfirmationSeconds,
		Memo:                       depositMemo,
	}

	// estimate the inbound info
	inboundAddress, _, inboundConfirmations, err := quoteInboundInfo(ctx, mgr, amount, asset.GetLayer1Asset().Chain)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.InboundAddress = inboundAddress.String()
	res.InboundConfirmationBlocks = wrapInt64(inboundConfirmations)

	// set info fields
	chain := asset.GetLayer1Asset().Chain
	if !chain.DustThreshold().IsZero() {
		res.DustThreshold = wrapString(chain.DustThreshold().String())
		res.RecommendedMinAmountIn = res.DustThreshold
	}
	res.Notes = chain.InboundNotes()
	res.Warning = quoteWarning
	res.Expiry = time.Now().Add(quoteExpiration).Unix()

	return json.MarshalIndent(res, "", "  ")
}

// -------------------------------------------------------------------------------------
// Saver Withdraw
// -------------------------------------------------------------------------------------

func queryQuoteSaverWithdraw(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// extract parameters
	params, err := quoteParseParams(req.Data)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// validate required parameters
	for _, p := range []string{assetParam, addressParam, withdrawBasisPointsParam} {
		if len(params[p]) == 0 {
			return quoteErrorResponse(fmt.Errorf("missing required parameter %s", p))
		}
	}

	// parse asset
	asset, err := common.NewAsset(params[assetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad asset: %w", err))
	}
	asset = asset.GetSyntheticAsset() // always use the vault asset

	// parse address
	address, err := common.NewAddress(params[addressParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad address: %w", err))
	}

	// parse basis points
	basisPoints, err := cosmos.ParseUint(params[withdrawBasisPointsParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad basis points: %w", err))
	}

	// validate basis points
	if basisPoints.GT(sdk.NewUint(10_000)) {
		return quoteErrorResponse(fmt.Errorf("basis points must be less than 10000"))
	}

	// get liquidity provider
	lp, err := mgr.Keeper().GetLiquidityProvider(ctx, asset, address)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to get liquidity provider: %w", err))
	}

	// get the pool
	pool, err := mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to get pool: %w", err))
	}

	// get the liquidity provider share of the pool
	lpShare := common.GetSafeShare(lp.Units, pool.LPUnits, pool.BalanceAsset)

	// calculate the withdraw amount
	amount := common.GetSafeShare(basisPoints, sdk.NewUint(10_000), lpShare)

	// create the memo
	memo := &SwapMemo{
		MemoBase: mem.MemoBase{
			TxType: TxSwap,
			Asset:  asset,
		},
		SlipLimit: sdk.ZeroUint(),
	}

	// create the message
	msg := &types.MsgSwap{
		Tx: common.Tx{
			ID:          common.BlankTxID,
			Chain:       common.THORChain,
			FromAddress: common.NoopAddress,
			ToAddress:   common.NoopAddress,
			Coins: []common.Coin{
				{
					Asset:  asset,
					Amount: amount,
				},
			},
			Gas: []common.Coin{
				{
					Asset:  common.RuneAsset(),
					Amount: sdk.NewUint(1),
				},
			},
			Memo: memo.String(),
		},
		TargetAsset:          asset.GetLayer1Asset(),
		TradeTarget:          sdk.ZeroUint(),
		AffiliateAddress:     common.NoAddress,
		AffiliateBasisPoints: sdk.ZeroUint(),
		Destination:          address,
	}

	// get the swap result
	swapRes, emitAmount, outboundFeeAmount, err := quoteSimulateSwap(ctx, mgr, amount, msg)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to simulate swap: %w", err))
	}

	// check invariant
	if emitAmount.LT(outboundFeeAmount) {
		return quoteErrorResponse(fmt.Errorf("invariant broken: emit %s less than outbound fee %s", emitAmount, outboundFeeAmount))
	}

	// the amount out will deduct the outbound fee
	swapRes.Fees.Outbound = outboundFeeAmount.String()

	// use the swap result info to generate the withdraw quote
	res := &openapi.QuoteSaverWithdrawResponse{
		ExpectedAmountOut: emitAmount.Sub(outboundFeeAmount).String(),
		Fees:              swapRes.Fees,
		SlippageBps:       swapRes.SlippageBps,
		Memo:              fmt.Sprintf("-:%s:%s", asset.String(), basisPoints.String()),
		DustAmount:        asset.GetLayer1Asset().Chain.DustThreshold().Add(basisPoints).String(),
	}

	// estimate the inbound info
	inboundAddress, _, _, err := quoteInboundInfo(ctx, mgr, amount, asset.GetLayer1Asset().Chain)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.InboundAddress = inboundAddress.String()

	// estimate the outbound info
	outboundCoin := common.Coin{Asset: asset.GetLayer1Asset(), Amount: emitAmount}
	outboundDelay, err := quoteOutboundInfo(ctx, mgr, outboundCoin)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.OutboundDelayBlocks = outboundDelay
	res.OutboundDelaySeconds = outboundDelay * common.THORChain.ApproximateBlockMilliseconds() / 1000

	// set info fields
	chain := asset.GetLayer1Asset().Chain
	if !chain.DustThreshold().IsZero() {
		res.DustThreshold = wrapString(chain.DustThreshold().String())
	}
	res.Notes = chain.InboundNotes()
	res.Warning = quoteWarning
	res.Expiry = time.Now().Add(quoteExpiration).Unix()

	return json.MarshalIndent(res, "", "  ")
}

// -------------------------------------------------------------------------------------
// Loan Open
// -------------------------------------------------------------------------------------

func queryQuoteLoanOpen(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// extract parameters
	params, err := quoteParseParams(req.Data)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// validate required parameters
	for _, p := range []string{assetParam, amountParam, targetAssetParam} {
		if len(params[p]) == 0 {
			return quoteErrorResponse(fmt.Errorf("missing required parameter %s", p))
		}
	}

	// invalidate unexpected parameters
	allowed := map[string]bool{
		assetParam:        true,
		amountParam:       true,
		minOutParam:       true,
		targetAssetParam:  true,
		destinationParam:  true,
		affiliateParam:    true,
		affiliateBpsParam: true,
	}
	for p := range params {
		if !allowed[p] {
			return quoteErrorResponse(fmt.Errorf("unexpected parameter %s", p))
		}
	}

	// parse asset
	asset, err := common.NewAsset(params[assetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad asset: %w", err))
	}

	// parse amount
	amount, err := cosmos.ParseUint(params[amountParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad amount: %w", err))
	}

	// parse min out
	minOut := sdk.ZeroUint()
	if len(params[minOutParam]) > 0 {
		minOut, err = cosmos.ParseUint(params[minOutParam][0])
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad min out: %w", err))
		}
	}

	// parse affiliate
	affiliate, affiliateMemo, affiliateBps, _, err := quoteHandleAffiliate(ctx, mgr, params, amount)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// TODO: remove after affiliates work
	if !affiliate.IsEmpty() || len(params[affiliateBpsParam]) > 0 {
		return quoteErrorResponse(fmt.Errorf("affiliate not yet supported"))
	}

	// parse target asset
	targetAsset, err := common.NewAsset(params[targetAssetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad target asset: %w", err))
	}

	// parse destination address or generate a random one
	sendMemo := true
	var destination common.Address
	if len(params[destinationParam]) > 0 {
		destination, err = quoteParseAddress(ctx, mgr, params[destinationParam][0], targetAsset.Chain)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad destination address: %w", err))
		}

	} else {
		destination, err = types.GetRandomPubKey().GetAddress(targetAsset.Chain)
		if err != nil {
			return nil, fmt.Errorf("failed to generate address: %w", err)
		}
		sendMemo = false // do not send memo if destination was random
	}

	// check that destination and affiliate are not the same
	if destination.Equals(affiliate) {
		return quoteErrorResponse(fmt.Errorf("destination and affiliate should not be the same"))
	}

	// generate random adddress for collateral owner
	collateralOwner, err := types.GetRandomPubKey().GetAddress(asset.Chain)
	if err != nil {
		return nil, fmt.Errorf("failed to generate address: %w", err)
	}

	// create message for simulation
	msg := &types.MsgLoanOpen{
		Owner:                collateralOwner,
		CollateralAsset:      asset,
		CollateralAmount:     amount,
		TargetAddress:        destination,
		TargetAsset:          targetAsset,
		MinOut:               minOut,
		AffiliateAddress:     affiliate,
		AffiliateBasisPoints: affiliateBps,

		// TODO: support aggregator
		Aggregator:              "",
		AggregatorTargetAddress: "",
		AggregatorTargetLimit:   sdk.ZeroUint(),
	}

	// simulate message handling
	events, err := simulate(ctx, mgr, msg)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// create response
	res := &openapi.QuoteLoanOpenResponse{
		Fees: openapi.QuoteFees{
			Asset: targetAsset.String(),
		},
		Expiry:  time.Now().Add(quoteExpiration).Unix(),
		Warning: quoteWarning,
		Notes:   asset.Chain.InboundNotes(),
	}

	// estimate the inbound info
	inboundAddress, routerAddress, inboundConfirmations, err := quoteInboundInfo(ctx, mgr, amount, asset.Chain)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.InboundAddress = wrapString(inboundAddress.String())
	if inboundConfirmations > 0 {
		res.InboundConfirmationBlocks = wrapInt64(inboundConfirmations)
		res.InboundConfirmationSeconds = wrapInt64(inboundConfirmations * asset.Chain.ApproximateBlockMilliseconds() / 1000)
	}

	// set info fields
	if asset.Chain.IsEVM() {
		res.Router = wrapString(routerAddress.String())
	}
	if !asset.Chain.DustThreshold().IsZero() {
		res.DustThreshold = wrapString(asset.Chain.DustThreshold().String())
	}

	// sum liquidity fees in rune from all swap events
	outboundFee := sdk.ZeroUint()
	liquidityFee := sdk.ZeroUint()
	affiliateFee := sdk.ZeroUint()
	expectedAmountOut := sdk.ZeroUint()

	// iterate events in reverse order
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]

		switch e.Type {

		// use final outbound event as expected amount - scheduled_outbound (L1) or outbound (native)
		case "scheduled_outbound":
			if res.ExpectedAmountOut == "" { // if not empty we already saw the last outbound event
				for _, attr := range e.Attributes {
					switch string(attr.Key) {
					case "coin_amount":
						res.ExpectedAmountOut = string(attr.Value)
						expectedAmountOut = sdk.NewUintFromString(string(attr.Value))
					case "coin_asset":
						if string(attr.Value) != targetAsset.String() { // should be unreachable
							return quoteErrorResponse(fmt.Errorf("unexpected outbound asset: %s", string(attr.Value)))
						}
					}
				}

				// estimate the outbound info
				outboundDelay, err := quoteOutboundInfo(ctx, mgr, common.NewCoin(targetAsset, sdk.NewUintFromString(res.ExpectedAmountOut)))
				if err != nil {
					return quoteErrorResponse(err)
				}
				res.OutboundDelayBlocks = outboundDelay
				res.OutboundDelaySeconds = outboundDelay * common.THORChain.ApproximateBlockMilliseconds() / 1000
			}
		case "outbound":
			// track coin and to address
			var coin common.Coin
			var toAddress common.Address

			for _, attr := range e.Attributes {
				switch string(attr.Key) {
				case "coin":
					// parse coin string for the outbound amount
					coin, err = common.ParseCoin(string(attr.Value))
					if err != nil {
						return quoteErrorResponse(fmt.Errorf("failed to parse coin: %w", err))
					}
				case "to":
					// ignore errors since the field may be a module name
					toAddress, _ = common.NewAddress(string(attr.Value))
				}
			}

			// check for the outbound event
			if toAddress.Equals(destination) {
				res.ExpectedAmountOut = coin.Amount.String()
				expectedAmountOut = coin.Amount

				if !coin.Asset.Equals(targetAsset) { // should be unreachable
					return quoteErrorResponse(fmt.Errorf("unexpected outbound asset: %s", coin.Asset))
				}
			}

			// check for affiliate
			if !affiliate.IsEmpty() && toAddress.Equals(affiliate) {
				if !coin.Asset.Equals(common.RuneNative) { // should be unreachable
					return quoteErrorResponse(fmt.Errorf("unexpected affiliate outbound asset: %s", coin.Asset))
				}
				affiliateFee = affiliateFee.Add(coin.Amount)
			}

		// sum liquidity fee in rune for all swap events
		case "swap":
			for _, attr := range e.Attributes {
				if string(attr.Key) == "liquidity_fee_in_rune" {
					liquidityFee = liquidityFee.Add(sdk.NewUintFromString(string(attr.Value)))
				}
			}

		// extract loan data from loan open event
		case "loan_open":
			for _, attr := range e.Attributes {
				switch string(attr.Key) {
				case "collateralization_ratio":
					res.ExpectedCollateralizationRatio = string(attr.Value)
				case "collateral_up":
					res.ExpectedCollateralUp = string(attr.Value)
				case "debt_up":
					res.ExpectedDebtUp = string(attr.Value)
				}
			}

		// catch refund if there was an issue
		case "refund":
			for _, attr := range e.Attributes {
				if string(attr.Key) == "reason" {
					return quoteErrorResponse(fmt.Errorf("failed to simulate loan open: %s", string(attr.Value)))
				}
			}

		// set outbound fee from fee event
		case "fee":
			for _, attr := range e.Attributes {
				if string(attr.Key) == "coins" {
					coin, err := common.ParseCoin(string(attr.Value))
					if err != nil {
						return quoteErrorResponse(fmt.Errorf("failed to parse coin: %w", err))
					}
					res.Fees.Outbound = coin.Amount.String() // already in target asset
					res.Fees.Asset = coin.Asset.String()
					outboundFee = coin.Amount

					if !coin.Asset.Equals(targetAsset) { // should be unreachable
						return quoteErrorResponse(fmt.Errorf("unexpected fee asset: %s", coin.Asset))
					}
				}
			}
		}
	}

	// convert fees to target asset if it is not rune
	if !targetAsset.Equals(common.RuneNative) {
		targetPool, err := mgr.Keeper().GetPool(ctx, targetAsset)
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("failed to get pool: %w", err))
		}
		affiliateFee = targetPool.RuneValueInAsset(affiliateFee)
		liquidityFee = targetPool.RuneValueInAsset(liquidityFee)
	}

	// set fee info
	res.Fees.Liquidity = wrapString(liquidityFee.String())
	totalFees := liquidityFee.Add(outboundFee).Add(affiliateFee)
	res.Fees.TotalBps = wrapString(totalFees.MulUint64(10000).Quo(expectedAmountOut.Add(totalFees)).String())
	if !affiliateFee.IsZero() {
		res.Fees.Affiliate = wrapString(affiliateFee.String())
	}

	// generate memo
	if sendMemo {
		memo := &mem.LoanOpenMemo{
			MemoBase: mem.MemoBase{
				TxType: TxLoanOpen,
			},
			TargetAsset:          targetAsset,
			TargetAddress:        destination,
			MinOut:               minOut,
			AffiliateAddress:     common.Address(affiliateMemo),
			AffiliateBasisPoints: affiliateBps,
			DexTargetLimit:       sdk.ZeroUint(),
		}
		res.Memo = wrapString(memo.String())
	}

	minLoanOpenAmount, err := calculateMinSwapAmount(ctx, mgr, asset, targetAsset)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("Failed to calculate min amount in: %s", err.Error()))
	}
	res.RecommendedMinAmountIn = wrapString(minLoanOpenAmount.String())

	return json.MarshalIndent(res, "", "  ")
}

// -------------------------------------------------------------------------------------
// Loan Close
// -------------------------------------------------------------------------------------

func queryQuoteLoanClose(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// extract parameters
	params, err := quoteParseParams(req.Data)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// validate required parameters
	for _, p := range []string{assetParam, amountParam, loanAssetParam, loanOwnerParam} {
		if len(params[p]) == 0 {
			return quoteErrorResponse(fmt.Errorf("missing required parameter %s", p))
		}
	}

	// invalidate unexpected parameters
	allowed := map[string]bool{
		assetParam:     true,
		amountParam:    true,
		loanAssetParam: true,
		loanOwnerParam: true,
		minOutParam:    true,
	}
	for p := range params {
		if !allowed[p] {
			return quoteErrorResponse(fmt.Errorf("unexpected parameter %s", p))
		}
	}

	// parse asset
	asset, err := common.NewAsset(params[assetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad asset: %w", err))
	}

	// parse amount
	amount, err := cosmos.ParseUint(params[amountParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad amount: %w", err))
	}

	// parse min out
	minOut := sdk.ZeroUint()
	if len(params[minOutParam]) > 0 {
		minOut, err = cosmos.ParseUint(params[minOutParam][0])
		if err != nil {
			return quoteErrorResponse(fmt.Errorf("bad min out: %w", err))
		}
	}

	// parse loan asset
	loanAsset, err := common.NewAsset(params[loanAssetParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad loan asset: %w", err))
	}

	// parse loan owner
	loanOwner, err := common.NewAddress(params[loanOwnerParam][0])
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad loan owner: %w", err))
	}

	// generate random from address
	fromAddress, err := types.GetRandomPubKey().GetAddress(asset.Chain)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("bad from address: %w", err))
	}

	// create message for simulation
	msg := &types.MsgLoanRepayment{
		Owner:           loanOwner,
		CollateralAsset: loanAsset,
		Coin:            common.NewCoin(asset, amount),
		From:            fromAddress,
		MinOut:          minOut,
	}

	// simulate message handling
	events, err := simulate(ctx, mgr, msg)
	if err != nil {
		return quoteErrorResponse(err)
	}

	// create response
	res := &openapi.QuoteLoanCloseResponse{
		Fees: openapi.QuoteFees{
			Asset: loanAsset.String(),
		},
		Expiry:  time.Now().Add(quoteExpiration).Unix(),
		Warning: quoteWarning,
		Notes:   asset.Chain.InboundNotes(),
	}

	// estimate the inbound info
	inboundAddress, routerAddress, inboundConfirmations, err := quoteInboundInfo(ctx, mgr, amount, asset.Chain)
	if err != nil {
		return quoteErrorResponse(err)
	}
	res.InboundAddress = wrapString(inboundAddress.String())
	if inboundConfirmations > 0 {
		res.InboundConfirmationBlocks = wrapInt64(inboundConfirmations)
		res.InboundConfirmationSeconds = wrapInt64(inboundConfirmations * asset.Chain.ApproximateBlockMilliseconds() / 1000)
	}

	// set info fields
	if asset.Chain.IsEVM() {
		res.Router = wrapString(routerAddress.String())
	}
	if !asset.Chain.DustThreshold().IsZero() {
		res.DustThreshold = wrapString(asset.Chain.DustThreshold().String())
	}

	// sum liquidity fees in rune from all swap events
	outboundFee := sdk.ZeroUint()
	liquidityFee := sdk.ZeroUint()
	affiliateFee := sdk.ZeroUint()
	expectedAmountOut := sdk.ZeroUint()

	// iterate events in reverse order
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]

		switch e.Type {

		// use final outbound event as expected amount - scheduled_outbound (L1) or outbound (native)
		case "scheduled_outbound":
			if res.ExpectedAmountOut == "" { // if not empty we already saw the last outbound event
				for _, attr := range e.Attributes {
					switch string(attr.Key) {
					case "coin_amount":
						res.ExpectedAmountOut = string(attr.Value)
						expectedAmountOut = sdk.NewUintFromString(string(attr.Value))
					case "coin_asset":
						if string(attr.Value) != loanAsset.String() { // should be unreachable
							return quoteErrorResponse(fmt.Errorf("unexpected outbound asset: %s", string(attr.Value)))
						}
					}
				}

				// estimate the outbound info
				outboundDelay, err := quoteOutboundInfo(ctx, mgr, common.NewCoin(loanAsset, sdk.NewUintFromString(res.ExpectedAmountOut)))
				if err != nil {
					return quoteErrorResponse(err)
				}
				res.OutboundDelayBlocks = outboundDelay
				res.OutboundDelaySeconds = outboundDelay * common.THORChain.ApproximateBlockMilliseconds() / 1000
			}
		case "outbound":
			// track coin and to address
			var coin common.Coin
			var toAddress common.Address

			for _, attr := range e.Attributes {
				switch string(attr.Key) {
				case "coin":
					// parse coin string for the outbound amount
					coin, err = common.ParseCoin(string(attr.Value))
					if err != nil {
						return quoteErrorResponse(fmt.Errorf("failed to parse coin: %w", err))
					}
				case "to":
					// ignore errors since the field may be a module name
					toAddress, _ = common.NewAddress(string(attr.Value))
				}
			}

			// check for the outbound event
			if toAddress.Equals(loanOwner) {
				res.ExpectedAmountOut = coin.Amount.String()
				expectedAmountOut = coin.Amount

				if !coin.Asset.Equals(loanAsset) { // should be unreachable
					return quoteErrorResponse(fmt.Errorf("unexpected outbound asset: %s", coin.Asset))
				}
			}

		// sum liquidity fee in rune for all swap events
		case "swap":
			for _, attr := range e.Attributes {
				if string(attr.Key) == "liquidity_fee_in_rune" {
					liquidityFee = liquidityFee.Add(sdk.NewUintFromString(string(attr.Value)))
				}
			}

		// extract loan data from loan close event
		case "loan_repayment":
			for _, attr := range e.Attributes {
				switch string(attr.Key) {
				case "collateral_down":
					res.ExpectedCollateralDown = string(attr.Value)
				case "debt_down":
					res.ExpectedDebtDown = string(attr.Value)
				}
			}

		// catch refund if there was an issue
		case "refund":
			for _, attr := range e.Attributes {
				if string(attr.Key) == "reason" {
					return quoteErrorResponse(fmt.Errorf("failed to simulate loan close: %s", string(attr.Value)))
				}
			}

		// set outbound fee from fee event
		case "fee":
			for _, attr := range e.Attributes {
				if string(attr.Key) == "coins" {
					coin, err := common.ParseCoin(string(attr.Value))
					if err != nil {
						return quoteErrorResponse(fmt.Errorf("failed to parse coin: %w", err))
					}
					res.Fees.Outbound = coin.Amount.String() // already in collateral asset
					res.Fees.Asset = coin.Asset.String()
					outboundFee = coin.Amount

					if !coin.Asset.Equals(loanAsset) { // should be unreachable
						return quoteErrorResponse(fmt.Errorf("unexpected fee asset: %s", coin.Asset))
					}
				}
			}
		}
	}

	// convert fees to target asset if it is not rune
	loanPool, err := mgr.Keeper().GetPool(ctx, loanAsset)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("failed to get pool: %w", err))
	}
	affiliateFee = loanPool.RuneValueInAsset(affiliateFee)
	liquidityFee = loanPool.RuneValueInAsset(liquidityFee)

	// set fee info
	res.Fees.Liquidity = wrapString(liquidityFee.String())
	totalFees := liquidityFee.Add(outboundFee).Add(affiliateFee)
	if !expectedAmountOut.IsZero() {
		res.Fees.TotalBps = wrapString(totalFees.MulUint64(10000).Quo(expectedAmountOut).String())
	}
	if !affiliateFee.IsZero() {
		res.Fees.Affiliate = wrapString(affiliateFee.String())
	}

	// generate memo
	memo := &mem.LoanRepaymentMemo{
		MemoBase: mem.MemoBase{
			TxType: TxLoanRepayment,
			Asset:  loanAsset,
		},
		Owner:  loanOwner,
		MinOut: minOut,
	}
	res.Memo = memo.String()

	minLoanCloseAmount, err := calculateMinSwapAmount(ctx, mgr, asset, loanAsset)
	if err != nil {
		return quoteErrorResponse(fmt.Errorf("Failed to calculate min amount in: %s", err.Error()))
	}
	res.RecommendedMinAmountIn = wrapString(minLoanCloseAmount.String())

	return json.MarshalIndent(res, "", "  ")
}
