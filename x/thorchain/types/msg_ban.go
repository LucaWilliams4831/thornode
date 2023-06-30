package types

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

var _ cosmos.Msg = &MsgBan{}

// NewMsgBan is a constructor function for NewMsgBan
func NewMsgBan(addr, signer cosmos.AccAddress) *MsgBan {
	return &MsgBan{
		NodeAddress: addr,
		Signer:      signer,
	}
}

// Route should return the name of the module
func (m *MsgBan) Route() string { return RouterKey }

// Type should return the action
func (m MsgBan) Type() string { return "ban" }

// ValidateBasic runs stateless checks on the message
func (m *MsgBan) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.NodeAddress.Empty() {
		return cosmos.ErrInvalidAddress(m.NodeAddress.String())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgBan) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners return all the signer who signed this message
func (m *MsgBan) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
