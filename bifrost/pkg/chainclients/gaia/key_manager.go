package gaia

import (
	"encoding/hex"
	"fmt"

	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type keyManager struct {
	privKey  ctypes.PrivKey
	addr     types.AccAddress
	pubkey   common.PubKey
	mnemonic string
}

func (m *keyManager) Pubkey() common.PubKey {
	return m.pubkey
}

func (m *keyManager) ExportAsMnemonic() (string, error) {
	if m.mnemonic == "" {
		return "", fmt.Errorf("this key manager is not recover from mnemonic or auto generated ")
	}
	return m.mnemonic, nil
}

func (m *keyManager) ExportAsPrivateKey() (string, error) {
	return hex.EncodeToString(m.privKey.Bytes()), nil
}

func (m *keyManager) Sign(msg []byte) ([]byte, error) {
	return m.privKey.Sign(msg)
}

func (m *keyManager) GetPrivKey() ctypes.PrivKey {
	return m.privKey
}

func (m *keyManager) GetAddr() cosmos.AccAddress {
	return m.addr
}
