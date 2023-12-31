package constants

import (
	"testing"

	. "gopkg.in/check.v1"
)

func TestPackage(t *testing.T) { TestingT(t) }

type ConstantsSuite struct{}

var _ = Suite(&ConstantsSuite{})

func (s *ConstantsSuite) Test010(c *C) {
	consts := NewConstantValue()
	c.Check(consts.GetInt64Value(PoolCycle), Equals, int64(43200))
}
