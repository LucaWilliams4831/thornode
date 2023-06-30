package types

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// TxOutItem represent the information of a tx bifrost need to process
type TxOutItem struct {
	Chain                 common.Chain   `json:"chain"`
	ToAddress             common.Address `json:"to"`
	VaultPubKey           common.PubKey  `json:"vault_pubkey"`
	Coins                 common.Coins   `json:"coins"`
	Memo                  string         `json:"memo"`
	MaxGas                common.Gas     `json:"max_gas"`
	GasRate               int64          `json:"gas_rate"`
	InHash                common.TxID    `json:"in_hash"`
	OutHash               common.TxID    `json:"out_hash"`
	Aggregator            string         `json:"aggregator"`
	AggregatorTargetAsset string         `json:"aggregator_target_asset,omitempty"`
	AggregatorTargetLimit *cosmos.Uint   `json:"aggregator_target_limit,omitempty"`
	Checkpoint            []byte         `json:"-"`
}

// Hash return a sha256 hash that can uniquely represent the TxOutItem
func (tx TxOutItem) Hash() string {
	str := fmt.Sprintf("%s|%s|%s|%s|%s|%s", tx.Chain, tx.ToAddress, tx.VaultPubKey, tx.Coins, tx.Memo, tx.InHash)
	return fmt.Sprintf("%X", sha256.Sum256([]byte(str)))
}

// CacheHash return a hash that doesn't include VaultPubKey , thus this one can be used as cache key for txOutItem across different vaults
func (tx TxOutItem) CacheHash() string {
	str := fmt.Sprintf("%s|%s|%s|%s|%s", tx.Chain, tx.ToAddress, tx.Coins, tx.Memo, tx.InHash)
	return fmt.Sprintf("%X", sha256.Sum256([]byte(str)))
}

// Equals compare two TxOutItem , return true when they are the same , otherwise false
func (tx TxOutItem) Equals(tx2 TxOutItem) bool {
	if !tx.Chain.Equals(tx2.Chain) {
		return false
	}
	if !tx.VaultPubKey.Equals(tx2.VaultPubKey) {
		return false
	}
	if !tx.ToAddress.Equals(tx2.ToAddress) {
		return false
	}
	if !tx.Coins.Equals(tx2.Coins) {
		return false
	}
	if !tx.InHash.Equals(tx2.InHash) {
		return false
	}
	if !strings.EqualFold(tx.Memo, tx2.Memo) {
		return false
	}
	if tx.GasRate != tx2.GasRate {
		return false
	}
	if !strings.EqualFold(tx.Aggregator, tx2.Aggregator) {
		return false
	}
	if !strings.EqualFold(tx.AggregatorTargetAsset, tx2.AggregatorTargetAsset) {
		return false
	}
	if tx.AggregatorTargetLimit == nil && tx2.AggregatorTargetLimit == nil {
		return true
	}
	if tx.AggregatorTargetLimit == nil && tx2.AggregatorTargetLimit != nil {
		return false
	}
	if tx.AggregatorTargetLimit != nil && tx2.AggregatorTargetLimit == nil {
		return false
	}
	if !tx.AggregatorTargetLimit.Equal(*tx2.AggregatorTargetLimit) {
		return false
	}
	return true
}

// TxArrayItem used to represent the tx out item coming from THORChain, there is little difference between TxArrayItem
// and TxOutItem defined above , only Coin <-> Coins field are different.
// TxArrayItem from THORChain has Coin , which only have a single coin
// TxOutItem used in bifrost need to support Coins , because when Yggdrasil return , it send all the coins back to asgard
// using multisend
type TxArrayItem struct {
	Chain                 common.Chain   `json:"chain,omitempty"`
	ToAddress             common.Address `json:"to_address,omitempty"`
	VaultPubKey           common.PubKey  `json:"vault_pub_key,omitempty"`
	Coin                  common.Coin    `json:"coin"`
	Memo                  string         `json:"memo,omitempty"`
	MaxGas                common.Gas     `json:"max_gas"`
	GasRate               int64          `json:"gas_rate,omitempty"`
	InHash                common.TxID    `json:"in_hash,omitempty"`
	OutHash               common.TxID    `json:"out_hash,omitempty"`
	Aggregator            string         `json:"aggregator,omitempty"`
	AggregatorTargetAsset string         `json:"aggregator_target_asset,omitempty"`
	AggregatorTargetLimit *cosmos.Uint   `json:"aggregator_target_limit,omitempty"`
}

// TxOutItem convert the information to TxOutItem
func (tx TxArrayItem) TxOutItem() TxOutItem {
	return TxOutItem{
		Chain:                 tx.Chain,
		ToAddress:             tx.ToAddress,
		VaultPubKey:           tx.VaultPubKey,
		Coins:                 common.Coins{tx.Coin},
		Memo:                  tx.Memo,
		MaxGas:                tx.MaxGas,
		GasRate:               tx.GasRate,
		InHash:                tx.InHash,
		OutHash:               tx.OutHash,
		Aggregator:            tx.Aggregator,
		AggregatorTargetAsset: tx.AggregatorTargetAsset,
		AggregatorTargetLimit: tx.AggregatorTargetLimit,
	}
}

// TxOut represent the tx out information , bifrost need to sign and process
type TxOut struct {
	Height  int64         `json:"height"`
	TxArray []TxArrayItem `json:"tx_array"`
}
