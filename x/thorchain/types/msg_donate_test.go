package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type MsgDonateSuite struct{}

var _ = Suite(&MsgDonateSuite{})

func (mas *MsgDonateSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (mas *MsgDonateSuite) TestMsgDonate(c *C) {
	tx := GetRandomTx()
	addr := GetRandomBech32Addr()
	c.Check(addr.Empty(), Equals, false)
	ma := NewMsgDonate(tx, common.BNBAsset, cosmos.NewUint(100000000), cosmos.NewUint(100000000), addr)
	c.Check(ma.Route(), Equals, RouterKey)
	c.Check(ma.Type(), Equals, "donate")
	err := ma.ValidateBasic()
	c.Assert(err, IsNil)
	buf := ma.GetSignBytes()
	c.Assert(buf, NotNil)
	c.Check(len(buf) > 0, Equals, true)
	signer := ma.GetSigners()
	c.Assert(signer, NotNil)
	c.Check(len(signer) > 0, Equals, true)

	inputs := []struct {
		ticker common.Asset
		rune   cosmos.Uint
		asset  cosmos.Uint
		txHash common.TxID
		signer cosmos.AccAddress
	}{
		{
			ticker: common.Asset{},
			rune:   cosmos.NewUint(100000000),
			asset:  cosmos.NewUint(100000000),
			txHash: tx.ID,
			signer: addr,
		},
		{
			ticker: common.BNBAsset,
			rune:   cosmos.NewUint(100000000),
			asset:  cosmos.NewUint(100000000),
			txHash: common.TxID(""),
			signer: addr,
		},
		{
			ticker: common.BNBAsset,
			rune:   cosmos.NewUint(100000000),
			asset:  cosmos.NewUint(100000000),
			txHash: tx.ID,
			signer: cosmos.AccAddress{},
		},
	}
	for _, item := range inputs {
		tx := GetRandomTx()
		tx.ID = item.txHash
		msgDonate := NewMsgDonate(tx, item.ticker, item.rune, item.asset, item.signer)
		err := msgDonate.ValidateBasic()
		c.Assert(err, NotNil)
	}
}
