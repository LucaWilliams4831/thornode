package keeperv1

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

// ratioLength ensures that the character length of the ratio store in the key
// of the index is always the same length. This is to ensure that the kvstore
// can iterate over the numbers numerically, even though it actually iterates
// over alphabetically (the two become the same). I suspect this number will
// never change as it does give a large granularity to attempt to swap. The
// amount of tokens emitted is, in the end, still respected by the swap limit.
// In the event that this number is changed, it has to be version'ed, and also
// a kvstore migration updating all ratios in the keys to be updated with the
// new length.
// A value of 18 means that granularity is maxed out at 1 trillion to 1 ratio.
const ratioLength int = 18

// OrderBooksEnabled return true if the orderbooks feature is enabled
func (k KVStore) OrderBooksEnabled(ctx cosmos.Context) bool {
	val := k.GetConfigInt64(ctx, constants.EnableOrderBooks)
	return val > 0
}

// SetOrderBookItem - writes a order book item to the kv store
func (k KVStore) SetOrderBookItem(ctx cosmos.Context, msg MsgSwap) error {
	if msg.Tx.Coins == nil || len(msg.Tx.Coins) != 1 {
		return fmt.Errorf("incorrect number of coins in transaction (%d)", len(msg.Tx.Coins))
	}
	if msg.OrderType == types.OrderType_limit && msg.TradeTarget.IsZero() {
		return fmt.Errorf("trade target cannot be zero for limit orders")
	}
	if msg.Tx.ID.IsEmpty() {
		return fmt.Errorf("invalid tx hash")
	}
	if err := k.SetOrderBookIndex(ctx, msg); err != nil {
		return err
	}
	k.setMsgSwap(ctx, k.GetKey(ctx, prefixOrderBookItem, msg.Tx.ID.String()), msg)
	return nil
}

// GetOrderBookItemIterator iterate order book items
func (k KVStore) GetOrderBookItemIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixOrderBookItem)
}

// GetOrderBookItem - read the given order book item information from key values store
func (k KVStore) GetOrderBookItem(ctx cosmos.Context, txID common.TxID) (MsgSwap, error) {
	record := MsgSwap{}
	ok, err := k.getMsgSwap(ctx, k.GetKey(ctx, prefixOrderBookItem, txID.String()), &record)
	if !ok {
		return record, errors.New("not found")
	}
	return record, err
}

// HasOrderBookItem - checks if order book item already exists
func (k KVStore) HasOrderBookItem(ctx cosmos.Context, txID common.TxID) bool {
	record := MsgSwap{}
	ok, _ := k.getMsgSwap(ctx, k.GetKey(ctx, prefixOrderBookItem, txID.String()), &record)
	return ok
}

// RemoveOrderBookItem - removes a order book item from the kv store
func (k KVStore) RemoveOrderBookItem(ctx cosmos.Context, txID common.TxID) error {
	msg, err := k.GetOrderBookItem(ctx, txID)
	if err != nil {
		_ = dbError(ctx, "failed to fetch order book item", err)
	} else {
		err = k.RemoveOrderBookIndex(ctx, msg)
	}
	k.del(ctx, k.GetKey(ctx, prefixOrderBookItem, txID.String()))
	return err
}

///-------------------------- Order Book Processor --------------------------///
// The Order Book Processor tracks a list of pairs to be processed in the next
// block to check for any limit orders that are available to be executed. This
// is stored as an array of bools.

// SetOrderBookProcessor - writes a list of pairs to process
func (k KVStore) SetOrderBookProcessor(ctx cosmos.Context, record []bool) error {
	key := k.GetKey(ctx, prefixOrderBookProcessor, "")
	k.setBools(ctx, key, record)
	return nil
}

// GetOrderBookProcessor - get a list of asset pairs to process
func (k KVStore) GetOrderBookProcessor(ctx cosmos.Context) ([]bool, error) {
	key := k.GetKey(ctx, prefixOrderBookProcessor, "")
	var record []bool
	_, err := k.getBools(ctx, key, &record)
	return record, err
}

///----------------------------------------------------------------------///

///-------------------------- Order Book Index --------------------------///

// SetOrderBookIndex - writes a order book index to the kv store
func (k KVStore) SetOrderBookIndex(ctx cosmos.Context, msg MsgSwap) error {
	ok, err := k.HasOrderBookIndex(ctx, msg)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}

	key := k.getOrderBookIndexKey(ctx, msg)
	record := make([]string, 0)
	_, err = k.getStrings(ctx, key, &record)
	if err != nil {
		return err
	}
	record = append(record, msg.Tx.ID.String())
	k.setStrings(ctx, key, record)
	return nil
}

// GetOrderBookIterator iterate order book items
func (k KVStore) GetOrderBookIndexIterator(ctx cosmos.Context, order types.OrderType, source, target common.Asset) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	switch order {
	case types.OrderType_limit:
		prefix := k.GetKey(ctx, prefixOrderBookLimitIndex, fmt.Sprintf("%s>%s/", source, target))
		return cosmos.KVStoreReversePrefixIterator(store, []byte(prefix))
	case types.OrderType_market:
		return nil
	default:
		return nil
	}
}

// GetOrderBookIndex - read the given order book index information from key values tore
func (k KVStore) GetOrderBookIndex(ctx cosmos.Context, msg MsgSwap) (common.TxIDs, error) {
	key := k.getOrderBookIndexKey(ctx, msg)
	record := make([]string, 0)
	_, err := k.getStrings(ctx, key, &record)
	if err != nil {
		return nil, err
	}
	result := make(common.TxIDs, len(record))
	for i, rec := range record {
		hash, err := common.NewTxID(rec)
		if err != nil {
			_ = dbError(ctx, fmt.Sprintf("failed to parse tx hash: (%s)", rec), err)
			continue
		}
		result[i] = hash
	}
	return result, nil
}

// HasOrderBookIndex - checks if order book item already exists
func (k KVStore) HasOrderBookIndex(ctx cosmos.Context, msg MsgSwap) (bool, error) {
	key := k.getOrderBookIndexKey(ctx, msg)
	record := make([]string, 0)
	_, err := k.getStrings(ctx, key, &record)
	if err != nil {
		return false, err
	}
	for _, r := range record {
		if strings.EqualFold(msg.Tx.ID.String(), r) {
			return true, nil
		}
	}
	return false, nil
}

// RemoveOrderBookIndex - removes a order book item from the kv store
func (k KVStore) RemoveOrderBookIndex(ctx cosmos.Context, msg MsgSwap) error {
	key := k.getOrderBookIndexKey(ctx, msg)
	record := make([]string, 0)
	_, err := k.getStrings(ctx, key, &record)
	if err != nil {
		return err
	}

	found := false
	for i, rec := range record {
		if strings.EqualFold(rec, msg.Tx.ID.String()) {
			record = removeString(record, i)
			found = true
			break
		}
	}

	if len(record) == 0 {
		k.del(ctx, key)
		return nil
	}
	if found {
		k.setStrings(ctx, key, record)
	}
	return nil
}

func (k KVStore) getOrderBookIndexKey(ctx cosmos.Context, msg MsgSwap) string {
	switch msg.OrderType {
	case types.OrderType_limit:
		ra := rewriteRatio(ratioLength, getRatio(msg.Tx.Coins[0].Amount, msg.TradeTarget))
		f := msg.Tx.Coins[0].Asset
		t := msg.TargetAsset
		return k.GetKey(ctx, prefixOrderBookLimitIndex, fmt.Sprintf("%s>%s/%s/", f.String(), t.String(), ra))
	case types.OrderType_market:
		return k.GetKey(ctx, prefixOrderBookMarketIndex, "")
	default:
		return ""
	}
}

func getRatio(input, output cosmos.Uint) string {
	if output.IsZero() {
		return "0"
	}
	return input.MulUint64(1e8).Quo(output).String()
}

// rewriteRatio. In order to ensure these ratios are stored in alphabetical
// order (instead of numerological order), the length of the string always
// needs to be consistent (ie 18 chars). If the length is larger than this,
// then we start to lose precision by chopping the end of the string off.
func rewriteRatio(length int, str string) string {
	switch {
	case len(str) < length:
		var b strings.Builder
		for i := 1; i <= length-len(str); i += 1 {
			b.WriteString("0")
		}
		b.WriteString(str)
		return b.String()
	case len(str) > length:
		return str[:length]
	}
	return str
}

// removeString - remove a string from the slice. Does NOT maintain order, but
// is faster.
func removeString(a []string, i int) []string {
	if i > len(a)-1 || i < 0 {
		return a
	}
	a[i] = a[len(a)-1]  // Copy last element to index i.
	a[len(a)-1] = ""    // Erase last element (write zero value).
	return a[:len(a)-1] // Truncate slice.
}
