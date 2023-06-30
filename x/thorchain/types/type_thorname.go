package types

import (
	"errors"
	"strings"

	b64 "encoding/base64"

	"gitlab.com/thorchain/thornode/common"
)

// NewTHORName create a new instance of network fee
func NewTHORName(name string, exp int64, aliases []THORNameAlias) THORName {
	return THORName{
		Name:              name,
		ExpireBlockHeight: exp,
		Aliases:           aliases,
	}
}

// Valid - check whether THORName struct represent valid information
func (m *THORName) Valid() error {
	if len(m.Name) == 0 {
		return errors.New("name can't be empty")
	}
	if len(m.Aliases) == 0 {
		return errors.New("aliases can't be empty")
	}
	for _, a := range m.Aliases {
		if a.Chain.IsEmpty() {
			return errors.New("chain can't be empty")
		}
		if a.Address.IsEmpty() {
			return errors.New("address cannot be empty")
		}
	}
	return nil
}

func (m *THORName) GetAlias(chain common.Chain) common.Address {
	for _, a := range m.Aliases {
		if a.Chain.Equals(chain) {
			return a.Address
		}
	}
	return common.NoAddress
}

func (m *THORName) SetAlias(chain common.Chain, addr common.Address) {
	for i, a := range m.Aliases {
		if a.Chain.Equals(chain) {
			m.Aliases[i].Address = addr
			return
		}
	}
	m.Aliases = append(m.Aliases, THORNameAlias{Chain: chain, Address: addr})
}

func (m *THORName) Key() string {
	// key is Base64 endoded
	return b64.StdEncoding.EncodeToString([]byte(strings.ToLower(m.Name)))
}
