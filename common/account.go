package common

import (
	"gitlab.com/thorchain/binance-sdk/common/types"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

// Account define a struct to hold account information across all chain
type Account struct {
	Sequence      int64
	AccountNumber int64
	Coins         Coins
	HasMemoFlag   bool
}

// GetCoins transforms from binance coins
func GetCoins(chain Chain, accCoins []types.Coin) (Coins, error) {
	coins := make(Coins, 0)
	for _, coin := range accCoins {
		asset, err := NewAsset(chain.String() + "." + coin.Denom)
		if err != nil {
			return nil, err
		}
		coins = append(coins, NewCoin(asset, cosmos.NewUint(uint64(coin.Amount))))
	}
	return coins, nil
}

// NewAccount create a new instance of Account
func NewAccount(sequence, accountNumber int64, coins Coins, hasMemoFlag bool) Account {
	return Account{
		Sequence:      sequence,
		AccountNumber: accountNumber,
		Coins:         coins,
		HasMemoFlag:   hasMemoFlag,
	}
}
