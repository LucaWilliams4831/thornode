package types

import (
	"errors"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
)

type MsgTssPoolSuite struct{}

var _ = Suite(&MsgTssPoolSuite{})

func (s *MsgTssPoolSuite) TestMsgTssPool(c *C) {
	pk := GetRandomPubKey()
	pks := []string{
		GetRandomPubKey().String(), GetRandomPubKey().String(), GetRandomPubKey().String(),
	}
	addr, err := common.PubKey(pks[0]).GetThorAddress()
	c.Assert(err, IsNil)
	keygenTime := int64(1024)
	msg, err := NewMsgTssPool(pks, pk, nil, KeygenType_AsgardKeygen, 1, Blame{}, []string{common.RuneAsset().Chain.String()}, addr, keygenTime)
	c.Assert(err, IsNil)
	c.Check(msg.Type(), Equals, "set_tss_pool")
	c.Assert(msg.ValidateBasic(), IsNil)
	EnsureMsgBasicCorrect(msg, c)

	chains := []string{common.RuneAsset().Chain.String()}
	m, err := NewMsgTssPool(pks, pk, nil, KeygenType_AsgardKeygen, 1, Blame{}, chains, nil, keygenTime)
	c.Assert(m, NotNil)
	c.Assert(err, IsNil)
	c.Assert(m.ValidateBasic(), NotNil)
	m, err = NewMsgTssPool(nil, pk, nil, KeygenType_AsgardKeygen, 1, Blame{}, chains, addr, keygenTime)
	c.Assert(m, NotNil)
	c.Assert(err, IsNil)
	c.Assert(m.ValidateBasic(), NotNil)
	m, err = NewMsgTssPool(pks, "", nil, KeygenType_AsgardKeygen, 1, Blame{}, chains, addr, keygenTime)
	c.Assert(m, NotNil)
	c.Assert(err, IsNil)
	c.Assert(m.ValidateBasic(), NotNil)
	m, err = NewMsgTssPool(pks, "bogusPubkey", nil, KeygenType_AsgardKeygen, 1, Blame{}, chains, addr, keygenTime)
	c.Assert(m, NotNil)
	c.Assert(err, IsNil)
	c.Assert(m.ValidateBasic(), NotNil)

	// fails on empty chain list
	msg, err = NewMsgTssPool(pks, pk, nil, KeygenType_AsgardKeygen, 1, Blame{}, []string{}, addr, keygenTime)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), NotNil)
	// fails on duplicates in chain list
	msg, err = NewMsgTssPool(pks, pk, nil, KeygenType_AsgardKeygen, 1, Blame{}, []string{common.RuneAsset().Chain.String(), common.RuneAsset().Chain.String()}, addr, keygenTime)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), NotNil)

	msg1, err := NewMsgTssPool(pks, pk, nil, KeygenType_AsgardKeygen, 1, Blame{}, chains, addr, keygenTime)
	c.Assert(err, IsNil)
	msg1.ID = ""
	err1 := msg1.ValidateBasic()
	c.Assert(err1, NotNil)
	c.Check(errors.Is(err1, se.ErrUnknownRequest), Equals, true)

	msg2, err := NewMsgTssPool(append(pks, ""), pk, nil, KeygenType_AsgardKeygen, 1, Blame{}, chains, addr, keygenTime)
	c.Assert(err, IsNil)
	err2 := msg2.ValidateBasic()
	c.Assert(err2, NotNil)
	c.Check(errors.Is(err2, se.ErrUnknownRequest), Equals, true)

	var allPks []string
	for i := 0; i < 110; i++ {
		allPks = append(allPks, GetRandomPubKey().String())
	}
	msg3, err := NewMsgTssPool(allPks, pk, nil, KeygenType_AsgardKeygen, 1, Blame{}, chains, addr, keygenTime)
	c.Assert(err, IsNil)
	err3 := msg3.ValidateBasic()
	c.Assert(err3, NotNil)
	c.Check(errors.Is(err3, se.ErrUnknownRequest), Equals, true)
}
