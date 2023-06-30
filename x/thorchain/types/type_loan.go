package types

import (
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

var _ codec.ProtoMarshaler = &Loan{}

// Loans a list of loans
type Loans []Loan

func NewLoan(owner common.Address, asset common.Asset, lastOpenHeight int64) Loan {
	return Loan{
		Owner:          owner,
		Asset:          asset,
		DebtUp:         cosmos.ZeroUint(),
		DebtDown:       cosmos.ZeroUint(),
		LastOpenHeight: lastOpenHeight,
		CollateralUp:   cosmos.ZeroUint(),
		CollateralDown: cosmos.ZeroUint(),
	}
}

// Valid check whether lp represent valid information
func (m *Loan) Valid() error {
	if m.LastOpenHeight <= 0 {
		return errors.New("last open loan height cannot be empty")
	}
	if m.Owner.IsEmpty() {
		return errors.New("owner address cannot be empty")
	}
	if m.Asset.IsEmpty() {
		return errors.New("asset cannot be empty")
	}
	return nil
}

func (m *Loan) Debt() cosmos.Uint {
	return common.SafeSub(m.DebtUp, m.DebtDown)
}

func (m *Loan) Collateral() cosmos.Uint {
	return common.SafeSub(m.CollateralUp, m.CollateralDown)
}

// Key return a string which can be used to identify loan
func (m Loan) Key() string {
	return fmt.Sprintf("%s/%s", m.Asset.String(), m.Owner.String())
}
