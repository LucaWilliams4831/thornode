package types

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgLoanOpen create new MsgLoan message
func NewMsgLoanOpen(owner common.Address, colAsset common.Asset, colAmount cosmos.Uint, targetAddress common.Address, asset common.Asset, minOut cosmos.Uint, affAddr common.Address, affPts cosmos.Uint, dexagg, dexTargetAddr string, dexTargetLimit cosmos.Uint, signer cosmos.AccAddress) *MsgLoanOpen {
	return &MsgLoanOpen{
		Owner:                   owner,
		CollateralAsset:         colAsset,
		CollateralAmount:        colAmount,
		TargetAddress:           targetAddress,
		TargetAsset:             asset,
		MinOut:                  minOut,
		AffiliateAddress:        affAddr,
		AffiliateBasisPoints:    affPts,
		Aggregator:              dexagg,
		AggregatorTargetAddress: dexTargetAddr,
		AggregatorTargetLimit:   dexTargetLimit,
		Signer:                  signer,
	}
}

// Route should return the router key of the module
func (m *MsgLoanOpen) Route() string { return RouterKey }

// Type should return the action
func (m MsgLoanOpen) Type() string { return "loan_open" }

// ValidateBasic runs stateless checks on the message
func (m *MsgLoanOpen) ValidateBasic() error {
	if m.Owner.IsEmpty() {
		return cosmos.ErrInvalidAddress("owner cannot be empty")
	}
	if m.CollateralAsset.IsEmpty() {
		return cosmos.ErrUnknownRequest("collateral asset cannot be empty")
	}
	if (!m.CollateralAsset.IsGasAsset() && !m.CollateralAsset.IsDerivedAsset()) || m.CollateralAsset.Equals(common.TOR) {
		return fmt.Errorf("unsupported collateral pool")
	}
	if m.CollateralAmount.IsZero() {
		return cosmos.ErrUnknownRequest("amount cannot be zero")
	}
	if !m.TargetAddress.IsChain(m.TargetAsset.Chain) {
		return cosmos.ErrUnknownRequest("target address does not match chain of target asset")
	}
	if m.TargetAddress.IsEmpty() {
		return cosmos.ErrInvalidAddress("target address cannot be empty")
	}
	if m.TargetAsset.IsEmpty() {
		return cosmos.ErrUnknownRequest("target asset cannot be empty")
	}
	if m.AffiliateAddress.IsEmpty() && !m.AffiliateBasisPoints.IsZero() {
		return cosmos.ErrUnknownRequest("affiliate address is empty while affiliate basis points is non-zero")
	}
	if !m.AffiliateAddress.IsEmpty() && !m.AffiliateAddress.IsChain(common.THORChain) {
		return cosmos.ErrUnknownRequest("swap affiliate address must be a THOR address")
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress("empty signer address")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgLoanOpen) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgLoanOpen) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}

// NewMsgLoanRepayment create new MsgLoan message
func NewMsgLoanRepayment(owner common.Address, asset common.Asset, minOut cosmos.Uint, from common.Address, coin common.Coin, signer cosmos.AccAddress) *MsgLoanRepayment {
	return &MsgLoanRepayment{
		Owner:           owner,
		CollateralAsset: asset,
		MinOut:          minOut,
		From:            from,
		Coin:            coin,
		Signer:          signer,
	}
}

// Route should return the router key of the module
func (m *MsgLoanRepayment) Route() string { return RouterKey }

// Type should return the action
func (m MsgLoanRepayment) Type() string { return "loan_repayment" }

// ValidateBasic runs stateless checks on the message
func (m *MsgLoanRepayment) ValidateBasic() error {
	if m.Owner.IsEmpty() {
		return cosmos.ErrInvalidAddress("owner cannot be empty")
	}
	if m.CollateralAsset.IsEmpty() {
		return cosmos.ErrUnknownRequest("collateral asset cannot be empty")
	}
	if m.Coin.IsEmpty() {
		return cosmos.ErrUnknownRequest("coin cannot be empty")
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress("empty signer address")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgLoanRepayment) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgLoanRepayment) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
