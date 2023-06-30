package common

// ChainNetwork is to indicate which chain environment THORNode are working with
type ChainNetwork uint8

const (
	// TestNet network for test
	TestNet ChainNetwork = iota
	// MainNet network for main net
	MainNet
	// MockNet network for main net
	MockNet
	// Stagenet network for stage net
	StageNet
)

// Soft Equals check is mainnet == mainet, or (testnet/mocknet == testnet/mocknet)
func (net ChainNetwork) SoftEquals(net2 ChainNetwork) bool {
	if net == MainNet && net2 == MainNet {
		return true
	}
	if net != MainNet && net2 != MainNet {
		return true
	}

	return false
}
