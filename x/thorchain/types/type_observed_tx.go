package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// ObservedTxs a list of ObservedTx
type ObservedTxs []ObservedTx

// NewObservedTx create a new instance of ObservedTx
func NewObservedTx(tx common.Tx, height int64, pk common.PubKey, finalisedHeight int64) ObservedTx {
	return ObservedTx{
		Tx:             tx,
		Status:         Status_incomplete,
		BlockHeight:    height,
		ObservedPubKey: pk,
		FinaliseHeight: finalisedHeight,
	}
}

// Valid check whether the observed tx represent valid information
func (m *ObservedTx) Valid() error {
	if err := m.Tx.Valid(); err != nil {
		return err
	}
	// Memo should not be empty, but it can't be checked here, because a
	// message failed validation will be rejected by THORNode.
	// Thus THORNode can't refund customer accordingly , which will result fund lost
	if m.BlockHeight <= 0 {
		return errors.New("block height can't be zero")
	}
	if m.ObservedPubKey.IsEmpty() {
		return errors.New("observed pool pubkey is empty")
	}
	if m.FinaliseHeight <= 0 {
		return errors.New("finalise block height can't be zero")
	}
	return nil
}

// IsEmpty check whether the Tx is empty
func (m *ObservedTx) IsEmpty() bool {
	return m.Tx.IsEmpty()
}

// Equals compare two ObservedTx
func (m ObservedTx) Equals(tx2 ObservedTx) bool {
	if !m.Tx.Equals(tx2.Tx) {
		return false
	}
	if !m.ObservedPubKey.Equals(tx2.ObservedPubKey) {
		return false
	}
	if m.BlockHeight != tx2.BlockHeight {
		return false
	}
	if m.FinaliseHeight != tx2.FinaliseHeight {
		return false
	}
	if !strings.EqualFold(m.Aggregator, tx2.Aggregator) {
		return false
	}
	if !strings.EqualFold(m.AggregatorTarget, tx2.AggregatorTarget) {
		return false
	}
	emptyAmt := cosmos.ZeroUint()
	if m.AggregatorTargetLimit == nil {
		m.AggregatorTargetLimit = &emptyAmt
	}
	if tx2.AggregatorTargetLimit == nil {
		tx2.AggregatorTargetLimit = &emptyAmt
	}
	if !m.AggregatorTargetLimit.Equal(*tx2.AggregatorTargetLimit) {
		return false
	}
	return true
}

// IsFinal indcate whether ObserveTx is final
func (m *ObservedTx) IsFinal() bool {
	return m.FinaliseHeight == m.BlockHeight
}

func (m *ObservedTx) GetOutHashes() common.TxIDs {
	txIDs := make(common.TxIDs, 0)
	for _, o := range m.OutHashes {
		txID, err := common.NewTxID(o)
		if err != nil {
			continue
		}
		txIDs = append(txIDs, txID)
	}
	return txIDs
}

// GetSigners return all the node address that had sign the tx
func (m *ObservedTx) GetSigners() []cosmos.AccAddress {
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

// String implement fmt.Stringer
func (m *ObservedTx) String() string {
	return m.Tx.String()
}

// HasSigned - check if given address has signed
func (m *ObservedTx) HasSigned(signer cosmos.AccAddress) bool {
	for _, sign := range m.GetSigners() {
		if sign.Equals(signer) {
			return true
		}
	}
	return false
}

// Sign add the given node account to signers list
// if the given signer is already in the list, it will return false, otherwise true
func (m *ObservedTx) Sign(signer cosmos.AccAddress) bool {
	if m.HasSigned(signer) {
		return false
	}
	m.Signers = append(m.Signers, signer.String())
	return true
}

// SetDone check the ObservedTx status, update it's status to done if the outbound tx had been processed
func (m *ObservedTx) SetDone(version semver.Version, hash common.TxID, numOuts int) {
	switch {
	case version.GTE(semver.MustParse("1.108.0")):
		m.SetDoneV108(hash, numOuts)
	default:
		m.SetDoneV1(hash, numOuts)
	}
}

func (m *ObservedTx) SetDoneV108(hash common.TxID, numOuts int) {
	// As an Asset->RUNE affiliate fee could also be RUNE,
	// allow multiple blank TxID OutHashes.
	// SetDone is still expected to only be called once (per ObservedTx) for each.
	if !hash.Equals(common.BlankTxID) {
		for _, done := range m.GetOutHashes() {
			if done.Equals(hash) {
				return
			}
		}
	}
	m.OutHashes = append(m.OutHashes, hash.String())
	if m.IsDone(numOuts) {
		m.Status = Status_done
	}
}

// IsDone will only return true when the number of out hashes is larger or equals the input number
func (m *ObservedTx) IsDone(numOuts int) bool {
	return len(m.OutHashes) >= numOuts
}

// ObservedTxVoters a list of observed tx voter
type ObservedTxVoters []ObservedTxVoter

// NewObservedTxVoter create a new instance of ObservedTxVoter
func NewObservedTxVoter(txID common.TxID, txs []ObservedTx) ObservedTxVoter {
	observedTxVoter := ObservedTxVoter{
		TxID: txID,
		Txs:  txs,
	}
	return observedTxVoter
}

// Valid check whether the tx is valid , if it is not , then an error will be returned
func (m *ObservedTxVoter) Valid() error {
	if m.TxID.IsEmpty() {
		return errors.New("cannot have an empty tx id")
	}

	// check all other normal tx
	for _, in := range m.Txs {
		if err := in.Valid(); err != nil {
			return err
		}
	}

	return nil
}

// Key is to get the txid
func (m *ObservedTxVoter) Key() common.TxID {
	return m.TxID
}

// String implement fmt.Stringer
func (m *ObservedTxVoter) String() string {
	return m.TxID.String()
}

// matchActionItem is to check the given outboundTx again the list of actions , return true of the outboundTx matched any of the actions
func (m *ObservedTxVoter) matchActionItem(outboundTx common.Tx) bool {
	for _, toi := range m.Actions {
		// note: Coins.Contains will match amount as well
		matchCoin := outboundTx.Coins.Contains(toi.Coin)
		if !matchCoin && toi.Coin.Asset.Equals(toi.Chain.GetGasAsset()) {
			asset := toi.Chain.GetGasAsset()
			intendToSpend := toi.Coin.Amount.Add(toi.MaxGas.ToCoins().GetCoin(asset).Amount)
			actualSpend := outboundTx.Coins.GetCoin(asset).Amount.Add(outboundTx.Gas.ToCoins().GetCoin(asset).Amount)
			if intendToSpend.Equal(actualSpend) {
				matchCoin = true
			}

		}
		if strings.EqualFold(toi.Memo, outboundTx.Memo) &&
			toi.ToAddress.Equals(outboundTx.ToAddress) &&
			toi.Chain.Equals(outboundTx.Chain) &&
			matchCoin {
			return true
		}
	}
	return false
}

// AddOutTx trying to add the outbound tx into OutTxs ,
// return value false indicate the given outbound tx doesn't match any of the
// actions items , node account should be slashed for a malicious tx
// true indicated the outbound tx matched an action item , and it has been
// added into internal OutTxs
func (m *ObservedTxVoter) AddOutTx(version semver.Version, in common.Tx) bool {
	switch {
	case version.GTE(semver.MustParse("1.108.0")):
		return m.AddOutTxV108(version, in)
	default:
		return m.AddOutTxV1(version, in)
	}
}

func (m *ObservedTxVoter) AddOutTxV108(version semver.Version, in common.Tx) bool {
	if !m.matchActionItem(in) {
		// no action item match the outbound tx
		return false
	}
	// As an Asset->RUNE affiliate fee could also be RUNE,
	// allow multiple OutTxs with blank TxIDs.
	// AddOutTxs is still expected to only be called once for each.
	if !in.ID.Equals(common.BlankTxID) {
		for _, t := range m.OutTxs {
			if in.ID.Equals(t.ID) {
				return true
			}
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

// IsDone check whether THORChain finished process the tx, all outbound tx had
// been sent and observed
func (m *ObservedTxVoter) IsDone() bool {
	return len(m.Actions) <= len(m.OutTxs)
}

// Add is trying to add the given observed tx into the voter , if the signer
// already sign , they will not add twice , it simply return false
func (m *ObservedTxVoter) Add(observedTx ObservedTx, signer cosmos.AccAddress) bool {
	// check if this signer has already signed, no take backs allowed
	votedIdx := -1
	for idx, transaction := range m.Txs {
		if !transaction.Equals(observedTx) {
			continue
		}
		votedIdx = idx
		// check whether the signer is already in the list
		for _, siggy := range transaction.GetSigners() {
			if siggy.Equals(signer) {
				return false
			}
		}

	}
	if votedIdx != -1 {
		return m.Txs[votedIdx].Sign(signer)
	}
	observedTx.Signers = []string{signer.String()}
	m.Txs = append(m.Txs, observedTx)
	return true
}

// HasConsensus is to check whether the tx with finalise = false in this ObservedTxVoter reach consensus
// if ObservedTxVoter HasFinalised , then this function will return true as well
func (m *ObservedTxVoter) HasConsensus(nodeAccounts NodeAccounts) bool {
	consensusTx := m.GetTx(nodeAccounts)
	return !consensusTx.IsEmpty()
}

// HasFinalised is to check whether the tx with finalise = true  reach super majority
func (m *ObservedTxVoter) HasFinalised(nodeAccounts NodeAccounts) bool {
	finalTx := m.GetTx(nodeAccounts)
	if finalTx.IsEmpty() {
		return false
	}
	return finalTx.IsFinal()
}

// GetTx return the tx that has super majority
func (m *ObservedTxVoter) GetTx(nodeAccounts NodeAccounts) ObservedTx {
	if !m.Tx.IsEmpty() && m.Tx.IsFinal() {
		return m.Tx
	}
	finalTx := m.getConsensusTx(nodeAccounts, true)
	if !finalTx.IsEmpty() {
		m.Tx = finalTx
	} else {
		discoverTx := m.getConsensusTx(nodeAccounts, false)
		if !discoverTx.IsEmpty() {
			m.Tx = discoverTx
		}
	}
	return m.Tx
}

func (m *ObservedTxVoter) getConsensusTx(accounts NodeAccounts, final bool) ObservedTx {
	for _, txFinal := range m.Txs {
		voters := make(map[string]bool)
		if txFinal.IsFinal() != final {
			continue
		}
		for _, txIn := range m.Txs {
			if txIn.IsFinal() != final {
				continue
			}
			if !txFinal.Tx.EqualsEx(txIn.Tx) {
				continue
			}
			for _, signer := range txIn.GetSigners() {
				_, exist := voters[signer.String()]
				if !exist && accounts.IsNodeKeys(signer) {
					voters[signer.String()] = true
				}
			}
		}
		if HasSuperMajority(len(voters), len(accounts)) {
			return txFinal
		}
	}
	return ObservedTx{}
}

// SetReverted set all the tx status to `Reverted` , only when a relevant errata tx had been processed
func (m *ObservedTxVoter) SetReverted() {
	m.setStatus(Status_reverted)
	m.Reverted = true
}

func (m *ObservedTxVoter) setStatus(toStatus Status) {
	for _, item := range m.Txs {
		item.Status = toStatus
	}
	if !m.Tx.IsEmpty() {
		m.Tx.Status = toStatus
	}
}

// SetDone set all the tx status to `done`
// usually the status will be set to done once the outbound tx get observed and processed
// there are some situation , it doesn't have outbound , those will need to set manually
func (m *ObservedTxVoter) SetDone() {
	m.setStatus(Status_done)
}

// MarshalJSON marshal Status to JSON in string form
func (x Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.String())
}

// UnmarshalJSON convert string form back to Status
func (x *Status) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if val, ok := Status_value[s]; ok {
		*x = Status(val)
		return nil
	}
	return fmt.Errorf("%s is not a valid status", s)
}

func (txs ObservedTxs) Contains(tx ObservedTx) bool {
	for _, item := range txs {
		if item.Equals(tx) {
			return true
		}
	}
	return false
}
