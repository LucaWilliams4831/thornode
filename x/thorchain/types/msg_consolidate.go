package types

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgConsolidate is a constructor function for MsgConsolidate
func NewMsgConsolidate(observedTx ObservedTx, signer cosmos.AccAddress) *MsgConsolidate {
	return &MsgConsolidate{
		ObservedTx: observedTx,
		Signer:     signer,
	}
}

// Route should return the pooldata of the module
func (m *MsgConsolidate) Route() string { return RouterKey }

// Type should return the action
func (m MsgConsolidate) Type() string { return "consolidate" }

// ValidateBasic runs stateless checks on the message
func (m *MsgConsolidate) ValidateBasic() error {
	if err := m.ObservedTx.Valid(); err != nil {
		return cosmos.ErrInvalidCoins(err.Error())
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgConsolidate) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgConsolidate) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
