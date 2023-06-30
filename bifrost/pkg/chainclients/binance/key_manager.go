package binance

// This file is largely a copy from https://github.com/binance-chain/go-sdk/blob/515ede99ef1b6c7b5eaf27c67fa7984d98be58e3/keys/keys.go.
// Needed a manual way to set `privKey` which the original source doesn't give a means to do so

import (
	"encoding/hex"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	ctypes "gitlab.com/thorchain/binance-sdk/common/types"
	"gitlab.com/thorchain/binance-sdk/keys"
	"gitlab.com/thorchain/binance-sdk/types/tx"

	"gitlab.com/thorchain/thornode/common"
)

type keyManager struct {
	privKey  cryptotypes.PrivKey
	addr     ctypes.AccAddress
	pubkey   common.PubKey
	mnemonic string
}

func (m *keyManager) Pubkey() common.PubKey {
	return m.pubkey
}

func (m *keyManager) ExportAsMnemonic() (string, error) {
	if m.mnemonic == "" {
		return "", fmt.Errorf("This key manager is not recover from mnemonic or anto generated ")
	}
	return m.mnemonic, nil
}

func (m *keyManager) ExportAsPrivateKey() (string, error) {
	return hex.EncodeToString(m.privKey.Bytes()), nil
}

func (m *keyManager) ExportAsKeyStore(password string) (*keys.EncryptedKeyJSON, error) {
	// Do nothing
	return nil, nil
}

func (m *keyManager) Sign(msg tx.StdSignMsg) ([]byte, error) {
	sig, err := m.makeSignature(msg)
	if err != nil {
		return nil, err
	}
	newTx := tx.NewStdTx(msg.Msgs, []tx.StdSignature{sig}, msg.Memo, msg.Source, msg.Data)
	bz, err := tx.Cdc.MarshalBinaryLengthPrefixed(&newTx)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

func (m *keyManager) GetPrivKey() cryptotypes.PrivKey {
	return m.privKey
}

func (m *keyManager) GetAddr() ctypes.AccAddress {
	return m.addr
}

func (m *keyManager) makeSignature(msg tx.StdSignMsg) (sig tx.StdSignature, err error) {
	if err != nil {
		return
	}
	sigBytes, err := m.privKey.Sign(msg.Bytes())
	if err != nil {
		return
	}
	// this return the pubkey type which is extend protob.message
	pubKey := m.privKey.PubKey()
	// this convert the protobuf based pubkey back to the old version tendermint pubkey
	tmPubKey, err := codec.ToTmPubKeyInterface(pubKey)
	if err != nil {
		return
	}
	return tx.StdSignature{
		AccountNumber: msg.AccountNumber,
		Sequence:      msg.Sequence,
		PubKey:        tmPubKey,
		Signature:     sigBytes,
	}, nil
}
