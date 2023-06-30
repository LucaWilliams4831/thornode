package types

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// NewMsgTssKeysignFail create a new instance of MsgTssKeysignFail message
func NewMsgTssKeysignFail(height int64, blame Blame, memo string, coins common.Coins, signer cosmos.AccAddress, pubKey common.PubKey) (*MsgTssKeysignFail, error) {
	id, err := getMsgTssKeysignFailID(blame.BlameNodes, height, memo, coins, pubKey)
	if err != nil {
		return nil, fmt.Errorf("fail to get keysign fail id:%w", err)
	}
	return &MsgTssKeysignFail{
		ID:     id,
		Height: height,
		Blame:  blame,
		Memo:   memo,
		Coins:  coins,
		Signer: signer,
		PubKey: pubKey,
	}, nil
}

// getTssKeysignFailID this method will use all the members that caused the tss
// keysign failure , as well as the block height of the txout item to generate
// a hash, given that , if the same party keep failing the same txout item ,
// then we will only slash it once.
func getMsgTssKeysignFailID(members []Node, height int64, memo string, coins common.Coins, pubKey common.PubKey) (string, error) {
	// ensure input pubkeys list is deterministically sorted
	sort.SliceStable(members, func(i, j int) bool {
		return members[i].Pubkey < members[j].Pubkey
	})
	sb := strings.Builder{}
	for _, item := range members {
		sb.WriteString(item.Pubkey)
	}
	sb.WriteString(fmt.Sprintf("%d", height))
	sb.WriteString(memo)
	sb.WriteString(pubKey.String())
	for _, c := range coins {
		sb.WriteString(c.String())
	}
	hash := sha256.New()
	_, err := hash.Write([]byte(sb.String()))
	if err != nil {
		return "", fmt.Errorf("fail to create hash id,err:%w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Route should return the route key of the module
func (m *MsgTssKeysignFail) Route() string { return RouterKey }

// Type should return the action
func (m MsgTssKeysignFail) Type() string { return "set_tss_keysign_fail" }

// ValidateBasic runs stateless checks on the message
func (m *MsgTssKeysignFail) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if len(m.ID) == 0 {
		return cosmos.ErrUnknownRequest("ID cannot be blank")
	}
	if len(m.Coins) == 0 {
		return cosmos.ErrUnknownRequest("no coins")
	}
	if err := m.Coins.Valid(); err != nil {
		return cosmos.ErrInvalidCoins(err.Error())
	}
	if m.Blame.IsEmpty() {
		return cosmos.ErrUnknownRequest("tss blame is empty")
	}
	if m.Height <= 0 {
		return cosmos.ErrUnknownRequest("block height cannot be equal to less than zero")
	}
	if len([]byte(m.Memo)) > constants.MaxMemoSize {
		err := fmt.Errorf("memo must not exceed %d bytes: %d", constants.MaxMemoSize, len([]byte(m.Memo)))
		return cosmos.ErrUnknownRequest(err.Error())
	}
	if m.PubKey.IsEmpty() {
		return cosmos.ErrUnknownRequest("vault pubkey cannot be empty")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgTssKeysignFail) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgTssKeysignFail) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
