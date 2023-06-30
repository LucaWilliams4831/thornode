package tss

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"filippo.io/age"
	"github.com/cosmos/go-bip39"
	"github.com/itchio/lzma"
	"github.com/rs/zerolog/log"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/tss/go-tss/storage"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	// AgeWorkFactor is the work factor used for encryption:
	// https://pkg.go.dev/filippo.io/age#ScryptRecipient.SetWorkFactor
	// NOTE: Periodically re-evaluate and increase work factor as needed.
	AgeWorkFactor = 18

	// Salts are used before hashing the passphrase for each layer of encryption.
	SaltAge    = 1
	SaltAES    = 2
	SaltChaCha = 3
)

// EncryptKeyshares encrypts the keyshares at the provided path using the passphrase.
func EncryptKeyshares(path, passphrase string) ([]byte, error) {
	// verify the passphrase is provided and a mnemonic
	if passphrase == "" {
		return nil, errors.New("failed keyshare encrypt: signer seed phrase is not set")
	}
	if len(strings.Split(passphrase, " ")) != 24 {
		return nil, errors.New("failed keyshare encrypt: signer seed phrase is not 24 words")
	}
	if !bip39.IsMnemonicValid(passphrase) {
		return nil, errors.New("failed keyshare encrypt: signer seed phrase is not valid bip39 mnemonic")
	}

	// validate passphrase entropy for added protection
	ent := common.Entropy([]byte(passphrase))
	if ent < MinimumMnemonicEntropy {
		msg := "low mnemonic entropy detected, seek guidance from devs via direct message"
		log.Error().Float64("entropy", ent).Msg(msg)
		return nil, errors.New("failed keyshare encrypt: signer seed phrase failed entropy check")
	}

	// open keyshares
	f, err := os.Open(path)
	if err != nil {
		log.Error().Str("path", path).Msg("failed to open file")
		return nil, fmt.Errorf("failed keyshare encrypt - cannot open key file: %w", err)
	}
	defer f.Close()

	// tee reader for verification
	var teeRaw bytes.Buffer
	tr := io.TeeReader(f, &teeRaw)

	// read keyshares
	var ks storage.KeygenLocalState
	err = json.NewDecoder(tr).Decode(&ks)
	if err != nil {
		return nil, fmt.Errorf("failed keyshare encrypt - cannot decode keyshares: %w", err)
	}

	// create age recipient
	agePassphrase := hex.EncodeToString(saltAndHash(passphrase, SaltAge))
	recipient, err := age.NewScryptRecipient(agePassphrase)
	if err != nil {
		return nil, fmt.Errorf("failed keyshare encrypt - cannot create recipient: %w", err)
	}
	recipient.SetWorkFactor(AgeWorkFactor)

	// encrypt keyshares with age
	encryptedBytes := new(bytes.Buffer)
	enc, err := age.Encrypt(encryptedBytes, recipient)
	if err != nil {
		return nil, fmt.Errorf("failed keyshare encrypt - cannot create encrypt writer: %w", err)
	}

	// tee writer for verification
	var teeCompress bytes.Buffer

	// compress keyshares from raw instead of the decoded struct for determinism
	cmpEnc := lzma.NewWriterLevel(io.MultiWriter(enc, &teeCompress), lzma.BestCompression)
	n, err := cmpEnc.Write(teeRaw.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed keyshare encrypt - cannot encode keyshares: %w", err)
	}
	if n != teeRaw.Len() {
		return nil, fmt.Errorf("failed keyshare encrypt - failed to write all bytes")
	}

	// finalize
	err = cmpEnc.Close()
	if err != nil {
		return nil, fmt.Errorf("failed keyshare encrypt - failed to close compression writer: %w", err)
	}
	err = enc.Close()
	if err != nil {
		return nil, fmt.Errorf("failed keyshare encrypt - failed to close encrypt writer: %w", err)
	}

	// encryption second pass (aes256)
	doubleEncrypted, err := encryptAES(saltAndHash(passphrase, SaltAES), encryptedBytes.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed keyshare encrypt - cannot aes encrypt keyshares: %w", err)
	}

	// encryption third pass (chacha)
	chaCha, err := chacha20poly1305.NewX(saltAndHash(passphrase, SaltChaCha))
	if err != nil {
		return nil, fmt.Errorf("failed keyshare encrypt - cannot create chacha: %w", err)
	}
	tripleEncrypted, err := encryptAEAD(chaCha, doubleEncrypted)
	if err != nil {
		return nil, fmt.Errorf("failed keyshare encrypt - cannot chacha encrypt keyshares: %w", err)
	}

	// verify encrypted does not contain any 3 words of the mnnemonic, there is reasonable
	// probability that one word will be included randomly in the encrypted data
	wordCount := 0
	for _, w := range strings.Split(passphrase, " ") {
		if bytes.Contains(tripleEncrypted, []byte(w)) {
			wordCount++
		}
	}
	if wordCount >= 3 {
		return nil, errors.New("failed keyshare encrypt: encrypted keyshares contains 3+ mnemonic words")
	}

	// verify encrypted does not equal raw
	if bytes.Equal(teeRaw.Bytes(), tripleEncrypted) {
		return nil, errors.New("failed keyshare encrypt - encrypted bytes equal raw bytes")
	}

	// verify encrypted does not contain input
	if bytes.Contains(tripleEncrypted, teeRaw.Bytes()) {
		return nil, errors.New("failed keyshare encrypt: encrypted keyshares contains input")
	}

	// verify encrypted does not equal compressed
	if bytes.Equal(teeCompress.Bytes(), tripleEncrypted) {
		return nil, errors.New("failed keyshare encrypt - encrypted bytes equal compressed bytes")
	}

	// verify decrypted equals compressed
	decrypted, err := DecryptKeyshares(tripleEncrypted, passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed keyshare encrypt - cannot read decrypted: %w", err)
	}
	if !bytes.Equal(teeCompress.Bytes(), decrypted) {
		return nil, errors.New("failed keyshare encrypt - decrypted bytes do not equal compressed bytes")
	}

	// verify uncompressed equals raw
	cmpDec := lzma.NewReader(bytes.NewReader(decrypted))
	uncompressed, err := io.ReadAll(cmpDec)
	if err != nil {
		return nil, fmt.Errorf("failed keyshare encrypt - cannot decompress unencrypted: %w", err)
	}
	if !bytes.Equal(teeRaw.Bytes(), uncompressed) {
		return nil, errors.New("failed keyshare encrypt - uncompressed bytes do not equal raw bytes")
	}

	return tripleEncrypted, nil
}

// EncryptKeyshares decrypts the provided encrypted keyshares using the passphrase.
func DecryptKeyshares(encrypted []byte, passphrase string) ([]byte, error) {
	// decrypt third pass (twofish)
	chaCha, err := chacha20poly1305.NewX(saltAndHash(passphrase, SaltChaCha))
	if err != nil {
		return nil, fmt.Errorf("failed keyshare decrypt - cannot create chacha: %w", err)
	}
	doubleEncrypted, err := decryptAEAD(chaCha, encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed keyshare decrypt - cannot chacha decrypt keyshares: %w", err)
	}

	// decrypt second pass (aes256)
	encryptedBytes, err := decryptAES(saltAndHash(passphrase, SaltAES), doubleEncrypted)
	if err != nil {
		return nil, fmt.Errorf("failed keyshare decrypt - cannot aes decrypt keyshares: %w", err)
	}

	// decrypt first pass (age)
	agePassphrase := hex.EncodeToString(saltAndHash(passphrase, SaltAge))
	identity, err := age.NewScryptIdentity(agePassphrase)
	if err != nil {
		return nil, fmt.Errorf("failed keyshare decrypt - cannot create identity: %w", err)
	}
	dec, err := age.Decrypt(bytes.NewReader(encryptedBytes), identity)
	if err != nil {
		return nil, fmt.Errorf("failed keyshare decrypt - cannot decrypt: %w", err)
	}
	return io.ReadAll(dec)
}

// -------------------------------------------------------------------------------------

// saltAndHash returns a salted SHA256 hash of the provided passphrase.
func saltAndHash(passphrase string, salt int) []byte {
	salted := fmt.Sprintf("%s+%d", passphrase, salt)
	hash := sha256.Sum256([]byte(salted))
	return hash[:]
}

// -------------------------------------------------------------------------------------

// encryptAES encrypts the provided data with AES using the provided key.
func encryptAES(key, data []byte) ([]byte, error) {
	// ensure key is 32 bytes so we encrypt with AES 256
	if len(key) != 32 {
		return nil, errors.New("failed to encrypt: key must be 32 bytes")
	}

	// create the cipher
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// encrypt the data
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	return encryptAEAD(gcm, data)
}

// decryptAES decrypts the provided data using the provided key.
func decryptAES(key, data []byte) ([]byte, error) {
	// ensure key is 32 bytes so we decrypt with AES 256
	if len(key) != 32 {
		return nil, errors.New("failed to decrypt: key must be 32 bytes")
	}

	// create the cipher
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// decrypt the data
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	return decryptAEAD(gcm, data)
}

// -------------------------------------------------------------------------------------

// encryptAEAD encrypts the provided data with the nonce as a prefix.
func encryptAEAD(aead cipher.AEAD, data []byte) ([]byte, error) {
	// create a new random nonce
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// encrypt the data
	ciphertext := aead.Seal(nil, nonce, data, nil)

	// prepend the nonce to the ciphertext
	return append(nonce, ciphertext...), nil
}

// decryptAEAD decrypts the provided data using the nonce from the prefix.
func decryptAEAD(aead cipher.AEAD, data []byte) ([]byte, error) {
	// extract the nonce
	nonceSize := aead.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	// decrypt the data
	return aead.Open(nil, nonce, ciphertext, nil)
}
