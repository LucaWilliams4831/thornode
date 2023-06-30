package tss

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"

	"github.com/itchio/lzma"
	"gitlab.com/thorchain/tss/go-tss/storage"
	. "gopkg.in/check.v1"
)

// -------------------------------------------------------------------------------------
// Setup
// -------------------------------------------------------------------------------------

type EncryptKeysharesSuite struct{}

var _ = Suite(&EncryptKeysharesSuite{})

const (
	LocalStateTestFile = "localstate-test.json"
	Mnemonic           = "profit used piece repeat real curtain endorse tennis tenant sentence include glass return learn upgrade apple crane polar attend before ripple doctor decrease depend"
)

// -------------------------------------------------------------------------------------
// Tests
// -------------------------------------------------------------------------------------

func (s *EncryptKeysharesSuite) TestEncryptKeysharesEmptyPassphrase(c *C) {
	ks, err := EncryptKeyshares("", "")
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "failed keyshare encrypt: signer seed phrase is not set")
	c.Assert(ks, IsNil)
}

func (s *EncryptKeysharesSuite) TestEncryptKeysharesBadMnemonic(c *C) {
	ks, err := EncryptKeyshares("", Mnemonic+" dog")
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "failed keyshare encrypt: signer seed phrase is not 24 words")
	c.Assert(ks, IsNil)

	ks, err = EncryptKeyshares("", "a b c d e f g h i j k l m n o p q r s t u v w x")
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "failed keyshare encrypt: signer seed phrase is not valid bip39 mnemonic")
	c.Assert(ks, IsNil)
}

func (s *EncryptKeysharesSuite) TestEncryptKeysharesBadMnemonicEntropy(c *C) {
	ks, err := EncryptKeyshares("", "dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog")
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "failed keyshare encrypt: signer seed phrase failed entropy check")
	c.Assert(ks, IsNil)
}

func (s *EncryptKeysharesSuite) TestEncryptKeysharesMissingFile(c *C) {
	ks, err := EncryptKeyshares("foo.json", Mnemonic)
	c.Assert(err, NotNil)
	c.Assert(strings.HasPrefix(err.Error(), "failed keyshare encrypt - cannot open key file"), Equals, true)
	c.Assert(ks, IsNil)
}

func (s *EncryptKeysharesSuite) TestEncryptKeysharesCompression(c *C) {
	// encrypt
	ks, err := EncryptKeyshares(LocalStateTestFile, Mnemonic)
	c.Assert(err, IsNil)
	c.Assert(ks, NotNil)

	// ensure we achieved expected compression ratio
	fi, err := os.Stat(LocalStateTestFile)
	c.Assert(err, IsNil)
	if float64(len(ks))/float64(fi.Size()) > 0.4 {
		c.Fatalf("compression ratio over expected: %f", float64(len(ks))/float64(fi.Size()))
	}
}

func (s *EncryptKeysharesSuite) TestEncryptKeyshares(c *C) {
	// encrypt
	ks, err := EncryptKeyshares(LocalStateTestFile, Mnemonic)
	c.Assert(err, IsNil)
	c.Assert(ks, NotNil)

	// decrypt with bad passphrase should fail
	dec, err := DecryptKeyshares(ks, Mnemonic+" y")
	c.Assert(err, NotNil)
	c.Assert(dec, IsNil)

	// decrypt with good passphrase should succeed
	dec, err = DecryptKeyshares(ks, Mnemonic)
	c.Assert(err, IsNil)
	cmpOut := bytes.NewBuffer(dec)
	out := lzma.NewReader(cmpOut)

	// decrypted value should match
	var original, decrypted storage.KeygenLocalState
	f, err := os.Open(LocalStateTestFile)
	c.Assert(err, IsNil)
	defer f.Close()
	err = json.NewDecoder(f).Decode(&original)
	c.Assert(err, IsNil)
	err = json.NewDecoder(out).Decode(&decrypted)
	c.Assert(err, IsNil)
	c.Assert(decrypted, DeepEquals, original)
}

func (s *EncryptKeysharesSuite) TestSaltAndHash(c *C) {
	hash := saltAndHash("foo", 1)
	c.Assert(len(hash), Equals, 32)

	hash2 := saltAndHash("foo", 2)
	c.Assert(len(hash2), Equals, 32)
	c.Assert(hash, Not(Equals), hash2)
}
