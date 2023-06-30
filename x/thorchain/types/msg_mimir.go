package types

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgMimir is a constructor function for MsgMimir
func NewMsgMimir(key string, value int64, signer cosmos.AccAddress) *MsgMimir {
	return &MsgMimir{
		Key:    key,
		Value:  value,
		Signer: signer,
	}
}

// Route should return the route key of the module
func (m *MsgMimir) Route() string { return RouterKey }

// Type should return the action
func (m MsgMimir) Type() string { return "set_mimir_attr" }

// ValidateBasic runs stateless checks on the message
func (m *MsgMimir) ValidateBasic() error {
	if m.Key == "" {
		return cosmos.ErrUnknownRequest("key cannot be empty")
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgMimir) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgMimir) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
