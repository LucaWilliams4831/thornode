package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgRefundTx is a constructor function for MsgOutboundTx
func NewMsgRefundTx(tx ObservedTx, txID common.TxID, signer cosmos.AccAddress) *MsgRefundTx {
	return &MsgRefundTx{
		Tx:     tx,
		InTxID: txID,
		Signer: signer,
	}
}

// Route should return the route key of the module
func (m *MsgRefundTx) Route() string { return RouterKey }

// Type should return the action
func (m MsgRefundTx) Type() string { return "set_tx_refund" }

// ValidateBasic runs stateless checks on the message
func (m *MsgRefundTx) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.InTxID.IsEmpty() {
		return cosmos.ErrUnknownRequest("In Tx ID cannot be empty")
	}
	if err := m.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgRefundTx) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgRefundTx) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
