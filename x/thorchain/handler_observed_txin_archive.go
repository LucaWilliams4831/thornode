package thorchain

import (
	se "github.com/cosmos/cosmos-sdk/types/errors"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h ObservedTxInHandler) handleV112(ctx cosmos.Context, msg MsgObservedTxIn) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}
	handler := NewInternalHandler(h.mgr)
	for _, tx := range msg.Txs {
		// check we are sending to a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", "observed pub key", tx.ObservedPubKey)
			continue
		}

		voter, err := h.mgr.Keeper().GetObservedTxInVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx in voter", "error", err)
			continue
		}

		voter, ok := h.preflightV1(ctx, voter, activeNodeAccounts, tx, msg.Signer)
		if !ok {
			if voter.Height == ctx.BlockHeight() || voter.FinalisedHeight == ctx.BlockHeight() {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}

		// all logic after this  is after consensus

		ctx.Logger().Info("handleMsgObservedTxIn request", "Tx:", tx.String())
		if voter.Reverted {
			ctx.Logger().Info("tx had been reverted", "Tx", tx.String())
			continue
		}

		var txIn ObservedTx
		if voter.HasFinalised(activeNodeAccounts) || voter.HasConsensus(activeNodeAccounts) {
			voter.Tx.Tx.Memo = tx.Tx.Memo
			txIn = voter.Tx
		}
		vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}

		if vault.IsAsgard() {
			if !voter.UpdatedVault {
				vault.AddFunds(tx.Tx.Coins)
				voter.UpdatedVault = true
			}
		}
		if voter.HasFinalised(activeNodeAccounts) {
			vault.InboundTxCount++
		}

		memo, _ := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), tx.Tx.Memo) // ignore err
		if vault.IsYggdrasil() && memo.IsType(TxYggdrasilFund) {
			// only add the fund to yggdrasil vault when the memo is yggdrasil+
			// no one should send fund to yggdrasil vault , if somehow scammer / airdrop send fund to yggdrasil vault
			// those will be ignored
			// also only asgard will send fund to yggdrasil , thus doesn't need to have confirmation counting
			fromAsgard, err := h.isFromAsgard(ctx, tx)
			if err != nil {
				ctx.Logger().Error("fail to determinate whether fund is from asgard or not, let's assume it is not", "error", err)
			}
			// make sure only funds replenished from asgard will be added to vault
			if !voter.UpdatedVault && fromAsgard {
				vault.AddFunds(tx.Tx.Coins)
				voter.UpdatedVault = true
			}
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}
		// save the changes in Tx Voter to key value store
		h.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to set vault", "error", err)
			continue
		}

		if !vault.IsAsgard() {
			ctx.Logger().Info("Vault is not an Asgard vault, transaction ignored.")
			continue
		}

		if memo.IsOutbound() || memo.IsInternal() {
			// do not process outbound handlers here, or internal handlers
			continue
		}

		if err := h.mgr.Keeper().SetLastChainHeight(ctx, tx.Tx.Chain, tx.BlockHeight); err != nil {
			ctx.Logger().Error("fail to set last chain height", "error", err)
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts

		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txIn.GetSigners())

		if !voter.HasFinalised(activeNodeAccounts) {
			ctx.Logger().Info("Tx has not been finalised yet , waiting for confirmation counting", "hash", voter.TxID)
			continue
		}

		if vault.Status == InactiveVault {
			ctx.Logger().Error("observed tx on inactive vault", "tx", tx.String())
			if newErr := refundTx(ctx, tx, h.mgr, CodeInvalidVault, "observed inbound tx to an inactive vault", ""); newErr != nil {
				ctx.Logger().Error("fail to refund", "error", newErr)
			}
			continue
		}

		// construct msg from memo
		m, txErr := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txIn, msg.Signer)
		if txErr != nil {
			ctx.Logger().Error("fail to process inbound tx", "error", txErr.Error(), "tx hash", tx.Tx.ID.String())
			if newErr := refundTx(ctx, tx, h.mgr, CodeInvalidMemo, txErr.Error(), ""); nil != newErr {
				ctx.Logger().Error("fail to refund", "error", err)
			}
			continue
		}

		// check if we've halted trading
		swapMsg, isSwap := m.(*MsgSwap)
		_, isAddLiquidity := m.(*MsgAddLiquidity)

		if isSwap || isAddLiquidity {
			if h.mgr.Keeper().IsTradingHalt(ctx, m) || h.mgr.Keeper().RagnarokInProgress(ctx) {
				if newErr := refundTx(ctx, tx, h.mgr, se.ErrUnauthorized.ABCICode(), "trading halted", ""); nil != newErr {
					ctx.Logger().Error("fail to refund for halted trading", "error", err)
				}
				continue
			}
		}

		// if its a swap, send it to our queue for processing later
		if isSwap {
			h.addSwap(ctx, *swapMsg)
			continue
		}

		// if it is a loan, inject the observed txid into the context
		_, isLoanOpen := m.(*MsgLoanOpen)
		_, isLoanRepayment := m.(*MsgLoanRepayment)
		mCtx := ctx
		if isLoanOpen || isLoanRepayment {
			mCtx = ctx.WithValue(constants.CtxLoanTxID, tx.Tx.ID)
		}

		_, err = handler(mCtx, m)
		if err != nil {
			if err := refundTx(ctx, tx, h.mgr, CodeTxFail, err.Error(), ""); err != nil {
				ctx.Logger().Error("fail to refund", "error", err)
			}
			continue
		}
		// for those Memo that will not have outbound at all , set the observedTx to done
		if !memo.GetType().HasOutbound() {
			voter.SetDone()
			h.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
		}
	}
	return &cosmos.Result{}, nil
}

func (h ObservedTxInHandler) handleV89(ctx cosmos.Context, msg MsgObservedTxIn) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}
	handler := NewInternalHandler(h.mgr)
	for _, tx := range msg.Txs {
		// check we are sending to a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", "observed pub key", tx.ObservedPubKey)
			continue
		}

		voter, err := h.mgr.Keeper().GetObservedTxInVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx in voter", "error", err)
			continue
		}

		voter, ok := h.preflightV1(ctx, voter, activeNodeAccounts, tx, msg.Signer)
		if !ok {
			if voter.Height == ctx.BlockHeight() || voter.FinalisedHeight == ctx.BlockHeight() {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}

		// all logic after this  is after consensus

		ctx.Logger().Info("handleMsgObservedTxIn request", "Tx:", tx.String())
		if voter.Reverted {
			ctx.Logger().Info("tx had been reverted", "Tx", tx.String())
			continue
		}

		var txIn ObservedTx
		if voter.HasFinalised(activeNodeAccounts) || voter.HasConsensus(activeNodeAccounts) {
			voter.Tx.Tx.Memo = tx.Tx.Memo
			txIn = voter.Tx
		}
		vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		// do not observe inactive vaults unless it's the confirmation for a txn
		// that reached consensus/updated the vault when the vault was still active
		if vault.Status == InactiveVault && !voter.UpdatedVault {
			ctx.Logger().Error("observed tx on inactive vault", "tx", tx.String())
			continue
		}
		if vault.IsAsgard() {
			if !voter.UpdatedVault {
				vault.AddFunds(tx.Tx.Coins)
				voter.UpdatedVault = true
			}
		}
		if voter.HasFinalised(activeNodeAccounts) {
			vault.InboundTxCount++
		}
		memo, _ := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), tx.Tx.Memo) // ignore err
		if vault.IsYggdrasil() && memo.IsType(TxYggdrasilFund) {
			// only add the fund to yggdrasil vault when the memo is yggdrasil+
			// no one should send fund to yggdrasil vault , if somehow scammer / airdrop send fund to yggdrasil vault
			// those will be ignored
			// also only asgard will send fund to yggdrasil , thus doesn't need to have confirmation counting
			fromAsgard, err := h.isFromAsgard(ctx, tx)
			if err != nil {
				ctx.Logger().Error("fail to determinate whether fund is from asgard or not, let's assume it is not", "error", err)
			}
			// make sure only funds replenished from asgard will be added to vault
			if !voter.UpdatedVault && fromAsgard {
				vault.AddFunds(tx.Tx.Coins)
				voter.UpdatedVault = true
			}
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}
		// save the changes in Tx Voter to key value store
		h.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to set vault", "error", err)
			continue
		}

		if !vault.IsAsgard() {
			ctx.Logger().Info("Vault is not an Asgard vault, transaction ignored.")
			continue
		}

		if memo.IsOutbound() || memo.IsInternal() {
			// do not process outbound handlers here, or internal handlers
			continue
		}

		if err := h.mgr.Keeper().SetLastChainHeight(ctx, tx.Tx.Chain, tx.BlockHeight); err != nil {
			ctx.Logger().Error("fail to set last chain height", "error", err)
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts

		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txIn.GetSigners())

		if !voter.HasFinalised(activeNodeAccounts) {
			ctx.Logger().Info("Tx has not been finalised yet , waiting for confirmation counting", "hash", voter.TxID)
			continue
		}
		// construct msg from memo
		m, txErr := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txIn, msg.Signer)
		if txErr != nil {
			ctx.Logger().Error("fail to process inbound tx", "error", txErr.Error(), "tx hash", tx.Tx.ID.String())
			if newErr := refundTx(ctx, tx, h.mgr, CodeInvalidMemo, txErr.Error(), ""); nil != newErr {
				ctx.Logger().Error("fail to refund", "error", err)
			}
			continue
		}

		// check if we've halted trading
		swapMsg, isSwap := m.(*MsgSwap)
		_, isAddLiquidity := m.(*MsgAddLiquidity)

		if isSwap || isAddLiquidity {
			if isTradingHalt(ctx, m, h.mgr) || h.mgr.Keeper().RagnarokInProgress(ctx) {
				if newErr := refundTx(ctx, tx, h.mgr, se.ErrUnauthorized.ABCICode(), "trading halted", ""); nil != newErr {
					ctx.Logger().Error("fail to refund for halted trading", "error", err)
				}
				continue
			}
		}

		// if its a swap, send it to our queue for processing later
		if isSwap {
			h.addSwap(ctx, *swapMsg)
			continue
		}

		_, err = handler(ctx, m)
		if err != nil {
			if err := refundTx(ctx, tx, h.mgr, CodeTxFail, err.Error(), ""); err != nil {
				ctx.Logger().Error("fail to refund", "error", err)
			}
			continue
		}
		// for those Memo that will not have outbound at all , set the observedTx to done
		if !memo.GetType().HasOutbound() {
			voter.SetDone()
			h.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
		}
	}
	return &cosmos.Result{}, nil
}

func (h ObservedTxInHandler) handleV78(ctx cosmos.Context, msg MsgObservedTxIn) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}
	handler := NewInternalHandler(h.mgr)
	for _, tx := range msg.Txs {
		// check we are sending to a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", "observed pub key", tx.ObservedPubKey)
			continue
		}

		voter, err := h.mgr.Keeper().GetObservedTxInVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx in voter", "error", err)
			continue
		}

		voter, ok := h.preflightV1(ctx, voter, activeNodeAccounts, tx, msg.Signer)
		if !ok {
			if voter.Height == ctx.BlockHeight() || voter.FinalisedHeight == ctx.BlockHeight() {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}

		// all logic after this  is after consensus

		ctx.Logger().Info("handleMsgObservedTxIn request", "Tx:", tx.String())
		if voter.Reverted {
			ctx.Logger().Info("tx had been reverted", "Tx", tx.String())
			continue
		}

		var txIn ObservedTx
		if voter.HasFinalised(activeNodeAccounts) || voter.HasConsensus(activeNodeAccounts) {
			voter.Tx.Tx.Memo = tx.Tx.Memo
			txIn = voter.Tx
		}
		vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		// do not observe inactive vaults unless it's the confirmation for a txn
		// that reached consensus/updated the vault when the vault was still active
		if vault.Status == InactiveVault && !voter.UpdatedVault {
			ctx.Logger().Error("observed tx on inactive vault", "tx", tx.String())
			continue
		}
		if vault.IsAsgard() {
			if !voter.UpdatedVault {
				vault.AddFunds(tx.Tx.Coins)
				voter.UpdatedVault = true
			}
		}
		if voter.HasFinalised(activeNodeAccounts) {
			vault.InboundTxCount++
		}
		memo, _ := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), tx.Tx.Memo) // ignore err
		if vault.IsYggdrasil() && memo.IsType(TxYggdrasilFund) {

			// only add the fund to yggdrasil vault when the memo is yggdrasil+
			// no one should send fund to yggdrasil vault , if somehow scammer / airdrop send fund to yggdrasil vault
			// those will be ignored
			// also only asgard will send fund to yggdrasil , thus doesn't need to have confirmation counting
			if !voter.UpdatedVault {
				vault.AddFunds(tx.Tx.Coins)
				voter.UpdatedVault = true
			}
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}
		// save the changes in Tx Voter to key value store
		h.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to set vault", "error", err)
			continue
		}

		if !vault.IsAsgard() {
			ctx.Logger().Info("Vault is not an Asgard vault, transaction ignored.")
			continue
		}

		if memo.IsOutbound() || memo.IsInternal() {
			// do not process outbound handlers here, or internal handlers
			continue
		}

		if err := h.mgr.Keeper().SetLastChainHeight(ctx, tx.Tx.Chain, tx.BlockHeight); err != nil {
			ctx.Logger().Error("fail to set last chain height", "error", err)
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts

		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txIn.GetSigners())

		if !voter.HasFinalised(activeNodeAccounts) {
			ctx.Logger().Info("Tx has not been finalised yet , waiting for confirmation counting", "hash", voter.TxID)
			continue
		}
		// construct msg from memo
		m, txErr := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txIn, msg.Signer)
		if txErr != nil {
			ctx.Logger().Error("fail to process inbound tx", "error", txErr.Error(), "tx hash", tx.Tx.ID.String())
			if newErr := refundTx(ctx, tx, h.mgr, CodeInvalidMemo, txErr.Error(), ""); nil != newErr {
				ctx.Logger().Error("fail to refund", "error", err)
			}
			continue
		}

		// check if we've halted trading
		swapMsg, isSwap := m.(*MsgSwap)
		_, isAddLiquidity := m.(*MsgAddLiquidity)

		if isSwap || isAddLiquidity {
			if isTradingHalt(ctx, m, h.mgr) || h.mgr.Keeper().RagnarokInProgress(ctx) {
				if newErr := refundTx(ctx, tx, h.mgr, se.ErrUnauthorized.ABCICode(), "trading halted", ""); nil != newErr {
					ctx.Logger().Error("fail to refund for halted trading", "error", err)
				}
				continue
			}
		}

		// if its a swap, send it to our queue for processing later
		if isSwap {
			h.addSwap(ctx, *swapMsg)
			continue
		}

		_, err = handler(ctx, m)
		if err != nil {
			if err := refundTx(ctx, tx, h.mgr, CodeTxFail, err.Error(), ""); err != nil {
				ctx.Logger().Error("fail to refund", "error", err)
			}
			continue
		}
		// for those Memo that will not have outbound at all , set the observedTx to done
		if !memo.GetType().HasOutbound() {
			voter.SetDone()
			h.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
		}
	}
	return &cosmos.Result{}, nil
}

func (h ObservedTxInHandler) addSwapV63(ctx cosmos.Context, msg MsgSwap) {
	amt := cosmos.ZeroUint()
	if !msg.AffiliateBasisPoints.IsZero() && msg.AffiliateAddress.IsChain(common.THORChain) {
		amt = common.GetSafeShare(
			msg.AffiliateBasisPoints,
			cosmos.NewUint(10000),
			msg.Tx.Coins[0].Amount,
		)
		msg.Tx.Coins[0].Amount = common.SafeSub(msg.Tx.Coins[0].Amount, amt)
	}

	if err := h.mgr.Keeper().SetSwapQueueItem(ctx, msg, 0); err != nil {
		ctx.Logger().Error("fail to add swap to queue", "error", err)
	}

	if !amt.IsZero() {
		affiliateSwap := NewMsgSwap(
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
		if affiliateSwap.Tx.Coins[0].Amount.GTE(amt) {
			affiliateSwap.Tx.Coins[0].Amount = amt
		}

		if err := h.mgr.Keeper().SetSwapQueueItem(ctx, *affiliateSwap, 1); err != nil {
			ctx.Logger().Error("fail to add swap to queue", "error", err)
		}
	}
}

func (h ObservedTxInHandler) handleV107(ctx cosmos.Context, msg MsgObservedTxIn) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}
	handler := NewInternalHandler(h.mgr)
	for _, tx := range msg.Txs {
		// check we are sending to a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", "observed pub key", tx.ObservedPubKey)
			continue
		}

		voter, err := h.mgr.Keeper().GetObservedTxInVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx in voter", "error", err)
			continue
		}

		voter, ok := h.preflightV1(ctx, voter, activeNodeAccounts, tx, msg.Signer)
		if !ok {
			if voter.Height == ctx.BlockHeight() || voter.FinalisedHeight == ctx.BlockHeight() {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}

		// all logic after this  is after consensus

		ctx.Logger().Info("handleMsgObservedTxIn request", "Tx:", tx.String())
		if voter.Reverted {
			ctx.Logger().Info("tx had been reverted", "Tx", tx.String())
			continue
		}

		var txIn ObservedTx
		if voter.HasFinalised(activeNodeAccounts) || voter.HasConsensus(activeNodeAccounts) {
			voter.Tx.Tx.Memo = tx.Tx.Memo
			txIn = voter.Tx
		}
		vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		// do not observe inactive vaults unless it's the confirmation for a txn
		// that reached consensus/updated the vault when the vault was still active
		if vault.Status == InactiveVault && !voter.UpdatedVault {
			ctx.Logger().Error("observed tx on inactive vault", "tx", tx.String())
			continue
		}
		if vault.IsAsgard() {
			if !voter.UpdatedVault {
				vault.AddFunds(tx.Tx.Coins)
				voter.UpdatedVault = true
			}
		}
		if voter.HasFinalised(activeNodeAccounts) {
			vault.InboundTxCount++
		}
		memo, _ := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), tx.Tx.Memo) // ignore err
		if vault.IsYggdrasil() && memo.IsType(TxYggdrasilFund) {
			// only add the fund to yggdrasil vault when the memo is yggdrasil+
			// no one should send fund to yggdrasil vault , if somehow scammer / airdrop send fund to yggdrasil vault
			// those will be ignored
			// also only asgard will send fund to yggdrasil , thus doesn't need to have confirmation counting
			fromAsgard, err := h.isFromAsgard(ctx, tx)
			if err != nil {
				ctx.Logger().Error("fail to determinate whether fund is from asgard or not, let's assume it is not", "error", err)
			}
			// make sure only funds replenished from asgard will be added to vault
			if !voter.UpdatedVault && fromAsgard {
				vault.AddFunds(tx.Tx.Coins)
				voter.UpdatedVault = true
			}
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}
		// save the changes in Tx Voter to key value store
		h.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to set vault", "error", err)
			continue
		}

		if !vault.IsAsgard() {
			ctx.Logger().Info("Vault is not an Asgard vault, transaction ignored.")
			continue
		}

		if memo.IsOutbound() || memo.IsInternal() {
			// do not process outbound handlers here, or internal handlers
			continue
		}

		if err := h.mgr.Keeper().SetLastChainHeight(ctx, tx.Tx.Chain, tx.BlockHeight); err != nil {
			ctx.Logger().Error("fail to set last chain height", "error", err)
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts

		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txIn.GetSigners())

		if !voter.HasFinalised(activeNodeAccounts) {
			ctx.Logger().Info("Tx has not been finalised yet , waiting for confirmation counting", "hash", voter.TxID)
			continue
		}
		// construct msg from memo
		m, txErr := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txIn, msg.Signer)
		if txErr != nil {
			ctx.Logger().Error("fail to process inbound tx", "error", txErr.Error(), "tx hash", tx.Tx.ID.String())
			if newErr := refundTx(ctx, tx, h.mgr, CodeInvalidMemo, txErr.Error(), ""); nil != newErr {
				ctx.Logger().Error("fail to refund", "error", err)
			}
			continue
		}

		// check if we've halted trading
		swapMsg, isSwap := m.(*MsgSwap)
		_, isAddLiquidity := m.(*MsgAddLiquidity)

		if isSwap || isAddLiquidity {
			if isTradingHalt(ctx, m, h.mgr) || h.mgr.Keeper().RagnarokInProgress(ctx) {
				if newErr := refundTx(ctx, tx, h.mgr, se.ErrUnauthorized.ABCICode(), "trading halted", ""); nil != newErr {
					ctx.Logger().Error("fail to refund for halted trading", "error", err)
				}
				continue
			}
		}

		// if its a swap, send it to our queue for processing later
		if isSwap {
			h.addSwap(ctx, *swapMsg)
			continue
		}

		// if it is a loan, inject the observed txid into the context
		_, isLoanOpen := m.(*MsgLoanOpen)
		_, isLoanRepayment := m.(*MsgLoanRepayment)
		mCtx := ctx
		if isLoanOpen || isLoanRepayment {
			mCtx = ctx.WithValue(constants.CtxLoanTxID, tx.Tx.ID)
		}

		_, err = handler(mCtx, m)
		if err != nil {
			if err := refundTx(ctx, tx, h.mgr, CodeTxFail, err.Error(), ""); err != nil {
				ctx.Logger().Error("fail to refund", "error", err)
			}
			continue
		}
		// for those Memo that will not have outbound at all , set the observedTx to done
		if !memo.GetType().HasOutbound() {
			voter.SetDone()
			h.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
		}
	}
	return &cosmos.Result{}, nil
}
