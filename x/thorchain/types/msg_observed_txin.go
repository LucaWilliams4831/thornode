package types

import (
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// NewMsgObservedTxIn is a constructor function for MsgObservedTxIn
func NewMsgObservedTxIn(txs ObservedTxs, signer cosmos.AccAddress) *MsgObservedTxIn {
	return &MsgObservedTxIn{
		Txs:    txs,
		Signer: signer,
	}
}

// Route should return the route key of the module
func (m *MsgObservedTxIn) Route() string { return RouterKey }

// Type should return the action
func (m MsgObservedTxIn) Type() string { return "set_observed_txin" }

// ValidateBasic runs stateless checks on the message
func (m *MsgObservedTxIn) ValidateBasic() error {
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
		if !tx.Tx.ToAddress.Equals(obAddr) {
			return cosmos.ErrUnknownRequest("request is not an inbound observed transaction")
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
func (m *MsgObservedTxIn) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgObservedTxIn) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
