package common

import (
	. "gopkg.in/check.v1"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type CoinSuite struct{}

var _ = Suite(&CoinSuite{})

func (s CoinSuite) TestCoin(c *C) {
	coin := NewCoin(BNBAsset, cosmos.NewUint(230000000))
	c.Check(coin.Asset.Equals(BNBAsset), Equals, true)
	c.Check(coin.Amount.Uint64(), Equals, uint64(230000000))
	c.Check(coin.Valid(), IsNil)
	c.Check(coin.IsEmpty(), Equals, false)
	c.Check(NoCoin.IsEmpty(), Equals, true)

	c.Check(coin.IsNative(), Equals, false)
	_, err := coin.Native()
	c.Assert(err, NotNil)
	coin = NewCoin(RuneNative, cosmos.NewUint(230))
	c.Check(coin.IsNative(), Equals, true)
	sdkCoin, err := coin.Native()
	c.Assert(err, IsNil)
	c.Check(sdkCoin.Denom, Equals, "rune")
	c.Check(sdkCoin.Amount.Equal(cosmos.NewInt(230)), Equals, true)
}

func (s CoinSuite) TestDistinct(c *C) {
	coins := Coins{
		NewCoin(BNBAsset, cosmos.NewUint(1000)),
		NewCoin(BNBAsset, cosmos.NewUint(1000)),
		NewCoin(BTCAsset, cosmos.NewUint(1000)),
		NewCoin(BTCAsset, cosmos.NewUint(1000)),
	}
	newCoins := coins.Distinct()
	c.Assert(len(newCoins), Equals, 2)
}

func (s CoinSuite) TestAdds(c *C) {
	oldCoins := Coins{
		NewCoin(BNBAsset, cosmos.NewUint(1000)),
		NewCoin(BCHAsset, cosmos.NewUint(1000)),
	}
	newCoins := oldCoins.Adds(NewCoins(
		NewCoin(BNBAsset, cosmos.NewUint(1000)),
		NewCoin(BTCAsset, cosmos.NewUint(1000)),
	))

	c.Assert(len(newCoins), Equals, 3)
	c.Assert(len(oldCoins), Equals, 2)
	// oldCoins asset types are unchanged, while newCoins has all types.

	c.Check(newCoins.GetCoin(BNBAsset).Amount.Uint64(), Equals, uint64(2000))
	c.Check(newCoins.GetCoin(BCHAsset).Amount.Uint64(), Equals, uint64(1000))
	c.Check(newCoins.GetCoin(BTCAsset).Amount.Uint64(), Equals, uint64(1000))
	// For newCoins, the amount adding works as expected.

	c.Check(oldCoins.GetCoin(BNBAsset).Amount.Uint64(), Equals, uint64(2000))
	// The oldCounts amount is also increased for the matching asset,
	// since type Coins is a slice (copies of slices referencing the same values).

	newerCoins := make(Coins, len(oldCoins))
	copy(newerCoins, oldCoins)
	newerCoins = newerCoins.Adds(NewCoins(
		NewCoin(BNBAsset, cosmos.NewUint(4000)),
	))
	c.Check(newerCoins.GetCoin(BNBAsset).Amount.Uint64(), Equals, uint64(6000))
	c.Check(oldCoins.GetCoin(BNBAsset).Amount.Uint64(), Equals, uint64(2000))
	// Having used make(Coins, len()) and copy(), oldCoins is unchanged.

	newAmount := oldCoins.GetCoin(BNBAsset).Amount.Add(NewCoin(BNBAsset, cosmos.NewUint(7000)).Amount)
	c.Check(newAmount.Uint64(), Equals, uint64(9000))
	c.Check(oldCoins.GetCoin(BNBAsset).Amount.Uint64(), Equals, uint64(2000))
	// When Add alone is used with .Amount and no sanitisation,
	// the newAmount is as expected while the oldCoins amount is unaffected.
}

func (s CoinSuite) TestNoneEmpty(c *C) {
	coins := Coins{
		NewCoin(BNBAsset, cosmos.NewUint(1000)),
		NewCoin(ETHAsset, cosmos.ZeroUint()),
	}
	newCoins := coins.NoneEmpty()
	c.Assert(newCoins, HasLen, 1)
	ethCoin := newCoins.GetCoin(ETHAsset)
	c.Assert(ethCoin.IsEmpty(), Equals, true)
}

func (s CoinSuite) TestHasSynthetic(c *C) {
	bnbSynthAsset, _ := NewAsset("BNB/BNB")
	coins := Coins{
		NewCoin(bnbSynthAsset, cosmos.NewUint(1000)),
		NewCoin(ETHAsset, cosmos.ZeroUint()),
	}
	c.Assert(coins.HasSynthetic(), Equals, true)
	coins = Coins{
		NewCoin(BNBAsset, cosmos.NewUint(1000)),
		NewCoin(ETHAsset, cosmos.ZeroUint()),
	}
	c.Assert(coins.HasSynthetic(), Equals, false)
}
