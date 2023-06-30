package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type MsgAddLiquiditySuite struct{}

var _ = Suite(&MsgAddLiquiditySuite{})

func (MsgAddLiquiditySuite) TestMsgAddLiquidity(c *C) {
	addr := GetRandomBech32Addr()
	c.Check(addr.Empty(), Equals, false)
	runeAddress := GetRandomRUNEAddress()
	assetAddress := GetRandomBNBAddress()
	txID := GetRandomTxHash()
	c.Check(txID.IsEmpty(), Equals, false)
	tx := common.NewTx(
		txID,
		runeAddress,
		GetRandomRUNEAddress(),
		common.Coins{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(100000000)),
		},
		BNBGasFeeSingleton,
		"",
	)
	m := NewMsgAddLiquidity(tx, common.BNBAsset, cosmos.NewUint(100000000), cosmos.NewUint(100000000), runeAddress, assetAddress, common.NoAddress, cosmos.ZeroUint(), addr)
	EnsureMsgBasicCorrect(m, c)
	c.Check(m.Type(), Equals, "add_liquidity")

	inputs := []struct {
		asset     common.Asset
		r         cosmos.Uint
		amt       cosmos.Uint
		runeAddr  common.Address
		assetAddr common.Address
		txHash    common.TxID
		signer    cosmos.AccAddress
	}{
		{
			asset:     common.Asset{},
			r:         cosmos.NewUint(100000000),
			amt:       cosmos.NewUint(100000000),
			runeAddr:  runeAddress,
			assetAddr: assetAddress,
			txHash:    txID,
			signer:    addr,
		},
		{
			asset:     common.BNBAsset,
			r:         cosmos.NewUint(100000000),
			amt:       cosmos.NewUint(100000000),
			runeAddr:  common.NoAddress,
			assetAddr: common.NoAddress,
			txHash:    txID,
			signer:    addr,
		},
		{
			asset:     common.BNBAsset,
			r:         cosmos.NewUint(100000000),
			amt:       cosmos.NewUint(100000000),
			runeAddr:  runeAddress,
			assetAddr: assetAddress,
			txHash:    common.TxID(""),
			signer:    addr,
		},
		{
			asset:     common.BNBAsset,
			r:         cosmos.NewUint(100000000),
			amt:       cosmos.NewUint(100000000),
			runeAddr:  runeAddress,
			assetAddr: assetAddress,
			txHash:    txID,
			signer:    cosmos.AccAddress{},
		},
	}
	for i, item := range inputs {
		tx := common.NewTx(
			item.txHash,
			GetRandomRUNEAddress(),
			GetRandomBNBAddress(),
			common.Coins{
				common.NewCoin(item.asset, item.r),
			},
			BNBGasFeeSingleton,
			"",
		)
		m := NewMsgAddLiquidity(tx, item.asset, item.r, item.amt, item.runeAddr, item.assetAddr, common.NoAddress, cosmos.ZeroUint(), item.signer)
		c.Assert(m.ValidateBasicV93(), NotNil, Commentf("%d) %s\n", i, m))
	}
	// If affiliate fee basis point is more than 1000 , the message should be rejected
	m1 := NewMsgAddLiquidity(tx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), GetRandomTHORAddress(), GetRandomBNBAddress(), GetRandomTHORAddress(), cosmos.NewUint(1024), GetRandomBech32Addr())
	c.Assert(m1.ValidateBasicV93(), NotNil)

	// check that we can have zero asset and zero rune amounts
	m1 = NewMsgAddLiquidity(tx, common.BNBAsset, cosmos.ZeroUint(), cosmos.ZeroUint(), GetRandomTHORAddress(), GetRandomBNBAddress(), GetRandomTHORAddress(), cosmos.ZeroUint(), GetRandomBech32Addr())
	c.Assert(m1.ValidateBasicV93(), IsNil)
}
