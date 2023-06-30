package thorchain

import (
	"strings"

	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type AddLiquidityMemo struct {
	MemoBase
	Address              common.Address
	AffiliateAddress     common.Address
	AffiliateBasisPoints cosmos.Uint
}

func (m AddLiquidityMemo) GetDestination() common.Address { return m.Address }

func (m AddLiquidityMemo) String() string {
	txType := m.TxType.String()
	if m.TxType == TxAdd {
		txType = "+"
	}

	args := []string{
		txType,
		m.Asset.String(),
		m.Address.String(),
		m.AffiliateAddress.String(),
		m.AffiliateBasisPoints.String(),
	}

	last := 2
	if !m.Address.IsEmpty() {
		last = 3
	}
	if !m.AffiliateAddress.IsEmpty() {
		last = 5
	}

	return strings.Join(args[:last], ":")
}

func NewAddLiquidityMemo(asset common.Asset, addr, affAddr common.Address, affPts cosmos.Uint) AddLiquidityMemo {
	return AddLiquidityMemo{
		MemoBase:             MemoBase{TxType: TxAdd, Asset: asset},
		Address:              addr,
		AffiliateAddress:     affAddr,
		AffiliateBasisPoints: affPts,
	}
}

func ParseAddLiquidityMemo(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string) (AddLiquidityMemo, error) {
	if keeper == nil {
		return ParseAddLiquidityMemoV1(ctx, keeper, asset, parts)
	}
	switch {
	case keeper.GetVersion().GTE(semver.MustParse("1.104.0")):
		return ParseAddLiquidityMemoV104(ctx, keeper, asset, parts)
	default:
		return ParseAddLiquidityMemoV1(ctx, keeper, asset, parts)
	}
}

func ParseAddLiquidityMemoV104(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string) (AddLiquidityMemo, error) {
	var err error
	addr := common.NoAddress
	affAddr := common.NoAddress
	affPts := cosmos.ZeroUint()
	if addrStr := GetPart(parts, 2); addrStr != "" {
		if keeper == nil {
			addr, err = common.NewAddress(addrStr)
		} else {
			addr, err = FetchAddress(ctx, keeper, addrStr, asset.Chain)
		}
		if err != nil {
			return AddLiquidityMemo{}, err
		}
	}

	affAddrStr := GetPart(parts, 3)
	affPtsStr := GetPart(parts, 4)
	if affAddrStr != "" && affPtsStr != "" {
		if keeper == nil {
			affAddr, err = common.NewAddress(affAddrStr)
		} else {
			affAddr, err = FetchAddress(ctx, keeper, affAddrStr, common.THORChain)
		}
		if err != nil {
			return AddLiquidityMemo{}, err
		}
		affPts, err = ParseAffiliateBasisPoints(ctx, keeper, affPtsStr)
		if err != nil {
			return AddLiquidityMemo{}, err
		}
	}
	return NewAddLiquidityMemo(asset, addr, affAddr, affPts), nil
}
