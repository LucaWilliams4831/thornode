package thorchain

import (
	"fmt"
	"strconv"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

func ParseLoanOpenMemoV1(ctx cosmos.Context, keeper keeper.Keeper, targetAsset common.Asset, parts []string) (LoanOpenMemo, error) {
	var err error
	var targetAddress common.Address
	affAddr := common.NoAddress
	affPts := cosmos.ZeroUint()
	minOut := cosmos.ZeroUint()
	var dexAgg, dexTargetAddr string
	dexTargetLimit := cosmos.ZeroUint()
	if len(parts) <= 2 {
		return LoanOpenMemo{}, fmt.Errorf("Not enough loan parameters")
	}

	destStr := GetPart(parts, 2)
	if keeper == nil {
		targetAddress, err = common.NewAddress(destStr)
	} else {
		targetAddress, err = FetchAddress(ctx, keeper, destStr, targetAsset.GetChain())
	}
	if err != nil {
		return LoanOpenMemo{}, err
	}

	if minOutStr := GetPart(parts, 3); minOutStr != "" {
		minOutUint, err := strconv.ParseUint(minOutStr, 10, 64)
		if err != nil {
			return LoanOpenMemo{}, err
		}
		minOut = cosmos.NewUint(minOutUint)
	}

	affAddrStr := GetPart(parts, 4)
	affPtsStr := GetPart(parts, 5)
	if affAddrStr != "" && affPtsStr != "" {
		if keeper == nil {
			affAddr, err = common.NewAddress(affAddrStr)
		} else {
			affAddr, err = FetchAddress(ctx, keeper, affAddrStr, common.THORChain)
		}
		if err != nil {
			return LoanOpenMemo{}, err
		}
		pts, err := strconv.ParseUint(affPtsStr, 10, 64)
		if err != nil {
			return LoanOpenMemo{}, err
		}
		affPts = cosmos.NewUint(pts)
	}

	dexAgg = GetPart(parts, 6)
	dexTargetAddr = GetPart(parts, 7)

	if x := GetPart(parts, 8); x != "" {
		dexTargetLimit, err = cosmos.ParseUint(x)
		if err != nil {
			if keeper != nil {
				ctx.Logger().Error("invalid dex target limit, ignore it", "limit", x)
			}
			dexTargetLimit = cosmos.ZeroUint()
		}
	}

	return NewLoanOpenMemo(targetAsset, targetAddress, minOut, affAddr, affPts, dexAgg, dexTargetAddr, dexTargetLimit), nil
}

func ParseLoanRepaymentMemoV1(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string) (LoanRepaymentMemo, error) {
	var err error
	var owner common.Address
	minOut := cosmos.ZeroUint()
	if len(parts) <= 2 {
		return LoanRepaymentMemo{}, fmt.Errorf("Not enough loan parameters")
	}

	ownerStr := GetPart(parts, 2)
	if keeper == nil {
		owner, err = common.NewAddress(ownerStr)
	} else {
		owner, err = FetchAddress(ctx, keeper, ownerStr, asset.Chain)
	}
	if err != nil {
		return LoanRepaymentMemo{}, err
	}

	if minOutStr := GetPart(parts, 3); minOutStr != "" {
		min, err := strconv.ParseUint(minOutStr, 10, 64)
		if err != nil {
			return LoanRepaymentMemo{}, err
		}
		minOut = cosmos.NewUint(min)
	}

	return NewLoanRepaymentMemo(asset, owner, minOut), nil
}
