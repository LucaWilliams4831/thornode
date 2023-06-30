package types

import (
	"errors"
	"strconv"
	"strings"

	"gitlab.com/thorchain/thornode/common"
)

// Valid check whether TxOutItem hold valid information
func (m TxOutItem) Valid() error {
	if m.Chain.IsEmpty() {
		return errors.New("chain cannot be empty")
	}
	if m.InHash.IsEmpty() {
		return errors.New("In Hash cannot be empty")
	}
	if m.ToAddress.IsEmpty() {
		return errors.New("To address cannot be empty")
	}
	if m.VaultPubKey.IsEmpty() {
		return errors.New("vault pubkey cannot be empty")
	}
	if m.GasRate == 0 {
		return errors.New("gas rate is zero")
	}
	if m.Chain.GetGasAsset().IsEmpty() {
		return errors.New("invalid base asset")
	}
	if err := m.Coin.Valid(); err != nil {
		return err
	}
	if err := m.MaxGas.Valid(); err != nil {
		return err
	}

	return nil
}

// TxHash return a hash value generated based on the TxOutItem
func (m TxOutItem) TxHash() (string, error) {
	fromAddr, err := m.VaultPubKey.GetAddress(m.Chain)
	if err != nil {
		return "", err
	}
	tx := common.Tx{
		FromAddress: fromAddr,
		ToAddress:   m.ToAddress,
		Coins:       common.Coins{m.Coin},
	}
	return tx.Hash(), nil
}

// Equals compare two tx out item
func (m TxOutItem) Equals(toi2 TxOutItem) bool {
	if !m.Chain.Equals(toi2.Chain) {
		return false
	}
	if !m.ToAddress.Equals(toi2.ToAddress) {
		return false
	}
	if !m.VaultPubKey.Equals(toi2.VaultPubKey) {
		return false
	}
	if !m.Coin.Equals(toi2.Coin) {
		return false
	}
	if !m.InHash.Equals(toi2.InHash) {
		return false
	}
	if m.Memo != toi2.Memo {
		return false
	}
	if m.GasRate != toi2.GasRate {
		return false
	}

	return true
}

// String implement stringer interface
func (m TxOutItem) String() string {
	sb := strings.Builder{}
	sb.WriteString("To Address:" + m.ToAddress.String())
	sb.WriteString("Asset:" + m.Coin.Asset.String())
	sb.WriteString("Amount:" + m.Coin.Amount.String())
	sb.WriteString("Memo:" + m.Memo)
	sb.WriteString("GasRate:" + strconv.FormatInt(m.GasRate, 10))
	return sb.String()
}

// NewTxOut create a new item ot TxOut
func NewTxOut(height int64) *TxOut {
	return &TxOut{
		Height:  height,
		TxArray: make([]TxOutItem, 0),
	}
}

// IsEmpty to determinate whether there are txitm in this TxOut
func (m *TxOut) IsEmpty() bool {
	return len(m.TxArray) == 0
}

// Valid check every item in it's internal txarray, return an error if it is not valid
func (m *TxOut) Valid() error {
	for _, tx := range m.TxArray {
		if err := tx.Valid(); err != nil {
			return err
		}
	}
	return nil
}
