package types

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewTssKeysignFailVoter create a new instance of TssKeysignFailVoter
func NewTssKeysignFailVoter(id string, height int64) TssKeysignFailVoter {
	return TssKeysignFailVoter{
		ID:     id,
		Height: height,
	}
}

func (m *TssKeysignFailVoter) GetSigners() []cosmos.AccAddress {
	addrs := make([]cosmos.AccAddress, 0)
	for _, a := range m.Signers {
		addr, err := cosmos.AccAddressFromBech32(a)
		if err != nil {
			continue
		}
		addrs = append(addrs, addr)
	}
	return addrs
}

// HasSigned - check if given address has signed
func (m *TssKeysignFailVoter) HasSigned(signer cosmos.AccAddress) bool {
	for _, sign := range m.GetSigners() {
		if sign.Equals(signer) {
			return true
		}
	}
	return false
}

// Sign this voter with given signer address
func (m *TssKeysignFailVoter) Sign(signer cosmos.AccAddress) bool {
	if m.HasSigned(signer) {
		return false
	}
	m.Signers = append(m.Signers, signer.String())
	return true
}

// HasConsensus determine if this tss pool has enough signers
func (m *TssKeysignFailVoter) HasConsensus(nas NodeAccounts) bool {
	var count int
	for _, signer := range m.GetSigners() {
		for _, item := range nas {
			if signer.Equals(item.NodeAddress) {
				count++
				break
			}
		}
	}
	return HasSimpleMajority(count, len(nas))
}

// Empty to check whether this Voter is empty or not
func (m *TssKeysignFailVoter) Empty() bool {
	return len(m.ID) == 0 || m.Height == 0
}

// String implement fmt.Stringer , return's the ID
func (m *TssKeysignFailVoter) String() string {
	return m.ID
}
