package types

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

var _ cosmos.Msg = &MsgNodePauseChain{}

// NewMsgNodePauseChain is a constructor function for NewMsgNodePauseChain
func NewMsgNodePauseChain(val int64, signer cosmos.AccAddress) *MsgNodePauseChain {
	return &MsgNodePauseChain{
		Value:  val,
		Signer: signer,
	}
}

// Route should return the name of the module
func (m *MsgNodePauseChain) Route() string { return RouterKey }

// Type should return the action
func (m MsgNodePauseChain) Type() string { return "node_pause_chain" }

// ValidateBasic runs stateless checks on the message
func (m *MsgNodePauseChain) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgNodePauseChain) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners return all the signer who signed this message
func (m *MsgNodePauseChain) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
