package types

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

var _ cosmos.Msg = &MsgYggdrasil{}

// NewMsgYggdrasil is a constructor function for MsgYggdrasil
func NewMsgYggdrasil(tx common.Tx, pk common.PubKey, blockHeight int64, addFunds bool, coins common.Coins, signer cosmos.AccAddress) *MsgYggdrasil {
	return &MsgYggdrasil{
		Tx:          tx,
		PubKey:      pk,
		AddFunds:    addFunds,
		Coins:       coins,
		BlockHeight: blockHeight,
		Signer:      signer,
	}
}

// Route should return the route key of the module
func (m *MsgYggdrasil) Route() string { return RouterKey }

// Type should return the action
func (m MsgYggdrasil) Type() string { return "set_yggdrasil" }

// ValidateBasic runs stateless checks on the message
func (m *MsgYggdrasil) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.PubKey.IsEmpty() {
		return cosmos.ErrUnknownRequest("pubkey cannot be empty")
	}
	if m.BlockHeight <= 0 {
		return cosmos.ErrUnknownRequest("invalid block height")
	}
	if err := m.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	if len(m.Coins) == 0 {
		return cosmos.ErrUnknownRequest("no coins")
	}
	if err := m.Coins.Valid(); err != nil {
		return cosmos.ErrInvalidCoins(err.Error())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgYggdrasil) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgYggdrasil) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
