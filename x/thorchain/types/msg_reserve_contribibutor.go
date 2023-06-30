package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgReserveContributor is a constructor function for MsgReserveContributor
func NewMsgReserveContributor(tx common.Tx, contrib ReserveContributor, signer cosmos.AccAddress) *MsgReserveContributor {
	return &MsgReserveContributor{
		Tx:          tx,
		Contributor: contrib,
		Signer:      signer,
	}
}

// Route return the route key of module
func (m *MsgReserveContributor) Route() string { return RouterKey }

// Type return a unique action
func (m MsgReserveContributor) Type() string { return "set_reserve_contributor" }

// ValidateBasic runs stateless checks on the message
func (m *MsgReserveContributor) ValidateBasic() error {
	if err := m.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if err := m.Contributor.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgReserveContributor) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgReserveContributor) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
