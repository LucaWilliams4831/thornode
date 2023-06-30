package thorchain

import (
	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type Swapper interface {
	Swap(ctx cosmos.Context,
		keeper keeper.Keeper,
		tx common.Tx,
		target common.Asset,
		destination common.Address,
		swapTarget cosmos.Uint,
		dexAgg string,
		dexAggTargetAsset string,
		dexAggLimit *cosmos.Uint,
		transactionFee cosmos.Uint,
		synthVirtualDepthMult int64,
		mgr Manager,
	) (cosmos.Uint, []*EventSwap, error)
	CalcAssetEmission(X, x, Y cosmos.Uint) cosmos.Uint
	CalcLiquidityFee(X, x, Y cosmos.Uint) cosmos.Uint
	CalcSwapSlip(Xi, xi cosmos.Uint) cosmos.Uint
}

// GetSwapper return an implementation of Swapper
func GetSwapper(version semver.Version) (Swapper, error) {
	switch {
	case version.GTE(semver.MustParse("1.110.0")):
		return newSwapperV110(), nil
	case version.GTE(semver.MustParse("1.103.0")):
		return newSwapperV103(), nil
	case version.GTE(semver.MustParse("1.102.0")):
		return newSwapperV102(), nil
	case version.GTE(semver.MustParse("1.98.0")):
		return newSwapperV98(), nil
	case version.GTE(semver.MustParse("1.95.0")):
		return newSwapperV95(), nil
	case version.GTE(semver.MustParse("1.94.0")):
		return newSwapperV94(), nil
	case version.GTE(semver.MustParse("1.92.0")):
		return newSwapperV92(), nil
	case version.GTE(semver.MustParse("1.91.0")):
		return newSwapperV91(), nil
	case version.GTE(semver.MustParse("1.90.0")):
		return newSwapperV90(), nil
	case version.GTE(semver.MustParse("0.81.0")):
		return newSwapperV81(), nil
	default:
		return nil, errInvalidVersion
	}
}
