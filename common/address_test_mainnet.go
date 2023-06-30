//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package common

import (
	"github.com/blang/semver"
	. "gopkg.in/check.v1"
)

type AddressSuite struct{}

var _ = Suite(&AddressSuite{})

func (s *AddressSuite) TestAddress(c *C) {
	// bnb tests
	addr, err := NewAddress("bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")
	c.Assert(err, IsNil)
	c.Check(addr.IsEmpty(), Equals, false)
	c.Check(addr.Equals(Address("bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")), Equals, true)
	c.Check(addr.String(), Equals, "bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")
	c.Check(addr.IsChain(BNBChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BTCChain), Equals, false)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, false)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BNBChain), Equals, MainNet)

	addr, err = NewAddress("tbnb12ymaslcrhnkj0tvmecyuejdvk25k2nnurqjvyp")
	c.Check(addr.IsChain(BNBChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BTCChain), Equals, false)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, false)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BNBChain), Equals, TestNet)

	// random
	c.Check(err, IsNil)
	_, err = NewAddress("1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6")
	c.Check(err, NotNil)
	_, err = NewAddress("bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlpn6X")
	c.Check(err, NotNil)
	_, err = NewAddress("bogus")
	c.Check(err, NotNil)
	c.Check(Address("").IsEmpty(), Equals, true)
	c.Check(NoAddress.Equals(Address("")), Equals, true)
	_, err = NewAddress("")
	c.Assert(err, IsNil)

	// thor tests
	addr, err = NewAddress("thor1kljxxccrheghavaw97u78le6yy3sdj7h696nl4")
	c.Assert(err, IsNil)
	c.Check(addr.IsChain(THORChain), Equals, true)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BTCChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, false)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), THORChain), Equals, MainNet)
	addr, err = NewAddress("tthor1x6m28lezv00ugcahqv5w2eagrm9396j2gf6zjpd4auf9mv4h")
	c.Assert(err, IsNil)
	c.Check(addr.IsChain(THORChain), Equals, true)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BTCChain), Equals, false)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, false)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), THORChain), Equals, TestNet)

	// eth tests
	addr, err = NewAddress("0x90f2b1ae50e6018230e90a33f98c7844a0ab635a")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(ETHChain), Equals, true)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(BTCChain), Equals, false)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, false)
	// wrong length
	_, err = NewAddress("0x90f2b1ae50e6018230e90a33f98c7844a0ab635aaaaaaaaa")
	c.Check(err, NotNil)

	// good length but not valid hex string
	_, err = NewAddress("0x90f2b1ae50e6018230e90a33f98c7844a0ab63zz")
	c.Check(err, NotNil)

	// btc tests
	// mainnet p2pkh
	addr, err = NewAddress("1MirQ9bwyQcGVJPwKUgapu5ouK2E2Ey4gX")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(DOGEChain), Equals, false)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// tesnet p2pkh
	addr, err = NewAddress("mrX9vMRYLfVy1BnZbc5gZjuyaqH3ZW2ZHz")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(LTCChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, TestNet)

	// mainnet p2pkh
	addr, err = NewAddress("12MzCDwodF9G1e7jfwLXfR164RNtx4BRVG")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(DOGEChain), Equals, false)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// mainnet p2sh
	addr, err = NewAddress("3QJmV3qfvL9SuYo34YihAf3sRCW3qSinyC")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(DOGEChain), Equals, false)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// mainnet p2sh 2
	addr, err = NewAddress("3NukJ6fYZJ5Kk8bPjycAnruZkE5Q7UW7i8")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(DOGEChain), Equals, false)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// testnet p2sh
	addr, err = NewAddress("2NBFNJTktNa7GZusGbDbGKRZTxdK9VVez3n")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, TestNet)

	// mainnet p2pk compressed (0x02)
	addr, err = NewAddress("02192d74d0cb94344c9569c2e77901573d8d7903c3ebec3a957724895dca52c6b4")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// mainnet p2pk compressed (0x03)
	addr, err = NewAddress("03b0bd634234abbb1ba1e986e884185c61cf43e001f9137f23c2c409273eb16e65")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// mainnet p2pk uncompressed (0x04)
	addr, err = NewAddress("0411db93e1dcdb8a016b49840f8c53bc1eb68a382e97b1482ecad7b148a6909a5cb2" +
		"e0eaddfb84ccf9744464f82e160bfa9b8b64f9d4c03f999b8643f656b412a3")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// mainnet p2pk hybrid (0x06)
	addr, err = NewAddress("06192d74d0cb94344c9569c2e77901573d8d7903c3ebec3a957724895dca52c6b4" +
		"0d45264838c0bd96852662ce6a847b197376830160c6d2eb5e6a4c44d33f453e")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// mainnet p2pk hybrid (0x07)
	addr, err = NewAddress("07b0bd634234abbb1ba1e986e884185c61cf43e001f9137f23c2c409273eb16e65" +
		"37a576782eba668a7ef8bd3b3cfb1edb7117ab65129b8a2e681f3c1e0908ef7b")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// testnet p2pk compressed (0x02)
	addr, err = NewAddress("02192d74d0cb94344c9569c2e77901573d8d7903c3ebec3a957724895dca52c6b4")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// segwit mainnet p2wpkh v0
	addr, err = NewAddress("BC1QW508D6QEJXTDG4Y5R3ZARVARY0C5XW7KV8F3T4")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// segwit mainnet p2wsh v0
	addr, err = NewAddress("bc1qrp33g0q5c5txsp9arysrx4k6zdkfs4nce4xj0gdcccefvpysxf3qccfmv3")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// segwit testnet p2wpkh v0
	addr, err = NewAddress("tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, TestNet)

	// segwit testnet p2wsh witness v0
	addr, err = NewAddress("tb1qqqqqp399et2xygdj5xreqhjjvcmzhxw4aywxecjdzew6hylgvsesrxh6hy")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, TestNet)

	// segwit mainnet witness v1
	addr, err = NewAddress("bc1pw508d6qejxtdg4y5r3zarvary0c5xw7kw508d6qejxtdg4y5r3zarvary0c5xw7k7grplx")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, false)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)

	// segwit mainnet witness v16
	addr, err = NewAddress("BC1SW50QA3JX3S")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, false)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MainNet)
	addr, err = NewAddress("bcrt1qqqnde7kqe5sf96j6zf8jpzwr44dh4gkd3ehaqh")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BTCChain), Equals, MockNet)

	// segwit invalid hrp bech32 succeed but IsChain fails
	addr, err = NewAddress("tc1qw508d6qejxtdg4y5r3zarvary0c5xw7kg3g4ty")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BTCChain), Equals, false)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, false)

	// bch tests
	// testnet bech32 address
	addr, err = NewAddress("qq0y8fmkq48rt3z5dlkv87ged93ranf2ggkuz9gfl8")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(BTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, false)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), BCHChain), Equals, TestNet)

	// doge tests
	addr, err = NewAddress("DJbKker23xfz3ufxAbqUuQwp1EBibGJJHu")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(BTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), DOGEChain), Equals, MainNet)

	addr, err = NewAddress("nfWiQeddE4zsYsDuYhvpgVC7y4gjr5RyqK")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BCHChain), Equals, false)
	c.Check(addr.IsChain(LTCChain), Equals, false)
	c.Check(addr.IsChain(BTCChain), Equals, false)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), DOGEChain), Equals, TestNet)

	addr, err = NewAddress("mtyBWSzMZaCxJ1xy9apJBZzXz648BZrpJg")
	c.Check(err, IsNil)
	c.Check(addr.IsChain(BCHChain), Equals, true)
	c.Check(addr.IsChain(LTCChain), Equals, true)
	c.Check(addr.IsChain(BTCChain), Equals, true)
	c.Check(addr.IsChain(ETHChain), Equals, false)
	c.Check(addr.IsChain(BNBChain), Equals, false)
	c.Check(addr.IsChain(THORChain), Equals, false)
	c.Check(addr.IsChain(DOGEChain), Equals, true)
	c.Check(addr.GetNetwork(semver.MustParse("999.0.0"), DOGEChain), Equals, MockNet)
}

func (s *AddressSuite) TestConvertToNewBCHAddressFormat(c *C) {
	addr1 := "1EFJFJm7Y9mTVsCBXA9PKuRuzjgrdBe4rR"
	addr1Result, err := ConvertToNewBCHAddressFormatV83(Address(addr1))
	c.Assert(err, IsNil)
	c.Assert(addr1Result.IsEmpty(), Equals, false)
	c.Assert(addr1Result.String(), Equals, "qzg5mkh7rkw3y8kw47l3rrnvhmenvctmd56xg38a70")

	addr3 := "qzg5mkh7rkw3y8kw47l3rrnvhmenvctmd56xg38a70"
	addr3Result, err := ConvertToNewBCHAddressFormatV83(Address(addr3))
	c.Assert(err, IsNil)
	c.Assert(addr3Result.IsEmpty(), Equals, false)
	c.Assert(addr3Result.String(), Equals, "qzg5mkh7rkw3y8kw47l3rrnvhmenvctmd56xg38a70")

	addr4 := "18P1smBRB8zgfHT2qU9mnrbkHuinL1VRQe"
	addr4Result, err := ConvertToNewBCHAddressFormatV83(Address(addr4))
	c.Assert(err, IsNil)
	c.Assert(addr4Result.IsEmpty(), Equals, false)
	c.Assert(addr4Result.String(), Equals, "qpg09septgjye6rw6lp3wep6s7j73je2tg5sea68x9")

	addr5 := "qrwz8uegrdd08x57uxzapthc6lm4fxmnwv0apsganr"
	addr5Result, err := ConvertToNewBCHAddressFormatV83(Address(addr5))
	c.Assert(err, NotNil)
	c.Assert(addr5Result.IsEmpty(), Equals, true)

	addr6 := "whatever"
	addr6Result, err := ConvertToNewBCHAddressFormatV83(Address(addr6))
	c.Assert(err, NotNil)
	c.Assert(addr6Result.IsEmpty(), Equals, true)

	addr7 := "3PLcoeUdBbYjQ3FZ98bSBdszNfXyEK3n91"
	addr7Result, err := ConvertToNewBCHAddressFormatV83(Address(addr7))
	c.Assert(err, IsNil)
	c.Assert(addr7Result.IsEmpty(), Equals, false)
	c.Assert(addr7Result.String(), Equals, "prkhwf3etusv88eu7fekcxgce7pj0vuf4sys9u2mns")
}
