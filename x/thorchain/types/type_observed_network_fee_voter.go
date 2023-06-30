package types

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewObservedNetworkFeeVoter create a new instance of ObservedNetworkFeeVoter
func NewObservedNetworkFeeVoter(reportBlockHeight int64, chain common.Chain) ObservedNetworkFeeVoter {
	return ObservedNetworkFeeVoter{
		ReportBlockHeight: reportBlockHeight,
		Chain:             chain,
	}
}

func (m *ObservedNetworkFeeVoter) GetSigners() []cosmos.AccAddress {
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
func (m *ObservedNetworkFeeVoter) HasSigned(signer cosmos.AccAddress) bool {
	for _, sign := range m.GetSigners() {
		if sign.Equals(signer) {
			return true
		}
	}
	return false
}

// Sign this voter with given signer address
func (m *ObservedNetworkFeeVoter) Sign(signer cosmos.AccAddress) bool {
	if m.HasSigned(signer) {
		return false
	}
	m.Signers = append(m.Signers, signer.String())
	return true
}

// HasConsensus Determine if this errata has enough signers
func (m *ObservedNetworkFeeVoter) HasConsensus(nas NodeAccounts) bool {
	var count int
	for _, signer := range m.GetSigners() {
		if nas.IsNodeKeys(signer) {
			count++
		}
	}
	return HasSuperMajority(count, len(nas))
}

// IsEmpty return true when chain is empty and block height is 0
func (m *ObservedNetworkFeeVoter) IsEmpty() bool {
	return m.Chain.IsEmpty() && m.ReportBlockHeight == 0
}

// String implement fmt.Stringer
func (m *ObservedNetworkFeeVoter) String() string {
	if m.FeeRate > 0 {
		return fmt.Sprintf("%s-%d-%d", m.Chain.String(), m.ReportBlockHeight, m.FeeRate)
	}
	return fmt.Sprintf("%s-%d", m.Chain.String(), m.ReportBlockHeight)
}
