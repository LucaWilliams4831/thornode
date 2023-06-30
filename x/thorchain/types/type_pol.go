package types

import (
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// NewProtocolOwnedLiquidity create a new instance ProtocolOwnedLiquidity it is empty though
func NewProtocolOwnedLiquidity() ProtocolOwnedLiquidity {
	return ProtocolOwnedLiquidity{
		RuneDeposited: cosmos.ZeroUint(),
		RuneWithdrawn: cosmos.ZeroUint(),
	}
}

func (pol ProtocolOwnedLiquidity) CurrentDeposit() cosmos.Int {
	deposited := cosmos.NewIntFromBigInt(pol.RuneDeposited.BigInt())
	withdrawn := cosmos.NewIntFromBigInt(pol.RuneWithdrawn.BigInt())
	return deposited.Sub(withdrawn)
}

// PnL - Profit and Loss
func (pol ProtocolOwnedLiquidity) PnL(value cosmos.Uint) cosmos.Int {
	deposited := cosmos.NewIntFromBigInt(pol.RuneDeposited.BigInt())
	withdrawn := cosmos.NewIntFromBigInt(pol.RuneWithdrawn.BigInt())
	v := cosmos.NewIntFromBigInt(value.BigInt())
	return withdrawn.Sub(deposited).Add(v)
}
