package thorchain

import (
	"crypto/sha256"
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// removeTransactions is a method used to remove a tx out item in the queue
func removeTransactions(ctx cosmos.Context, mgr Manager, hashes ...string) {
	for _, txID := range hashes {
		inTxID, err := common.NewTxID(txID)
		if err != nil {
			ctx.Logger().Error("fail to parse tx id", "error", err, "tx_id", inTxID)
			continue
		}
		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, inTxID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx voter", "error", err)
			continue
		}
		// all outbound action get removed
		voter.Actions = []TxOutItem{}
		if voter.Tx.IsEmpty() {
			continue
		}
		version := mgr.GetVersion()
		voter.Tx.SetDone(version, common.BlankTxID, 0)
		// set the tx outbound with a blank txid will mark it as down , and will be skipped in the reschedule logic
		for idx := range voter.Txs {
			voter.Txs[idx].SetDone(version, common.BlankTxID, 0)
		}
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	}
}

//nolint:unused
type DroppedSwapOutTx struct {
	inboundHash string
	gasAsset    common.Asset
}

// refundDroppedSwapOutFromRUNE refunds a dropped swap out TX that originated from $RUNE

// These txs completed the swap to the EVM gas asset, but bifrost dropped the final swap out outbound
// To refund:
// 1. Credit the gas asset pool the amount of gas asset that never left
// 2. Deduct the corresponding amount of RUNE from the pool, as that will be refunded
// 3. Send the user their RUNE back
//
//nolint:unused,deadcode
func refundDroppedSwapOutFromRUNE(ctx cosmos.Context, mgr *Mgrs, droppedTx DroppedSwapOutTx) error {
	version := mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.106.0")):
		return refundDroppedSwapOutFromRUNEV106(ctx, mgr, droppedTx)
	default:
		return refundDroppedSwapOutFromRUNEV103(ctx, mgr, droppedTx)
	}
}

func refundDroppedSwapOutFromRUNEV106(ctx cosmos.Context, mgr *Mgrs, droppedTx DroppedSwapOutTx) error {
	txId, err := common.NewTxID(droppedTx.inboundHash)
	if err != nil {
		return err
	}

	txVoter, err := mgr.Keeper().GetObservedTxInVoter(ctx, txId)
	if err != nil {
		return err
	}

	if txVoter.OutTxs != nil {
		return fmt.Errorf("For a dropped swap out there should be no out_txs")
	}

	// Get the original inbound, if it's not for RUNE, skip
	inboundTx := txVoter.Tx.Tx
	if !inboundTx.Chain.IsTHORChain() {
		return fmt.Errorf("Inbound tx isn't from thorchain")
	}

	inboundCoins := inboundTx.Coins
	if len(inboundCoins) != 1 || !inboundCoins[0].Asset.IsNativeRune() {
		return fmt.Errorf("Inbound coin is not native RUNE")
	}

	inboundRUNE := inboundCoins[0]
	swapperRUNEAddr := inboundTx.FromAddress

	if txVoter.Actions == nil || len(txVoter.Actions) == 0 {
		return fmt.Errorf("Tx Voter has empty Actions")
	}

	// gasAssetCoin is the gas asset that was swapped to for the swap out
	// Since the swap out was dropped, this amount of the gas asset never left the pool.
	// This amount should be credited back to the pool since it was originally deducted when thornode sent the swap out
	gasAssetCoin := txVoter.Actions[0].Coin
	if !gasAssetCoin.Asset.Equals(droppedTx.gasAsset) {
		return fmt.Errorf("Tx Voter action coin isn't swap out gas asset")
	}

	gasPool, err := mgr.Keeper().GetPool(ctx, droppedTx.gasAsset)
	if err != nil {
		return err
	}

	totalGasAssetAmt := cosmos.NewUint(0)

	// If the outbound was split between multiple Asgards, add up the full amount here
	for _, action := range txVoter.Actions {
		totalGasAssetAmt = totalGasAssetAmt.Add(action.Coin.Amount)
	}

	// Credit Gas Pool the Gas Asset balance, deduct the RUNE balance
	gasPool.BalanceAsset = gasPool.BalanceAsset.Add(totalGasAssetAmt)
	gasPool.BalanceRune = gasPool.BalanceRune.Sub(inboundRUNE.Amount)

	// Update the pool
	if err := mgr.Keeper().SetPool(ctx, gasPool); err != nil {
		return err
	}

	addrAcct, err := swapperRUNEAddr.AccAddress()
	if err != nil {
		ctx.Logger().Error("fail to create acct in migrate store to v98", "error", err)
	}

	runeCoins := common.NewCoins(inboundRUNE)

	// Send user their funds
	err = mgr.Keeper().SendFromModuleToAccount(ctx, AsgardName, addrAcct, runeCoins)
	if err != nil {
		return err
	}

	memo := fmt.Sprintf("REFUND:%s", inboundTx.ID)

	// Generate a fake TxID from the refund memo for Midgard to record.
	// Since the inbound hash is expected to be unique, the sha256 hash is expected to be unique.
	hash := fmt.Sprintf("%X", sha256.Sum256([]byte(memo)))
	fakeTxID, err := common.NewTxID(hash)
	if err != nil {
		return err
	}

	// create and emit a fake tx and swap event to keep pools balanced in Midgard
	fakeSwapTx := common.Tx{
		ID:          fakeTxID,
		Chain:       common.ETHChain,
		FromAddress: txVoter.Actions[0].ToAddress,
		ToAddress:   common.Address(txVoter.Actions[0].Aggregator),
		Coins:       common.NewCoins(gasAssetCoin),
		Memo:        memo,
	}

	swapEvt := NewEventSwap(
		droppedTx.gasAsset,
		cosmos.ZeroUint(),
		cosmos.ZeroUint(),
		cosmos.ZeroUint(),
		cosmos.ZeroUint(),
		fakeSwapTx,
		inboundRUNE,
		cosmos.ZeroUint(),
	)

	if err := mgr.EventMgr().EmitEvent(ctx, swapEvt); err != nil {
		ctx.Logger().Error("fail to emit fake swap event", "error", err)
	}

	return nil
}

// When an ObservedTxInVoter has dangling Actions items swallowed by the vaults, requeue them.
func requeueDanglingActionsV108(ctx cosmos.Context, mgr *Mgrs, txIDs []common.TxID) {
	// Select the least secure ActiveVault Asgard for all outbounds.
	// Even if it fails (as in if the version changed upon the keygens-complete block of a churn),
	// updating the voter's FinalisedHeight allows another MaxOutboundAttempts for LackSigning vault selection.
	activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil || len(activeAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}
	if len(activeAsgards) > 1 {
		signingTransactionPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
		activeAsgards = mgr.Keeper().SortBySecurity(ctx, activeAsgards, signingTransactionPeriod)
	}
	vaultPubKey := activeAsgards[0].PubKey

	for _, txID := range txIDs {
		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, txID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx voter", "error", err)
			continue
		}

		if len(voter.OutTxs) >= len(voter.Actions) {
			log := fmt.Sprintf("(%d) OutTxs present for (%s), despite expecting fewer than the (%d) Actions.", len(voter.OutTxs), txID.String(), len(voter.Actions))
			ctx.Logger().Debug(log)
			continue
		}

		var indices []int
		for i := range voter.Actions {
			if isActionsItemDangling(voter, i) {
				indices = append(indices, i)
			}
		}
		if len(indices) == 0 {
			log := fmt.Sprintf("No dangling Actions item found for (%s).", txID.String())
			ctx.Logger().Debug(log)
			continue
		}

		if len(voter.Actions)-len(voter.OutTxs) != len(indices) {
			log := fmt.Sprintf("(%d) Actions and (%d) OutTxs present for (%s), yet there appeared to be (%d) dangling Actions.", len(voter.Actions), len(voter.OutTxs), txID.String(), len(indices))
			ctx.Logger().Debug(log)
			continue
		}

		// Update the voter's FinalisedHeight to give another MaxOutboundAttempts.
		voter.FinalisedHeight = ctx.BlockHeight()
		voter.OutboundHeight = ctx.BlockHeight()

		for _, index := range indices {
			// Use a pointer to update the voter as well.
			actionItem := &voter.Actions[index]

			// Update the vault pubkey.
			actionItem.VaultPubKey = vaultPubKey

			// Update the Actions item's MaxGas and GasRate.
			// Note that nothing in this function should require a GasManager BeginBlock.
			gasCoin, err := mgr.GasMgr().GetMaxGas(ctx, actionItem.Chain)
			if err != nil {
				ctx.Logger().Error("fail to get max gas", "chain", actionItem.Chain, "error", err)
				continue
			}
			actionItem.MaxGas = common.Gas{gasCoin}
			actionItem.GasRate = int64(mgr.GasMgr().GetGasRate(ctx, actionItem.Chain).Uint64())

			// UnSafeAddTxOutItem is used to queue the txout item directly, without for instance deducting another fee.
			err = mgr.TxOutStore().UnSafeAddTxOutItem(ctx, mgr, *actionItem)
			if err != nil {
				ctx.Logger().Error("fail to add outbound tx", "error", err)
				continue
			}
		}

		// Having requeued all dangling Actions items, set the updated voter.
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	}
}

// makeFakeTxInObservation - accepts an array of unobserved inbounds, queries for active node accounts, and makes
// a fake observation for each validator and unobserved TxIn. Once enough nodes have "observed" each inbound the tx will be
// processed as normal.
func makeFakeTxInObservation(ctx cosmos.Context, mgr *Mgrs, txs ObservedTxs) error {
	activeNodes, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("Failed to get active nodes", "err", err)
		return err
	}

	handler := NewObservedTxInHandler(mgr)

	for _, na := range activeNodes {
		txInMsg := NewMsgObservedTxIn(txs, na.NodeAddress)
		_, err := handler.handle(ctx, *txInMsg)
		if err != nil {
			ctx.Logger().Error("failed ObservedTxIn handler", "error", err)
			continue
		}
	}

	return nil
}

// resetObservationHeights will force reset the last chain and last observed heights for
// all active nodes.
func resetObservationHeights(ctx cosmos.Context, mgr *Mgrs, version int, chain common.Chain, height int64) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error(fmt.Sprintf("fail to migrate store to v%d", version), "error", err)
		}
	}()

	// get active nodes
	activeNodes, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("failed to get active nodes", "err", err)
		return
	}

	// force set last observed height on all nodes
	for _, node := range activeNodes {
		mgr.Keeper().ForceSetLastObserveHeight(ctx, chain, node.NodeAddress, height)
	}

	// force set chain height
	mgr.Keeper().ForceSetLastChainHeight(ctx, chain, height)
}
