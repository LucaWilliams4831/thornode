//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package thorchain

import _ "embed"

//go:embed preregister_thornames.json
var preregisterTHORNames []byte
