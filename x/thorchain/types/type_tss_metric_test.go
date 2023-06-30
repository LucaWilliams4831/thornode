package types

import (
	. "gopkg.in/check.v1"
)

type TypeTssMetricSuite struct{}

var _ = Suite(&TypeTssMetricSuite{})

func (s *TypeTssMetricSuite) TestTssKeygenMetric(c *C) {
	keygenMetric := NewTssKeygenMetric(GetRandomPubKey())
	c.Assert(keygenMetric, NotNil)
	addr := GetRandomBech32Addr()
	keygenMetric.AddNodeTssTime(addr, 10)
	keygenMetric.AddNodeTssTime(GetRandomBech32Addr(), 100)
	c.Assert(keygenMetric.NodeTssTimes, HasLen, 2)
}

func (s *TypeTssMetricSuite) TestTssKeysignMetric(c *C) {
	keysignMetric := NewTssKeysignMetric(GetRandomTxHash())
	keysignMetric.AddNodeTssTime(GetRandomBech32Addr(), 10)
	keysignMetric.AddNodeTssTime(GetRandomBech32Addr(), 100)
	c.Assert(keysignMetric, NotNil)
	c.Assert(keysignMetric.NodeTssTimes, HasLen, 2)
	keysignMetric.AddNodeTssTime(GetRandomBech32Addr(), 50)
	keysignMetric.AddNodeTssTime(GetRandomBech32Addr(), 60)
	keysignMetric.AddNodeTssTime(GetRandomBech32Addr(), 70)
	median := keysignMetric.GetMedianTime()
	c.Assert(median, Equals, int64(60))
	keysignMetric.AddNodeTssTime(GetRandomBech32Addr(), 120)
	median = keysignMetric.GetMedianTime()
	c.Assert(median, Equals, int64(65))
}
