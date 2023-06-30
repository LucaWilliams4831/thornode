package types

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewJail create a new Jail instance
func NewJail(addr cosmos.AccAddress) Jail {
	return Jail{
		NodeAddress: addr,
	}
}

// IsJailed on a given height , check whether a node is jailed or not
func (m *Jail) IsJailed(ctx cosmos.Context) bool {
	return m.ReleaseHeight > ctx.BlockHeight()
}
