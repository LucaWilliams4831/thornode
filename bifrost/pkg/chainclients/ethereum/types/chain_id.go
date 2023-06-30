package types

// ChainID represent Ethereum chain id type
type ChainID int

const (
	// Mainnet - mainnet
	Mainnet ChainID = iota + 1
	_
	Ropsten
	Rinkeby
	Localnet = iota + 11
)
