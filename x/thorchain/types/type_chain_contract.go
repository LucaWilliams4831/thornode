package types

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
)

// NewChainContract create a new instance of ChainContract
func NewChainContract(chain common.Chain, router common.Address) ChainContract {
	return ChainContract{
		Chain:  chain,
		Router: router,
	}
}

// IsEmpty returns true when both chain and Contract address are empty
func (m *ChainContract) IsEmpty() bool {
	return m.Chain.IsEmpty() || m.Router.IsEmpty()
}

// String implement fmt.Stringer, return a string representation of ChainContract
func (m *ChainContract) String() string {
	return fmt.Sprintf("%s-%s", m.Chain, m.Router)
}
