package types

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgMigrate is a constructor function for MsgMigrate
func NewMsgMigrate(tx ObservedTx, blockHeight int64, signer cosmos.AccAddress) *MsgMigrate {
	return &MsgMigrate{
		Tx:          tx,
		BlockHeight: blockHeight,
		Signer:      signer,
	}
}

// Route should return the name of the module
func (m *MsgMigrate) Route() string { return RouterKey }

// Type should return the action
func (m MsgMigrate) Type() string { return "migrate" }

// ValidateBasic runs stateless checks on the message
func (m *MsgMigrate) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.BlockHeight <= 0 {
		return cosmos.ErrUnknownRequest("invalid block height")
	}
	if err := m.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgMigrate) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgMigrate) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
