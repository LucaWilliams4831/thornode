package types

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	mem "gitlab.com/thorchain/thornode/x/thorchain/memo"
)

type TxIn struct {
	Count                string       `json:"count"`
	Chain                common.Chain `json:"chain"`
	TxArray              []TxInItem   `json:"txArray"`
	Filtered             bool         `json:"filtered"`
	MemPool              bool         `json:"mem_pool"`          // indicate whether this item is in the mempool or not
	SentUnFinalised      bool         `json:"sent_un_finalised"` // indicate whehter unfinalised tx had been sent to THORChain
	Finalised            bool         `json:"finalised"`
	ConfirmationRequired int64        `json:"confirmation_required"`
}

type TxInItem struct {
	BlockHeight           int64         `json:"block_height"`
	Tx                    string        `json:"tx"`
	Memo                  string        `json:"memo"`
	Sender                string        `json:"sender"`
	To                    string        `json:"to"` // to adddress
	Coins                 common.Coins  `json:"coins"`
	Gas                   common.Gas    `json:"gas"`
	ObservedVaultPubKey   common.PubKey `json:"observed_vault_pub_key"`
	Aggregator            string        `json:"aggregator"`
	AggregatorTarget      string        `json:"aggregator_target"`
	AggregatorTargetLimit *cosmos.Uint  `json:"aggregator_target_limit"`
}
type TxInStatus byte

func NewTxInItem(
	blockHeight int64,
	tx string,
	memo string,
	sender string,
	to string,
	coins common.Coins,
	gas common.Gas,
	observedVaultPubKey common.PubKey,
	aggregator string,
	aggregatorTarget string,
	aggregatorTargetLimit *cosmos.Uint,
) *TxInItem {
	return &TxInItem{
		BlockHeight:           blockHeight,
		Tx:                    tx,
		Memo:                  memo,
		Sender:                sender,
		To:                    to,
		Coins:                 coins,
		Gas:                   gas,
		ObservedVaultPubKey:   observedVaultPubKey,
		Aggregator:            aggregator,
		AggregatorTarget:      aggregatorTarget,
		AggregatorTargetLimit: aggregatorTargetLimit,
	}
}

const (
	Processing TxInStatus = iota
	Failed
)

// IsEmpty return true only when every field in TxInItem is empty
func (t TxInItem) IsEmpty() bool {
	return t.BlockHeight == 0 &&
		t.Tx == "" &&
		t.Memo == "" &&
		t.Sender == "" &&
		t.To == "" &&
		t.Coins.IsEmpty() &&
		t.Gas.IsEmpty() &&
		t.ObservedVaultPubKey.IsEmpty()
}

// CacheHash calculate the has used for signer cache
func (t TxInItem) CacheHash(chain common.Chain, inboundHash string) string {
	str := fmt.Sprintf("%s|%s|%s|%s|%s", chain, t.To, t.Coins, t.Memo, inboundHash)
	return fmt.Sprintf("%X", sha256.Sum256([]byte(str)))
}

// GetTotalTransactionValue return the total value of the requested asset
func (t TxIn) GetTotalTransactionValue(asset common.Asset, excludeFrom []common.Address) cosmos.Uint {
	total := cosmos.ZeroUint()
	for _, item := range t.TxArray {
		fromAsgard := false
		for _, fromAddress := range excludeFrom {
			if strings.EqualFold(fromAddress.String(), item.Sender) {
				fromAsgard = true
			}
		}
		if fromAsgard {
			continue
		}
		// skip confirmation counting if it is internal tx
		m, err := mem.ParseMemo(common.LatestVersion, item.Memo)
		if err == nil && m.IsInternal() {
			continue
		}
		c := item.Coins.GetCoin(asset)
		if c.IsEmpty() {
			continue
		}
		total = total.Add(c.Amount)
	}
	return total
}

// GetTotalGas return the total gas
func (t TxIn) GetTotalGas() cosmos.Uint {
	total := cosmos.ZeroUint()
	for _, item := range t.TxArray {
		if item.Gas == nil {
			continue
		}
		if err := item.Gas.Valid(); err != nil {
			continue
		}
		total = total.Add(item.Gas[0].Amount)
	}
	return total
}
