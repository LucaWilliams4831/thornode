package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type MsgLoanSuite struct{}

var _ = Suite(&MsgLoanSuite{})

func (MsgLoanSuite) TestMsgLoanOpenSuite(c *C) {
	acc := GetRandomBech32Addr()

	owner := GetRandomTHORAddress()
	colA := common.BTCAsset
	units := cosmos.NewUint(100)
	targetA := GetRandomBTCAddress()
	msg := NewMsgLoanOpen(owner, colA, units, targetA, colA, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", cosmos.ZeroUint(), acc)
	c.Assert(msg.Route(), Equals, RouterKey)
	c.Assert(msg.Type(), Equals, "loan_open")
	c.Assert(msg.ValidateBasic(), IsNil)
	c.Assert(len(msg.GetSignBytes()) > 0, Equals, true)
	c.Assert(msg.GetSigners(), NotNil)
	c.Assert(msg.GetSigners()[0].String(), Equals, acc.String())

	msg = NewMsgLoanOpen(common.NoAddress, colA, units, targetA, colA, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", cosmos.ZeroUint(), acc)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgLoanOpen(owner, common.EmptyAsset, units, targetA, colA, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", cosmos.ZeroUint(), acc)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgLoanOpen(owner, common.TOR, units, targetA, colA, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", cosmos.ZeroUint(), acc)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgLoanOpen(owner, colA, cosmos.ZeroUint(), targetA, colA, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", cosmos.ZeroUint(), acc)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgLoanOpen(owner, colA, units, GetRandomBNBAddress(), colA, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", cosmos.ZeroUint(), acc)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgLoanOpen(owner, colA, units, common.NoAddress, colA, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", cosmos.ZeroUint(), acc)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgLoanOpen(owner, colA, units, targetA, common.EmptyAsset, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", cosmos.ZeroUint(), acc)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgLoanOpen(owner, colA, units, targetA, colA, cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), "", "", cosmos.ZeroUint(), cosmos.AccAddress{})
	c.Assert(msg.ValidateBasic(), NotNil)
}

func (MsgLoanSuite) TestMsgLoanRepaySuite(c *C) {
	acc := GetRandomBech32Addr()

	owner := GetRandomBNBAddress()
	colA := common.BTCAsset
	coin := common.NewCoin(common.BNBAsset, cosmos.NewUint(90*common.One))
	msg := NewMsgLoanRepayment(owner, colA, cosmos.OneUint(), owner, coin, acc)
	c.Assert(msg.Route(), Equals, RouterKey)
	c.Assert(msg.Type(), Equals, "loan_repayment")
	c.Assert(msg.ValidateBasic(), IsNil)
	c.Assert(len(msg.GetSignBytes()) > 0, Equals, true)
	c.Assert(msg.GetSigners(), NotNil)
	c.Assert(msg.GetSigners()[0].String(), Equals, acc.String())

	msg = NewMsgLoanRepayment(common.NoAddress, colA, cosmos.OneUint(), owner, coin, acc)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgLoanRepayment(owner, common.EmptyAsset, cosmos.OneUint(), owner, coin, acc)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgLoanRepayment(owner, colA, cosmos.OneUint(), owner, common.Coin{}, acc)
	c.Assert(msg.ValidateBasic(), NotNil)
	msg = NewMsgLoanRepayment(owner, colA, cosmos.OneUint(), owner, coin, cosmos.AccAddress{})
	c.Assert(msg.ValidateBasic(), NotNil)
}
