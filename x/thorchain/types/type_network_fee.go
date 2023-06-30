package types

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
)

// NewNetworkFee create a new instance of network fee
func NewNetworkFee(chain common.Chain, transactionSize, transactionFeeRate uint64) NetworkFee {
	return NetworkFee{
		Chain:              chain,
		TransactionSize:    transactionSize,
		TransactionFeeRate: transactionFeeRate,
	}
}

// Valid - check whether NetworkFee struct represent valid information
func (m *NetworkFee) Valid() error {
	if m.Chain.IsEmpty() {
		return errors.New("chain can't be empty")
	}
	if m.TransactionSize <= 0 {
		return fmt.Errorf("transaction size can't be zero or negative: %v", m.TransactionSize)
	}
	if m.TransactionFeeRate <= 0 {
		return errors.New("transaction fee rate can't be zero")
	}
	return nil
}
