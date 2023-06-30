package wasm

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Route implements sdk.Msg
func (msg MsgExecuteContract) Route() string {
	return "wasm"
}

// Type implements sdk.Msg
func (msg MsgExecuteContract) Type() string {
	return "execute_contract"
}

// ValidateBasic implements sdk.Msg
func (msg MsgExecuteContract) ValidateBasic() error {
	return nil
}

// GetSignBytes implements sdk.Msg
func (msg MsgExecuteContract) GetSignBytes() []byte {
	amino := codec.NewLegacyAmino()
	ModuleCdc := codec.NewAminoCodec(amino)
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

// GetSigners implements sdk.Msg
func (msg MsgExecuteContract) GetSigners() []sdk.AccAddress {
	sender, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		// NOTE: This shouldn't be reached since this function is only necessary to implement
		// sdk.Msg to register the type for serialization.
		panic(err)
	}

	return []sdk.AccAddress{sender}
}
