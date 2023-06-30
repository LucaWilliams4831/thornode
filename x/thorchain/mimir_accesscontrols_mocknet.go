//go:build mocknet
// +build mocknet

package thorchain

import (
	re "regexp"
)

var adminMimirDenyList = []*re.Regexp{
	// For mocknet, admin mimir can set any key.
}
