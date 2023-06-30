package types

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgErrataTx is a constructor function for NewMsgErrataTx
func NewMsgErrataTx(txID common.TxID, chain common.Chain, signer cosmos.AccAddress) *MsgErrataTx {
	return &MsgErrataTx{
		TxID:   txID,
		Chain:  chain,
		Signer: signer,
	}
}

// Route should return the name of the module
func (m *MsgErrataTx) Route() string { return RouterKey }

// Type should return the action
func (m MsgErrataTx) Type() string { return "errata_tx" }

// ValidateBasic runs stateless checks on the message
func (m *MsgErrataTx) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.TxID.IsEmpty() {
		return cosmos.ErrUnknownRequest("Tx ID cannot be empty")
	}
	if m.Chain.IsEmpty() {
		return cosmos.ErrUnknownRequest("chain cannot be empty")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgErrataTx) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgErrataTx) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
