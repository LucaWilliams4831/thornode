package keeperv1

import (
	. "gopkg.in/check.v1"

	"github.com/blang/semver"
)

type KeeperVersionSuite struct{}

var _ = Suite(&KeeperVersionSuite{})

func (s *KeeperVersionSuite) TestVersion(c *C) {
	ctx, k := setupKeeperForTest(c)

	// no version stored yet, should return false
	_, hasV := k.GetVersionWithCtx(ctx)
	c.Assert(hasV, Equals, false,
		Commentf("should not have stored version"))

	// stored version should be returned when present
	k.SetVersionWithCtx(ctx, semver.MustParse("4.5.6"))
	v, hasV := k.GetVersionWithCtx(ctx)
	c.Assert(hasV, Equals, true,
		Commentf("should have stored version"))
	c.Assert(v.String(), Equals, "4.5.6",
		Commentf("should use stored version"))
}
