package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type WithdrawLiquidityMemo struct {
	MemoBase
	Amount          cosmos.Uint
	WithdrawalAsset common.Asset
}

func (m WithdrawLiquidityMemo) GetAmount() cosmos.Uint           { return m.Amount }
func (m WithdrawLiquidityMemo) GetWithdrawalAsset() common.Asset { return m.WithdrawalAsset }

func NewWithdrawLiquidityMemo(asset common.Asset, amt cosmos.Uint, withdrawalAsset common.Asset) WithdrawLiquidityMemo {
	return WithdrawLiquidityMemo{
		MemoBase:        MemoBase{TxType: TxWithdraw, Asset: asset},
		Amount:          amt,
		WithdrawalAsset: withdrawalAsset,
	}
}

func ParseWithdrawLiquidityMemo(asset common.Asset, parts []string) (WithdrawLiquidityMemo, error) {
	var err error
	if len(parts) < 2 {
		return WithdrawLiquidityMemo{}, fmt.Errorf("not enough parameters")
	}
	withdrawalBasisPts := cosmos.ZeroUint()
	withdrawalAsset := common.EmptyAsset
	if len(parts) > 2 {
		withdrawalBasisPts, err = cosmos.ParseUint(parts[2])
		if err != nil {
			return WithdrawLiquidityMemo{}, err
		}
		if withdrawalBasisPts.IsZero() || withdrawalBasisPts.GT(cosmos.NewUint(types.MaxWithdrawBasisPoints)) {
			return WithdrawLiquidityMemo{}, fmt.Errorf("withdraw amount %s is invalid", parts[2])
		}
	}
	if len(parts) > 3 {
		withdrawalAsset, err = common.NewAsset(parts[3])
		if err != nil {
			return WithdrawLiquidityMemo{}, err
		}
	}
	return NewWithdrawLiquidityMemo(asset, withdrawalBasisPts, withdrawalAsset), nil
}
