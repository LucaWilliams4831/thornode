package types

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgManageTHORName create a new instance of MsgManageTHORName
func NewMsgManageTHORName(name string, chain common.Chain, addr common.Address, coin common.Coin, exp int64, asset common.Asset, owner, signer cosmos.AccAddress) *MsgManageTHORName {
	return &MsgManageTHORName{
		Name:              name,
		Chain:             chain,
		Address:           addr,
		Coin:              coin,
		ExpireBlockHeight: exp,
		PreferredAsset:    asset,
		Owner:             owner,
		Signer:            signer,
	}
}

// Route should return the Route of the module
func (m *MsgManageTHORName) Route() string { return RouterKey }

// Type should return the action
func (m MsgManageTHORName) Type() string { return "manage_thorname" }

// ValidateBasic runs stateless checks on the message
func (m *MsgManageTHORName) ValidateBasic() error {
	// validate n
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Chain.IsEmpty() {
		return cosmos.ErrUnknownRequest("chain can't be empty")
	}
	if m.Address.IsEmpty() {
		return cosmos.ErrUnknownRequest("address can't be empty")
	}
	if !m.Address.IsChain(m.Chain) {
		return cosmos.ErrUnknownRequest("address and chain must match")
	}
	if !m.Coin.Asset.IsNativeRune() {
		return cosmos.ErrUnknownRequest("coin must be native rune")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgManageTHORName) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgManageTHORName) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
