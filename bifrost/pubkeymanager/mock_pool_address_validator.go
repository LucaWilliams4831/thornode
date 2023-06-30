package pubkeymanager

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
)

var (
	previous   = "tbnb1hzwfk6t3sqjfuzlr0ur9lj920gs37gg92gtay9"
	current    = "tbnb1yycn4mh6ffwpjf584t8lpp7c27ghu03gpvqkfj"
	next       = "tbnb1hzwfk6t3sqjfuzlr0ur9lj920gs37gg92gtay9"
	top        = "tbnb186nvjtqk4kkea3f8a30xh4vqtkrlu2rm9xgly3"
	MockPubkey = "tthorpub1addwnpepqt8tnluxnk3y5quyq952klgqnlmz2vmaynm40fp592s0um7ucvjh5lc2l2z"
)

type MockPoolAddressValidator struct{}

func NewMockPoolAddressValidator() *MockPoolAddressValidator {
	return &MockPoolAddressValidator{}
}

func matchTestAddress(addr, testAddr string, chain common.Chain) (bool, common.ChainPoolInfo) {
	if strings.EqualFold(testAddr, addr) {
		pubKey, _ := common.NewPubKey(MockPubkey)
		cpi, err := common.NewChainPoolInfo(chain, pubKey)
		if err != nil {
			fmt.Println(err)
		}
		cpi.PoolAddress = common.Address(testAddr)
		return true, cpi
	}
	return false, common.EmptyChainPoolInfo
}

func (mpa *MockPoolAddressValidator) GetPubKeys() common.PubKeys { return nil }
func (mpa *MockPoolAddressValidator) GetSignPubKeys() common.PubKeys {
	pubKey, _ := common.NewPubKey(MockPubkey)
	return common.PubKeys{pubKey}
}
func (mpa *MockPoolAddressValidator) GetNodePubKey() common.PubKey { return common.EmptyPubKey }
func (mpa *MockPoolAddressValidator) HasPubKey(pk common.PubKey) bool {
	return pk.String() == MockPubkey
}
func (mpa *MockPoolAddressValidator) AddPubKey(pk common.PubKey, _ bool) {}
func (mpa *MockPoolAddressValidator) AddNodePubKey(pk common.PubKey)     {}
func (mpa *MockPoolAddressValidator) RemovePubKey(pk common.PubKey)      {}
func (mpa *MockPoolAddressValidator) Start() error                       { return errors.New("kaboom") }
func (mpa *MockPoolAddressValidator) Stop() error                        { return errors.New("kaboom") }

func (mpa *MockPoolAddressValidator) IsValidPoolAddress(addr string, chain common.Chain) (bool, common.ChainPoolInfo) {
	matchCurrent, cpi := matchTestAddress(addr, current, chain)
	if matchCurrent {
		return matchCurrent, cpi
	}
	matchPrevious, cpi := matchTestAddress(addr, previous, chain)
	if matchPrevious {
		return matchPrevious, cpi
	}
	matchNext, cpi := matchTestAddress(addr, next, chain)
	if matchNext {
		return matchNext, cpi
	}
	matchTop, cpi := matchTestAddress(addr, top, chain)
	if matchTop {
		return matchTop, cpi
	}
	return false, common.EmptyChainPoolInfo
}

func (mpa *MockPoolAddressValidator) RegisterCallback(callback OnNewPubKey) {
}

func (mpa *MockPoolAddressValidator) GetContracts(chain common.Chain) []common.Address {
	return nil
}

func (mpa *MockPoolAddressValidator) GetContract(chain common.Chain, pk common.PubKey) common.Address {
	return common.NoAddress
}
