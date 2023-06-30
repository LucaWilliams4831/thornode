package common

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/tendermint/tendermint/crypto"
	tmsecp256k1 "github.com/tendermint/tendermint/crypto/secp256k1"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

// One is useful type so THORNode doesn't need to manage 8 zeroes all the time
const One = 100000000

// GetSafeShare does the same as GetUncappedShare , but GetSafeShare will guarantee the result will not more than total
func GetSafeShare(part, total, allocation cosmos.Uint) cosmos.Uint {
	if part.GTE(total) {
		part = total
	}
	return GetUncappedShare(part, total, allocation)
}

// GetUncappedShare this method will panic if any of the input parameter can't be convert to cosmos.Dec
// which shouldn't happen
func GetUncappedShare(part, total, allocation cosmos.Uint) (share cosmos.Uint) {
	if part.IsZero() || total.IsZero() {
		return cosmos.ZeroUint()
	}
	defer func() {
		if err := recover(); err != nil {
			share = cosmos.ZeroUint()
		}
	}()
	// use string to convert cosmos.Uint to cosmos.Dec is the only way I can find out without being constrain to uint64
	// cosmos.Uint can hold values way larger than uint64 , because it is using big.Int internally
	aD, err := cosmos.NewDecFromStr(allocation.String())
	if err != nil {
		panic(fmt.Errorf("fail to convert %s to cosmos.Dec: %w", allocation.String(), err))
	}

	pD, err := cosmos.NewDecFromStr(part.String())
	if err != nil {
		panic(fmt.Errorf("fail to convert %s to cosmos.Dec: %w", part.String(), err))
	}
	tD, err := cosmos.NewDecFromStr(total.String())
	if err != nil {
		panic(fmt.Errorf("fail to convert%s to cosmos.Dec: %w", total.String(), err))
	}
	// A / (Total / part) == A * (part/Total) but safer when part < Totals
	result := aD.Quo(tD.Quo(pD))
	share = cosmos.NewUintFromBigInt(result.RoundInt().BigInt())
	return
}

// SafeSub subtract input2 from input1, given cosmos.Uint can't be negative , otherwise it will panic
// thus in this method,when input2 is larger than input 1, it will just return cosmos.ZeroUint
func SafeSub(input1, input2 cosmos.Uint) cosmos.Uint {
	if input2.GT(input1) {
		return cosmos.ZeroUint()
	}
	return input1.Sub(input2)
}

// CosmosPrivateKeyToTMPrivateKey convert cosmos implementation of private key to tendermint private key
func CosmosPrivateKeyToTMPrivateKey(privateKey cryptotypes.PrivKey) crypto.PrivKey {
	switch k := privateKey.(type) {
	case *secp256k1.PrivKey:
		return tmsecp256k1.PrivKey(k.Bytes())
	default:
		return nil
	}
}
