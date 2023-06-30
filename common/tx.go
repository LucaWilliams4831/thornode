package common

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type (
	// TxID is a string that can uniquely represent a transaction on different
	// block chain
	TxID string
	// TxIDs is a slice of TxID
	TxIDs []TxID
)

// BlankTxID represent blank
var BlankTxID = TxID("0000000000000000000000000000000000000000000000000000000000000000")

// NewTxID parse the input hash as TxID
func NewTxID(hash string) (TxID, error) {
	switch len(hash) {
	case 64:
		// do nothing
	case 66: // ETH check
		if !strings.HasPrefix(hash, "0x") {
			err := fmt.Errorf("txid error: must be 66 characters (got %d)", len(hash))
			return TxID(""), err
		}
	default:
		err := fmt.Errorf("txid error: must be 64 characters (got %d)", len(hash))
		return TxID(""), err
	}

	return TxID(strings.ToUpper(hash)), nil
}

// Equals check whether two TxID are the same
func (tx TxID) Equals(tx2 TxID) bool {
	return strings.EqualFold(tx.String(), tx2.String())
}

// IsEmpty return true when the tx represent empty string
func (tx TxID) IsEmpty() bool {
	return strings.TrimSpace(tx.String()) == ""
}

// String implement fmt.Stringer
func (tx TxID) String() string {
	return string(tx)
}

// Reverse returns a reversed version of the TxID
func (tx TxID) Reverse() TxID {
	t := make([]rune, len(tx))
	for i := 0; i < len(tx); i++ {
		t[i] = rune(tx[len(tx)-1-i])
	}
	return TxID(string(t))
}

// Txs a list of Tx
type Txs []Tx

// GetRagnarokTx return a tx used for ragnarok
func GetRagnarokTx(chain Chain, fromAddr, toAddr Address) Tx {
	return Tx{
		Chain:       chain,
		ID:          BlankTxID,
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Coins: Coins{
			// used for ragnarok, so doesn't really matter
			NewCoin(BNBAsset, cosmos.OneUint()),
		},
		Gas: Gas{
			// used for ragnarok, so doesn't really matter
			NewCoin(BNBAsset, cosmos.OneUint()),
		},
		Memo: "Ragnarok",
	}
}

// NewTx create a new instance of Tx based on the input information
func NewTx(txID TxID, from, to Address, coins Coins, gas Gas, memo string) Tx {
	var chain Chain
	for _, coin := range coins {
		chain = coin.Asset.GetChain()
		break
	}
	return Tx{
		ID:          txID,
		Chain:       chain,
		FromAddress: from,
		ToAddress:   to,
		Coins:       coins,
		Gas:         gas,
		Memo:        memo,
	}
}

// Hash calculate a hash based on from address, coins and to address
func (tx Tx) Hash() string {
	str := fmt.Sprintf("%s|%s", tx.FromAddress, tx.ToAddress)
	return fmt.Sprintf("%X", sha256.Sum256([]byte(str)))
}

// String implement fmt.Stringer return a string representation of the tx
func (tx Tx) String() string {
	return fmt.Sprintf("%s: %s ==> %s (Memo: %s) %s (gas: %s)", tx.ID, tx.FromAddress, tx.ToAddress, tx.Memo, tx.Coins, tx.Gas)
}

// IsEmpty check whether the ID field is empty or not
func (tx Tx) IsEmpty() bool {
	return tx.ID.IsEmpty()
}

// Equals compare two Tx to see whether they represent the same Tx
// Note:
// 1) this method has a side effect , it is using coins.Equals to compare coins , which potentially will  change the order of coins in tx & tx2
// 2) for the places that are already using Equals , it can't be changed, continue to use Equals otherwise it will cause consensus failure on CHAOSNET
// Deprecated
func (tx Tx) Equals(tx2 Tx) bool {
	if !tx.ID.Equals(tx2.ID) {
		return false
	}
	if !tx.Chain.Equals(tx2.Chain) {
		return false
	}
	if !tx.FromAddress.Equals(tx2.FromAddress) {
		return false
	}
	if !tx.ToAddress.Equals(tx2.ToAddress) {
		return false
	}
	if !tx.Coins.Equals(tx2.Coins) {
		return false
	}
	if !tx.Gas.Equals(tx2.Gas) {
		return false
	}
	if !strings.EqualFold(tx.Memo, tx2.Memo) {
		return false
	}
	return true
}

// EqualsEx compare two Tx to see whether they represent the same Tx
// This method will not change the original tx & tx2
func (tx Tx) EqualsEx(tx2 Tx) bool {
	if !tx.ID.Equals(tx2.ID) {
		return false
	}
	if !tx.Chain.Equals(tx2.Chain) {
		return false
	}
	if !tx.FromAddress.Equals(tx2.FromAddress) {
		return false
	}
	if !tx.ToAddress.Equals(tx2.ToAddress) {
		return false
	}
	if !tx.Coins.EqualsEx(tx2.Coins) {
		return false
	}
	if !tx.Gas.Equals(tx2.Gas) {
		return false
	}
	if !strings.EqualFold(tx.Memo, tx2.Memo) {
		return false
	}
	return true
}

// Valid do some data sanity check , if the tx contains invalid information
// it will return an none nil error
func (tx Tx) Valid() error {
	if tx.ID.IsEmpty() {
		return errors.New("Tx ID cannot be empty")
	}
	if tx.FromAddress.IsEmpty() {
		return errors.New("from address cannot be empty")
	}
	if tx.ToAddress.IsEmpty() {
		return errors.New("to address cannot be empty")
	}
	if tx.Chain.IsEmpty() {
		return errors.New("chain cannot be empty")
	}
	if len(tx.Coins) == 0 {
		return errors.New("must have at least 1 coin")
	}
	if err := tx.Coins.Valid(); err != nil {
		return err
	}
	if !tx.Chain.Equals(THORChain) && len(tx.Gas) == 0 {
		return errors.New("must have at least 1 gas coin")
	}
	if err := tx.Gas.Valid(); err != nil {
		return err
	}
	// relax this check from 150 -> 180
	if len([]byte(tx.Memo)) > constants.MaxMemoSize {
		return fmt.Errorf("memo must not exceed %d bytes: %d", constants.MaxMemoSize, len([]byte(tx.Memo)))
	}
	return nil
}

// ToAttributes push all the tx fields into a slice of cosmos Attribute(key value pairs)
func (tx Tx) ToAttributes() []cosmos.Attribute {
	return []cosmos.Attribute{
		cosmos.NewAttribute("id", tx.ID.String()),
		cosmos.NewAttribute("chain", tx.Chain.String()),
		cosmos.NewAttribute("from", tx.FromAddress.String()),
		cosmos.NewAttribute("to", tx.ToAddress.String()),
		cosmos.NewAttribute("coin", tx.Coins.String()),
		cosmos.NewAttribute("memo", tx.Memo),
	}
}
