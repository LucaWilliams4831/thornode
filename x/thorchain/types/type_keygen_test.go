package types

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type KeygenSuite struct{}

var _ = Suite(&KeygenSuite{})

func (s *KeygenSuite) TestKengenType(c *C) {
	asgardType := KeygenType_AsgardKeygen
	buf, err := json.Marshal(asgardType)
	c.Check(err, IsNil)
	var kt KeygenType
	c.Check(json.Unmarshal(buf, &kt), IsNil)
	c.Check(kt, Equals, asgardType)
}

func (s *KeygenSuite) TestKeygen(c *C) {
	var members []string
	for i := 0; i < 4; i++ {
		members = append(members, GetRandomPubKey().String())
	}
	keygen, err := NewKeygen(1, members, KeygenType_AsgardKeygen)
	c.Assert(err, IsNil)
	c.Assert(keygen.IsEmpty(), Equals, false)
	c.Assert(keygen.Valid(), IsNil)
	c.Log(keygen.String())
}

func (s *KeygenSuite) TestGetKeygenID(c *C) {
	var members []string
	for i := 0; i < 4; i++ {
		members = append(members, GetRandomPubKey().String())
	}
	txID, err := getKeygenID(1, members, KeygenType_AsgardKeygen)
	c.Assert(err, IsNil)
	c.Assert(txID.IsEmpty(), Equals, false)
	txID1, err := getKeygenID(2, members, KeygenType_AsgardKeygen)
	c.Assert(err, IsNil)
	c.Assert(txID1.IsEmpty(), Equals, false)
	// with different block height two keygen item should be different
	c.Assert(txID1.Equals(txID), Equals, false)
	// with different
	txID2, err := getKeygenID(1, members, KeygenType_YggdrasilKeygen)
	c.Assert(err, IsNil)
	c.Assert(txID.Equals(txID2), Equals, false)

	txID3, err := getKeygenID(1, members, KeygenType_AsgardKeygen)
	c.Assert(err, IsNil)
	c.Assert(txID3.Equals(txID), Equals, true)
}

func (s *KeygenSuite) TestNewKeygenBlock(c *C) {
	kb := NewKeygenBlock(1)
	c.Check(kb.IsEmpty(), Equals, false)
	var members []string
	for i := 0; i < 4; i++ {
		members = append(members, GetRandomPubKey().String())
	}
	keygen, err := NewKeygen(1, members, KeygenType_AsgardKeygen)
	c.Check(err, IsNil)
	kb.Keygens = []Keygen{
		keygen,
	}
	c.Check(len(kb.String()) > 0, Equals, true)
	c.Check(kb.Contains(keygen), Equals, true)
	kg1, err := NewKeygen(1024, members, KeygenType_YggdrasilKeygen)
	c.Check(err, IsNil)
	c.Check(kb.Contains(kg1), Equals, false)
}
