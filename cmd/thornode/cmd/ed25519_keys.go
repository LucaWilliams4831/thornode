package cmd

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/99designs/keyring"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bech32 "github.com/cosmos/cosmos-sdk/types/bech32/legacybech32" // nolint SA1019 deprecated
	"github.com/cosmos/go-bip39"
	"github.com/decred/dcrd/dcrec/edwards"
	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/crypto/ed25519"

	"gitlab.com/thorchain/thornode/app"
)

const (
	DefaultEd25519KeyName           = `THORChain-ED25519`
	ThorchainDefaultBIP39PassPhrase = "thorchain"
	BIP44Prefix                     = "44'/931'/"
	PartialPath                     = "0'/0/0"
	FullPath                        = BIP44Prefix + PartialPath
)

func GetEd25519Keys() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ed25519",
		Short: "Generate an ed25519 keys",
		Long:  ``,
		Args:  cobra.ExactArgs(0),
		RunE:  ed25519Keys,
	}
	return cmd
}

func ed25519Keys(cmd *cobra.Command, args []string) error {
	buf := bufio.NewReader(cmd.InOrStdin())
	password, err := input.GetPassword("Enter password", buf)
	if err != nil {
		return err
	}
	db, err := keyring.Open(getKeyringConfig(sdk.KeyringServiceName(), app.DefaultNodeHome(), password))
	if err != nil {
		return fmt.Errorf("fail to open key store: %w", err)
	}
	item, err := db.Get(DefaultEd25519KeyName)
	if err != nil {
		// create new one
		if errors.Is(err, keyring.ErrKeyNotFound) {
			newItem, err := generateNewKey(buf)
			if err != nil {
				return fmt.Errorf("fail to create a new ED25519 key: %w", err)
			}
			if err := db.Set(*newItem); err != nil {
				return fmt.Errorf("fail to save ED25519 key: %w", err)
			}
			item = *newItem
		} else {
			return fmt.Errorf("fail to get ED25519 key : %w", err)
		}
	}
	// now we test the ed25519 key can sign and verify
	_, pk, err := edwards.PrivKeyFromScalar(edwards.Edwards(), item.Data)
	if err != nil {
		return fmt.Errorf("fail to parse private key")
	}
	pkey := ed25519.PubKey(pk.Serialize())
	tmp, err := codec.FromTmPubKeyInterface(pkey)
	if err != nil {
		return fmt.Errorf("fail to get ED25519 key : %w", err)
	}
	// nolint
	pubKey, err := bech32.MarshalPubKey(bech32.AccPK, tmp)
	if err != nil {
		return fmt.Errorf("fail generate bech32 account pub key")
	}
	fmt.Println(pubKey)
	return nil
}

func generateNewKey(buf *bufio.Reader) (*keyring.Item, error) {
	mnemonic, err := input.GetString("Enter mnemonic", buf)
	if err != nil {
		return nil, fmt.Errorf("fail to get mnemonic: %w", err)
	}
	data, err := mnemonicToEddKey(mnemonic, "")
	if err != nil {
		return nil, fmt.Errorf("fail to generate ed25519 keys: %w", err)
	}
	item := &keyring.Item{
		Key:                         DefaultEd25519KeyName,
		Data:                        data,
		Label:                       "ed25519-private-key",
		Description:                 "ed25519-private-key",
		KeychainNotTrustApplication: false,
		KeychainNotSynchronizable:   false,
	}

	return item, nil
}

func getKeyringConfig(appName, dir, password string) keyring.Config {
	return keyring.Config{
		ServiceName:     appName,
		AllowedBackends: []keyring.BackendType{keyring.FileBackend},
		FileDir:         dir,
		FilePasswordFunc: func(_ string) (string, error) {
			return password, nil
		},
	}
}

func i64(key, data []byte) (il, ir [32]byte) {
	mac := hmac.New(sha512.New, key)
	// sha512 does not err
	_, _ = mac.Write(data)
	i := mac.Sum(nil)
	copy(il[:], i[:32])
	copy(ir[:], i[32:])
	return
}

func uint32ToBytes(i uint32) []byte {
	b := [4]byte{}
	binary.BigEndian.PutUint32(b[:], i)
	return b[:]
}

func addScalars(a, b []byte) [32]byte {
	aInt := new(big.Int).SetBytes(a)
	bInt := new(big.Int).SetBytes(b)
	sInt := new(big.Int).Add(aInt, bInt)
	x := sInt.Mod(sInt, edwards.Edwards().N).Bytes()
	x2 := [32]byte{}
	copy(x2[32-len(x):], x)
	return x2
}

func derivePrivateKey(privKeyBytes, chainCode [32]byte, index uint32, harden bool) ([32]byte, [32]byte) {
	var data []byte
	if harden {
		index |= 0x80000000
		data = append([]byte{byte(0)}, privKeyBytes[:]...)
	} else {
		// this can't return an error:
		_, ecPub, err := edwards.PrivKeyFromScalar(edwards.Edwards(), privKeyBytes[:])
		if err != nil {
			panic("it should not fail")
		}
		pubKeyBytes := ecPub.SerializeCompressed()
		data = pubKeyBytes
	}
	data = append(data, uint32ToBytes(index)...)
	data2, chainCode2 := i64(chainCode[:], data)
	x := addScalars(privKeyBytes[:], data2[:])
	return x, chainCode2
}

func derivePrivateKeyForPath(privKeyBytes, chainCode [32]byte, path string) ([32]byte, error) {
	data := privKeyBytes
	parts := strings.Split(path, "/")
	for _, part := range parts {
		// do we have an apostrophe?
		harden := part[len(part)-1:] == "'"
		// harden == private derivation, else public derivation:
		if harden {
			part = part[:len(part)-1]
		}
		idx, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return [32]byte{}, fmt.Errorf("invalid BIP 32 path: %s", err)
		}
		if idx < 0 {
			return [32]byte{}, errors.New("invalid BIP 32 path: index negative ot too large")
		}
		data, chainCode = derivePrivateKey(data, chainCode, uint32(idx), harden)
	}
	var derivedKey [32]byte
	n := copy(derivedKey[:], data[:])
	if n != 32 || len(data) != 32 {
		return [32]byte{}, fmt.Errorf("expected a (secp256k1) key of length 32, got length: %v", len(data))
	}

	return derivedKey, nil
}

func mnemonicToEddKey(mnemonic, masterSecret string) ([]byte, error) {
	words := strings.Split(mnemonic, " ")
	if len(words) != 12 && len(words) != 24 {
		return nil, errors.New("mnemonic length should either be 12 or 24")
	}
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, ThorchainDefaultBIP39PassPhrase)
	if err != nil {
		return nil, err
	}
	masterPriv, ch := i64([]byte(masterSecret), seed)
	derivedPriv, err := derivePrivateKeyForPath(masterPriv, ch, FullPath)
	if err != nil {
		return nil, err
	}
	return derivedPriv[:], nil
}
