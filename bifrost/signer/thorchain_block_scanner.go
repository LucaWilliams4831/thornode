package signer

import (
	"errors"
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	btypes "gitlab.com/thorchain/thornode/bifrost/blockscanner/types"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pubkeymanager"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/config"
	ttypes "gitlab.com/thorchain/thornode/x/thorchain/types"
)

type ThorchainBlockScan struct {
	logger         zerolog.Logger
	wg             *sync.WaitGroup
	stopChan       chan struct{}
	txOutChan      chan types.TxOut
	keygenChan     chan ttypes.KeygenBlock
	cfg            config.BifrostBlockScannerConfiguration
	scannerStorage blockscanner.ScannerStorage
	thorchain      thorclient.ThorchainBridge
	errCounter     *prometheus.CounterVec
	pubkeyMgr      pubkeymanager.PubKeyValidator
}

// NewThorchainBlockScan create a new instance of thorchain block scanner
func NewThorchainBlockScan(cfg config.BifrostBlockScannerConfiguration, scanStorage blockscanner.ScannerStorage, thorchain thorclient.ThorchainBridge, m *metrics.Metrics, pubkeyMgr pubkeymanager.PubKeyValidator) (*ThorchainBlockScan, error) {
	if scanStorage == nil {
		return nil, errors.New("scanStorage is nil")
	}
	if m == nil {
		return nil, errors.New("metric is nil")
	}
	return &ThorchainBlockScan{
		logger:         log.With().Str("module", "blockscanner").Str("chain", "THOR").Logger(),
		wg:             &sync.WaitGroup{},
		stopChan:       make(chan struct{}),
		txOutChan:      make(chan types.TxOut),
		keygenChan:     make(chan ttypes.KeygenBlock),
		cfg:            cfg,
		scannerStorage: scanStorage,
		thorchain:      thorchain,
		errCounter:     m.GetCounterVec(metrics.ThorchainBlockScannerError),
		pubkeyMgr:      pubkeyMgr,
	}, nil
}

// GetMessages return the channel
func (b *ThorchainBlockScan) GetTxOutMessages() <-chan types.TxOut {
	return b.txOutChan
}

func (b *ThorchainBlockScan) GetKeygenMessages() <-chan ttypes.KeygenBlock {
	return b.keygenChan
}

func (b *ThorchainBlockScan) GetHeight() (int64, error) {
	return b.thorchain.GetBlockHeight()
}

func (c *ThorchainBlockScan) FetchMemPool(height int64) (types.TxIn, error) {
	return types.TxIn{}, nil
}

func (b *ThorchainBlockScan) FetchTxs(height, _ int64) (types.TxIn, error) {
	if err := b.processTxOutBlock(height); err != nil {
		return types.TxIn{}, err
	}
	if err := b.processKeygenBlock(height); err != nil {
		return types.TxIn{}, err
	}
	return types.TxIn{}, nil
}

func (b *ThorchainBlockScan) processKeygenBlock(blockHeight int64) error {
	pk := b.pubkeyMgr.GetNodePubKey()
	keygen, err := b.thorchain.GetKeygenBlock(blockHeight, pk.String())
	if err != nil {
		return fmt.Errorf("fail to get keygen from thorchain: %w", err)
	}

	// custom error (to be dropped and not logged) because the block is
	// available yet
	if keygen.Height == 0 {
		return btypes.ErrUnavailableBlock
	}

	if len(keygen.Keygens) > 0 {
		b.keygenChan <- keygen
	}
	return nil
}

func (b *ThorchainBlockScan) processTxOutBlock(blockHeight int64) error {
	for _, pk := range b.pubkeyMgr.GetSignPubKeys() {
		if len(pk.String()) == 0 {
			continue
		}
		tx, err := b.thorchain.GetKeysign(blockHeight, pk.String())
		if err != nil {
			if errors.Is(err, btypes.ErrUnavailableBlock) {
				// custom error (to be dropped and not logged) because the block is
				// available yet
				return btypes.ErrUnavailableBlock
			}
			return fmt.Errorf("fail to get keysign from block scanner: %w", err)
		}

		if len(tx.TxArray) == 0 {
			b.logger.Debug().Int64("block", blockHeight).Msg("nothing to process")
			continue
		}
		b.txOutChan <- tx
	}
	return nil
}
