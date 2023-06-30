package thorchain

import (
	"github.com/blang/semver"
)

type LadderDispatch[F interface{}] struct {
	versions []semver.Version
	handlers []F
}

// Register a handler for a specific version.
// Enforces descending order of registered versions.
func (l LadderDispatch[F]) Register(version string, handler F) LadderDispatch[F] {
	ver := semver.MustParse(version)
	if len(l.versions) > 0 {
		if l.versions[len(l.versions)-1].LTE(ver) {
			panic("Versions out of order in handler registration")
		}
	}
	l.versions = append(l.versions, ver)
	l.handlers = append(l.handlers, handler)
	return l
}

// Return the most recent handler that supports the target version.
func (l LadderDispatch[F]) Get(targetVersion semver.Version) F {
	for idx, version := range l.versions {
		if targetVersion.GTE(version) {
			return l.handlers[idx]
		}
	}
	var empty F
	return empty
}
