package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgSetNodeKeys is a constructor function for NewMsgAddNodeKeys
func NewMsgSetNodeKeys(nodePubKeySet common.PubKeySet, validatorConsPubKey string, signer cosmos.AccAddress) *MsgSetNodeKeys {
	return &MsgSetNodeKeys{
		PubKeySetSet:        nodePubKeySet,
		ValidatorConsPubKey: validatorConsPubKey,
		Signer:              signer,
	}
}

// Route should return the router key of the module
func (m *MsgSetNodeKeys) Route() string { return RouterKey }

// Type should return the action
func (m MsgSetNodeKeys) Type() string { return "set_node_keys" }

// ValidateBasic runs stateless checks on the message
func (m *MsgSetNodeKeys) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if _, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeConsPub, m.ValidatorConsPubKey); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	if m.PubKeySetSet.IsEmpty() {
		return cosmos.ErrUnknownRequest("node pub keys cannot be empty")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgSetNodeKeys) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgSetNodeKeys) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
