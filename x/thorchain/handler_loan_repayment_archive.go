package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h LoanRepaymentHandler) validateV110(ctx cosmos.Context, msg MsgLoanRepayment) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// if a swap is required (ie the coin is NOT TOR), then ensure that the
	// from address passed in the msg matches the chain of the coin.Asset. This
	// is a sanity check that msg.From is not a btc address, while coin.Asset
	// is ETH.ETH
	if !msg.Coin.Asset.Equals(common.TOR) && !msg.From.IsChain(msg.Coin.Asset.GetChain()) {
		return fmt.Errorf("address %s does not match input coin %s", msg.From.String(), msg.Coin.String())
	}

	pauseLoans := fetchConfigInt64(ctx, h.mgr, constants.PauseLoans)
	if pauseLoans > 0 {
		return fmt.Errorf("loans are currently paused")
	}

	if !h.mgr.Keeper().PoolExist(ctx, msg.CollateralAsset) {
		ctx.Logger().Error("pool does not exist", "asset", msg.CollateralAsset)
		return fmt.Errorf("pool does not exist")
	}

	loan, err := h.mgr.Keeper().GetLoan(ctx, msg.CollateralAsset, msg.Owner)
	if err != nil {
		ctx.Logger().Error("fail to get loan", "error", err)
		return err
	}

	if loan.Debt().IsZero() {
		return fmt.Errorf("loan contains no debt to pay off")
	}

	maturity := fetchConfigInt64(ctx, h.mgr, constants.LoanRepaymentMaturity)
	if loan.LastOpenHeight+maturity > ctx.BlockHeight() {
		return fmt.Errorf("loan repayment is unavailable: loan hasn't reached maturity")
	}

	return nil
}

func (h LoanRepaymentHandler) validateV107(ctx cosmos.Context, msg MsgLoanRepayment) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	pauseLoans := fetchConfigInt64(ctx, h.mgr, constants.PauseLoans)
	if pauseLoans > 0 {
		return fmt.Errorf("loans are currently paused")
	}

	if !h.mgr.Keeper().PoolExist(ctx, msg.CollateralAsset) {
		ctx.Logger().Error("pool does not exist", "asset", msg.CollateralAsset)
		return fmt.Errorf("pool does not exist")
	}

	loan, err := h.mgr.Keeper().GetLoan(ctx, msg.CollateralAsset, msg.Owner)
	if err != nil {
		ctx.Logger().Error("fail to get loan", "error", err)
		return err
	}

	if loan.Debt().IsZero() {
		return fmt.Errorf("loan contains no debt to pay off")
	}

	maturity := fetchConfigInt64(ctx, h.mgr, constants.LoanRepaymentMaturity)
	if loan.LastOpenHeight+maturity > ctx.BlockHeight() {
		return fmt.Errorf("loan repayment is unavailable: loan hasn't reached maturity")
	}

	return nil
}

func (h LoanRepaymentHandler) handleV111(ctx cosmos.Context, msg MsgLoanRepayment) error {
	// inject txid into the context if unset
	var err error
	ctx, err = storeContextTxID(ctx, constants.CtxLoanTxID)
	if err != nil {
		return err
	}

	// if the inbound asset is TOR, then lets repay the loan. If not, lets
	// swap first and try again later
	if msg.Coin.Asset.Equals(common.TOR) {
		return h.repay(ctx, msg)
	} else {
		return h.swap(ctx, msg)
	}
}

func (h LoanRepaymentHandler) handleV110(ctx cosmos.Context, msg MsgLoanRepayment) error {
	// inject txid into the context if unset
	var err error
	ctx, err = storeContextTxID(ctx, constants.CtxLoanTxID)
	if err != nil {
		return err
	}

	// if the inbound asset is TOR, then lets repay the loan. If not, lets
	// swap first and try again later
	if msg.Coin.Asset.Equals(common.TOR) {
		return h.repayV110(ctx, msg)
	} else {
		return h.swapV110(ctx, msg)
	}
}

func (h LoanRepaymentHandler) repayV111(ctx cosmos.Context, msg MsgLoanRepayment) error {
	// collect data
	lendAddr, err := h.mgr.Keeper().GetModuleAddress(LendingName)
	if err != nil {
		ctx.Logger().Error("fail to get lending address", "error", err)
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

	// update Loan record
	loan.DebtDown = loan.DebtDown.Add(msg.Coin.Amount)
	loan.LastRepayHeight = ctx.BlockHeight()

	// burn TOR coins
	if err := h.mgr.Keeper().SendFromModuleToModule(ctx, LendingName, ModuleName, common.NewCoins(msg.Coin)); err != nil {
		ctx.Logger().Error("fail to move coins during loan repayment", "error", err)
		return err
	} else {
		err := h.mgr.Keeper().BurnFromModule(ctx, ModuleName, msg.Coin)
		if err != nil {
			ctx.Logger().Error("fail to burn coins during loan repayment", "error", err)
			return err
		}
		burnEvt := NewEventMintBurn(BurnSupplyType, msg.Coin.Asset.Native(), msg.Coin.Amount, "loan_repayment")
		if err := h.mgr.EventMgr().EmitEvent(ctx, burnEvt); err != nil {
			ctx.Logger().Error("fail to emit burn event", "error", err)
		}
	}

	// loan must be fully repaid to return collateral
	if !loan.Debt().IsZero() {
		h.mgr.Keeper().SetLoan(ctx, loan)

		// emit events and metrics
		evt := NewEventLoanRepayment(cosmos.ZeroUint(), msg.Coin.Amount, msg.CollateralAsset, msg.Owner)
		if err := h.mgr.EventMgr().EmitEvent(ctx, evt); nil != err {
			ctx.Logger().Error("fail to emit repayment open event", "error", err)
		}

		return nil
	}

	redeem := loan.Collateral()
	// only return collateral when collateral is non-zero
	if redeem.IsZero() {
		return nil
	}

	loan.CollateralDown = loan.CollateralDown.Add(redeem)

	txID, ok := ctx.Value(constants.CtxLoanTxID).(common.TxID)
	if !ok {
		return fmt.Errorf("fail to get txid")
	}

	coins := common.NewCoins(common.NewCoin(msg.CollateralAsset.GetDerivedAsset(), redeem))

	// transfer derived asset from the lending to asgard before swap to L1 collateral
	err = h.mgr.Keeper().SendFromModuleToModule(ctx, LendingName, AsgardName, coins)
	if err != nil {
		ctx.Logger().Error("fail to send from lending to asgard", "error", err)
		return err
	}

	fakeGas := common.NewCoin(msg.Coin.Asset, cosmos.OneUint())
	tx := common.NewTx(txID, lendAddr, lendAddr, coins, common.Gas{fakeGas}, "noop")
	swapMsg := NewMsgSwap(tx, msg.CollateralAsset, msg.Owner, msg.MinOut, common.NoAddress, cosmos.ZeroUint(), "", "", nil, 0, msg.Signer)
	handler := NewSwapHandler(h.mgr)
	if _, err := handler.Run(ctx, swapMsg); err != nil {
		ctx.Logger().Error("fail to make second swap when closing a loan", "error", err)
		return err
	}

	// update kvstore
	h.mgr.Keeper().SetLoan(ctx, loan)
	h.mgr.Keeper().SetTotalCollateral(ctx, msg.CollateralAsset, common.SafeSub(totalCollateral, redeem))

	// emit events and metrics
	evt := NewEventLoanRepayment(redeem, msg.Coin.Amount, msg.CollateralAsset, msg.Owner)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); nil != err {
		ctx.Logger().Error("fail to emit loan repayment event", "error", err)
	}
	return nil
}

func (h LoanRepaymentHandler) repayV110(ctx cosmos.Context, msg MsgLoanRepayment) error {
	// collect data
	lendAddr, err := h.mgr.Keeper().GetModuleAddress(LendingName)
	if err != nil {
		ctx.Logger().Error("fail to get lending address", "error", err)
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

	redeem := common.GetSafeShare(msg.Coin.Amount, loan.Debt(), loan.Collateral())
	if redeem.IsZero() {
		return fmt.Errorf("redeem cannot be zero")
	}

	// update Loan record
	loan.DebtDown = loan.DebtDown.Add(msg.Coin.Amount)
	loan.CollateralDown = loan.CollateralDown.Add(redeem)
	loan.LastRepayHeight = ctx.BlockHeight()

	// burn TOR coins
	if err := h.mgr.Keeper().SendFromModuleToModule(ctx, LendingName, ModuleName, common.NewCoins(msg.Coin)); err != nil {
		ctx.Logger().Error("fail to move coins during loan repayment", "error", err)
		return err
	} else {
		err := h.mgr.Keeper().BurnFromModule(ctx, ModuleName, msg.Coin)
		if err != nil {
			ctx.Logger().Error("fail to burn coins during loan repayment", "error", err)
			return err
		}
	}

	txID, ok := ctx.Value(constants.CtxLoanTxID).(common.TxID)
	if !ok {
		return fmt.Errorf("fail to get txid")
	}

	coins := common.NewCoins(common.NewCoin(msg.CollateralAsset.GetDerivedAsset(), redeem))

	// transfer derived asset from the lending to asgard before swap to L1 collateral
	err = h.mgr.Keeper().SendFromModuleToModule(ctx, LendingName, AsgardName, coins)
	if err != nil {
		ctx.Logger().Error("fail to send from lending to asgard", "error", err)
		return err
	}

	fakeGas := common.NewCoin(msg.Coin.Asset, cosmos.OneUint())
	tx := common.NewTx(txID, lendAddr, lendAddr, coins, common.Gas{fakeGas}, "noop")
	swapMsg := NewMsgSwap(tx, msg.CollateralAsset, msg.Owner, msg.MinOut, common.NoAddress, cosmos.ZeroUint(), "", "", nil, 0, msg.Signer)
	handler := NewSwapHandler(h.mgr)
	if _, err := handler.Run(ctx, swapMsg); err != nil {
		ctx.Logger().Error("fail to make second swap when closing a loan", "error", err)
		return err
	}

	// update kvstore
	h.mgr.Keeper().SetLoan(ctx, loan)
	h.mgr.Keeper().SetTotalCollateral(ctx, msg.CollateralAsset, common.SafeSub(totalCollateral, redeem))

	// emit events and metrics
	evt := NewEventLoanRepayment(redeem, msg.Coin.Amount, msg.CollateralAsset, msg.Owner)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); nil != err {
		ctx.Logger().Error("fail to emit loan open event", "error", err)
	}

	return nil
}

func (h LoanRepaymentHandler) handleV108(ctx cosmos.Context, msg MsgLoanRepayment) error {
	// inject txid into the context if unset
	var err error
	ctx, err = storeContextTxID(ctx, constants.CtxLoanTxID)
	if err != nil {
		return err
	}

	// if the inbound asset is TOR, then lets repay the loan. If not, lets
	// swap first and try again later
	if msg.Coin.Asset.Equals(common.TOR) {
		return h.repayV108(ctx, msg)
	} else {
		return h.swapV108(ctx, msg)
	}
}

func (h LoanRepaymentHandler) repayV108(ctx cosmos.Context, msg MsgLoanRepayment) error {
	// collect data
	lendAddr, err := h.mgr.Keeper().GetModuleAddress(LendingName)
	if err != nil {
		ctx.Logger().Error("fail to get lending address", "error", err)
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

	redeem := common.GetSafeShare(msg.Coin.Amount, loan.Debt(), loan.Collateral())
	if redeem.IsZero() {
		return fmt.Errorf("redeem cannot be zero")
	}

	// update Loan record
	loan.DebtDown = loan.DebtDown.Add(msg.Coin.Amount)
	loan.CollateralDown = loan.CollateralDown.Add(redeem)
	loan.LastRepayHeight = ctx.BlockHeight()

	// burn TOR coins
	if err := h.mgr.Keeper().SendFromModuleToModule(ctx, LendingName, ModuleName, common.NewCoins(msg.Coin)); err != nil {
		ctx.Logger().Error("fail to move coins during loan repayment", "error", err)
		return err
	} else {
		err := h.mgr.Keeper().BurnFromModule(ctx, ModuleName, msg.Coin)
		if err != nil {
			ctx.Logger().Error("fail to burn coins during loan repayment", "error", err)
			return err
		}
	}

	txID, ok := ctx.Value(constants.CtxLoanTxID).(common.TxID)
	if !ok {
		return fmt.Errorf("fail to get txid")
	}

	coins := common.NewCoins(common.NewCoin(msg.CollateralAsset.GetDerivedAsset(), redeem))

	// transfer derived asset from the lending to asgard before swap to L1 collateral
	err = h.mgr.Keeper().SendFromModuleToModule(ctx, LendingName, AsgardName, coins)
	if err != nil {
		ctx.Logger().Error("fail to send from lending to asgard", "error", err)
		return err
	}

	fakeGas := common.NewCoin(msg.Coin.Asset, cosmos.OneUint())
	tx := common.NewTx(txID, lendAddr, lendAddr, coins, common.Gas{fakeGas}, "noop")
	swapMsg := NewMsgSwap(tx, msg.CollateralAsset, msg.Owner, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, 0, msg.Signer)
	handler := NewSwapHandler(h.mgr)
	if _, err := handler.Run(ctx, swapMsg); err != nil {
		ctx.Logger().Error("fail to make second swap when closing a loan", "error", err)
		return err
	}

	// update kvstore
	h.mgr.Keeper().SetLoan(ctx, loan)
	h.mgr.Keeper().SetTotalCollateral(ctx, msg.CollateralAsset, common.SafeSub(totalCollateral, redeem))

	// emit events and metrics
	evt := NewEventLoanRepayment(redeem, msg.Coin.Amount, msg.CollateralAsset, msg.Owner)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); nil != err {
		ctx.Logger().Error("fail to emit loan open event", "error", err)
	}

	return nil
}

func (h LoanRepaymentHandler) swapV110(ctx cosmos.Context, msg MsgLoanRepayment) error {
	lendAddr, err := h.mgr.Keeper().GetModuleAddress(LendingName)
	if err != nil {
		ctx.Logger().Error("fail to get lending address", "error", err)
		return err
	}

	txID, ok := ctx.Value(constants.CtxLoanTxID).(common.TxID)
	if !ok {
		return fmt.Errorf("fail to get txid")
	}

	memo := fmt.Sprintf("loan-:%s:%s:%s", msg.CollateralAsset, msg.Owner, msg.MinOut)
	fakeGas := common.NewCoin(msg.Coin.Asset, cosmos.OneUint())
	tx := common.NewTx(txID, msg.From, lendAddr, common.NewCoins(msg.Coin), common.Gas{fakeGas}, memo)
	swapMsg := NewMsgSwap(tx, common.TOR, lendAddr, cosmos.ZeroUint(), lendAddr, cosmos.ZeroUint(), "", "", nil, 0, msg.Signer)
	if err := h.mgr.Keeper().SetSwapQueueItem(ctx, *swapMsg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
		return err
	}

	return nil
}

func (h LoanRepaymentHandler) swapV108(ctx cosmos.Context, msg MsgLoanRepayment) error {
	lendAddr, err := h.mgr.Keeper().GetModuleAddress(LendingName)
	if err != nil {
		ctx.Logger().Error("fail to get lending address", "error", err)
		return err
	}

	txID, ok := ctx.Value(constants.CtxLoanTxID).(common.TxID)
	if !ok {
		return fmt.Errorf("fail to get txid")
	}

	memo := fmt.Sprintf("loan-:%s:%s", msg.CollateralAsset, msg.Owner)
	fakeGas := common.NewCoin(msg.Coin.Asset, cosmos.OneUint())
	tx := common.NewTx(txID, lendAddr, lendAddr, common.NewCoins(msg.Coin), common.Gas{fakeGas}, memo)
	swapMsg := NewMsgSwap(tx, common.TOR, lendAddr, cosmos.ZeroUint(), lendAddr, cosmos.ZeroUint(), "", "", nil, 0, msg.Signer)
	if err := h.mgr.Keeper().SetSwapQueueItem(ctx, *swapMsg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
		return err
	}

	return nil
}

func (h LoanRepaymentHandler) handleV107(ctx cosmos.Context, msg MsgLoanRepayment) error {
	// inject txid into the context if unset
	var err error
	ctx, err = storeContextTxID(ctx, constants.CtxLoanTxID)
	if err != nil {
		return err
	}

	// if the inbound asset is TOR, then lets repay the loan. If not, lets
	// swap first and try again later
	if msg.Coin.Asset.Equals(common.TOR) {
		return h.repayV107(ctx, msg)
	} else {
		return h.swapV107(ctx, msg)
	}
}

func (h LoanRepaymentHandler) repayV107(ctx cosmos.Context, msg MsgLoanRepayment) error {
	// collect data
	lendAddr, err := h.mgr.Keeper().GetModuleAddress(LendingName)
	if err != nil {
		ctx.Logger().Error("fail to get lending address", "error", err)
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

	redeem := common.GetSafeShare(msg.Coin.Amount, loan.Debt(), loan.Collateral())
	if redeem.IsZero() {
		return fmt.Errorf("redeem cannot be zero")
	}

	// update Loan record
	loan.DebtDown = loan.DebtDown.Add(msg.Coin.Amount)
	loan.CollateralDown = loan.CollateralDown.Add(redeem)
	loan.LastRepayHeight = ctx.BlockHeight()

	// burn TOR coins
	if err := h.mgr.Keeper().SendFromModuleToModule(ctx, LendingName, ModuleName, common.NewCoins(msg.Coin)); err != nil {
		ctx.Logger().Error("fail to move coins during loan repayment", "error", err)
		return err
	} else {
		err := h.mgr.Keeper().BurnFromModule(ctx, ModuleName, msg.Coin)
		if err != nil {
			ctx.Logger().Error("fail to burn coins during loan repayment", "error", err)
			return err
		}
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

	coins := common.NewCoins(common.NewCoin(msg.CollateralAsset.GetDerivedAsset(), redeem))

	// transfer derived asset from the lending to asgard before swap to L1 collateral
	err = h.mgr.Keeper().SendFromModuleToModule(ctx, LendingName, AsgardName, coins)
	if err != nil {
		ctx.Logger().Error("fail to send from lending to asgard", "error", err)
		return err
	}

	fakeGas := common.NewCoin(msg.Coin.Asset, cosmos.OneUint())
	tx := common.NewTx(txID, lendAddr, lendAddr, coins, common.Gas{fakeGas}, "noop")
	swapMsg := NewMsgSwap(tx, msg.CollateralAsset, msg.Owner, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", nil, 0, msg.Signer)
	if err := h.mgr.Keeper().SetSwapQueueItem(ctx, *swapMsg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
		return err
	}

	// update kvstore
	h.mgr.Keeper().SetLoan(ctx, loan)
	h.mgr.Keeper().SetTotalCollateral(ctx, msg.CollateralAsset, common.SafeSub(totalCollateral, redeem))

	// emit events and metrics
	evt := NewEventLoanRepayment(redeem, msg.Coin.Amount, msg.CollateralAsset, msg.Owner)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); nil != err {
		ctx.Logger().Error("fail to emit loan open event", "error", err)
	}

	return nil
}

func (h LoanRepaymentHandler) swapV107(ctx cosmos.Context, msg MsgLoanRepayment) error {
	lendAddr, err := h.mgr.Keeper().GetModuleAddress(LendingName)
	if err != nil {
		ctx.Logger().Error("fail to get lending address", "error", err)
		return err
	}

	// the first swap has a reversed txid
	txID, ok := ctx.Value(constants.CtxLoanTxID).(common.TxID)
	if !ok {
		return fmt.Errorf("fail to get txid")
	}
	txID = txID.Reverse()

	// ensure TxID does NOT have a collision with another swap, this could
	// happen if the user submits two identical loan requests in the same
	// block
	if ok := h.mgr.Keeper().HasSwapQueueItem(ctx, txID, 0); ok {
		return fmt.Errorf("txn hash conflict")
	}

	memo := fmt.Sprintf("loan-:%s:%s", msg.CollateralAsset, msg.Owner)
	fakeGas := common.NewCoin(msg.Coin.Asset, cosmos.OneUint())
	tx := common.NewTx(txID, lendAddr, lendAddr, common.NewCoins(msg.Coin), common.Gas{fakeGas}, memo)
	swapMsg := NewMsgSwap(tx, common.TOR, lendAddr, cosmos.ZeroUint(), lendAddr, cosmos.ZeroUint(), "", "", nil, 0, msg.Signer)
	if err := h.mgr.Keeper().SetSwapQueueItem(ctx, *swapMsg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
		return err
	}

	return nil
}
