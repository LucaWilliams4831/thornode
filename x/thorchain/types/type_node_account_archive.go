package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (bp *BondProviders) AdjustV1(nodeBond cosmos.Uint) {
	totalBond := cosmos.ZeroUint()
	if len(bp.Providers) == 0 {
		// no adjustment needed
		return
	}

	for _, provider := range bp.Providers {
		totalBond = totalBond.Add(provider.Bond)
	}

	if totalBond.Equal(nodeBond) {
		// no adjustment needed
		return
	}

	// deduct node operator fee from income
	fee := cosmos.ZeroUint()
	if totalBond.LT(nodeBond) {
		surplus := common.SafeSub(nodeBond, totalBond)
		fee = common.GetSafeShare(bp.NodeOperatorFee, cosmos.NewUint(10000), surplus)
	}
	nodeBond = common.SafeSub(nodeBond, fee)

	for i := range bp.Providers {
		bond := bp.Providers[i].Bond
		bp.Providers[i].Bond = common.GetSafeShare(bond, totalBond, nodeBond)
		if i == 0 { // first bond provider is node operator
			bp.Providers[i].Bond = bp.Providers[i].Bond.Add(fee)
		}
	}
}
