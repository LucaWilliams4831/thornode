package thorchain

import (
	"context"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h ObservedTxOutHandler) handleV109(ctx cosmos.Context, msg MsgObservedTxOut) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	handler := NewInternalHandler(h.mgr)

	for _, tx := range msg.Txs {
		// check we are sending from a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", tx.ObservedPubKey)
			continue
		}
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get keysing metric", "error", err)
			} else {
				keysignMetric.AddNodeTssTime(msg.Signer, tx.KeysignMs)
				h.mgr.Keeper().SetTssKeysignMetric(ctx, keysignMetric)
			}
		}
		voter, err := h.mgr.Keeper().GetObservedTxOutVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx out voter", "error", err)
			continue
		}

		// check whether the tx has consensus
		voter, ok := h.preflight(ctx, voter, activeNodeAccounts, tx, msg.Signer)
		if !ok {
			if voter.FinalisedHeight == ctx.BlockHeight() {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}
		ctx.Logger().Info("handleMsgObservedTxOut request", "Tx:", tx.String())

		// if memo isn't valid or its an inbound memo, and its funds moving
		// from a yggdrasil vault, slash the node
		memo, _ := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), tx.Tx.Memo)
		if memo.IsEmpty() || memo.IsInbound() {
			vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get vault", "error", err)
				continue
			}
			toSlash := make(common.Coins, len(tx.Tx.Coins))
			copy(toSlash, tx.Tx.Coins)
			toSlash = toSlash.Adds(tx.Tx.Gas.ToCoins())

			slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
				telemetry.NewLabel("reason", "sent_extra_funds"),
				telemetry.NewLabel("chain", string(tx.Tx.Chain)),
			}))

			if err := h.mgr.Slasher().SlashVault(slashCtx, tx.ObservedPubKey, toSlash, h.mgr); err != nil {
				ctx.Logger().Error("fail to slash account for sending extra fund", "error", err)
			}
			vault.SubFunds(toSlash)
			if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				ctx.Logger().Error("fail to save vault", "error", err)
			}

			continue
		}

		txOut := voter.GetTx(activeNodeAccounts) // get consensus tx, in case our for loop is incorrect
		txOut.Tx.Memo = tx.Tx.Memo
		m, err := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txOut, msg.Signer)
		if err != nil || tx.Tx.Chain.IsEmpty() {
			ctx.Logger().Error("fail to process txOut",
				"error", err,
				"tx", tx.Tx.String())
			continue
		}

		// Apply Gas fees
		if err := addGasFees(ctx, h.mgr, tx); err != nil {
			ctx.Logger().Error("fail to add gas fee", "error", err)
			continue
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts
		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txOut.GetSigners())

		// emit tss keysign metrics
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get tss keysign metric", "error", err, "hash", tx.Tx.ID)
			} else {
				evt := NewEventTssKeysignMetric(keysignMetric.TxID, keysignMetric.GetMedianTime())
				if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}
		}
		_, err = handler(ctx, m)
		if err != nil {
			ctx.Logger().Error("handler failed:", "error", err)
			continue
		}
		voter.SetDone()
		h.mgr.Keeper().SetObservedTxOutVoter(ctx, voter)
		// process the msg first , and then deduct the fund from vault last
		// If sending from one of our vaults, decrement coins
		vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		vault.SubFunds(tx.Tx.Coins)
		vault.OutboundTxCount++
		if vault.IsAsgard() && memo.IsType(TxMigrate) {
			// only remove the block height that had been specified in the memo
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}

		if !vault.HasFunds() && vault.Status == RetiringVault {
			// we have successfully removed all funds from a retiring vault,
			// mark it as inactive
			vault.Status = InactiveVault
		}
		// if the vault is frozen, then unfreeze it. Since we saw that a
		// transaction was signed
		for _, coin := range tx.Tx.Coins {
			for i := range vault.Frozen {
				if strings.EqualFold(coin.Asset.GetChain().String(), vault.Frozen[i]) {
					vault.Frozen = append(vault.Frozen[:i], vault.Frozen[i+1:]...)
					break
				}
			}
		}
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
			continue
		}
	}
	return &cosmos.Result{}, nil
}

func (h ObservedTxOutHandler) handleV96(ctx cosmos.Context, msg MsgObservedTxOut) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	handler := NewInternalHandler(h.mgr)

	for _, tx := range msg.Txs {
		// check we are sending from a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", tx.ObservedPubKey)
			continue
		}
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get keysing metric", "error", err)
			} else {
				keysignMetric.AddNodeTssTime(msg.Signer, tx.KeysignMs)
				h.mgr.Keeper().SetTssKeysignMetric(ctx, keysignMetric)
			}
		}
		voter, err := h.mgr.Keeper().GetObservedTxOutVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx out voter", "error", err)
			continue
		}

		// check whether the tx has consensus
		voter, ok := h.preflight(ctx, voter, activeNodeAccounts, tx, msg.Signer)
		if !ok {
			if voter.FinalisedHeight == ctx.BlockHeight() {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}
		ctx.Logger().Info("handleMsgObservedTxOut request", "Tx:", tx.String())

		// if memo isn't valid or its an inbound memo, and its funds moving
		// from a yggdrasil vault, slash the node
		memo, _ := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), tx.Tx.Memo)
		if memo.IsEmpty() || memo.IsInbound() {
			vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get vault", "error", err)
				continue
			}
			toSlash := make(common.Coins, len(tx.Tx.Coins))
			copy(toSlash, tx.Tx.Coins)
			toSlash = toSlash.Adds(tx.Tx.Gas.ToCoins())

			slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
				telemetry.NewLabel("reason", "sent_extra_funds"),
				telemetry.NewLabel("chain", string(tx.Tx.Chain)),
			}))

			if err := h.mgr.Slasher().SlashVault(slashCtx, tx.ObservedPubKey, toSlash, h.mgr); err != nil {
				ctx.Logger().Error("fail to slash account for sending extra fund", "error", err)
			}
			vault.SubFunds(toSlash)
			if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				ctx.Logger().Error("fail to save vault", "error", err)
			}

			continue
		}

		txOut := voter.GetTx(activeNodeAccounts) // get consensus tx, in case our for loop is incorrect
		txOut.Tx.Memo = tx.Tx.Memo
		m, err := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txOut, msg.Signer)
		if err != nil || tx.Tx.Chain.IsEmpty() {
			ctx.Logger().Error("fail to process txOut",
				"error", err,
				"tx", tx.Tx.String())
			continue
		}

		// Apply Gas fees
		if err := addGasFees(ctx, h.mgr, tx); err != nil {
			ctx.Logger().Error("fail to add gas fee", "error", err)
			continue
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts
		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txOut.GetSigners())

		// emit tss keysign metrics
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get tss keysign metric", "error", err, "hash", tx.Tx.ID)
			} else {
				evt := NewEventTssKeysignMetric(keysignMetric.TxID, keysignMetric.GetMedianTime())
				if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}
		}
		_, err = handler(ctx, m)
		if err != nil {
			ctx.Logger().Error("handler failed:", "error", err)
			continue
		}
		voter.SetDone()
		h.mgr.Keeper().SetObservedTxOutVoter(ctx, voter)
		// process the msg first , and then deduct the fund from vault last
		// If sending from one of our vaults, decrement coins
		vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		vault.SubFunds(tx.Tx.Coins)
		vault.OutboundTxCount++
		if vault.IsAsgard() && memo.IsType(TxMigrate) {
			// only remove the block height that had been specified in the memo
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}

		if !vault.HasFunds() && vault.Status == RetiringVault {
			// we have successfully removed all funds from a retiring vault,
			// mark it as inactive
			vault.Status = InactiveVault
		}
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
			continue
		}
	}
	return &cosmos.Result{}, nil
}

func (h ObservedTxOutHandler) handleV89(ctx cosmos.Context, msg MsgObservedTxOut) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	handler := NewInternalHandler(h.mgr)

	for _, tx := range msg.Txs {
		// check we are sending from a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", tx.ObservedPubKey)
			continue
		}
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get keysing metric", "error", err)
			} else {
				keysignMetric.AddNodeTssTime(msg.Signer, tx.KeysignMs)
				h.mgr.Keeper().SetTssKeysignMetric(ctx, keysignMetric)
			}
		}
		voter, err := h.mgr.Keeper().GetObservedTxOutVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx out voter", "error", err)
			continue
		}

		// check whether the tx has consensus
		voter, ok := h.preflight(ctx, voter, activeNodeAccounts, tx, msg.Signer)
		if !ok {
			if voter.FinalisedHeight == ctx.BlockHeight() {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}
		ctx.Logger().Info("handleMsgObservedTxOut request", "Tx:", tx.String())

		// if memo isn't valid or its an inbound memo, and its funds moving
		// from a yggdrasil vault, slash the node
		memo, _ := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), tx.Tx.Memo)
		if memo.IsEmpty() || memo.IsInbound() {
			vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get vault", "error", err)
				continue
			}
			toSlash := tx.Tx.Coins.Adds(tx.Tx.Gas.ToCoins())

			slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
				telemetry.NewLabel("reason", "sent_extra_funds"),
				telemetry.NewLabel("chain", string(tx.Tx.Chain)),
			}))

			if err := h.mgr.Slasher().SlashVault(slashCtx, tx.ObservedPubKey, toSlash, h.mgr); err != nil {
				ctx.Logger().Error("fail to slash account for sending extra fund", "error", err)
			}
			vault.SubFunds(toSlash)
			if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				ctx.Logger().Error("fail to save vault", "error", err)
			}

			continue
		}

		txOut := voter.GetTx(activeNodeAccounts) // get consensus tx, in case our for loop is incorrect
		txOut.Tx.Memo = tx.Tx.Memo
		m, err := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txOut, msg.Signer)
		if err != nil || tx.Tx.Chain.IsEmpty() {
			ctx.Logger().Error("fail to process txOut",
				"error", err,
				"tx", tx.Tx.String())
			continue
		}

		// Apply Gas fees
		if err := addGasFees(ctx, h.mgr, tx); err != nil {
			ctx.Logger().Error("fail to add gas fee", "error", err)
			continue
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts
		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txOut.GetSigners())

		// emit tss keysign metrics
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get tss keysign metric", "error", err, "hash", tx.Tx.ID)
			} else {
				evt := NewEventTssKeysignMetric(keysignMetric.TxID, keysignMetric.GetMedianTime())
				if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}
		}
		_, err = handler(ctx, m)
		if err != nil {
			ctx.Logger().Error("handler failed:", "error", err)
			continue
		}
		voter.SetDone()
		h.mgr.Keeper().SetObservedTxOutVoter(ctx, voter)
		// process the msg first , and then deduct the fund from vault last
		// If sending from one of our vaults, decrement coins
		vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		vault.SubFunds(tx.Tx.Coins)
		vault.OutboundTxCount++
		if vault.IsAsgard() && memo.IsType(TxMigrate) {
			// only remove the block height that had been specified in the memo
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}

		if !vault.HasFunds() && vault.Status == RetiringVault {
			// we have successfully removed all funds from a retiring vault,
			// mark it as inactive
			vault.Status = InactiveVault
		}
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
			continue
		}
	}
	return &cosmos.Result{}, nil
}

func (h ObservedTxOutHandler) handleV58(ctx cosmos.Context, msg MsgObservedTxOut) (*cosmos.Result, error) {
	activeNodeAccounts, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	handler := NewInternalHandler(h.mgr)

	for _, tx := range msg.Txs {
		// check we are sending from a valid vault
		if !h.mgr.Keeper().VaultExists(ctx, tx.ObservedPubKey) {
			ctx.Logger().Info("Not valid Observed Pubkey", tx.ObservedPubKey)
			continue
		}
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get keysing metric", "error", err)
			} else {
				keysignMetric.AddNodeTssTime(msg.Signer, tx.KeysignMs)
				h.mgr.Keeper().SetTssKeysignMetric(ctx, keysignMetric)
			}
		}
		voter, err := h.mgr.Keeper().GetObservedTxOutVoter(ctx, tx.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to get tx out voter", "error", err)
			continue
		}

		// check whether the tx has consensus
		voter, ok := h.preflightV1(ctx, voter, activeNodeAccounts, tx, msg.Signer)
		if !ok {
			if voter.FinalisedHeight == ctx.BlockHeight() {
				// we've already process the transaction, but we should still
				// update the observing addresses
				h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, msg.GetSigners())
			}
			continue
		}
		ctx.Logger().Info("handleMsgObservedTxOut request", "Tx:", tx.String())

		// if memo isn't valid or its an inbound memo, and its funds moving
		// from a yggdrasil vault, slash the node
		memo, _ := ParseMemoWithTHORNames(ctx, h.mgr.Keeper(), tx.Tx.Memo)
		if memo.IsEmpty() || memo.IsInbound() {
			vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get vault", "error", err)
				continue
			}
			toSlash := tx.Tx.Coins.Adds(tx.Tx.Gas.ToCoins())

			slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
				telemetry.NewLabel("reason", "sent_extra_funds"),
				telemetry.NewLabel("chain", string(tx.Tx.Chain)),
			}))

			if err := h.mgr.Slasher().SlashVault(slashCtx, tx.ObservedPubKey, toSlash, h.mgr); err != nil {
				ctx.Logger().Error("fail to slash account for sending extra fund", "error", err)
			}
			vault.SubFunds(toSlash)
			if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				ctx.Logger().Error("fail to save vault", "error", err)
			}

			continue
		}

		txOut := voter.GetTx(activeNodeAccounts) // get consensus tx, in case our for loop is incorrect
		txOut.Tx.Memo = tx.Tx.Memo
		m, err := processOneTxIn(ctx, h.mgr.GetVersion(), h.mgr.Keeper(), txOut, msg.Signer)
		if err != nil || tx.Tx.Chain.IsEmpty() {
			ctx.Logger().Error("fail to process txOut",
				"error", err,
				"tx", tx.Tx.String())
			continue
		}

		// Apply Gas fees
		if err := addGasFees(ctx, h.mgr, tx); err != nil {
			ctx.Logger().Error("fail to add gas fee", "error", err)
			continue
		}

		// If sending from one of our vaults, decrement coins
		vault, err := h.mgr.Keeper().GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		vault.SubFunds(tx.Tx.Coins)
		vault.OutboundTxCount++
		if vault.IsAsgard() && memo.IsType(TxMigrate) {
			// only remove the block height that had been specified in the memo
			vault.RemovePendingTxBlockHeights(memo.GetBlockHeight())
		}

		if !vault.HasFunds() && vault.Status == RetiringVault {
			// we have successfully removed all funds from a retiring vault,
			// mark it as inactive
			vault.Status = InactiveVault
		}
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
			continue
		}

		// add addresses to observing addresses. This is used to detect
		// active/inactive observing node accounts
		h.mgr.ObMgr().AppendObserver(tx.Tx.Chain, txOut.GetSigners())

		// emit tss keysign metrics
		if tx.KeysignMs > 0 {
			keysignMetric, err := h.mgr.Keeper().GetTssKeysignMetric(ctx, tx.Tx.ID)
			if err != nil {
				ctx.Logger().Error("fail to get tss keysign metric", "error", err, "hash", tx.Tx.ID)
			} else {
				evt := NewEventTssKeysignMetric(keysignMetric.TxID, keysignMetric.GetMedianTime())
				if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}
		}
		_, err = handler(ctx, m)
		if err != nil {
			ctx.Logger().Error("handler failed:", "error", err)
			continue
		}
		voter.SetDone()
		h.mgr.Keeper().SetObservedTxOutVoter(ctx, voter)
	}
	return &cosmos.Result{}, nil
}

func (h ObservedTxOutHandler) preflightV1(ctx cosmos.Context, voter ObservedTxVoter, nas NodeAccounts, tx ObservedTx, signer cosmos.AccAddress) (ObservedTxVoter, bool) {
	observeSlashPoints := h.mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := h.mgr.GetConstants().GetInt64Value(constants.ObservationDelayFlexibility)
	ok := false

	slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
		telemetry.NewLabel("reason", "failed_observe_txout"),
		telemetry.NewLabel("chain", string(tx.Tx.Chain)),
	}))
	h.mgr.Slasher().IncSlashPoints(slashCtx, observeSlashPoints, signer)

	if err := h.mgr.Keeper().SetLastObserveHeight(ctx, tx.Tx.Chain, signer, tx.BlockHeight); err != nil {
		ctx.Logger().Error("fail to save last observe height", "error", err, "signer", signer, "chain", tx.Tx.Chain)
	}
	if !voter.Add(tx, signer) {
		// when the signer already sign it
		return voter, ok
	}
	if voter.HasFinalised(nas) {
		if voter.FinalisedHeight == 0 {
			ok = true
			voter.FinalisedHeight = ctx.BlockHeight()
			voter.Tx = voter.GetTx(nas)
			// tx has consensus now, so decrease the slashing point for all the signers whom voted for it
			h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, voter.Tx.GetSigners()...)

		} else if ctx.BlockHeight() <= (voter.FinalisedHeight+observeFlex) && voter.Tx.Equals(tx) {
			// event the tx had been processed , given the signer just a bit late , so we still take away their slash points
			h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, signer)
		}
	}
	h.mgr.Keeper().SetObservedTxOutVoter(ctx, voter)

	// Check to see if we have enough identical observations to process the transaction
	return voter, ok
}
