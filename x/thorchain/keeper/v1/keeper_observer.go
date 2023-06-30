package keeperv1

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// GetObservingAddresses - get list of observed addresses. This is a list of
// addresses that have recently contributed via observing a tx that got 2/3rds
// majority
func (k KVStore) GetObservingAddresses(ctx cosmos.Context) ([]cosmos.AccAddress, error) {
	record := make([]cosmos.AccAddress, 0)
	_, err := k.getAccAddresses(ctx, k.GetKey(ctx, prefixObservingAddresses, ""), &record)
	return record, err
}

// AddObservingAddresses - add a list of addresses that have been helpful in
// getting enough observations to process an inbound tx.
func (k KVStore) AddObservingAddresses(ctx cosmos.Context, inAddresses []cosmos.AccAddress) error {
	if len(inAddresses) == 0 {
		return nil
	}

	// combine addresses
	curr, err := k.GetObservingAddresses(ctx)
	if err != nil {
		return err
	}
	curr = append(curr, inAddresses...)

	// ensure uniqueness
	uniq := make([]cosmos.AccAddress, 0, len(curr))
	m := make(map[string]bool)
	for _, val := range curr {
		if _, ok := m[val.String()]; !ok {
			m[val.String()] = true
			uniq = append(uniq, val)
		}
	}

	k.setAccAddresses(ctx, k.GetKey(ctx, prefixObservingAddresses, ""), uniq)
	return nil
}

// ClearObservingAddresses - clear all observing addresses
func (k KVStore) ClearObservingAddresses(ctx cosmos.Context) {
	k.del(ctx, k.GetKey(ctx, prefixObservingAddresses, ""))
}
