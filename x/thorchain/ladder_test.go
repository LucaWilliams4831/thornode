package thorchain

import (
	"github.com/blang/semver"

	. "gopkg.in/check.v1"
)

type LadderSuite struct{}

var _ = Suite(&LadderSuite{})

func (s *LadderSuite) TestCorrectOrder(c *C) {
	callCount100 := 0
	callCount010 := 0
	callCount001 := 0

	l := LadderDispatch[func()]{}.
		Register("1.0.0", func() { callCount100++ }).
		Register("0.1.0", func() { callCount010++ }).
		Register("0.0.1", func() { callCount001++ })

	c.Assert(len(l.versions), Equals, 3)
	c.Assert(len(l.handlers), Equals, 3)

	for _, ver := range []string{"2.0.0", "0.8.9", "0.4.0", "0.0.1"} {
		handler := l.Get(semver.MustParse(ver))
		c.Assert(handler, NotNil)
		handler()
	}

	c.Assert(callCount100, Equals, 1)
	c.Assert(callCount010, Equals, 2)
	c.Assert(callCount001, Equals, 1)

	handler := l.Get(semver.MustParse("0.0.0"))
	c.Assert(handler, IsNil)
}

func (s *LadderSuite) TestIncorrectOrder(c *C) {
	defer func() {
		if err := recover(); err != nil {
			c.Assert(err, Equals, "Versions out of order in handler registration")
		} else {
			panic("Incorrect version ordering failed to panic")
		}
	}()

	LadderDispatch[func()]{}.
		Register("1.0.0", nil).
		Register("2.0.0", nil)
}
