package types

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgSetVersion is a constructor function for NewMsgSetVersion
func NewMsgSetVersion(version string, signer cosmos.AccAddress) *MsgSetVersion {
	return &MsgSetVersion{
		Version: version,
		Signer:  signer,
	}
}

// Route should return the route key of the module
func (m *MsgSetVersion) Route() string { return RouterKey }

// Type should return the action
func (m MsgSetVersion) Type() string { return "set_version" }

// ValidateBasic runs stateless checks on the message
func (m *MsgSetVersion) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if _, err := semver.Make(m.Version); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	return nil
}

// GetVersion return the semantic version
func (m *MsgSetVersion) GetVersion() (semver.Version, error) {
	return semver.Make(m.Version)
}

// GetSignBytes encodes the message for signing
func (m *MsgSetVersion) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgSetVersion) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
