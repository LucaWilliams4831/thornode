package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

// RefundHandler a handle to process tx that had refund memo
// usually this type or tx is because Thorchain fail to process the tx, which result in a refund, signer honour the tx and refund customer accordingly
type RefundHandler struct {
	ch  CommonOutboundTxHandler
	mgr Manager
}

// NewRefundHandler create a new refund handler
func NewRefundHandler(mgr Manager) RefundHandler {
	return RefundHandler{
		ch:  NewCommonOutboundTxHandler(mgr),
		mgr: mgr,
	}
}

// Run is the main entry point to process refund outbound message
func (h RefundHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgRefundTx)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgRefund", "tx ID", msg.InTxID.String())
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgRefund fail validation", "error", err)
		return nil, err
	}

	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process MsgRefund", "error", err)
	}
	return result, err
}

func (h RefundHandler) validate(ctx cosmos.Context, msg MsgRefundTx) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h RefundHandler) validateV1(ctx cosmos.Context, msg MsgRefundTx) error {
	return msg.ValidateBasic()
}

func (h RefundHandler) handle(ctx cosmos.Context, msg MsgRefundTx) (*cosmos.Result, error) {
	return h.ch.handle(ctx, msg.Tx, msg.InTxID)
}
