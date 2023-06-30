package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgSwitch is a constructor function for NewMsgSwitch
func NewMsgSwitch(tx common.Tx, addr common.Address, signer cosmos.AccAddress) *MsgSwitch {
	return &MsgSwitch{
		Tx:          tx,
		Destination: addr,
		Signer:      signer,
	}
}

// Route should return the route key of the module
func (m *MsgSwitch) Route() string { return RouterKey }

// Type should return the action
func (m MsgSwitch) Type() string { return "switch" }

// ValidateBasic runs stateless checks on the message
func (m *MsgSwitch) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Destination.IsEmpty() {
		return cosmos.ErrInvalidAddress(m.Destination.String())
	}
	if err := m.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	// cannot be more or less than one coin
	if len(m.Tx.Coins) != 1 {
		return cosmos.ErrInvalidCoins("must be only one coin (rune)")
	}
	if !m.Tx.Coins[0].Asset.IsRune() {
		return cosmos.ErrInvalidCoins("must be rune")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgSwitch) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgSwitch) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
