package common

import (
	"fmt"
	"testing"

	. "gopkg.in/check.v1"
)

type EncryptionSuite struct{}

var _ = Suite(&EncryptionSuite{})

func (s *EncryptionSuite) TestEncryption(c *C) {
	body := []byte("hello world!")
	passphrase := "my super secret password!"

	encryp, err := Encrypt(body, passphrase)
	c.Assert(err, IsNil)

	decryp, err := Decrypt(encryp, passphrase)
	c.Assert(err, IsNil)

	c.Check(body, DeepEquals, decryp)
}

func BenchmarkEncrypt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		body := fmt.Sprintf("hello world! %d", i)
		passphrase := fmt.Sprintf("my super secret password! %d", i)
		result, err := Encrypt([]byte(body), passphrase)
		if err != nil {
			fmt.Println(err)
			b.FailNow()
		}
		decryptResult, err := Decrypt(result, passphrase)
		if err != nil {
			fmt.Println(err)
			b.FailNow()
		}
		_ = decryptResult
	}
}
