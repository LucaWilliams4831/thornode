package common

import "github.com/kzahedi/goent/discrete"

func Entropy(b []byte) float64 {
	dist := make([]float64, 1<<8)
	for _, c := range b {
		dist[int(c)]++
	}
	for i := range dist {
		dist[i] /= float64(len(b))
	}
	return discrete.EntropyBase2(dist)
}
