package types

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgNetworkFee create a new instance of MsgNetworkFee
func NewMsgNetworkFee(blockHeight int64, chain common.Chain, transactionSize, transactionFeeRate uint64, signer cosmos.AccAddress) *MsgNetworkFee {
	return &MsgNetworkFee{
		BlockHeight:        blockHeight,
		Chain:              chain,
		TransactionSize:    transactionSize,
		TransactionFeeRate: transactionFeeRate,
		Signer:             signer,
	}
}

// Route should return the Route of the module
func (m *MsgNetworkFee) Route() string { return RouterKey }

// Type should return the action
func (m MsgNetworkFee) Type() string { return "set_network_fee" }

// ValidateBasic runs stateless checks on the message
func (m *MsgNetworkFee) ValidateBasic() error {
	if m.BlockHeight <= 0 {
		return cosmos.ErrUnknownRequest("block height can't be negative, or zero")
	}
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Chain.IsEmpty() {
		return cosmos.ErrUnknownRequest("chain can't be empty")
	}
	if m.TransactionSize <= 0 {
		return cosmos.ErrUnknownRequest("invalid transaction size")
	}
	if m.TransactionFeeRate <= 0 {
		return cosmos.ErrUnknownRequest("invalid transaction fee rate")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgNetworkFee) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgNetworkFee) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
