package thorchain

import (
	"context"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h ConsolidateHandler) slashV1(ctx cosmos.Context, tx ObservedTx) error {
	toSlash := tx.Tx.Coins.Adds(tx.Tx.Gas.ToCoins())

	ctx = ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{ // nolint
		telemetry.NewLabel("reason", "failed_consolidation"),
		telemetry.NewLabel("chain", string(tx.Tx.Chain)),
	}))

	return h.mgr.Slasher().SlashVault(ctx, tx.ObservedPubKey, toSlash, h.mgr)
}

func (h ConsolidateHandler) handleV1(ctx cosmos.Context, msg MsgConsolidate) (*cosmos.Result, error) {
	shouldSlash := false

	// ensure transaction is sending to/from same address
	if !msg.ObservedTx.Tx.FromAddress.Equals(msg.ObservedTx.Tx.ToAddress) {
		shouldSlash = true
	}

	vault, err := h.mgr.Keeper().GetVault(ctx, msg.ObservedTx.ObservedPubKey)
	if err != nil {
		ctx.Logger().Error("unable to get vault for consolidation", "error", err)
	} else { // nolint
		if !vault.IsAsgard() {
			shouldSlash = true
		}
	}

	if shouldSlash {
		ctx.Logger().Info("slash vault, invalid consolidation", "tx", msg.ObservedTx.Tx)
		if err := h.slashV1(ctx, msg.ObservedTx); err != nil {
			return nil, ErrInternal(err, "fail to slash account")
		}
	}

	return &cosmos.Result{}, nil
}
