package utxo

import (
	"strings"

	"github.com/rs/zerolog/log"
)

// -------------------------------------------------------------------------------------
// BlockMeta
// -------------------------------------------------------------------------------------

// BlockMeta contains a subset of meta information about a block relevant to Bifrost's
// book keeping of pending operations.
type BlockMeta struct {
	Height       int64  `json:"height"`
	PreviousHash string `json:"previous_hash"`
	BlockHash    string `json:"block_hash"`

	// SelfTransactions records txids that our vaults have broadcast.
	SelfTransactions []string `json:"self_transactions,omitempty"`

	// CustomerTransactions records txids that our vaults have received.
	CustomerTransactions []string `json:"customer_transactions,omitempty"`
}

func NewBlockMeta(previousHash string, height int64, blockHash string) *BlockMeta {
	return &BlockMeta{
		PreviousHash: previousHash,
		Height:       height,
		BlockHash:    blockHash,
	}
}

// AddSelfTransaction adds the provided txid to the BlockMeta self transactions.
func (b *BlockMeta) AddSelfTransaction(txid string) {
	b.SelfTransactions = addTransaction(b.SelfTransactions, txid)
}

// AddCustomerTransaction adds the provided txid to the BlockMeta customer transactions.
func (b *BlockMeta) AddCustomerTransaction(txid string) {
	for _, tx := range b.SelfTransactions {
		if strings.EqualFold(tx, txid) {
			log.Info().Str("txid", txid).Msg("customer txn with matching self txn seen")
			// TODO: when can this occur? comment if expected, or error log if sanity check
			return
		}
	}

	b.CustomerTransactions = addTransaction(b.CustomerTransactions, txid)
}

// RemoveCustomerTransaction removes the provided txid from the BlockMeta customer
// transactions.
func (b *BlockMeta) RemoveCustomerTransaction(txid string) {
	b.CustomerTransactions = removeTransaction(b.CustomerTransactions, txid)
}

// TransactionHashExist returns true if the txid exists in either the self or customer
// transactions for the BlockMeta.
func (b *BlockMeta) TransactionHashExists(hash string) bool {
	for _, item := range b.CustomerTransactions {
		if strings.EqualFold(item, hash) {
			return true
		}
	}
	for _, item := range b.SelfTransactions {
		if strings.EqualFold(item, hash) {
			return true
		}
	}
	return false
}

// ------------------------------ internal ------------------------------

func addTransaction(hashes []string, txid string) []string {
	var exist bool
	for _, tx := range hashes {
		if strings.EqualFold(tx, txid) {
			exist = true
			break
		}
	}
	if !exist {
		hashes = append(hashes, txid)
	}
	return hashes
}

func removeTransaction(hashes []string, txid string) []string {
	idx := 0
	toDelete := false
	for i, tx := range hashes {
		if strings.EqualFold(tx, txid) {
			idx = i
			toDelete = true
			break
		}
	}
	if toDelete {
		hashes = append(hashes[:idx], hashes[idx+1:]...)
	}
	return hashes
}
