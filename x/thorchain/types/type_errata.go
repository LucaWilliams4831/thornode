package types

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewErrataTxVoter create a new instance of ErrataTxVoter
func NewErrataTxVoter(txID common.TxID, chain common.Chain) ErrataTxVoter {
	return ErrataTxVoter{
		TxID:  txID,
		Chain: chain,
	}
}

func (m *ErrataTxVoter) GetSigners() []cosmos.AccAddress {
	signers := make([]cosmos.AccAddress, 0)
	for _, str := range m.Signers {
		signer, err := cosmos.AccAddressFromBech32(str)
		if err != nil {
			continue
		}
		signers = append(signers, signer)
	}
	return signers
}

// HasSigned - check if given address has signed
func (m *ErrataTxVoter) HasSigned(signer cosmos.AccAddress) bool {
	for _, sign := range m.GetSigners() {
		if sign.Equals(signer) {
			return true
		}
	}
	return false
}

// Sign this voter with the given signer address, if the given signer is already signed , it return false
// otherwise it add the given signer to the signers list and return true
func (m *ErrataTxVoter) Sign(signer cosmos.AccAddress) bool {
	if m.HasSigned(signer) {
		return false
	}
	m.Signers = append(m.Signers, signer.String())
	return true
}

// HasConsensus determine if this errata has enough signers
func (m *ErrataTxVoter) HasConsensus(nas NodeAccounts) bool {
	var count int
	for _, signer := range m.GetSigners() {
		if nas.IsNodeKeys(signer) {
			count++
		}
	}
	return HasSuperMajority(count, len(nas))
}

// Empty check whether TxID or Chain is empty
func (m *ErrataTxVoter) Empty() bool {
	return m.TxID.IsEmpty() || m.Chain.IsEmpty()
}

// String implement fmt.Stinger , return a string representation of errata tx voter
func (m *ErrataTxVoter) String() string {
	return fmt.Sprintf("%s-%s", m.Chain.String(), m.TxID.String())
}
