package types

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// NewMsgDeposit is a constructor function for NewMsgDeposit
func NewMsgDeposit(coins common.Coins, memo string, signer cosmos.AccAddress) *MsgDeposit {
	return &MsgDeposit{
		Coins:  coins,
		Memo:   memo,
		Signer: signer,
	}
}

// Route should return the route key of the module
func (m *MsgDeposit) Route() string { return RouterKey }

// Type should return the action
func (m MsgDeposit) Type() string { return "deposit" }

// ValidateBasic runs stateless checks on the message
func (m *MsgDeposit) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	for _, coin := range m.Coins {
		if !coin.IsNative() {
			return cosmos.ErrUnknownRequest("all coins must be native to THORChain")
		}
	}
	if len([]byte(m.Memo)) > constants.MaxMemoSize {
		err := fmt.Errorf("memo must not exceed %d bytes: %d", constants.MaxMemoSize, len([]byte(m.Memo)))
		return cosmos.ErrUnknownRequest(err.Error())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgDeposit) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgDeposit) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
