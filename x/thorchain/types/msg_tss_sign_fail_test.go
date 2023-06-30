package types

import (
	"errors"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type MsgTssKeysignFailSuite struct{}

var _ = Suite(&MsgTssKeysignFailSuite{})

func (s MsgTssKeysignFailSuite) TestMsgTssKeysignFail(c *C) {
	b := Blame{
		FailReason: "fail to TSS sign",
		BlameNodes: []Node{
			{Pubkey: GetRandomPubKey().String()},
			{Pubkey: GetRandomPubKey().String()},
		},
	}
	coins := common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100)),
	}
	msg, err := NewMsgTssKeysignFail(1, b, "hello", coins, GetRandomBech32Addr(), GetRandomPubKey())
	c.Assert(err, IsNil)
	c.Check(msg.Type(), Equals, "set_tss_keysign_fail")
	EnsureMsgBasicCorrect(msg, c)
	m, err := NewMsgTssKeysignFail(1, Blame{}, "hello", coins, GetRandomBech32Addr(), GetRandomPubKey())
	c.Assert(m, NotNil)
	c.Assert(err, IsNil)
	m, err = NewMsgTssKeysignFail(1, b, "", coins, GetRandomBech32Addr(), GetRandomPubKey())
	c.Assert(m, NotNil)
	c.Assert(err, IsNil)
	m, err = NewMsgTssKeysignFail(1, b, "hello", common.Coins{}, GetRandomBech32Addr(), GetRandomPubKey())
	c.Assert(m, NotNil)
	c.Assert(err, IsNil)
	m, err = NewMsgTssKeysignFail(1, b, "hello", common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100)),
		common.NewCoin(common.EmptyAsset, cosmos.ZeroUint()),
	}, GetRandomBech32Addr(), GetRandomPubKey())
	c.Assert(m, NotNil)
	c.Assert(err, IsNil)
	m, err = NewMsgTssKeysignFail(1, b, "hello", coins, cosmos.AccAddress{}, GetRandomPubKey())
	c.Assert(m, NotNil)
	c.Assert(err, IsNil)
	msg2, err := NewMsgTssKeysignFail(1, b, "hello", coins, cosmos.AccAddress{}, GetRandomPubKey())
	c.Assert(err, IsNil)
	err2 := msg2.ValidateBasic()
	c.Check(err2, NotNil)
	c.Check(errors.Is(err2, se.ErrInvalidAddress), Equals, true)

	msg3, err := NewMsgTssKeysignFail(1, b, "hello", coins, GetRandomBech32Addr(), GetRandomPubKey())
	c.Assert(err, IsNil)
	msg3.ID = ""
	err3 := msg3.ValidateBasic()
	c.Check(err3, NotNil)
	c.Check(errors.Is(err3, se.ErrUnknownRequest), Equals, true)

	msg4, err := NewMsgTssKeysignFail(1, Blame{}, "hello", coins, GetRandomBech32Addr(), GetRandomPubKey())
	c.Assert(err, IsNil)
	err4 := msg4.ValidateBasic()
	c.Check(err4, NotNil)
	c.Check(errors.Is(err4, se.ErrUnknownRequest), Equals, true)

	msg4.Coins = append(msg4.Coins, common.NewCoin(common.EmptyAsset, cosmos.ZeroUint()))
	err4 = msg4.ValidateBasic()
	c.Check(err4, NotNil)
	c.Check(errors.Is(err4, se.ErrInvalidCoins), Equals, true)

	msg5, err := NewMsgTssKeysignFail(1, b, "hello", common.Coins{}, GetRandomBech32Addr(), GetRandomPubKey())
	c.Assert(err, IsNil)
	err5 := msg5.ValidateBasic()
	c.Check(err5, NotNil)
	c.Check(errors.Is(err5, se.ErrUnknownRequest), Equals, true)

	msg6, err := NewMsgTssKeysignFail(1, b, "hello", coins, GetRandomBech32Addr(), common.EmptyPubKey)
	c.Assert(err, IsNil)
	err6 := msg6.ValidateBasic()
	c.Check(err6, NotNil)
	c.Check(errors.Is(err6, se.ErrUnknownRequest), Equals, true)
}
