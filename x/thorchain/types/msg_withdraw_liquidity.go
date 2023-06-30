package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// MaxWithdrawBasisPoints basis points for withdrawals
const MaxWithdrawBasisPoints = 10_000

// NewMsgWithdrawLiquidity is a constructor function for MsgWithdrawLiquidity
func NewMsgWithdrawLiquidity(tx common.Tx, withdrawAddress common.Address, withdrawBasisPoints cosmos.Uint, asset, withdrawalAsset common.Asset, signer cosmos.AccAddress) *MsgWithdrawLiquidity {
	return &MsgWithdrawLiquidity{
		Tx:              tx,
		WithdrawAddress: withdrawAddress,
		BasisPoints:     withdrawBasisPoints,
		Asset:           asset,
		WithdrawalAsset: withdrawalAsset,
		Signer:          signer,
	}
}

// Route should return the route key of the module
func (m *MsgWithdrawLiquidity) Route() string { return RouterKey }

// Type should return the action
func (m MsgWithdrawLiquidity) Type() string { return "withdraw" }

// ValidateBasic runs stateless checks on the message
func (m *MsgWithdrawLiquidity) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	// here we can't call m.Tx.Valid , because we allow user to send withdraw request without any coins in it
	// m.Tx.Valid will reject this kind request , which result withdraw to fail
	if m.Tx.ID.IsEmpty() {
		return cosmos.ErrInvalidAddress("tx id cannot be empty")
	}
	if m.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("pool asset cannot be empty")
	}
	if m.Asset.IsRune() {
		return cosmos.ErrUnknownRequest("asset cannot be rune")
	}
	if m.WithdrawAddress.IsEmpty() {
		return cosmos.ErrUnknownRequest("address cannot be empty")
	}
	if m.BasisPoints.IsZero() {
		return cosmos.ErrUnknownRequest("basis points can't be zero")
	}
	if m.BasisPoints.GT(cosmos.NewUint(MaxWithdrawBasisPoints)) {
		return cosmos.ErrUnknownRequest("basis points is larger than maximum withdraw basis points")
	}
	if !m.WithdrawalAsset.IsEmpty() && !m.WithdrawalAsset.IsRune() && !m.WithdrawalAsset.Equals(m.Asset) {
		return cosmos.ErrUnknownRequest("withdrawal asset must be empty, rune, or pool asset")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgWithdrawLiquidity) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgWithdrawLiquidity) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
