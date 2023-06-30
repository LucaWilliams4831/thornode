package bitcoincash

import (
	"encoding/base64"
	"fmt"
	"math/big"

	"github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/gcash/bchd/bchec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/thornode/bifrost/tss"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// TssSignable is a signable implementation backed by tss
type TssSignable struct {
	poolPubKey    common.PubKey
	tssKeyManager tss.ThorchainKeyManager
	logger        zerolog.Logger
}

// NewTssSignable create a new instance of TssSignable
func NewTssSignable(pubKey common.PubKey, manager tss.ThorchainKeyManager) (*TssSignable, error) {
	return &TssSignable{
		poolPubKey:    pubKey,
		tssKeyManager: manager,
		logger:        log.Logger.With().Str("module", "tss_signable").Logger(),
	}, nil
}

// SignECDSA signs the given payload using ECDSA
func (ts *TssSignable) SignECDSA(payload []byte) (*bchec.Signature, error) {
	ts.logger.Info().Msgf("msg to sign: %s", base64.StdEncoding.EncodeToString(payload))
	result, _, err := ts.tssKeyManager.RemoteSign(payload, ts.poolPubKey.String())
	if err != nil {
		return nil, err
	}
	var sig bchec.Signature
	sig.R = new(big.Int).SetBytes(result[:32])
	sig.S = new(big.Int).SetBytes(result[32:])
	// let's verify the signature
	if sig.Verify(payload, ts.GetPubKey()) {
		ts.logger.Info().Msg("we can verify the signature successfully")
	} else {
		ts.logger.Info().Msg("the signature can't be verified")
	}
	return &sig, nil
}

// SignSchnorr signs the given payload using Schnorr
func (ts *TssSignable) SignSchnorr(payload []byte) (*bchec.Signature, error) {
	return nil, fmt.Errorf("schnorr signature not yet implemented in TSS")
}

func (ts *TssSignable) GetPubKey() *bchec.PublicKey {
	cpk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeAccPub, ts.poolPubKey.String())
	if err != nil {
		ts.logger.Err(err).Str("pubkey", ts.poolPubKey.String()).Msg("fail to get pubic key from the bech32 pool public key string")
		return nil
	}

	secpPubKey, err := codec.ToTmPubKeyInterface(cpk)
	if err != nil {
		ts.logger.Err(err).Msgf("%s is not a secp256 k1 public key", ts.poolPubKey)
		return nil
	}

	newPubkey, err := bchec.ParsePubKey(secpPubKey.Bytes(), bchec.S256())
	if err != nil {
		ts.logger.Err(err).Msg("fail to parse public key")
		return nil
	}
	return newPubkey
}
