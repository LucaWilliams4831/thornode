package thorchain

import (
	"fmt"
	"strconv"

	"gitlab.com/thorchain/thornode/common/cosmos"

	"github.com/blang/semver"
)

type BondMemo struct {
	MemoBase
	NodeAddress         cosmos.AccAddress
	BondProviderAddress cosmos.AccAddress
	NodeOperatorFee     int64
}

func (m BondMemo) GetAccAddress() cosmos.AccAddress { return m.NodeAddress }

func NewBondMemo(addr, additional cosmos.AccAddress, operatorFee int64) BondMemo {
	return BondMemo{
		MemoBase:            MemoBase{TxType: TxBond},
		NodeAddress:         addr,
		BondProviderAddress: additional,
		NodeOperatorFee:     operatorFee,
	}
}

func ParseBondMemo(version semver.Version, parts []string) (BondMemo, error) {
	switch {
	case version.GTE(semver.MustParse("1.88.0")):
		return ParseBondMemoV88(parts)
	case version.GTE(semver.MustParse("0.81.0")):
		return ParseBondMemoV81(parts)
	default:
		return BondMemo{}, fmt.Errorf("invalid version(%s)", version.String())
	}
}

func ParseBondMemoV88(parts []string) (BondMemo, error) {
	additional := cosmos.AccAddress{}
	var operatorFee int64 = -1
	if len(parts) < 2 {
		return BondMemo{}, fmt.Errorf("not enough parameters")
	}
	addr, err := cosmos.AccAddressFromBech32(parts[1])
	if err != nil {
		return BondMemo{}, fmt.Errorf("%s is an invalid thorchain address: %w", parts[1], err)
	}
	if len(parts) == 3 || len(parts) == 4 {
		additional, err = cosmos.AccAddressFromBech32(parts[2])
		if err != nil {
			return BondMemo{}, fmt.Errorf("%s is an invalid thorchain address: %w", parts[2], err)
		}
	}
	if len(parts) == 4 {
		operatorFee, err = strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return BondMemo{}, fmt.Errorf("%s invalid operator fee: %w", parts[3], err)
		}
	}
	return NewBondMemo(addr, additional, operatorFee), nil
}

func ParseBondMemoV81(parts []string) (BondMemo, error) {
	additional := cosmos.AccAddress{}
	if len(parts) < 2 {
		return BondMemo{}, fmt.Errorf("not enough parameters")
	}
	addr, err := cosmos.AccAddressFromBech32(parts[1])
	if err != nil {
		return BondMemo{}, fmt.Errorf("%s is an invalid thorchain address: %w", parts[1], err)
	}
	if len(parts) >= 3 {
		additional, err = cosmos.AccAddressFromBech32(parts[2])
		if err != nil {
			return BondMemo{}, fmt.Errorf("%s is an invalid thorchain address: %w", parts[2], err)
		}
	}
	return NewBondMemo(addr, additional, -1), nil
}
