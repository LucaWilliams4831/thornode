package types

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

var _ codec.ProtoMarshaler = &LiquidityProvider{}

// LiquidityProviders a list of liquidity providers
type LiquidityProviders []LiquidityProvider

// Valid check whether lp represent valid information
func (m *LiquidityProvider) Valid() error {
	if m.LastAddHeight == 0 {
		return errors.New("last add liquidity height cannot be empty")
	}
	if m.AssetAddress.IsEmpty() && m.RuneAddress.IsEmpty() {
		return errors.New("asset address and rune address cannot be empty")
	}
	return nil
}

func (lp LiquidityProvider) GetAddress() common.Address {
	if !lp.RuneAddress.IsEmpty() {
		return lp.RuneAddress
	}
	return lp.AssetAddress
}

// Key return a string which can be used to identify lp
func (lp LiquidityProvider) Key() string {
	return fmt.Sprintf("%s/%s", lp.Asset.String(), lp.GetAddress().String())
}

func (lp LiquidityProvider) GetRuneRedeemValue(version semver.Version, pool Pool, synthSupply cosmos.Uint) (error, cosmos.Uint) {
	if !lp.Asset.Equals(pool.Asset) {
		return fmt.Errorf("LP and Pool assets do not match (%s, %s)", lp.Asset.String(), pool.Asset.String()), cosmos.ZeroUint()
	}

	bigInt := &big.Int{}
	lpUnits := lp.Units.BigInt()
	poolRuneDepth := pool.BalanceRune.BigInt()
	num := bigInt.Mul(lpUnits, poolRuneDepth)

	pool.CalcUnits(version, synthSupply)
	denom := pool.GetPoolUnits().BigInt()
	if len(denom.Bits()) == 0 {
		return nil, cosmos.ZeroUint()
	}
	result := bigInt.Quo(num, denom)
	return nil, cosmos.NewUintFromBigInt(result)
}

func (lp LiquidityProvider) GetAssetRedeemValue(version semver.Version, pool Pool, synthSupply cosmos.Uint) (error, cosmos.Uint) {
	if !lp.Asset.Equals(pool.Asset) {
		return fmt.Errorf("LP and Pool assets do not match (%s, %s)", lp.Asset.String(), pool.Asset.String()), cosmos.ZeroUint()
	}

	bigInt := &big.Int{}
	lpUnits := lp.Units.BigInt()
	poolAssetDepth := pool.BalanceAsset.BigInt()
	num := bigInt.Mul(lpUnits, poolAssetDepth)

	pool.CalcUnits(version, synthSupply)
	denom := pool.GetPoolUnits().BigInt()
	if len(denom.Bits()) == 0 {
		return nil, cosmos.ZeroUint()
	}
	result := bigInt.Quo(num, denom)
	return nil, cosmos.NewUintFromBigInt(result)
}

func (lp LiquidityProvider) GetLuviDepositValue(pool Pool) (error, cosmos.Uint) {
	if !lp.Asset.Equals(pool.Asset) {
		return fmt.Errorf("LP and Pool assets do not match (%s, %s)", lp.Asset.String(), pool.Asset.String()), cosmos.ZeroUint()
	}

	bigInt := &big.Int{}
	runeDeposit := lp.RuneDepositValue.MulUint64(1e8).BigInt()
	assetDeposit := lp.AssetDepositValue.MulUint64(1e8).BigInt()
	num := bigInt.Mul(runeDeposit, assetDeposit)
	num = bigInt.Sqrt(num)
	denom := lp.Units.BigInt()
	if len(denom.Bits()) == 0 {
		return nil, cosmos.ZeroUint()
	}
	result := bigInt.Quo(num, denom)
	return nil, cosmos.NewUintFromBigInt(result)
}

func (lp LiquidityProvider) GetLuviRedeemValue(runeRedeemValue, assetRedeemValue cosmos.Uint) (error, cosmos.Uint) {
	bigInt := &big.Int{}
	runeValue := runeRedeemValue.MulUint64(1e8).BigInt()
	assetValue := assetRedeemValue.MulUint64(1e8).BigInt()
	num := bigInt.Mul(runeValue, assetValue)
	num = bigInt.Sqrt(num)
	denom := lp.Units.BigInt()
	if len(denom.Bits()) == 0 {
		return nil, cosmos.ZeroUint()
	}
	result := bigInt.Quo(num, denom)
	return nil, cosmos.NewUintFromBigInt(result)
}

func (lp LiquidityProvider) GetSaversAssetRedeemValue(pool Pool) cosmos.Uint {
	bigInt := &big.Int{}
	lpUnits := lp.Units.BigInt()
	saversDepth := pool.BalanceAsset.BigInt()
	num := bigInt.Mul(lpUnits, saversDepth)
	denom := pool.LPUnits.BigInt()
	if len(denom.Bits()) == 0 {
		return cosmos.ZeroUint()
	}
	result := bigInt.Quo(num, denom)
	return cosmos.NewUintFromBigInt(result)
}
