package thorchain

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// "LOAN+:BTC.BTC:bc1YYYYYY:minBTC:affAddr:affPts:dexAgg:dexTarAddr:DexTargetLimit"

type LoanOpenMemo struct {
	MemoBase
	TargetAsset          common.Asset
	TargetAddress        common.Address
	MinOut               cosmos.Uint
	AffiliateAddress     common.Address
	AffiliateBasisPoints cosmos.Uint
	DexAggregator        string
	DexTargetAddress     string
	DexTargetLimit       cosmos.Uint
}

func (m LoanOpenMemo) GetTargetAsset() common.Asset         { return m.TargetAsset }
func (m LoanOpenMemo) GetTargetAddress() common.Address     { return m.TargetAddress }
func (m LoanOpenMemo) GetMinOut() cosmos.Uint               { return m.MinOut }
func (m LoanOpenMemo) GetAffiliateAddress() common.Address  { return m.AffiliateAddress }
func (m LoanOpenMemo) GetAffiliateBasisPoints() cosmos.Uint { return m.AffiliateBasisPoints }
func (m LoanOpenMemo) GetDexAggregator() string             { return m.DexAggregator }
func (m LoanOpenMemo) GetDexTargetAddress() string          { return m.DexTargetAddress }

func (m LoanOpenMemo) String() string {
	args := []string{
		TxLoanOpen.String(),
		m.TargetAsset.String(),
		m.TargetAddress.String(),
		m.MinOut.String(),
		m.AffiliateAddress.String(),
		m.AffiliateBasisPoints.String(),
		m.DexAggregator,
		m.DexTargetAddress,
		m.DexTargetLimit.String(),
	}
	last := 3

	switch {
	case !m.DexTargetLimit.IsZero():
		last = 9
	case m.DexTargetAddress != "":
		last = 8
	case m.DexAggregator != "":
		last = 7
	case !m.AffiliateBasisPoints.IsZero():
		last = 6
	case !m.AffiliateAddress.IsEmpty():
		last = 5
	case !m.MinOut.IsZero():
		last = 4
	}

	return strings.Join(args[:last], ":")
}

func NewLoanOpenMemo(targetAsset common.Asset, targetAddr common.Address, minOut cosmos.Uint, affAddr common.Address, affPts cosmos.Uint, dexAgg, dexTargetAddr string, dexTargetLimit cosmos.Uint) LoanOpenMemo {
	return LoanOpenMemo{
		MemoBase:             MemoBase{TxType: TxLoanOpen},
		TargetAsset:          targetAsset,
		TargetAddress:        targetAddr,
		MinOut:               minOut,
		AffiliateAddress:     affAddr,
		AffiliateBasisPoints: affPts,
		DexAggregator:        dexAgg,
		DexTargetAddress:     dexTargetAddr,
		DexTargetLimit:       dexTargetLimit,
	}
}

func ParseLoanOpenMemo(ctx cosmos.Context, version semver.Version, keeper keeper.Keeper, asset common.Asset, parts []string) (LoanOpenMemo, error) {
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		return ParseLoanOpenMemoV112(ctx, keeper, asset, parts)
	default:
		return ParseLoanOpenMemoV1(ctx, keeper, asset, parts)
	}
}

func ParseLoanOpenMemoV112(ctx cosmos.Context, keeper keeper.Keeper, targetAsset common.Asset, parts []string) (LoanOpenMemo, error) {
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
		minOut, err = parseTradeTarget(minOutStr)
		if err != nil {
			return LoanOpenMemo{}, err
		}
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

// "LOAN-:BTC.BTC:bc1YYYYYY:minOut"

type LoanRepaymentMemo struct {
	MemoBase
	Owner  common.Address
	MinOut cosmos.Uint
}

func (m LoanRepaymentMemo) String() string {
	args := []string{TxLoanRepayment.String(), m.Asset.String(), m.Owner.String()}
	if !m.MinOut.IsZero() {
		args = append(args, m.MinOut.String())
	}
	return strings.Join(args, ":")
}

func NewLoanRepaymentMemo(asset common.Asset, owner common.Address, minOut cosmos.Uint) LoanRepaymentMemo {
	return LoanRepaymentMemo{
		MemoBase: MemoBase{TxType: TxLoanRepayment, Asset: asset},
		Owner:    owner,
		MinOut:   minOut,
	}
}

func ParseLoanRepaymentMemo(ctx cosmos.Context, version semver.Version, keeper keeper.Keeper, asset common.Asset, parts []string) (LoanRepaymentMemo, error) {
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		return ParseLoanRepaymentMemoV112(ctx, keeper, asset, parts)
	default:
		return ParseLoanRepaymentMemoV1(ctx, keeper, asset, parts)
	}
}

func ParseLoanRepaymentMemoV112(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string) (LoanRepaymentMemo, error) {
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
		minOut, err = parseTradeTarget(minOutStr)
		if err != nil {
			return LoanRepaymentMemo{}, err
		}
	}

	return NewLoanRepaymentMemo(asset, owner, minOut), nil
}
