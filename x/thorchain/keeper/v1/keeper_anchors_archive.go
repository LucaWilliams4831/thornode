package keeperv1

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStore) DollarInRune(ctx cosmos.Context) cosmos.Uint {
	// check for mimir override
	dollarInRune, err := k.GetMimir(ctx, "DollarInRune")
	if err == nil && dollarInRune > 0 {
		return cosmos.NewUint(uint64(dollarInRune))
	}

	usdAssets := k.GetAnchors(ctx, common.TOR)

	return k.AnchorMedian(ctx, usdAssets)
}
