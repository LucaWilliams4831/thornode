package common

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewFee return a new instance of Fee
func NewFee(coins Coins, poolDeduct cosmos.Uint) Fee {
	return Fee{
		Coins:      coins,
		PoolDeduct: poolDeduct,
	}
}

func (f Fee) String() string {
	return fmt.Sprintf("%d: %s", f.PoolDeduct.Uint64(), f.Coins.String())
}
