package types

import (
	"net"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgSetIPAddress is a constructor function for NewMsgSetIPAddress
func NewMsgSetIPAddress(ip string, signer cosmos.AccAddress) *MsgSetIPAddress {
	return &MsgSetIPAddress{
		IPAddress: ip,
		Signer:    signer,
	}
}

// Route should return the name of the module
func (m *MsgSetIPAddress) Route() string { return RouterKey }

// Type should return the action
func (m MsgSetIPAddress) Type() string { return "set_ip_address" }

// ValidateBasic runs stateless checks on the message
func (m *MsgSetIPAddress) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if net.ParseIP(m.IPAddress) == nil {
		return cosmos.ErrUnknownRequest("invalid IP address")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgSetIPAddress) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgSetIPAddress) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
