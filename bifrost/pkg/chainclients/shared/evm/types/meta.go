package types

import (
	"github.com/ethereum/go-ethereum/core/types"

	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
)

type SignedTxItem struct {
	Hash        string `json:"hash,omitempty"`
	Height      int64  `json:"height,omitempty"`
	VaultPubKey string `json:"vault_pub_key,omitempty"`
}

// String implement fmt.Stringer
func (st SignedTxItem) String() string {
	return st.Hash
}

// BlockMeta is a structure to store the blocks bifrost scanned
type BlockMeta struct {
	PreviousHash string            `json:"previous_hash"`
	Height       int64             `json:"height"`
	BlockHash    string            `json:"block_hash"`
	Transactions []TransactionMeta `json:"transactions"`
}

// TransactionMeta transaction meta data
type TransactionMeta struct {
	Hash        string `json:"hash"`
	BlockHeight int64  `json:"block_height"`
}

// TokenMeta is a struct to store token meta data
type TokenMeta struct {
	Symbol  string `json:"symbol"`
	Address string `json:"address"`
	Decimal uint64 `json:"decimal"` // Decimal means the number of decimals https://docs.openzeppelin.com/contracts/3.x/api/token/erc20#ERC20-decimals--
}

// NewBlockMeta create a new instance of BlockMeta
func NewBlockMeta(block *types.Header, txIn stypes.TxIn) *BlockMeta {
	txsMeta := make([]TransactionMeta, 0)

	return &BlockMeta{
		PreviousHash: block.ParentHash.Hex(),
		Height:       block.Number.Int64(),
		BlockHash:    block.Hash().Hex(),
		Transactions: txsMeta,
	}
}

// NewTokenMeta create a new instance of TokenMeta
func NewTokenMeta(symbol, address string, decimal uint64) TokenMeta {
	return TokenMeta{
		Symbol:  symbol,
		Address: address,
		Decimal: decimal,
	}
}

// IsEmpty return true when both symbol and address are empty
func (tm TokenMeta) IsEmpty() bool {
	return tm.Symbol == "" && tm.Address == ""
}
