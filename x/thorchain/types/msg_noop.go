package types

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgNoOp is a constructor function for MsgNoOp
func NewMsgNoOp(observedTx ObservedTx, signer cosmos.AccAddress, action string) *MsgNoOp {
	return &MsgNoOp{
		ObservedTx: observedTx,
		Signer:     signer,
		Action:     action,
	}
}

// Route should return the pooldata of the module
func (m *MsgNoOp) Route() string { return RouterKey }

// Type should return the action
func (m MsgNoOp) Type() string { return "set_noop" }

// ValidateBasic runs stateless checks on the message
func (m *MsgNoOp) ValidateBasic() error {
	if err := m.ObservedTx.Valid(); err != nil {
		return cosmos.ErrInvalidCoins(err.Error())
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgNoOp) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgNoOp) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
