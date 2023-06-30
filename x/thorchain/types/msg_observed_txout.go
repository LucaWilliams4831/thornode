package types

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgObservedTxOut is a constructor function for MsgObservedTxOut
func NewMsgObservedTxOut(txs ObservedTxs, signer cosmos.AccAddress) *MsgObservedTxOut {
	return &MsgObservedTxOut{
		Txs:    txs,
		Signer: signer,
	}
}

// Route should return the route key of the module
func (m *MsgObservedTxOut) Route() string { return RouterKey }

// Type should return the action
func (m MsgObservedTxOut) Type() string { return "set_observed_txout" }

// ValidateBasic runs stateless checks on the message
func (m *MsgObservedTxOut) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if len(m.Txs) == 0 {
		return cosmos.ErrUnknownRequest("Txs cannot be empty")
	}
	for _, tx := range m.Txs {
		if err := tx.Valid(); err != nil {
			return cosmos.ErrUnknownRequest(err.Error())
		}
		obAddr, err := tx.ObservedPubKey.GetAddress(tx.Tx.Coins[0].Asset.GetChain())
		if err != nil {
			return cosmos.ErrUnknownRequest(err.Error())
		}
		if !tx.Tx.FromAddress.Equals(obAddr) {
			return cosmos.ErrUnknownRequest("Request is not an outbound observed transaction")
		}
		if len(tx.Signers) > 0 {
			return cosmos.ErrUnknownRequest("signers must be empty")
		}
		if len(tx.OutHashes) > 0 {
			return cosmos.ErrUnknownRequest("out hashes must be empty")
		}
		if tx.Status != Status_incomplete {
			return cosmos.ErrUnknownRequest("status must be incomplete")
		}
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgObservedTxOut) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgObservedTxOut) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
