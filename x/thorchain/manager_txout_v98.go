package thorchain

import (
	"errors"
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// TxOutStorageV98 is going to manage all the outgoing tx
type TxOutStorageV98 struct {
	keeper        keeper.Keeper
	constAccessor constants.ConstantValues
	eventMgr      EventManager
	gasManager    GasManager
}

// newTxOutStorageV98 will create a new instance of TxOutStore.
func newTxOutStorageV98(keeper keeper.Keeper, constAccessor constants.ConstantValues, eventMgr EventManager, gasManager GasManager) *TxOutStorageV98 {
	return &TxOutStorageV98{
		keeper:        keeper,
		eventMgr:      eventMgr,
		constAccessor: constAccessor,
		gasManager:    gasManager,
	}
}

func (tos *TxOutStorageV98) EndBlock(ctx cosmos.Context, mgr Manager) error {
	// update the max gas for all outbounds in this block. This can be useful
	// if an outbound transaction was scheduled into the future, and the gas
	// for that blockchain changes in that time span. This avoids the need to
	// reschedule the transaction to Asgard, as well as avoids slash point
	// accural on ygg nodes.
	txOut, err := tos.GetBlockOut(ctx)
	if err != nil {
		return err
	}

	maxGasCache := make(map[common.Chain]common.Coin)
	gasRateCache := make(map[common.Chain]int64)

	for i, tx := range txOut.TxArray {
		// update max gas, take the larger of the current gas, or the last gas used

		// update cache if needed
		if _, ok := maxGasCache[tx.Chain]; !ok {
			maxGasCache[tx.Chain], _ = mgr.GasMgr().GetMaxGas(ctx, tx.Chain)
		}
		if _, ok := gasRateCache[tx.Chain]; !ok {
			gasRateCache[tx.Chain] = int64(mgr.GasMgr().GetGasRate(ctx, tx.Chain).Uint64())
		}

		maxGas := maxGasCache[tx.Chain]
		gasRate := gasRateCache[tx.Chain]
		if len(tx.MaxGas) == 0 || maxGas.Amount.GT(tx.MaxGas[0].Amount) {
			txOut.TxArray[i].MaxGas = common.Gas{maxGas}
			// Update MaxGas in ObservedTxVoter action as well
			err := updateTxOutGas(ctx, tos.keeper, tx, common.Gas{maxGas})
			if err != nil {
				ctx.Logger().Error("Failed to update MaxGas of action in ObservedTxVoter", "hash", tx.InHash, "error", err)
			}
		}
		txOut.TxArray[i].GasRate = gasRate
	}

	if err := tos.keeper.SetTxOut(ctx, txOut); err != nil {
		return fmt.Errorf("fail to save tx out : %w", err)
	}
	return nil
}

// GetBlockOut read the TxOut from kv store
func (tos *TxOutStorageV98) GetBlockOut(ctx cosmos.Context) (*TxOut, error) {
	return tos.keeper.GetTxOut(ctx, ctx.BlockHeight())
}

// GetOutboundItems read all the outbound item from kv store
func (tos *TxOutStorageV98) GetOutboundItems(ctx cosmos.Context) ([]TxOutItem, error) {
	block, err := tos.keeper.GetTxOut(ctx, ctx.BlockHeight())
	if block == nil {
		return nil, nil
	}
	return block.TxArray, err
}

// GetOutboundItemByToAddress read all the outbound items filter by the given to address
func (tos *TxOutStorageV98) GetOutboundItemByToAddress(ctx cosmos.Context, to common.Address) []TxOutItem {
	filterItems := make([]TxOutItem, 0)
	items, _ := tos.GetOutboundItems(ctx)
	for _, item := range items {
		if item.ToAddress.Equals(to) {
			filterItems = append(filterItems, item)
		}
	}
	return filterItems
}

// ClearOutboundItems remove all the tx out items , mostly used for test
func (tos *TxOutStorageV98) ClearOutboundItems(ctx cosmos.Context) {
	_ = tos.keeper.ClearTxOut(ctx, ctx.BlockHeight())
}

// TryAddTxOutItem add an outbound tx to block
// return bool indicate whether the transaction had been added successful or not
// return error indicate error
func (tos *TxOutStorageV98) TryAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem, minOut cosmos.Uint) (bool, error) {
	outputs, err := tos.prepareTxOutItem(ctx, toi)
	if err != nil {
		return false, fmt.Errorf("fail to prepare outbound tx: %w", err)
	}
	if len(outputs) == 0 {
		return false, ErrNotEnoughToPayFee
	}

	sumOut := cosmos.ZeroUint()
	for _, o := range outputs {
		sumOut = sumOut.Add(o.Coin.Amount)
	}
	if sumOut.LT(minOut) {
		// **NOTE** this error string is utilized by the order book manager to
		// catch the error. DO NOT change this error string without updating
		// the order book manager as well
		return false, fmt.Errorf("outbound amount does not meet requirements (%d/%d)", sumOut.Uint64(), minOut.Uint64())
	}

	// blacklist binance exchange as an outbound destination. This is because
	// the format of THORChain memos are NOT compatible with the memo
	// requirements of binance inbound transactions.
	blacklist := []string{
		"bnb136ns6lfw4zs5hg4n85vdthaad7hq5m4gtkgf23", // binance CEX address
	}
	for _, b := range blacklist {
		if toi.ToAddress.Equals(common.Address(b)) {
			return false, fmt.Errorf("non-supported outbound address")
		}
	}

	// calculate the single block height to send all of these txout items,
	// using the summed amount
	outboundHeight := ctx.BlockHeight()
	if !toi.Chain.IsTHORChain() && !toi.InHash.IsEmpty() && !toi.InHash.Equals(common.BlankTxID) {
		toi.Memo = outputs[0].Memo
		targetHeight, err := tos.CalcTxOutHeight(ctx, mgr.GetVersion(), toi)
		if err != nil {
			ctx.Logger().Error("failed to calc target block height for txout item", "error", err)
		}
		if targetHeight > outboundHeight {
			outboundHeight = targetHeight
		}
		voter, err := tos.keeper.GetObservedTxInVoter(ctx, toi.InHash)
		if err != nil {
			ctx.Logger().Error("fail to get observe tx in voter", "error", err)
			return false, fmt.Errorf("fail to get observe tx in voter,err:%w", err)
		}

		// When the inbound transaction already has an outbound , the make sure the outbound will be scheduled on the same block
		if voter.OutboundHeight > 0 {
			outboundHeight = voter.OutboundHeight
		} else {
			voter.OutboundHeight = outboundHeight
			tos.keeper.SetObservedTxInVoter(ctx, voter)
		}
	}

	// add tx to block out
	for _, output := range outputs {
		if err := tos.addToBlockOut(ctx, mgr, output, outboundHeight); err != nil {
			return false, err
		}
	}
	return true, nil
}

// UnSafeAddTxOutItem - blindly adds a tx out, skipping vault selection, transaction
// fee deduction, etc
func (tos *TxOutStorageV98) UnSafeAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem) error {
	// BCH chain will convert legacy address to new format automatically , thus when observe it back can't be associated with the original inbound
	// so here convert the legacy address to new format
	if toi.Chain.Equals(common.BCHChain) {
		newBCHAddress, err := common.ConvertToNewBCHAddressFormatV83(toi.ToAddress)
		if err != nil {
			return fmt.Errorf("fail to convert BCH address to new format: %w", err)
		}
		if newBCHAddress.IsEmpty() {
			return fmt.Errorf("empty to address , can't send out")
		}
		toi.ToAddress = newBCHAddress
	}
	return tos.addToBlockOut(ctx, mgr, toi, ctx.BlockHeight())
}

func (tos *TxOutStorageV98) discoverOutbounds(ctx cosmos.Context, transactionFeeAsset cosmos.Uint, maxGasAsset common.Coin, toi TxOutItem, vaults Vaults) ([]TxOutItem, cosmos.Uint) {
	var outputs []TxOutItem
	for _, vault := range vaults {
		// Ensure THORNode are not sending from and to the same address
		fromAddr, err := vault.PubKey.GetAddress(toi.Chain)
		if err != nil || fromAddr.IsEmpty() || toi.ToAddress.Equals(fromAddr) {
			continue
		}
		// if the asset in the vault is not enough to pay for the fee , then skip it
		if vault.GetCoin(toi.Coin.Asset).Amount.LTE(transactionFeeAsset) {
			continue
		}
		// if the vault doesn't have gas asset in it , or it doesn't have enough to pay for gas
		gasAsset := vault.GetCoin(toi.Chain.GetGasAsset())
		if gasAsset.IsEmpty() || gasAsset.Amount.LT(maxGasAsset.Amount) {
			continue
		}

		toi.VaultPubKey = vault.PubKey
		if toi.Coin.Amount.LTE(vault.GetCoin(toi.Coin.Asset).Amount) {
			outputs = append(outputs, toi)
			toi.Coin.Amount = cosmos.ZeroUint()
			break
		} else {
			remainingAmount := common.SafeSub(toi.Coin.Amount, vault.GetCoin(toi.Coin.Asset).Amount)
			toi.Coin.Amount = common.SafeSub(toi.Coin.Amount, remainingAmount)
			outputs = append(outputs, toi)
			toi.Coin.Amount = remainingAmount
		}
	}
	return outputs, toi.Coin.Amount
}

// prepareTxOutItem will do some data validation which include the following
// 1. Make sure it has a legitimate memo
// 2. choose an appropriate vault(s) to send from (ygg first, active asgard, then retiring asgard)
// 3. deduct transaction fee, keep in mind, only take transaction fee when active nodes are  more then minimumBFT
// return list of outbound transactions
func (tos *TxOutStorageV98) prepareTxOutItem(ctx cosmos.Context, toi TxOutItem) ([]TxOutItem, error) {
	var outputs []TxOutItem
	var remaining cosmos.Uint

	// Default the memo to the standard outbound memo
	if toi.Memo == "" {
		toi.Memo = NewOutboundMemo(toi.InHash).String()
	}
	// Ensure the InHash is set
	if toi.InHash.IsEmpty() {
		toi.InHash = common.BlankTxID
	}
	if toi.ToAddress.IsEmpty() {
		return outputs, fmt.Errorf("empty to address, can't send out")
	}
	if !toi.ToAddress.IsChain(toi.Chain) {
		return outputs, fmt.Errorf("to address(%s), is not of chain(%s)", toi.ToAddress, toi.Chain)
	}

	// BCH chain will convert legacy address to new format automatically , thus when observe it back can't be associated with the original inbound
	// so here convert the legacy address to new format
	if toi.Chain.Equals(common.BCHChain) {
		newBCHAddress, err := common.ConvertToNewBCHAddressFormatV83(toi.ToAddress)
		if err != nil {
			return outputs, fmt.Errorf("fail to convert BCH address to new format: %w", err)
		}
		if newBCHAddress.IsEmpty() {
			return outputs, fmt.Errorf("empty to address , can't send out")
		}
		toi.ToAddress = newBCHAddress
	}

	// ensure amount is rounded to appropriate decimals
	toiPool, err := tos.keeper.GetPool(ctx, toi.Coin.Asset.GetLayer1Asset())
	if err != nil {
		return nil, fmt.Errorf("fail to get pool for txout manager: %w", err)
	}

	signingTransactionPeriod := tos.constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	transactionFeeRune := tos.gasManager.GetFee(ctx, toi.Chain, common.RuneAsset())
	transactionFeeAsset := tos.gasManager.GetFee(ctx, toi.Chain, toi.Coin.Asset)
	maxGasAsset, err := tos.gasManager.GetMaxGas(ctx, toi.Chain)
	if err != nil {
		ctx.Logger().Error("fail to get max gas asset", "error", err)
	}
	if toi.Chain.Equals(common.THORChain) {
		outputs = append(outputs, toi)
	} else {
		if !toi.VaultPubKey.IsEmpty() {
			// a vault is already manually selected, blindly go forth with that
			outputs = append(outputs, toi)
		} else {
			// THORNode don't have a vault already selected to send from, discover one.
			// List all pending outbounds for the asset, this will be used
			// to deduct balances of vaults that have outstanding txs assigned
			pendingOutbounds := tos.getPendingOutbounds(ctx, toi.Coin.Asset)
			// ///////////// COLLECT YGGDRASIL VAULTS ///////////////////////////
			// When deciding which Yggdrasil pool will send out our tx out, we
			// should consider which ones observed the inbound request tx, as
			// yggdrasil pools can go offline. Here THORNode get the voter record and
			// only consider Yggdrasils where their observed saw the "correct"
			// tx.

			activeNodeAccounts, err := tos.keeper.ListActiveValidators(ctx)
			if err != nil {
				ctx.Logger().Error("fail to get all active node accounts", "error", err)
			}
			yggs := make(Vaults, 0)
			if len(activeNodeAccounts) > 0 {
				voter, err := tos.keeper.GetObservedTxInVoter(ctx, toi.InHash)
				if err != nil {
					return nil, fmt.Errorf("fail to get observed tx voter: %w", err)
				}
				tx := voter.GetTx(activeNodeAccounts)

				// collect yggdrasil pools is going to get a list of yggdrasil
				// vault that THORChain can used to send out fund
				yggs, err = tos.collectYggdrasilPools(ctx, tx, toi.Chain.GetGasAsset())
				if err != nil {
					return nil, fmt.Errorf("fail to collect yggdrasil pool: %w", err)
				}
				for i := range yggs {
					// deduct the value of any assigned pending outbounds
					yggs[i] = tos.deductVaultPendingOutbounds(yggs[i], pendingOutbounds)
				}
				yggs = yggs.SortBy(toi.Coin.Asset)
			}
			// //////////////////////////////////////////////////////////////

			// ///////////// COLLECT ACTIVE ASGARD VAULTS ///////////////////
			active, err := tos.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
			if err != nil {
				ctx.Logger().Error("fail to get active vaults", "error", err)
			}

			for i := range active {
				// deduct the value of any assigned pending outbounds
				active[i] = tos.deductVaultPendingOutbounds(active[i], pendingOutbounds)
			}
			asgards := tos.keeper.SortBySecurity(ctx, active, signingTransactionPeriod)
			// //////////////////////////////////////////////////////////////

			// ///////////// COLLECT RETIRING ASGARD VAULTS /////////////////
			retiring, err := tos.keeper.GetAsgardVaultsByStatus(ctx, RetiringVault)
			if err != nil {
				ctx.Logger().Error("fail to get retiring vaults", "error", err)
			}
			for i := range retiring {
				// deduct the value of any assigned pending outbounds
				retiring[i] = tos.deductVaultPendingOutbounds(retiring[i], pendingOutbounds)
			}
			retiringAsgards := tos.keeper.SortBySecurity(ctx, retiring, signingTransactionPeriod)

			// //////////////////////////////////////////////////////////////

			// iterate over discovered vaults and find vaults to send funds from

			// evaluate the outputs if we process yggs first
			outputs, remaining = tos.discoverOutbounds(ctx, transactionFeeAsset, maxGasAsset, toi, append(yggs, asgards...))
			// evaluate the outputs if we process active asgards first
			outputsB, remainingB := tos.discoverOutbounds(ctx, transactionFeeAsset, maxGasAsset, toi, append(asgards, yggs...))

			// pick the output plan that has less outbound transactions to reduce on gas fees to the user
			if len(outputs) > len(outputsB) && remaining.GTE(remainingB) {
				outputs = outputsB
				remaining = remainingB
			}

			// most of the time , there is no retiring vaults, thus only apply the logic when retiring vaults are available
			if len(retiringAsgards) > 0 {
				// evaluate the outputs if we process it using retiring asgards only
				outputsC, remainingC := tos.discoverOutbounds(ctx, transactionFeeAsset, maxGasAsset, toi, retiringAsgards)

				// use retiring asgards if we cannot satisfy with active ones, or if we can satisfy in fewer outbounds
				if (len(outputs) == 0 || len(outputs) > len(outputsC)) && remaining.GTE(remainingC) {
					outputs = outputsC
					remaining = remainingC
				}
			}

			// Check we found enough funds to satisfy the request, error if we didn't
			if !remaining.IsZero() {
				return nil, fmt.Errorf("insufficient funds for outbound request: %s %s remaining", toi.ToAddress.String(), remaining.String())
			}
		}
	}
	var finalOutput []TxOutItem
	var pool Pool
	var feeEvents []*EventFee
	finalRuneFee := cosmos.ZeroUint()
	for i := range outputs {
		if outputs[i].MaxGas.IsEmpty() {
			maxGasCoin, err := tos.gasManager.GetMaxGas(ctx, outputs[i].Chain)
			if err != nil {
				return nil, fmt.Errorf("fail to get max gas coin: %w", err)
			}
			outputs[i].MaxGas = common.Gas{
				maxGasCoin,
			}
			// THOR Chain doesn't need to have max gas
			if outputs[i].MaxGas.IsEmpty() && !outputs[i].Chain.Equals(common.THORChain) {
				return nil, fmt.Errorf("max gas cannot be empty: %s", outputs[i].MaxGas)
			}
			outputs[i].GasRate = int64(tos.gasManager.GetGasRate(ctx, outputs[i].Chain).Uint64())
		}

		runeFee := transactionFeeRune // Fee is the prescribed fee

		// Deduct OutboundTransactionFee from TOI and add to Reserve
		memo, err := ParseMemoWithTHORNames(ctx, tos.keeper, outputs[i].Memo)
		if err == nil && !memo.IsType(TxYggdrasilFund) && !memo.IsType(TxYggdrasilReturn) && !memo.IsType(TxMigrate) && !memo.IsType(TxRagnarok) {
			if outputs[i].Coin.Asset.IsRune() {
				if outputs[i].Coin.Amount.LTE(transactionFeeRune) {
					runeFee = outputs[i].Coin.Amount // Fee is the full amount
				}
				finalRuneFee = finalRuneFee.Add(runeFee)
				outputs[i].Coin.Amount = common.SafeSub(outputs[i].Coin.Amount, runeFee)
				fee := common.NewFee(common.Coins{common.NewCoin(outputs[i].Coin.Asset, runeFee)}, cosmos.ZeroUint())
				feeEvents = append(feeEvents, NewEventFee(outputs[i].InHash, fee, cosmos.ZeroUint()))
			} else {
				if pool.IsEmpty() {
					var err error
					pool, err = tos.keeper.GetPool(ctx, toi.Coin.Asset.GetLayer1Asset()) // Get pool
					if err != nil {
						// the error is already logged within kvstore
						return nil, fmt.Errorf("fail to get pool: %w", err)
					}
				}

				// if pool units is zero, no asset fee is taken
				if !pool.GetPoolUnits().IsZero() {
					assetFee := transactionFeeAsset
					if outputs[i].Coin.Amount.LTE(assetFee) {
						assetFee = outputs[i].Coin.Amount // Fee is the full amount
					}

					outputs[i].Coin.Amount = common.SafeSub(outputs[i].Coin.Amount, assetFee) // Deduct Asset fee
					if outputs[i].Coin.Asset.IsSyntheticAsset() {
						// burn the synth asset which used to pay for fee, that's only required when the synth is sending from asgard
						if outputs[i].ModuleName == "" || outputs[i].ModuleName == AsgardName {
							if err := tos.keeper.SendFromModuleToModule(ctx,
								AsgardName,
								ModuleName,
								common.NewCoins(common.NewCoin(outputs[i].Coin.Asset, assetFee))); err != nil {
								ctx.Logger().Error("fail to move synth asset fee from asgard to Module", "error", err)
							} else if err := tos.keeper.BurnFromModule(ctx, ModuleName, common.NewCoin(outputs[i].Coin.Asset, assetFee)); err != nil {
								ctx.Logger().Error("fail to burn synth asset", "error", err)
							}
						}
					}
					var poolDeduct cosmos.Uint
					runeFee = pool.RuneDisbursementForAssetAdd(assetFee)
					if runeFee.GT(pool.BalanceRune) {
						poolDeduct = pool.BalanceRune
					} else {
						poolDeduct = runeFee
					}
					finalRuneFee = finalRuneFee.Add(poolDeduct)
					if !outputs[i].Coin.Asset.IsSyntheticAsset() {
						pool.BalanceAsset = pool.BalanceAsset.Add(assetFee) // Add Asset fee to Pool
					}
					pool.BalanceRune = common.SafeSub(pool.BalanceRune, poolDeduct) // Deduct Rune from Pool
					fee := common.NewFee(common.Coins{common.NewCoin(outputs[i].Coin.Asset, assetFee)}, poolDeduct)
					feeEvents = append(feeEvents, NewEventFee(outputs[i].InHash, fee, cosmos.ZeroUint()))
				}
			}
		}

		// when it is ragnarok , the network doesn't charge fee , however if the output asset is gas asset,
		// then the amount of max gas need to be taken away from the customer , otherwise the vault will be insolvent and doesn't
		// have enough to fulfill outbound
		// Also the MaxGas has not put back to pool ,so there is no need to subside pool when ragnarok is in progress
		if memo.IsType(TxRagnarok) && outputs[i].Coin.Asset.IsGasAsset() {
			gasAmt := outputs[i].MaxGas.ToCoins().GetCoin(outputs[i].Coin.Asset).Amount
			outputs[i].Coin.Amount = common.SafeSub(outputs[i].Coin.Amount, gasAmt)
		}
		// When we request Yggdrasil pool to return the fund, the coin field is actually empty
		// Signer when it sees an tx out item with memo "yggdrasil-" it will query the account on relevant chain
		// and coin field will be filled there, thus we have to let this one go
		if outputs[i].Coin.IsEmpty() && !memo.IsType(TxYggdrasilReturn) {
			ctx.Logger().Info("tx out item has zero coin", "tx_out", outputs[i].String())

			// Need to determinate whether the outbound is triggered by a withdrawal request
			// if the outbound is trigger by withdrawal request, and emit asset is not enough to pay for the fee
			// this need to return with an error , thus handler_withdraw can restore LP's LPUnits
			// and also the fee event will not be emitted
			if !outputs[i].InHash.IsEmpty() && !outputs[i].InHash.Equals(common.BlankTxID) {
				inboundVoter, err := tos.keeper.GetObservedTxInVoter(ctx, outputs[i].InHash)
				if err != nil {
					ctx.Logger().Error("fail to get observed txin voter", "error", err)
					continue
				}
				if inboundVoter.Tx.IsEmpty() {
					continue
				}
				inboundMemo, err := ParseMemoWithTHORNames(ctx, tos.keeper, inboundVoter.Tx.Tx.Memo)
				if err != nil {
					ctx.Logger().Error("fail to parse inbound transaction memo", "error", err)
					continue
				}
				if inboundMemo.IsType(TxWithdraw) {
					return nil, errors.New("tx out item has zero coin")
				}
			}
			continue
		}

		// If the outbound coin is synthetic, respecting decimals is unnecessary
		// and leaves unburnt synths in the Pool Module
		if !outputs[i].Coin.Asset.IsSyntheticAsset() {
			// sanity check: ensure outbound amount respect asset decimals
			outputs[i].Coin.Amount = cosmos.RoundToDecimal(outputs[i].Coin.Amount, toiPool.Decimals)
		}

		if !outputs[i].InHash.Equals(common.BlankTxID) {
			// increment out number of out tx for this in tx
			voter, err := tos.keeper.GetObservedTxInVoter(ctx, outputs[i].InHash)
			if err != nil {
				return nil, fmt.Errorf("fail to get observed tx voter: %w", err)
			}
			voter.FinalisedHeight = ctx.BlockHeight()
			voter.Actions = append(voter.Actions, outputs[i])
			tos.keeper.SetObservedTxInVoter(ctx, voter)
		}

		finalOutput = append(finalOutput, outputs[i])
	}

	if !pool.IsEmpty() {
		if err := tos.keeper.SetPool(ctx, pool); err != nil { // Set Pool
			return nil, fmt.Errorf("fail to save pool: %w", err)
		}
	}
	for _, feeEvent := range feeEvents {
		if err := tos.eventMgr.EmitFeeEvent(ctx, feeEvent); err != nil {
			ctx.Logger().Error("fail to emit fee event", "error", err)
		}
	}
	if !finalRuneFee.IsZero() {
		if toi.ModuleName == BondName {
			if err := tos.keeper.AddBondFeeToReserve(ctx, finalRuneFee); err != nil {
				ctx.Logger().Error("fail to add bond fee to reserve", "error", err)
			}
		} else {
			if err := tos.keeper.AddPoolFeeToReserve(ctx, finalRuneFee); err != nil {
				ctx.Logger().Error("fail to add pool fee to reserve", "error", err)
			}
		}
	}

	return finalOutput, nil
}

func (tos *TxOutStorageV98) addToBlockOut(ctx cosmos.Context, mgr Manager, item TxOutItem, outboundHeight int64) error {
	// if we're sending native assets, transfer them now and return
	if item.Chain.IsTHORChain() {
		return tos.nativeTxOut(ctx, mgr, item)
	}

	vault, err := tos.keeper.GetVault(ctx, item.VaultPubKey)
	if err != nil {
		ctx.Logger().Error("fail to get vault", "error", err)
	}
	memo, _ := ParseMemo(mgr.GetVersion(), item.Memo) // ignore err
	labels := []metrics.Label{
		telemetry.NewLabel("vault_type", vault.Type.String()),
		telemetry.NewLabel("pubkey", item.VaultPubKey.String()),
		telemetry.NewLabel("memo_type", memo.GetType().String()),
	}
	telemetry.SetGaugeWithLabels([]string{"thornode", "vault", "out_txn"}, float32(1), labels)

	if err := tos.eventMgr.EmitEvent(ctx, NewEventScheduledOutbound(item)); err != nil {
		ctx.Logger().Error("fail to emit scheduled outbound event", "error", err)
	}

	return tos.keeper.AppendTxOut(ctx, outboundHeight, item)
}

func (tos *TxOutStorageV98) CalcTxOutHeight(ctx cosmos.Context, version semver.Version, toi TxOutItem) (int64, error) {
	// non-outbound transactions are skipped. This is so this code does not
	// affect internal transactions (ie consolidation and migrate txs)
	memo, _ := ParseMemo(version, toi.Memo) // ignore err
	if !memo.IsType(TxRefund) && !memo.IsType(TxOutbound) {
		return ctx.BlockHeight(), nil
	}

	minTxOutVolumeThreshold, err := tos.keeper.GetMimir(ctx, constants.MinTxOutVolumeThreshold.String())
	if minTxOutVolumeThreshold <= 0 || err != nil {
		minTxOutVolumeThreshold = tos.constAccessor.GetInt64Value(constants.MinTxOutVolumeThreshold)
	}
	minVolumeThreshold := cosmos.NewUint(uint64(minTxOutVolumeThreshold))
	txOutDelayRate, err := tos.keeper.GetMimir(ctx, constants.TxOutDelayRate.String())
	if txOutDelayRate <= 0 || err != nil {
		txOutDelayRate = tos.constAccessor.GetInt64Value(constants.TxOutDelayRate)
	}
	txOutDelayMax, err := tos.keeper.GetMimir(ctx, constants.TxOutDelayMax.String())
	if txOutDelayMax <= 0 || err != nil {
		txOutDelayMax = tos.constAccessor.GetInt64Value(constants.TxOutDelayMax)
	}
	maxTxOutOffset, err := tos.keeper.GetMimir(ctx, constants.MaxTxOutOffset.String())
	if maxTxOutOffset <= 0 || err != nil {
		maxTxOutOffset = tos.constAccessor.GetInt64Value(constants.MaxTxOutOffset)
	}

	// if volume threshold is zero
	if minVolumeThreshold.IsZero() || txOutDelayRate == 0 {
		return ctx.BlockHeight(), nil
	}

	// get txout item value in rune
	runeValue := toi.Coin.Amount
	if !toi.Coin.Asset.IsRune() {
		pool, err := tos.keeper.GetPool(ctx, toi.Coin.Asset.GetLayer1Asset())
		if err != nil {
			ctx.Logger().Error("fail to get pool for appending txout item", "error", err)
			return ctx.BlockHeight() + maxTxOutOffset, err
		}
		runeValue = pool.AssetValueInRune(toi.Coin.Amount)
	}

	// sum value of scheduled txns (including this one)
	sumValue := runeValue
	for height := ctx.BlockHeight() + 1; height <= ctx.BlockHeight()+txOutDelayMax; height++ {
		value, err := tos.keeper.GetTxOutValue(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			continue
		}
		if height > ctx.BlockHeight()+maxTxOutOffset && value.IsZero() {
			// we've hit our max offset, and an empty block, we can assume the
			// rest will be empty as well
			break
		}
		sumValue = sumValue.Add(value)
	}
	// reduce delay rate relative to the total scheduled value. In high volume
	// scenarios, this causes the network to send outbound transactions slower,
	// giving the community & NOs time to analyze and react. In an attack
	// scenario, the attacker is likely going to move as much value as possible
	// (as we've seen in the past). The act of doing this will slow down their
	// own transaction(s), reducing the attack's effectiveness.
	txOutDelayRate -= int64(sumValue.Uint64()) / minTxOutVolumeThreshold
	if txOutDelayRate < 1 {
		txOutDelayRate = 1
	}

	// calculate the minimum number of blocks in the future the txn has to be
	minBlocks := int64(runeValue.Uint64()) / txOutDelayRate
	// min shouldn't be anything longer than the max txout offset
	if minBlocks > maxTxOutOffset {
		minBlocks = maxTxOutOffset
	}
	targetBlock := ctx.BlockHeight() + minBlocks

	// find targetBlock that has space for new txout item.
	count := int64(0)
	for count < txOutDelayMax { // max set 1 day into the future
		txOutValue, err := tos.keeper.GetTxOutValue(ctx, targetBlock)
		if err != nil {
			ctx.Logger().Error("fail to get txOutValue for block height", "error", err)
			break
		}
		if txOutValue.IsZero() {
			// the txout has no outbound txns, let's use this one
			break
		}
		if txOutValue.Add(runeValue).LTE(minVolumeThreshold) {
			// the txout + this txout item has enough space to fit, lets use this one
			break
		}
		targetBlock++
		count++
	}

	return targetBlock, nil
}

func (tos *TxOutStorageV98) nativeTxOut(ctx cosmos.Context, mgr Manager, toi TxOutItem) error {
	addr, err := cosmos.AccAddressFromBech32(toi.ToAddress.String())
	if err != nil {
		return err
	}

	if toi.ModuleName == "" {
		toi.ModuleName = AsgardName
	}

	// mint if we're sending from THORChain module
	if toi.ModuleName == ModuleName {
		if err := tos.keeper.MintToModule(ctx, toi.ModuleName, toi.Coin); err != nil {
			return fmt.Errorf("fail to mint coins during txout: %w", err)
		}
	}

	polAddress, err := tos.keeper.GetModuleAddress(ReserveName)
	if err != nil {
		ctx.Logger().Error("fail to get from address", "err", err)
		return err
	}

	// send funds from module
	if polAddress.Equals(toi.ToAddress) {
		sdkErr := tos.keeper.SendFromModuleToModule(ctx, toi.ModuleName, ReserveName, common.NewCoins(toi.Coin))
		if sdkErr != nil {
			return errors.New(sdkErr.Error())
		}
	} else {
		sdkErr := tos.keeper.SendFromModuleToAccount(ctx, toi.ModuleName, addr, common.NewCoins(toi.Coin))
		if sdkErr != nil {
			return errors.New(sdkErr.Error())
		}
	}

	from, err := tos.keeper.GetModuleAddress(toi.ModuleName)
	if err != nil {
		ctx.Logger().Error("fail to get from address", "err", err)
		return err
	}
	outboundTxFee, err := tos.keeper.GetMimir(ctx, constants.OutboundTransactionFee.String())
	if outboundTxFee < 0 || err != nil {
		outboundTxFee = tos.constAccessor.GetInt64Value(constants.OutboundTransactionFee)
	}

	tx := common.NewTx(
		common.BlankTxID,
		from,
		toi.ToAddress,
		common.Coins{toi.Coin},
		common.Gas{common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(outboundTxFee)))},
		toi.Memo,
	)

	active, err := tos.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active vaults", "err", err)
		return err
	}

	if len(active) == 0 {
		return fmt.Errorf("dev error: no pubkey for native txn")
	}

	observedTx := ObservedTx{
		ObservedPubKey: active[0].PubKey,
		BlockHeight:    ctx.BlockHeight(),
		Tx:             tx,
		FinaliseHeight: ctx.BlockHeight(),
	}
	m, err := processOneTxIn(ctx, mgr.GetVersion(), tos.keeper, observedTx, tos.keeper.GetModuleAccAddress(AsgardName))
	if err != nil {
		ctx.Logger().Error("fail to process txOut", "error", err, "tx", tx.String())
		return err
	}

	handler := NewInternalHandler(mgr)

	_, err = handler(ctx, m)
	if err != nil {
		ctx.Logger().Error("TxOut Handler failed:", "error", err)
		return err
	}

	return nil
}

// collectYggdrasilPools is to get all the yggdrasil vaults , that THORChain can used to send out fund
func (tos *TxOutStorageV98) collectYggdrasilPools(ctx cosmos.Context, tx ObservedTx, gasAsset common.Asset) (Vaults, error) {
	// collect yggdrasil pools
	var vaults Vaults
	iterator := tos.keeper.GetVaultIterator(ctx)
	defer func() {
		if err := iterator.Close(); err != nil {
			ctx.Logger().Error("fail to close vault iterator", "error", err)
		}
	}()
	for ; iterator.Valid(); iterator.Next() {
		var vault Vault
		if err := tos.keeper.Cdc().Unmarshal(iterator.Value(), &vault); err != nil {
			return nil, fmt.Errorf("fail to unmarshal vault: %w", err)
		}
		if !vault.IsYggdrasil() {
			continue
		}
		// When trying to choose a ygg pool candidate to send out fund , let's
		// make sure the ygg pool has gasAsset , for example, if it is
		// on Binance chain , make sure ygg pool has BNB asset in it ,
		// otherwise it won't be able to pay the transaction fee
		if !vault.HasAsset(gasAsset) {
			continue
		}

		// if THORNode are already sending assets from this ygg pool, deduct them.
		addr, err := vault.PubKey.GetThorAddress()
		if err != nil {
			return nil, fmt.Errorf("fail to get thor address from pub key(%s):%w", vault.PubKey, err)
		}

		// if the ygg pool didn't observe the TxIn, and didn't sign the TxIn,
		// THORNode is not going to choose them to send out fund , because they
		// might offline
		if !tx.HasSigned(addr) {
			continue
		}

		jail, err := tos.keeper.GetNodeAccountJail(ctx, addr)
		if err != nil {
			return nil, fmt.Errorf("fail to get ygg jail:%w", err)
		}
		if jail.IsJailed(ctx) {
			continue
		}

		vaults = append(vaults, vault)
	}

	return vaults, nil
}

// getPendingOutbounds only deduct the delayed outbound , it doesn't need to consider already scheduled but not sent outbound
func (tos *TxOutStorageV98) getPendingOutbounds(ctx cosmos.Context, asset common.Asset) []TxOutItem {
	// There is no need to go back SigningTransactionPeriod blocks to check pending outbound , as the logic is already in place
	// in keeper_vault.go SortBySecurity
	startHeight := ctx.BlockHeight()
	if startHeight < 1 {
		startHeight = 1
	}
	txOutDelayMax, err := tos.keeper.GetMimir(ctx, constants.TxOutDelayMax.String())
	if txOutDelayMax <= 0 || err != nil {
		txOutDelayMax = tos.constAccessor.GetInt64Value(constants.TxOutDelayMax)
	}
	maxTxOutOffset, err := tos.keeper.GetMimir(ctx, constants.MaxTxOutOffset.String())
	if maxTxOutOffset <= 0 || err != nil {
		maxTxOutOffset = tos.constAccessor.GetInt64Value(constants.MaxTxOutOffset)
	}
	var outbounds []TxOutItem
	for height := startHeight; height <= ctx.BlockHeight()+txOutDelayMax; height++ {
		blockOut, err := tos.keeper.GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get block tx out", "error", err)
		}
		if height > ctx.BlockHeight()+maxTxOutOffset && len(blockOut.TxArray) == 0 {
			// we've hit our max offset, and an empty block, we can assume the
			// rest will be empty as well
			break
		}
		for _, txOutItem := range blockOut.TxArray {
			// only need to look at outbounds for the same asset
			if !txOutItem.Coin.Asset.Equals(asset) {
				continue
			}
			// only still outstanding txout will be considered
			if !txOutItem.OutHash.IsEmpty() {
				continue
			}
			outbounds = append(outbounds, txOutItem)
		}
	}
	return outbounds
}

func (tos *TxOutStorageV98) deductVaultPendingOutbounds(vault Vault, pendingOutbounds []TxOutItem) Vault {
	for _, txOutItem := range pendingOutbounds {
		if !txOutItem.VaultPubKey.Equals(vault.PubKey) {
			continue
		}
		// only still outstanding txout will be considered
		if !txOutItem.OutHash.IsEmpty() {
			continue
		}
		// deduct the gas asset from the vault as well
		var gasCoin common.Coin
		if !txOutItem.MaxGas.IsEmpty() {
			gasCoin = txOutItem.MaxGas.ToCoins().GetCoin(txOutItem.Chain.GetGasAsset())
		}
		for i, yggCoin := range vault.Coins {
			if yggCoin.Asset.Equals(txOutItem.Coin.Asset) {
				vault.Coins[i].Amount = common.SafeSub(vault.Coins[i].Amount, txOutItem.Coin.Amount)
			}
			if yggCoin.Asset.Equals(gasCoin.Asset) {
				vault.Coins[i].Amount = common.SafeSub(vault.Coins[i].Amount, gasCoin.Amount)
			}
		}
	}
	return vault
}
