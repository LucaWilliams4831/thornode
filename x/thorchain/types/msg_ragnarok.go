package types

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgRagnarok is a constructor function for MsgRagnarok
func NewMsgRagnarok(tx ObservedTx, blockHeight int64, signer cosmos.AccAddress) *MsgRagnarok {
	return &MsgRagnarok{
		Tx:          tx,
		BlockHeight: blockHeight,
		Signer:      signer,
	}
}

// Route should return the name of the module
func (m *MsgRagnarok) Route() string { return RouterKey }

// Type should return the action
func (m MsgRagnarok) Type() string { return "ragnarok" }

// ValidateBasic runs stateless checks on the message
func (m *MsgRagnarok) ValidateBasic() error {
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
func (m *MsgRagnarok) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgRagnarok) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
