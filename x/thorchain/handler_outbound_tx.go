package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type OutboundTxHandler struct {
	ch  CommonOutboundTxHandler
	mgr Manager
}

func NewOutboundTxHandler(mgr Manager) OutboundTxHandler {
	return OutboundTxHandler{
		ch:  NewCommonOutboundTxHandler(mgr),
		mgr: mgr,
	}
}

func (h OutboundTxHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgOutboundTx)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgOutboundTx failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to handle MsgOutboundTx", "error", err)
	}
	return result, err
}

func (h OutboundTxHandler) validate(ctx cosmos.Context, msg MsgOutboundTx) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h OutboundTxHandler) validateV1(ctx cosmos.Context, msg MsgOutboundTx) error {
	return msg.ValidateBasic()
}

func (h OutboundTxHandler) handle(ctx cosmos.Context, msg MsgOutboundTx) (*cosmos.Result, error) {
	if !msg.Tx.Tx.ID.Equals(common.BlankTxID) {
		ctx.Logger().Info("receive MsgOutboundTx", "request outbound tx hash", msg.Tx.Tx.ID)
	} else {
		ctx.Logger().Debug("receive MsgOutboundTx", "request outbound tx hash", msg.Tx.Tx.ID)
	}
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	return nil, errBadVersion
}

func (h OutboundTxHandler) handleV1(ctx cosmos.Context, msg MsgOutboundTx) (*cosmos.Result, error) {
	return h.ch.handle(ctx, msg.Tx, msg.InTxID)
}
