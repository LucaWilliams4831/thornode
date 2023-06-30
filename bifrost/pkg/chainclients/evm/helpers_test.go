package evm

import (
	"fmt"

	ecore "github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"
	. "gopkg.in/check.v1"
)

type HelpersTestSuite struct{}

var _ = Suite(&HelpersTestSuite{})

func (s *HelpersTestSuite) TestIsAcceptableError(c *C) {
	c.Assert(isAcceptableError(nil), Equals, true)
	c.Assert(isAcceptableError(txpool.ErrAlreadyKnown), Equals, true)
	c.Assert(isAcceptableError(ecore.ErrNonceTooLow), Equals, true)
	c.Assert(isAcceptableError(fmt.Errorf("%w: foo", ecore.ErrNonceTooLow)), Equals, true)
	c.Assert(isAcceptableError(fmt.Errorf("foo: %w", ecore.ErrNonceTooLow)), Equals, false)
}
