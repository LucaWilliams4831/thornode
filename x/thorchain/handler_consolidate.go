package thorchain

import (
	"context"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// ConsolidateHandler handles transactions the network sends to itself, to consolidate UTXOs
type ConsolidateHandler struct {
	mgr Manager
}

// NewConsolidateHandler create a new instance of the ConsolidateHandler
func NewConsolidateHandler(mgr Manager) ConsolidateHandler {
	return ConsolidateHandler{
		mgr: mgr,
	}
}

func (h ConsolidateHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgConsolidate)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgConsolidate failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("failed to process MsgConsolidate", "error", err)
		return nil, err
	}
	return result, nil
}

func (h ConsolidateHandler) validate(ctx cosmos.Context, msg MsgConsolidate) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h ConsolidateHandler) validateV1(ctx cosmos.Context, msg MsgConsolidate) error {
	return msg.ValidateBasic()
}

func (h ConsolidateHandler) slashV96(ctx cosmos.Context, tx ObservedTx) error {
	toSlash := make(common.Coins, len(tx.Tx.Coins))
	copy(toSlash, tx.Tx.Coins)
	toSlash = toSlash.Adds(tx.Tx.Gas.ToCoins())

	ctx = ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{ // nolint
		telemetry.NewLabel("reason", "failed_consolidation"),
		telemetry.NewLabel("chain", string(tx.Tx.Chain)),
	}))

	return h.mgr.Slasher().SlashVault(ctx, tx.ObservedPubKey, toSlash, h.mgr)
}

func (h ConsolidateHandler) handle(ctx cosmos.Context, msg MsgConsolidate) (*cosmos.Result, error) {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.96.0")):
		return h.handleV96(ctx, msg)
	case version.GTE(semver.MustParse("0.1.0")):
		return h.handleV1(ctx, msg)
	default:
		return nil, errBadVersion
	}
}

func (h ConsolidateHandler) handleV96(ctx cosmos.Context, msg MsgConsolidate) (*cosmos.Result, error) {
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
		if err := h.slashV96(ctx, msg.ObservedTx); err != nil {
			return nil, ErrInternal(err, "fail to slash account")
		}
	}

	return &cosmos.Result{}, nil
}
