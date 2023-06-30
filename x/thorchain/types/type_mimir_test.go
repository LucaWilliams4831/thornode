package types

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
	. "gopkg.in/check.v1"
)

type MimirTestSuite struct{}

var _ = Suite(&MimirTestSuite{})

func (MimirTestSuite) TestNodeMimir(c *C) {
	m := NodeMimirs{}
	acc1 := GetRandomBech32Addr()
	acc2 := GetRandomBech32Addr()
	acc3 := GetRandomBech32Addr()
	active := []cosmos.AccAddress{acc1, acc2, acc3}
	key1 := "foo"
	key2 := "bar"
	key3 := "baz"

	m.Set(key1, 1, acc1)
	m.Set(key1, 1, acc2)
	m.Set(key1, 1, acc3)
	m.Set(key2, 1, acc1)
	m.Set(key2, 2, acc2)
	m.Set(key1, 3, acc3)
	m.Set(key3, 4, acc1)
	m.Set(key3, 5, acc2)
	m.Set(key3, 5, acc3)

	// test key1
	val, ok := m.HasSuperMajority(key1, active)
	c.Check(val, Equals, int64(1))
	c.Check(ok, Equals, true)
	val, ok = m.HasSimpleMajority(key1, active)
	c.Check(val, Equals, int64(1))
	c.Check(ok, Equals, true)

	// test key2
	_, ok = m.HasSuperMajority(key2, active)
	c.Check(ok, Equals, false)
	_, ok = m.HasSimpleMajority(key2, active)
	c.Check(ok, Equals, false)

	// test key3
	val, ok = m.HasSuperMajority(key3, active)
	c.Check(val, Equals, int64(5))
	c.Check(ok, Equals, true)
	val, ok = m.HasSimpleMajority(key3, active)
	c.Check(val, Equals, int64(5))
	c.Check(ok, Equals, true)
}
