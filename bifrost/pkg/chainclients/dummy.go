package chainclients

import (
	"errors"

	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
)

var errKaboom = errors.New("kaboom")

// This is a full implementation of a dummy chain, intended for testing purposes

type DummyChain struct{}

func (DummyChain) SignTx(tx stypes.TxOutItem, height int64) ([]byte, error) {
	return nil, errKaboom
}
func (DummyChain) BroadcastTx(_ stypes.TxOutItem, tx []byte) error { return errKaboom }
func (DummyChain) CheckIsTestNet() (string, bool)                  { return "", false }
func (DummyChain) GetHeight() (int64, error)                       { return 0, errKaboom }
func (DummyChain) IsBlockScannerHealthy() bool                     { return true }
func (DummyChain) GetAddress(poolPubKey common.PubKey) string      { return "" }
func (DummyChain) GetAccount(addr string) (common.Account, error) {
	return common.Account{}, errKaboom
}
func (DummyChain) GetChain() common.Chain                { return "" }
func (DummyChain) GetGasFee(count uint64) common.Gas     { return nil }
func (DummyChain) Start(globalTxsQueue chan stypes.TxIn) {}
func (DummyChain) Stop()                                 {}
