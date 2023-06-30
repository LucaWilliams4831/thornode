package types

import (
	"crypto/sha256"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgSolvency create a new MsgSolvency
func NewMsgSolvency(chain common.Chain, pubKey common.PubKey, coins common.Coins, height int64, signer cosmos.AccAddress) (*MsgSolvency, error) {
	input := fmt.Sprintf("%s|%s|%s|%d", chain, pubKey, coins, height)
	id, err := common.NewTxID(fmt.Sprintf("%X", sha256.Sum256([]byte(input))))
	if err != nil {
		return nil, fmt.Errorf("fail to create msg solvency hash")
	}
	return &MsgSolvency{
		Id:     id,
		Chain:  chain,
		PubKey: pubKey,
		Coins:  coins,
		Height: height,
		Signer: signer,
	}, nil
}

// Route Implements Msg.
func (m *MsgSolvency) Route() string { return RouterKey }

// Type Implements Msg.
func (m MsgSolvency) Type() string { return "solvency" }

// ValidateBasic Implements Msg.
func (m *MsgSolvency) ValidateBasic() error {
	if m.Id.IsEmpty() {
		return cosmos.ErrUnknownRequest("invalid id")
	}
	if m.Chain.IsEmpty() {
		return cosmos.ErrUnknownRequest("chain can't be empty")
	}
	if m.PubKey.IsEmpty() {
		return cosmos.ErrUnknownRequest("pubkey is empty")
	}
	if m.Height <= 0 {
		return cosmos.ErrUnknownRequest("block height is invalid")
	}
	if m.Signer.Empty() {
		return cosmos.ErrUnauthorized("invalid sender")
	}
	return nil
}

// GetSignBytes Implements Msg.
func (m *MsgSolvency) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners Implements Msg.
func (m *MsgSolvency) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
