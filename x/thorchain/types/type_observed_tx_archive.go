package types

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
)

func (m *ObservedTx) SetDoneV1(hash common.TxID, numOuts int) {
	for _, done := range m.GetOutHashes() {
		if done.Equals(hash) {
			return
		}
	}
	m.OutHashes = append(m.OutHashes, hash.String())
	if m.IsDone(numOuts) {
		m.Status = Status_done
	}
}

func (m *ObservedTxVoter) AddOutTxV1(version semver.Version, in common.Tx) bool {
	if !m.matchActionItem(in) {
		// no action item match the outbound tx
		return false
	}
	for _, t := range m.OutTxs {
		if in.ID.Equals(t.ID) {
			return true
		}
	}
	m.OutTxs = append(m.OutTxs, in)
	for i := range m.Txs {
		m.Txs[i].SetDone(version, in.ID, len(m.Actions))
	}

	if !m.Tx.IsEmpty() {
		m.Tx.SetDone(version, in.ID, len(m.Actions))
	}

	return true
}
