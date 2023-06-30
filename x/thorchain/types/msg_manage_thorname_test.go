package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	. "gopkg.in/check.v1"
)

type MsgManageTHORNameSuite struct{}

var _ = Suite(&MsgManageTHORNameSuite{})

func (MsgManageTHORNameSuite) TestMsgManageTHORNameSuite(c *C) {
	owner := GetRandomBech32Addr()
	signer := GetRandomBech32Addr()
	coin := common.NewCoin(common.RuneAsset(), cosmos.NewUint(10*common.One))
	msg := NewMsgManageTHORName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, owner, signer)
	c.Assert(msg.Route(), Equals, RouterKey)
	c.Assert(msg.Type(), Equals, "manage_thorname")
	c.Assert(msg.ValidateBasic(), IsNil)
	c.Assert(len(msg.GetSignBytes()) > 0, Equals, true)
	c.Assert(msg.GetSigners(), NotNil)
	c.Assert(msg.GetSigners()[0].String(), Equals, signer.String())

	// unhappy paths
	msg = NewMsgManageTHORName("myname", common.BNBChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, owner, cosmos.AccAddress{})
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgManageTHORName("myname", common.EmptyChain, GetRandomBNBAddress(), coin, 0, common.BNBAsset, owner, signer)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgManageTHORName("myname", common.BNBChain, common.NoAddress, coin, 0, common.BNBAsset, owner, signer)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgManageTHORName("myname", common.BNBChain, GetRandomBTCAddress(), coin, 0, common.BNBAsset, owner, signer)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgManageTHORName("myname", common.BNBChain, GetRandomBNBAddress(), common.NewCoin(common.BNBAsset, cosmos.NewUint(10*common.One)), 0, common.BNBAsset, owner, signer)
	c.Assert(msg.ValidateBasic(), NotNil)
}
