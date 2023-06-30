package types

import (
	"math/big"

	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/config"
)

// ChainClient is the interface for chain clients.
type ChainClient interface {
	// Start starts the chain client with the given queues.
	Start(
		globalTxsQueue chan stypes.TxIn,
		globalErrataQueue chan stypes.ErrataBlock,
		globalSolvencyQueue chan stypes.Solvency,
	)

	// Stop stops the chain client.
	Stop()

	// IsBlockScannerHealthy returns true if the block scanner is healthy.
	IsBlockScannerHealthy() bool

	// SignTx returns the signed transaction.
	SignTx(tx stypes.TxOutItem, height int64) ([]byte, []byte, *stypes.TxInItem, error)

	// BroadcastTx broadcasts the transaction and returns the transaction hash.
	BroadcastTx(_ stypes.TxOutItem, _ []byte) (string, error)

	// GetHeight returns the current height of the chain.
	GetHeight() (int64, error)

	// GetAddress returns the address for the given public key.
	GetAddress(poolPubKey common.PubKey) string

	// GetAccount returns the account for the given public key.
	GetAccount(poolPubKey common.PubKey, height *big.Int) (common.Account, error)

	// GetAccountByAddress returns the account for the given address.
	GetAccountByAddress(address string, height *big.Int) (common.Account, error)

	// GetChain returns the chain.
	GetChain() common.Chain

	// GetConfig returns the chain configuration.
	GetConfig() config.BifrostChainConfiguration

	// OnObservedTxIn is called when a new observed tx is received.
	OnObservedTxIn(txIn types.TxInItem, blockHeight int64)

	// GetConfirmationCount returns the confirmation count for the given tx.
	GetConfirmationCount(txIn stypes.TxIn) int64

	// ConfirmationCountReady returns true if the confirmation count is ready.
	ConfirmationCountReady(txIn stypes.TxIn) bool
}

// SolvencyReporter reports the solvency of the chain at the given height.
type SolvencyReporter func(height int64) error
