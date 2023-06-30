package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// LoanOpenHandler a handler to process bond
type LoanOpenHandler struct {
	mgr Manager
}

// NewLoanOpenHandler create new LoanOpenHandler
func NewLoanOpenHandler(mgr Manager) LoanOpenHandler {
	return LoanOpenHandler{
		mgr: mgr,
	}
}

// Run execute the handler
func (h LoanOpenHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgLoanOpen)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgLoanOpen",
		"owner", msg.Owner,
		"col_asset", msg.CollateralAsset,
		"col_amount", msg.CollateralAmount,
		"target_address", msg.TargetAddress,
		"target_asset", msg.TargetAsset)

	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg loan fail validation", "error", err)
		return nil, err
	}

	err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process msg loan", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h LoanOpenHandler) validate(ctx cosmos.Context, msg MsgLoanOpen) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.111.0")):
		return h.validateV111(ctx, msg)
	case version.GTE(semver.MustParse("1.108.0")):
		return h.validateV108(ctx, msg)
	case version.GTE(semver.MustParse("1.107.0")):
		return h.validateV107(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h LoanOpenHandler) validateV111(ctx cosmos.Context, msg MsgLoanOpen) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	pauseLoans := fetchConfigInt64(ctx, h.mgr, constants.PauseLoans)
	if pauseLoans > 0 {
		return fmt.Errorf("loans are currently paused")
	}

	// Circuit Breaker: check if we're hit the max supply
	supply := h.mgr.Keeper().GetTotalSupply(ctx, common.RuneAsset())
	maxAmt := fetchConfigInt64(ctx, h.mgr, constants.MaxRuneSupply)
	if maxAmt <= 0 {
		return fmt.Errorf("no max supply set")
	}
	if supply.GTE(cosmos.NewUint(uint64(maxAmt))) {
		return fmt.Errorf("loans are currently paused, due to rune supply cap (%d/%d)", supply.Uint64(), maxAmt)
	}

	// ensure collateral pool exists
	if !h.mgr.Keeper().PoolExist(ctx, msg.CollateralAsset) {
		return fmt.Errorf("collateral asset does not have a pool")
	}

	// The lending key for the ETH.ETH pool would be LENDING-THOR-ETH .
	key := "LENDING-" + msg.CollateralAsset.GetDerivedAsset().MimirString()
	val, err := h.mgr.Keeper().GetMimir(ctx, key)
	if err != nil {
		ctx.Logger().Error("fail to fetch LENDING key", "pool", msg.CollateralAsset.GetDerivedAsset().String(), "error", err)
		return err
	}
	if val <= 0 {
		return fmt.Errorf("Lending is not available for this collateral asset")
	}

	// convert collateral asset back to layer1 asset
	// NOTE: if the symbol of a derived asset isn't the chain, this won't work
	// (ie TERRA.LUNA)
	msg.CollateralAsset.Chain, err = common.NewChain(msg.CollateralAsset.Symbol.String())
	if err != nil {
		return err
	}

	totalCollateral, err := h.mgr.Keeper().GetTotalCollateral(ctx, msg.CollateralAsset)
	if err != nil {
		return err
	}
	totalRune, err := h.getTotalLiquidityRUNELoanPools(ctx)
	if err != nil {
		return err
	}
	if totalRune.IsZero() {
		return fmt.Errorf("no liquidity, lending unavailable")
	}
	lever := fetchConfigInt64(ctx, h.mgr, constants.LendingLever)
	runeBurnt := common.SafeSub(cosmos.NewUint(uint64(maxAmt)), supply)
	totalAvailableRuneForProtocol := common.GetSafeShare(cosmos.NewUint(uint64(lever)), cosmos.NewUint(10_000), runeBurnt) // calculate how much of that rune is available for loans
	if totalAvailableRuneForProtocol.IsZero() {
		return fmt.Errorf("no availability (0), lending unavailable")
	}
	pool, err := h.mgr.Keeper().GetPool(ctx, msg.CollateralAsset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return err
	}
	totalAvailableRuneForPool := common.GetSafeShare(pool.BalanceRune, totalRune, totalAvailableRuneForProtocol)
	totalAvailableAssetForPool := pool.RuneValueInAsset(totalAvailableRuneForPool)
	if totalCollateral.Add(msg.CollateralAmount).GT(totalAvailableAssetForPool) {
		return fmt.Errorf("no availability (%d/%d), lending unavailable", totalCollateral.Add(msg.CollateralAmount).Uint64(), totalAvailableAssetForPool.Uint64())
	}

	return nil
}

func (h LoanOpenHandler) handle(ctx cosmos.Context, msg MsgLoanOpen) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.113.0")):
		return h.handleV113(ctx, msg)
	case version.GTE(semver.MustParse("1.111.0")):
		return h.handleV111(ctx, msg)
	case version.GTE(semver.MustParse("1.108.0")):
		return h.handleV108(ctx, msg)
	case version.GTE(semver.MustParse("1.107.0")):
		return h.handleV107(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h LoanOpenHandler) openLoan(ctx cosmos.Context, msg MsgLoanOpen) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.113.0")):
		return h.openLoanV113(ctx, msg)
	case version.GTE(semver.MustParse("1.112.0")):
		return h.openLoanV112(ctx, msg)
	case version.GTE(semver.MustParse("1.111.0")):
		return h.openLoanV111(ctx, msg)
	case version.GTE(semver.MustParse("1.108.0")):
		return h.openLoanV108(ctx, msg)
	case version.GTE(semver.MustParse("1.107.0")):
		return h.openLoanV107(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h LoanOpenHandler) swap(ctx cosmos.Context, msg MsgLoanOpen) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.113.0")):
		return h.swapV113(ctx, msg)
	case version.GTE(semver.MustParse("1.108.0")):
		return h.swapV108(ctx, msg)
	case version.GTE(semver.MustParse("1.107.0")):
		return h.swapV107(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h LoanOpenHandler) handleV113(ctx cosmos.Context, msg MsgLoanOpen) error {
	// if the inbound asset is TOR, then lets repay the loan. If not, lets
	// swap first and try again later
	if msg.CollateralAsset.IsDerivedAsset() {
		return h.openLoan(ctx, msg)
	} else {
		return h.swap(ctx, msg)
	}
}

func (h LoanOpenHandler) openLoanV113(ctx cosmos.Context, msg MsgLoanOpen) error {
	var err error
	zero := cosmos.ZeroUint()

	// convert collateral asset back to layer1 asset
	// NOTE: if the symbol of a derived asset isn't the chain, this won't work
	// (ie TERRA.LUNA)
	msg.CollateralAsset.Chain, err = common.NewChain(msg.CollateralAsset.Symbol.String())
	if err != nil {
		return err
	}

	pool, err := h.mgr.Keeper().GetPool(ctx, msg.CollateralAsset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return err
	}
	loan, err := h.mgr.Keeper().GetLoan(ctx, msg.CollateralAsset, msg.Owner)
	if err != nil {
		ctx.Logger().Error("fail to get loan", "error", err)
		return err
	}
	totalCollateral, err := h.mgr.Keeper().GetTotalCollateral(ctx, msg.CollateralAsset)
	if err != nil {
		return err
	}
	totalRune, err := h.getTotalLiquidityRUNELoanPools(ctx)
	if err != nil {
		return err
	}
	if totalRune.IsZero() {
		return fmt.Errorf("no liquidity, lending unavailable")
	}

	// get configs
	minCR := fetchConfigInt64(ctx, h.mgr, constants.MinCR)
	maxCR := fetchConfigInt64(ctx, h.mgr, constants.MaxCR)
	lever := fetchConfigInt64(ctx, h.mgr, constants.LendingLever)
	enableDerived := fetchConfigInt64(ctx, h.mgr, constants.EnableDerivedAssets)

	// calculate CR
	currentRuneSupply := h.mgr.Keeper().GetTotalSupply(ctx, common.RuneAsset())
	maxRuneSupply := fetchConfigInt64(ctx, h.mgr, constants.MaxRuneSupply)
	if maxRuneSupply <= 0 {
		return fmt.Errorf("no max supply set")
	}
	runeBurnt := common.SafeSub(cosmos.NewUint(uint64(maxRuneSupply)), currentRuneSupply)
	totalAvailableRuneForProtocol := common.GetSafeShare(cosmos.NewUint(uint64(lever)), cosmos.NewUint(10_000), runeBurnt) // calculate how much of that rune is available for loans
	if totalAvailableRuneForProtocol.IsZero() {
		return fmt.Errorf("no availability (0), lending unavailable")
	}
	totalAvailableRuneForPool := common.GetSafeShare(pool.BalanceRune, totalRune, totalAvailableRuneForProtocol)
	totalAvailableAssetForPool := pool.RuneValueInAsset(totalAvailableRuneForPool)
	if totalCollateral.Add(msg.CollateralAmount).GT(totalAvailableAssetForPool) {
		return fmt.Errorf("no availability (%d/%d), lending unavailable", totalCollateral.Add(msg.CollateralAmount).Uint64(), totalAvailableAssetForPool.Uint64())
	}
	cr := h.getCR(totalCollateral.Add(msg.CollateralAmount), totalAvailableAssetForPool, minCR, maxCR)

	price := h.mgr.Keeper().DollarsPerRune(ctx)
	if price.IsZero() {
		return fmt.Errorf("TOR price cannot be zero")
	}

	collateralValueInRune := pool.AssetValueInRune(msg.CollateralAmount)
	collateralValueInTOR := collateralValueInRune.Mul(price).QuoUint64(1e8)
	debt := collateralValueInTOR.Quo(cr).MulUint64(10_000)
	ctx.Logger().Info("Loan Details", "collateral", common.NewCoin(msg.CollateralAsset, msg.CollateralAmount), "debt", debt.Uint64(), "rune price", price.Uint64(), "colRune", collateralValueInRune.Uint64(), "colTOR", collateralValueInTOR.Uint64())

	// sanity checks
	if debt.IsZero() {
		return fmt.Errorf("debt cannot be zero")
	}

	// if the user has over-repayed the loan, credit the difference on the next open
	cumulativeDebt := debt
	if loan.DebtDown.GT(loan.DebtUp) {
		cumulativeDebt = cumulativeDebt.Add(loan.DebtDown.Sub(loan.DebtUp))
	}

	// update Loan record
	loan.DebtUp = loan.DebtUp.Add(cumulativeDebt)
	loan.CollateralUp = loan.CollateralUp.Add(msg.CollateralAmount)
	loan.LastOpenHeight = ctx.BlockHeight()

	if msg.TargetAsset.Equals(common.TOR) && enableDerived > 0 {
		toi := TxOutItem{
			Chain:      msg.TargetAsset.GetChain(),
			ToAddress:  msg.TargetAddress,
			Coin:       common.NewCoin(common.TOR, cumulativeDebt),
			ModuleName: ModuleName,
		}
		ok, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi, zero)
		if err != nil {
			return err
		}
		if !ok {
			return errFailAddOutboundTx
		}
	} else {
		txID, ok := ctx.Value(constants.CtxLoanTxID).(common.TxID)
		if !ok {
			return fmt.Errorf("fail to get txid")
		}

		torCoin := common.NewCoin(common.TOR, cumulativeDebt)

		if err := h.mgr.Keeper().MintToModule(ctx, ModuleName, torCoin); err != nil {
			return fmt.Errorf("fail to mint loan tor debt: %w", err)
		}
		mintEvt := NewEventMintBurn(MintSupplyType, torCoin.Asset.Native(), torCoin.Amount, "swap")
		if err := h.mgr.EventMgr().EmitEvent(ctx, mintEvt); err != nil {
			ctx.Logger().Error("fail to emit mint event", "error", err)
		}

		if err := h.mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(torCoin)); err != nil {
			return fmt.Errorf("fail to send TOR vault funds: %w", err)
		}

		lendingAddr, err := h.mgr.Keeper().GetModuleAddress(LendingName)
		if err != nil {
			ctx.Logger().Error("fail to get lending address", "error", err)
			return err
		}
		asgardAddr, err := h.mgr.Keeper().GetModuleAddress(AsgardName)
		if err != nil {
			ctx.Logger().Error("fail to get asgard address", "error", err)
			return err
		}

		// As this is to be a swap from TOR which has been sent to AsgardName, the ToAddress should be AsgardName's address.
		tx := common.NewTx(txID, lendingAddr, asgardAddr, common.NewCoins(torCoin), nil, "noop")
		// we do NOT pass affiliate info here as it was already taken out on the swap of the collateral to derived asset
		swapMsg := NewMsgSwap(tx, msg.TargetAsset, msg.TargetAddress, msg.MinOut, common.NoAddress, zero, msg.Aggregator, msg.AggregatorTargetAddress, &msg.AggregatorTargetLimit, 0, msg.Signer)
		handler := NewSwapHandler(h.mgr)
		if _, err := handler.Run(ctx, swapMsg); err != nil {
			ctx.Logger().Error("fail to make second swap when opening a loan", "error", err)
			return err
		}
	}

	// update kvstore
	h.mgr.Keeper().SetLoan(ctx, loan)
	h.mgr.Keeper().SetTotalCollateral(ctx, msg.CollateralAsset, totalCollateral.Add(msg.CollateralAmount))

	// emit events and metrics
	evt := NewEventLoanOpen(msg.CollateralAmount, cr, debt, msg.CollateralAsset, msg.TargetAsset, msg.Owner)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); nil != err {
		ctx.Logger().Error("fail to emit loan open event", "error", err)
	}

	return nil
}

func (h LoanOpenHandler) getCR(a, b cosmos.Uint, minCR, maxCR int64) cosmos.Uint {
	// (maxCR - minCR) / (b / a) + minCR
	// NOTE: a should include the collateral currently being deposited
	crCalc := cosmos.NewUint(uint64(maxCR - minCR))
	cr := common.GetUncappedShare(a, b, crCalc)
	return cr.AddUint64(uint64(minCR))
}

func (h LoanOpenHandler) swapV113(ctx cosmos.Context, msg MsgLoanOpen) error {
	lendAddr, err := h.mgr.Keeper().GetModuleAddress(LendingName)
	if err != nil {
		ctx.Logger().Error("fail to get lending address", "error", err)
		return err
	}

	txID, ok := ctx.Value(constants.CtxLoanTxID).(common.TxID)
	if !ok {
		return fmt.Errorf("fail to get txid")
	}
	// ensure TxID does NOT have a collision with another swap, this could
	// happen if the user submits two identical loan requests in the same
	// block
	if ok := h.mgr.Keeper().HasSwapQueueItem(ctx, txID, 0); ok {
		return fmt.Errorf("txn hash conflict")
	}

	toAddress, ok := ctx.Value(constants.CtxLoanToAddress).(common.Address)
	// An empty ToAddress fails Tx validation,
	// and a querier quote or unit test has no provided ToAddress.
	// As this only affects emitted swap event contents, do not return an error.
	if !ok || toAddress.IsEmpty() {
		toAddress = "no to address available"
	}

	memo := fmt.Sprintf("loan+:%s:%s:%d:%s:%d:%s:%s:%d", msg.TargetAsset, msg.TargetAddress, msg.MinOut.Uint64(), msg.AffiliateAddress, msg.AffiliateBasisPoints.Uint64(), msg.Aggregator, msg.AggregatorTargetAddress, msg.AggregatorTargetLimit.Uint64())
	fakeGas := common.NewCoin(msg.CollateralAsset.GetChain().GetGasAsset(), cosmos.OneUint())

	tx := common.NewTx(txID, msg.Owner, toAddress, common.NewCoins(common.NewCoin(msg.CollateralAsset, msg.CollateralAmount)), common.Gas{fakeGas}, memo)
	swapMsg := NewMsgSwap(tx, msg.CollateralAsset.GetDerivedAsset(), lendAddr, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, 0, msg.Signer)
	if err := h.mgr.Keeper().SetSwapQueueItem(ctx, *swapMsg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
		return err
	}

	// TODO: send affiliate fee

	return nil
}

func (h LoanOpenHandler) getTotalLiquidityRUNELoanPools(ctx cosmos.Context) (cosmos.Uint, error) {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.108.0")):
		return h.getTotalLiquidityRUNELoanPoolsV108(ctx)
	case version.GTE(semver.MustParse("1.107.0")):
		return h.getTotalLiquidityRUNELoanPoolsV107(ctx)
	default:
		return cosmos.ZeroUint(), errBadVersion
	}
}

// getTotalLiquidityRUNE we have in all pools
func (h LoanOpenHandler) getTotalLiquidityRUNELoanPoolsV108(ctx cosmos.Context) (cosmos.Uint, error) {
	pools, err := h.mgr.Keeper().GetPools(ctx)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get pools from data store: %w", err)
	}
	total := cosmos.ZeroUint()
	for _, p := range pools {
		// ignore suspended pools
		if p.Status == PoolSuspended {
			continue
		}
		if p.Asset.IsVaultAsset() {
			continue
		}
		if p.Asset.IsDerivedAsset() {
			continue
		}

		key := "LENDING-" + p.Asset.GetDerivedAsset().MimirString()
		val, err := h.mgr.Keeper().GetMimir(ctx, key)
		if err != nil {
			continue
		}
		if val <= 0 {
			continue
		}
		total = total.Add(p.BalanceRune)
	}
	return total, nil
}
