package keeperv1

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// AddPoolFeeToReserve add fee to reserve, the fee is always in RUNE
func (k KVStore) AddPoolFeeToReserve(ctx cosmos.Context, fee cosmos.Uint) error {
	coin := common.NewCoin(common.RuneNative, fee)
	sdkErr := k.SendFromModuleToModule(ctx, AsgardName, ReserveName, common.NewCoins(coin))
	if sdkErr != nil {
		return dbError(ctx, "fail to send pool fee to reserve", sdkErr)
	}
	return nil
}

// AddBondFeeToReserve add fee to reserve, the fee is always in RUNE
func (k KVStore) AddBondFeeToReserve(ctx cosmos.Context, fee cosmos.Uint) error {
	coin := common.NewCoin(common.RuneNative, fee)
	sdkErr := k.SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin))
	if sdkErr != nil {
		return dbError(ctx, "fail to send bond fee to reserve", sdkErr)
	}
	return nil
}
