package thorchain

import (
	"context"
	"errors"
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

func (h YggdrasilHandler) slashV1(ctx cosmos.Context, pk common.PubKey, coins common.Coins) error {
	return h.mgr.Slasher().SlashVault(ctx, pk, coins, h.mgr)
}

func (h YggdrasilHandler) handleV1(ctx cosmos.Context, msg MsgYggdrasil) (*cosmos.Result, error) {
	// update txOut record with our TxID that sent funds out of the pool
	txOut, err := h.mgr.Keeper().GetTxOut(ctx, msg.BlockHeight)
	if err != nil {
		return nil, ErrInternal(err, "unable to get txOut record")
	}

	shouldSlash := true

	// if ygg is returning funds, don't slash if they are sending to asgard vault
	if !msg.AddFunds {
		// check if the node account is active, it shouldn't be
		na, err := h.mgr.Keeper().GetNodeAccountByPubKey(ctx, msg.PubKey)
		if err != nil {
			ctx.Logger().Error("fail to get node account", "error", err)
		}
		if na.Status != NodeActive {
			active, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
			if err != nil {
				ctx.Logger().Error("fail to get vaults", "error", err)
			}
			retiring, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
			if err != nil {
				ctx.Logger().Error("fail to get vaults", "error", err)
			}
			for _, v := range append(active, retiring...) {
				addr, err := v.PubKey.GetAddress(msg.Tx.Chain)
				if err != nil {
					ctx.Logger().Error("fail to get address from pubkey", "error", err)
				}
				if !addr.IsEmpty() && addr.Equals(msg.Tx.ToAddress) {
					shouldSlash = false
					break
				}
			}
		}
	}

	for i, tx := range txOut.TxArray {
		// yggdrasil is the memo used by thorchain to identify fund migration
		// to a yggdrasil vault.
		// it use yggdrasil+/-:{block height} to mark a tx out caused by vault
		// rotation
		// this type of tx out is special , because it doesn't have relevant tx
		// in to trigger it, it is trigger by thorchain itself.
		fromAddress, _ := tx.VaultPubKey.GetAddress(tx.Chain)

		if tx.InHash.Equals(common.BlankTxID) &&
			tx.OutHash.IsEmpty() &&
			tx.ToAddress.Equals(msg.Tx.ToAddress) &&
			fromAddress.Equals(msg.Tx.FromAddress) {

			matchCoin := msg.Tx.Coins.Equals(common.Coins{tx.Coin})
			// when outbound is gas asset
			if !matchCoin && tx.Coin.Asset.Equals(tx.Chain.GetGasAsset()) {
				asset := tx.Chain.GetGasAsset()
				intendToSpend := tx.Coin.Amount.Add(tx.MaxGas.ToCoins().GetCoin(asset).Amount)
				actualSpend := msg.Tx.Coins.GetCoin(asset).Amount.Add(msg.Tx.Gas.ToCoins().GetCoin(asset).Amount)
				if intendToSpend.Equal(actualSpend) {
					maxGasAmt := tx.MaxGas.ToCoins().GetCoin(asset).Amount
					realGasAmt := msg.Tx.Gas.ToCoins().GetCoin(asset).Amount
					if maxGasAmt.GTE(realGasAmt) {
						ctx.Logger().Info("override match coin", "intend to spend", intendToSpend, "actual spend", actualSpend)
						matchCoin = true
					}
				}
			}

			// only need to check the coin if yggdrasil+
			if msg.AddFunds && !matchCoin {
				continue
			}

			txOut.TxArray[i].OutHash = msg.Tx.ID
			shouldSlash = false
			if err := h.mgr.Keeper().SetTxOut(ctx, txOut); nil != err {
				ctx.Logger().Error("fail to save tx out", "error", err)
			}
			break
		}
	}

	if shouldSlash {
		ctx.Logger().Info("slash node account, no matched tx out item", "outbound tx", msg.Tx)
		toSlash := msg.Tx.Coins.Adds(msg.Tx.Gas.ToCoins())

		slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
			telemetry.NewLabel("reason", "failed_yggdrasil_return"),
			telemetry.NewLabel("chain", string(msg.Tx.Chain)),
		}))

		if err := h.slashV1(slashCtx, msg.PubKey, toSlash); err != nil {
			return nil, ErrInternal(err, "fail to slash account")
		}
	}

	vault, err := h.mgr.Keeper().GetVault(ctx, msg.PubKey)
	if err != nil && !errors.Is(err, kvTypes.ErrVaultNotFound) {
		return nil, fmt.Errorf("fail to get yggdrasil: %w", err)
	}
	if vault.IsType(UnknownVault) {
		vault.Status = ActiveVault
		vault.Type = YggdrasilVault
	}

	if err := h.mgr.Keeper().SetLastSignedHeight(ctx, msg.BlockHeight); err != nil {
		ctx.Logger().Info("fail to update last signed height", "error", err)
	}

	if msg.AddFunds {
		return h.handleYggdrasilFundV1(ctx, msg, vault)
	}
	return h.handleYggdrasilReturnV1(ctx, msg, vault)
}
