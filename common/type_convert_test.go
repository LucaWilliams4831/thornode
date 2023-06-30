package common

import (
	"math/big"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

type TypeConvertTestSuite struct{}

var _ = Suite(&TypeConvertTestSuite{})

func (TypeConvertTestSuite) TestSafeSub(c *C) {
	input1 := cosmos.NewUint(1)
	input2 := cosmos.NewUint(2)

	result1 := SafeSub(input2, input2)
	result2 := SafeSub(input1, input2)
	result3 := SafeSub(input2, input1)

	c.Check(result1.Equal(cosmos.ZeroUint()), Equals, true, Commentf("%d", result1.Uint64()))
	c.Check(result2.Equal(cosmos.ZeroUint()), Equals, true, Commentf("%d", result2.Uint64()))
	c.Check(result3.Equal(cosmos.NewUint(1)), Equals, true, Commentf("%d", result3.Uint64()))
	c.Check(result3.Equal(input2.Sub(input1)), Equals, true, Commentf("%d", result3.Uint64()))
}

func (TypeConvertTestSuite) TestSafeDivision(c *C) {
	input1 := cosmos.NewUint(1)
	input2 := cosmos.NewUint(2)
	total := input1.Add(input2)
	allocation := cosmos.NewUint(100000000)

	result1 := GetUncappedShare(input1, total, allocation)
	c.Check(result1.Equal(cosmos.NewUint(33333333)), Equals, true, Commentf("%d", result1.Uint64()))

	result2 := GetUncappedShare(input2, total, allocation)
	c.Check(result2.Equal(cosmos.NewUint(66666667)), Equals, true, Commentf("%d", result2.Uint64()))

	result3 := GetUncappedShare(cosmos.ZeroUint(), total, allocation)
	c.Check(result3.Equal(cosmos.ZeroUint()), Equals, true, Commentf("%d", result3.Uint64()))

	result4 := GetUncappedShare(input1, cosmos.ZeroUint(), allocation)
	c.Check(result4.Equal(cosmos.ZeroUint()), Equals, true, Commentf("%d", result4.Uint64()))

	result5 := GetUncappedShare(input1, total, cosmos.ZeroUint())
	c.Check(result5.Equal(cosmos.ZeroUint()), Equals, true, Commentf("%d", result5.Uint64()))

	result6 := GetUncappedShare(cosmos.NewUint(1014), cosmos.NewUint(3), cosmos.NewUint(1000_000*One))
	c.Check(result6.Equal(cosmos.NewUint(33799999999999997)), Equals, true, Commentf("%s", result6.String()))
}

func (TypeConvertTestSuite) TestGetUncappedShare(c *C) {
	x := cosmos.NewUint(0)
	data := []byte{
		0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30,
		0x30, 0x30,
		0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30,
		0x30, 0x30,
	}
	z2 := new(big.Int)
	z2.SetBytes(data)
	c.Log(z2.String())
	y := cosmos.NewUintFromBigInt(z2)
	share := GetUncappedShare(y, cosmos.NewUint(10000), x)
	c.Assert(share.IsZero(), Equals, true)
}
