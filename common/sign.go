package common

import (
	"encoding/base64"
	"os"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

// Sign an array of bytes.
// Returns (signature, pubkey, error)
func Sign(buf []byte) ([]byte, []byte, error) {
	kbs, err := cosmos.GetKeybase(os.Getenv(cosmos.EnvChainHome))
	if err != nil {
		return nil, nil, err
	}

	sig, pubkey, err := kbs.Keybase.Sign(kbs.SignerName, buf)
	if err != nil {
		return nil, nil, err
	}

	return sig, pubkey.Bytes(), nil
}

func SignBase64(buf []byte) (string, string, error) {
	sig, pubkey, err := Sign(buf)
	if err != nil {
		return "", "", err
	}

	return base64.StdEncoding.EncodeToString(sig),
		base64.StdEncoding.EncodeToString(pubkey), nil
}
