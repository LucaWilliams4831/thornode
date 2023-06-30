package common

import (
	"sort"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func GetMedianUint(vals []cosmos.Uint) cosmos.Uint {
	switch len(vals) {
	case 0:
		return cosmos.ZeroUint()
	case 1:
		return vals[0]
	}

	sort.SliceStable(vals, func(i, j int) bool {
		return vals[i].Uint64() < vals[j].Uint64()
	})

	// calculate median
	var median cosmos.Uint
	if len(vals)%2 > 0 {
		// odd number of figures in our slice. Take the middle figure. Since
		// slices start with an index of zero, just need to length divide by two.
		medianSpot := len(vals) / 2
		median = vals[medianSpot]
	} else {
		// even number of figures in our slice. Average the middle two figures.
		pt1 := vals[len(vals)/2-1]
		pt2 := vals[len(vals)/2]
		median = pt1.Add(pt2).QuoUint64(2)
	}
	return median
}
