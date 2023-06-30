package common

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

// THORChainDecimals indicate the number of decimal points used in THORChain
const THORChainDecimals = 8

// NoCoin is empty Coin
var NoCoin = Coin{
	Asset:  EmptyAsset,
	Amount: cosmos.ZeroUint(),
}

// Coins represent a slice of Coin
type Coins []Coin

// NewCoin return a new instance of Coin
func NewCoin(asset Asset, amount cosmos.Uint) Coin {
	return Coin{
		Asset:  asset,
		Amount: amount,
	}
}

// ParseCoin parses a coin string and panics if it is invalid.
func ParseCoin(coinStr string) (Coin, error) {
	// split "<amount> <asset>"
	split := strings.Split(coinStr, " ")
	if len(split) != 2 {
		return NoCoin, fmt.Errorf("invalid coin string: %s", coinStr)
	}

	// parse the amount
	amount, err := cosmos.ParseUint(split[0])
	if err != nil {
		return NoCoin, fmt.Errorf("invalid coin string: %s: %w", coinStr, err)
	}

	// parse the asset
	asset, err := NewAsset(split[1])
	if err != nil {
		return NoCoin, fmt.Errorf("invalid coin string: %s: %w", coinStr, err)
	}

	return NewCoin(asset, amount), nil
}

// NewCoins create a new Coins structure
func NewCoins(coins ...Coin) Coins {
	result := make(Coins, len(coins))
	copy(result, coins)
	return result
}

// Equals compare two coins to see whether they represent the same information
func (c Coin) Equals(cc Coin) bool {
	if !c.Asset.Equals(cc.Asset) {
		return false
	}
	if !c.Amount.Equal(cc.Amount) {
		return false
	}
	return true
}

// IsEmpty check whether asset is empty and also amount is zero
func (c Coin) IsEmpty() bool {
	if c.Asset.IsEmpty() {
		return true
	}
	if c.Amount.IsZero() {
		return true
	}
	return false
}

// Valid return an error if the coin is not correct
func (c Coin) Valid() error {
	if c.Asset.IsEmpty() {
		return errors.New("denom cannot be empty")
	}
	if c.Amount.IsZero() {
		return errors.New("amount cannot be zero")
	}

	return nil
}

// IsNative check whether the coin is native on THORChain
func (c Coin) IsNative() bool {
	return c.Asset.GetChain().Equals(THORChain)
}

// Native create a new instance of cosmos.Coin
func (c Coin) Native() (cosmos.Coin, error) {
	if !c.IsNative() {
		return cosmos.Coin{}, errors.New("coin is not on thorchain")
	}
	return cosmos.NewCoin(
		c.Asset.Native(),
		cosmos.NewIntFromBigInt(c.Amount.BigInt()),
	), nil
}

// String implement fmt.Stringer
func (c Coin) String() string {
	return fmt.Sprintf("%s %s", c.Amount.String(), c.Asset.String())
}

// WithDecimals update coin with a decimal
func (c Coin) WithDecimals(decimal int64) Coin {
	c.Decimals = decimal
	return c
}

// Valid check whether all the coins are valid , if not , then return an error
func (cs Coins) Valid() error {
	for _, coin := range cs {
		if err := coin.Valid(); err != nil {
			return err
		}
	}
	return nil
}

// Equals Check if two lists of coins are equal to each other. Order does not matter
// This method has side effect because it uses sort.Slice on the input parameter
// which will potentially change the order of cs1 & cs2
// Note: quite a few places already using this method , which can't be changed, otherwise it will cause consensus failure
// on CHAOSNET
// Deprecated
func (cs Coins) Equals(cs2 Coins) bool {
	if len(cs) != len(cs2) {
		return false
	}

	// sort both lists
	sort.Slice(cs[:], func(i, j int) bool {
		return cs[i].Asset.String() < cs[j].Asset.String()
	})
	sort.Slice(cs2[:], func(i, j int) bool {
		return cs2[i].Asset.String() < cs2[j].Asset.String()
	})

	for i := range cs {
		if !cs[i].Equals(cs2[i]) {
			return false
		}
	}

	return true
}

// EqualsEx Check if two lists of coins are equal to each other.
// This method will make a copy of cs1 & cs2 , thus the original coins order will not be changed
func (cs Coins) EqualsEx(cs2 Coins) bool {
	if len(cs) != len(cs2) {
		return false
	}

	source := make(Coins, len(cs))
	dest := make(Coins, len(cs2))
	copy(source, cs)
	copy(dest, cs2)

	// sort both lists
	sort.Slice(source[:], func(i, j int) bool {
		return source[i].Asset.String() < source[j].Asset.String()
	})
	sort.Slice(dest[:], func(i, j int) bool {
		return dest[i].Asset.String() < dest[j].Asset.String()
	})
	for i := range source {
		if !source[i].Equals(dest[i]) {
			return false
		}
	}

	return true
}

func (cs Coins) IsEmpty() bool {
	if len(cs) == 0 {
		return true
	}
	for _, coin := range cs {
		if !coin.IsEmpty() {
			return false
		}
	}
	return true
}

func (cs Coins) Native() (cosmos.Coins, error) {
	var err error
	coins := make(cosmos.Coins, len(cs))
	for i, coin := range cs {
		coins[i], err = coin.Native()
		if err != nil {
			return nil, err
		}
	}
	return coins, nil
}

// String implement fmt.Stringer
func (cs Coins) String() string {
	coins := make([]string, len(cs))
	for i, c := range cs {
		coins[i] = c.String()
	}
	return strings.Join(coins, ", ")
}

// Contains check whether the given coin is in the list
func (cs Coins) Contains(c Coin) bool {
	for _, item := range cs {
		if c.Equals(item) {
			return true
		}
	}
	return false
}

// GetCoin gets a specific coin by asset. Assumes there is only one of this coin in the
// list.
func (cs Coins) GetCoin(asset Asset) Coin {
	for _, item := range cs {
		if asset.Equals(item.Asset) {
			return item
		}
	}
	return NoCoin
}

// Distinct return a new Coins ,which duplicated coins had been removed
func (cs Coins) Distinct() Coins {
	newCoins := Coins{}
	for _, item := range cs {
		if !newCoins.Contains(item) {
			newCoins = append(newCoins, item)
		}
	}
	return newCoins
}

func (cs Coins) Add(coin Coin) Coins {
	for i, c := range cs {
		if c.Asset.Equals(coin.Asset) {
			cs[i].Amount = cs[i].Amount.Add(coin.Amount)
			return cs
		}
	}

	return append(cs, coin)
}

// This overwrites cs by changing its slice-referenced values,
// so it is recommended to use destination := make(Coins, len(source))
// and copy(destination, source) first.
func (cs Coins) Adds(coins Coins) Coins {
	for _, c := range coins {
		cs = cs.Add(c)
	}
	return cs
}

// HasNoneNativeRune check whether the coins contains NoneNativeRUNE
//
//	At the moment, there are BEP2 RUNE, ERC20 RUNE and Native RUNE
func (cs Coins) HasNoneNativeRune() bool {
	for _, c := range cs {
		if c.Asset.IsRune() && !c.IsNative() {
			return true
		}
	}
	return false
}

// HasSynthetic check whether the coins contains synth coin
func (cs Coins) HasSynthetic() bool {
	for _, c := range cs {
		if c.Asset.IsSyntheticAsset() {
			return true
		}
	}
	return false
}

// NoneEmpty return a new Coins , which ignore the coin that is empty
// either Coin asset is empty or amount is empty
func (cs Coins) NoneEmpty() Coins {
	newCoins := Coins{}
	for _, item := range cs {
		if item.IsEmpty() {
			continue
		}
		newCoins = append(newCoins, item)
	}
	return newCoins
}
