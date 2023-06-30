package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// From using an empty ID, the emitted swap event wasn't picked up by Midgard;
// for memos REFUND:B07A6B1B40ADBA2E404D9BCE1BEF6EDE6F70AD135E83806E4F4B6863CF637D0B
// and REFUND:4795A3C036322493A9692B5D44E7D4FF29C3E2C1E848637184E98FE8B05FD06E
// Keccak-258 TxIDs were manually added to Midgard of
// EE31ACC02D631DC3220990A1DD2E9030F4CFC227A61E975B5DEF1037106D1CCD
// and 0A61B99DC6B1A4499A72238AC767C09C310326875F9E7B870C908357B09202E9 respectively.
func refundDroppedSwapOutFromRUNEV103(ctx cosmos.Context, mgr *Mgrs, droppedTx DroppedSwapOutTx) error {
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

	// create and emit a fake tx and swap event to keep pools balanced in Midgard
	fakeSwapTx := common.Tx{
		ID:          "",
		Chain:       common.ETHChain,
		FromAddress: txVoter.Actions[0].ToAddress,
		ToAddress:   common.Address(txVoter.Actions[0].Aggregator),
		Coins:       common.NewCoins(gasAssetCoin),
		Memo:        fmt.Sprintf("REFUND:%s", inboundTx.ID),
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
