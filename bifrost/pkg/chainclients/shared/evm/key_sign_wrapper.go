package evm

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/cosmos/cosmos-sdk/crypto/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	etypes "github.com/ethereum/go-ethereum/core/types"
	ecrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/thornode/bifrost/tss"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func GetPrivateKey(key cryptotypes.PrivKey) (*ecdsa.PrivateKey, error) {
	return ecrypto.ToECDSA(key.Bytes())
}

// KeySignWrapper is a wrap of private key and also tss instance
type KeySignWrapper struct {
	privKey       *ecdsa.PrivateKey
	pubKey        common.PubKey
	tssKeyManager tss.ThorchainKeyManager
	logger        zerolog.Logger
	eipSigner     etypes.EIP155Signer
}

// NewKeySignWrapper create a new instance of keysign wrapper
func NewKeySignWrapper(privateKey *ecdsa.PrivateKey, pubKey common.PubKey, keyManager tss.ThorchainKeyManager, chainID *big.Int, chain string) (*KeySignWrapper, error) {
	return &KeySignWrapper{
		privKey:       privateKey,
		pubKey:        pubKey,
		tssKeyManager: keyManager,
		eipSigner:     etypes.NewEIP155Signer(chainID),
		logger:        log.With().Str("module", "signer").Str("chain", chain).Logger(),
	}, nil
}

// GetPrivKey return the private key
func (w *KeySignWrapper) GetPrivKey() *ecdsa.PrivateKey {
	return w.privKey
}

// GetPubKey return the public key
func (w *KeySignWrapper) GetPubKey() common.PubKey {
	return w.pubKey
}

// Sign the given transaction
func (w *KeySignWrapper) Sign(tx *etypes.Transaction, poolPubKey common.PubKey) ([]byte, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if poolPubKey.IsEmpty() {
		return nil, errors.New("pool public key is empty")
	}
	var err error
	var sig []byte
	if w.pubKey.Equals(poolPubKey) {
		sig, err = w.sign(tx)
	} else {
		sig, err = w.signTSS(tx, poolPubKey.String())
	}
	if err != nil {
		return nil, fmt.Errorf("fail to sign tx: %w", err)
	}
	newTx, err := tx.WithSignature(w.eipSigner, sig)
	if err != nil {
		return nil, fmt.Errorf("fail to apply signature to tx: %w", err)
	}
	enc, err := newTx.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("fail to marshal tx to json: %w", err)
	}
	return enc, nil
}

func (w *KeySignWrapper) sign(tx *etypes.Transaction) ([]byte, error) {
	hash := w.eipSigner.Hash(tx)
	return ecrypto.Sign(hash[:], w.privKey)
}

func (w *KeySignWrapper) signTSS(tx *etypes.Transaction, poolPubKey string) ([]byte, error) {
	pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeAccPub, poolPubKey)
	if err != nil {
		return nil, fmt.Errorf("fail to get pub key: %w", err)
	}

	hash := w.eipSigner.Hash(tx)
	sig, recovery, err := w.tssKeyManager.RemoteSign(hash[:], poolPubKey)
	if err != nil || sig == nil {
		return nil, fmt.Errorf("fail to TSS sign: %w", err)
	}
	secpPubKey, err := codec.ToTmPubKeyInterface(pk)
	if err != nil {
		return nil, err
	}
	if ecrypto.VerifySignature(secpPubKey.Bytes(), hash[:], sig) {
		w.logger.Info().Msg("we can successfully verify the bytes")
	} else {
		w.logger.Error().Msg("Oops! we cannot verify the bytes")
	}
	// add the recovery id at the end
	result := make([]byte, 65)
	copy(result, sig)
	result[64] = recovery[0]
	return result, nil
}
