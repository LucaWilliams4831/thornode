package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgUnBond create new MsgUnBond message
func NewMsgUnBond(txin common.Tx, nodeAddr cosmos.AccAddress, amount cosmos.Uint, bondAddress common.Address, provider, signer cosmos.AccAddress) *MsgUnBond {
	return &MsgUnBond{
		TxIn:                txin,
		NodeAddress:         nodeAddr,
		Amount:              amount,
		BondAddress:         bondAddress,
		BondProviderAddress: provider,
		Signer:              signer,
	}
}

// Route should return the router key of the module
func (m *MsgUnBond) Route() string { return RouterKey }

// Type should return the action
func (m MsgUnBond) Type() string { return "unbond" }

// ValidateBasic runs stateless checks on the message
func (m *MsgUnBond) ValidateBasic() error {
	if m.NodeAddress.Empty() {
		return cosmos.ErrInvalidAddress("node address cannot be empty")
	}
	if m.Amount.IsZero() {
		return cosmos.ErrUnknownRequest("unbond amount cannot be zero")
	}
	if m.BondAddress.IsEmpty() {
		return cosmos.ErrInvalidAddress("bond address cannot be empty")
	}
	// here we can't call m.TxIn.Valid , because we allow user to send unbond request without any coins in it
	// m.TxIn.Valid will reject this kind request , which result unbond to fail
	if m.TxIn.ID.IsEmpty() {
		return cosmos.ErrUnknownRequest("tx id cannot be empty")
	}
	if m.TxIn.FromAddress.IsEmpty() {
		return cosmos.ErrInvalidAddress("tx from address cannot be empty")
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress("empty signer address")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgUnBond) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgUnBond) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
