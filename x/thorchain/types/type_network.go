package types

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// NewNetwork create a new instance Network it is empty though
func NewNetwork() Network {
	return Network{
		BondRewardRune:  cosmos.ZeroUint(),
		TotalBondUnits:  cosmos.ZeroUint(),
		BurnedBep2Rune:  cosmos.ZeroUint(),
		BurnedErc20Rune: cosmos.ZeroUint(),
	}
}

// CalcNodeRewards calculate node rewards
func (m *Network) CalcNodeRewards(nodeUnits cosmos.Uint) cosmos.Uint {
	return common.GetUncappedShare(nodeUnits, m.TotalBondUnits, m.BondRewardRune)
}
