package blockscanner

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	btypes "gitlab.com/thorchain/thornode/bifrost/blockscanner/types"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/config"
	"gitlab.com/thorchain/thornode/constants"
)

// BlockScannerFetcher define the methods a block scanner need to implement
type BlockScannerFetcher interface {
	// FetchMemPool scan the mempool
	FetchMemPool(height int64) (types.TxIn, error)
	// FetchTxs scan block with the given height
	FetchTxs(fetchHeight, chainHeight int64) (types.TxIn, error)
	// GetHeight return current block height
	GetHeight() (int64, error)
}

type Block struct {
	Height int64
	Txs    []string
}

// BlockScanner is used to discover block height
type BlockScanner struct {
	cfg             config.BifrostBlockScannerConfiguration
	logger          zerolog.Logger
	wg              *sync.WaitGroup
	scanChan        chan int64
	stopChan        chan struct{}
	scannerStorage  ScannerStorage
	metrics         *metrics.Metrics
	previousBlock   int64
	globalTxsQueue  chan types.TxIn
	errorCounter    *prometheus.CounterVec
	thorchainBridge thorclient.ThorchainBridge
	chainScanner    BlockScannerFetcher
	healthy         bool // status of scanner, if last attempt to scan a block was successful or not
}

// NewBlockScanner create a new instance of BlockScanner
func NewBlockScanner(cfg config.BifrostBlockScannerConfiguration, scannerStorage ScannerStorage, m *metrics.Metrics, thorchainBridge thorclient.ThorchainBridge, chainScanner BlockScannerFetcher) (*BlockScanner, error) {
	var err error
	if scannerStorage == nil {
		return nil, errors.New("scannerStorage is nil")
	}
	if m == nil {
		return nil, errors.New("metrics instance is nil")
	}
	if thorchainBridge == nil {
		return nil, errors.New("thorchain bridge is nil")
	}

	logger := log.Logger.With().Str("module", "blockscanner").Str("chain", cfg.ChainID.String()).Logger()
	scanner := &BlockScanner{
		cfg:             cfg,
		logger:          logger,
		wg:              &sync.WaitGroup{},
		stopChan:        make(chan struct{}),
		scanChan:        make(chan int64),
		scannerStorage:  scannerStorage,
		metrics:         m,
		errorCounter:    m.GetCounterVec(metrics.CommonBlockScannerError),
		thorchainBridge: thorchainBridge,
		chainScanner:    chainScanner,
		healthy:         false,
	}

	scanner.previousBlock, err = scanner.FetchLastHeight()
	logger.Info().Int64("block height", scanner.previousBlock).Msg("block scanner last fetch height")
	return scanner, err
}

// IsHealthy return if the block scanner is healthy or not
func (b *BlockScanner) IsHealthy() bool {
	return b.healthy
}

// GetMessages return the channel
func (b *BlockScanner) GetMessages() <-chan int64 {
	return b.scanChan
}

// Start block scanner
func (b *BlockScanner) Start(globalTxsQueue chan types.TxIn) {
	b.globalTxsQueue = globalTxsQueue
	currentPos, err := b.scannerStorage.GetScanPos()
	if err != nil {
		b.logger.Error().Err(err).Msgf("fail to get current block scan pos, %s will start from %d", b.cfg.ChainID, b.previousBlock)
	} else if currentPos > b.previousBlock {
		b.previousBlock = currentPos
	}
	b.wg.Add(2)
	go b.scanBlocks()
	go b.scanMempool()
}

func (b *BlockScanner) scanMempool() {
	b.logger.Debug().Msg("start to scan mempool")
	defer b.logger.Debug().Msg("stop scan mempool")
	defer b.wg.Done()

	for {
		select {
		case <-b.stopChan:
			return
		default:
			// mempool scan will continue even the chain get halted , thus the network can still aware of outbound transaction
			// during chain halt
			preBlockHeight := atomic.LoadInt64(&b.previousBlock)
			currentBlock := preBlockHeight + 1
			txInMemPool, err := b.chainScanner.FetchMemPool(currentBlock)
			if err != nil {
				b.logger.Error().Err(err).Msg("fail to fetch MemPool")
			}
			if len(txInMemPool.TxArray) > 0 {
				select {
				case <-b.stopChan:
					return
				case b.globalTxsQueue <- txInMemPool:
				}
			} else {
				// nothing in the mempool or for some chain like BNB & ETH, which doesn't need to scan
				// mempool , back off here
				time.Sleep(constants.ThorchainBlockTime)
			}
		}
	}
}

// Checks current mimir settings to determine if the current chain is paused
// either globally or specifically
func (b *BlockScanner) isChainPaused() bool {
	var haltHeight, solvencyHaltHeight, nodeHaltHeight, thorHeight int64

	// Check if chain has been halted via mimir
	haltHeight, err := b.thorchainBridge.GetMimir(fmt.Sprintf("Halt%sChain", b.cfg.ChainID))
	if err != nil {
		b.logger.Error().Err(err).Msgf("fail to get mimir setting %s", fmt.Sprintf("Halt%sChain", b.cfg.ChainID))
	}
	// Check if chain has been halted by auto solvency checks
	solvencyHaltHeight, err = b.thorchainBridge.GetMimir(fmt.Sprintf("SolvencyHalt%sChain", b.cfg.ChainID))
	if err != nil {
		b.logger.Error().Err(err).Msgf("fail to get mimir %s", fmt.Sprintf("SolvencyHalt%sChain", b.cfg.ChainID))
	}
	// Check if all chains halted globally
	globalHaltHeight, err := b.thorchainBridge.GetMimir("HaltChainGlobal")
	if err != nil {
		b.logger.Error().Err(err).Msg("fail to get mimir setting HaltChainGlobal")
	}
	if globalHaltHeight > haltHeight {
		haltHeight = globalHaltHeight
	}
	// Check if a node paused all chains
	nodeHaltHeight, err = b.thorchainBridge.GetMimir("NodePauseChainGlobal")
	if err != nil {
		b.logger.Error().Err(err).Msg("fail to get mimir setting NodePauseChainGlobal")
	}
	thorHeight, err = b.thorchainBridge.GetBlockHeight()
	if err != nil {
		b.logger.Error().Err(err).Msg("fail to get THORChain block height")
	}

	if nodeHaltHeight > 0 && thorHeight < nodeHaltHeight {
		haltHeight = 1
	}

	return (haltHeight > 0 && thorHeight > haltHeight) || (solvencyHaltHeight > 0 && thorHeight > solvencyHaltHeight)
}

// scanBlocks
func (b *BlockScanner) scanBlocks() {
	b.logger.Debug().Msg("start to scan blocks")
	defer b.logger.Debug().Msg("stop scan blocks")
	defer b.wg.Done()

	lastMimirCheck := time.Now().Add(-constants.ThorchainBlockTime)
	isChainPaused := false

	// start up to grab those blocks
	for {
		select {
		case <-b.stopChan:
			return
		default:
			preBlockHeight := atomic.LoadInt64(&b.previousBlock)
			currentBlock := preBlockHeight + 1
			// check if mimir has disabled this chain
			if time.Since(lastMimirCheck) >= constants.ThorchainBlockTime {
				isChainPaused = b.isChainPaused()
				lastMimirCheck = time.Now()
			}

			// Chain is paused, mark as unhealthy
			if isChainPaused {
				b.healthy = false
				time.Sleep(constants.ThorchainBlockTime)
				continue
			}

			chainHeight, err := b.chainScanner.GetHeight()
			if err != nil {
				b.logger.Error().Err(err).Msg("fail to get chain block height")
				time.Sleep(b.cfg.BlockHeightDiscoverBackoff)
				continue
			}
			if chainHeight < currentBlock {
				time.Sleep(b.cfg.BlockHeightDiscoverBackoff)
				continue
			}
			txIn, err := b.chainScanner.FetchTxs(currentBlock, chainHeight)
			if err != nil {
				// don't log an error if its because the block doesn't exist yet
				if !errors.Is(err, btypes.ErrUnavailableBlock) {
					b.logger.Error().Err(err).Int64("block height", currentBlock).Msg("fail to get RPCBlock")
					b.healthy = false
				}
				time.Sleep(b.cfg.BlockHeightDiscoverBackoff)
				continue
			}

			// determine how often we print a info log line for scanner
			// progress. General goal is about once per minute
			ms := b.cfg.ChainID.ApproximateBlockMilliseconds()
			mod := (60_000 + ms - 1) / ms
			// enable this one , so we could see how far it is behind
			if currentBlock%mod == 0 || !b.healthy {
				b.logger.Info().Int64("block height", currentBlock).Int("txs", len(txIn.TxArray)).Msg("scan block")

				b.logger.Info().Msgf("the gap is %d , healthy: %+v", chainHeight-currentBlock, b.healthy)
			}
			atomic.AddInt64(&b.previousBlock, 1)
			// if current block height is less than 50 blocks behind the tip , then it should catch up soon, should be safe to mark block scanner as healthy
			// if the block scanner is too far away from tip , should not mark the block scanner as healthy , otherwise it might cause , reschedule and double send
			if chainHeight-currentBlock <= 50 {
				b.healthy = true
			}
			b.logger.Debug().Msgf("the gap is %d , healthy: %+v", chainHeight-currentBlock, b.healthy)

			b.metrics.GetCounter(metrics.TotalBlockScanned).Inc()
			if len(txIn.TxArray) > 0 {
				select {
				case <-b.stopChan:
					return
				case b.globalTxsQueue <- txIn:
				}
			}
			if err := b.scannerStorage.SetScanPos(b.previousBlock); err != nil {
				b.logger.Error().Err(err).Msg("fail to save block scan pos")
				// alert!!
				continue
			}
		}
	}
}

// FetchLastHeight retrieves the last height to start scanning blocks from on startup
//  1. Check if we have a height specified in config AND
//     its higher than the block scanner storage one, use that
//  2. Get the last observed height from THORChain if available
//  3. Use block scanner storage if > 0
//  4. Fetch last height from the chain itself
func (b *BlockScanner) FetchLastHeight() (int64, error) {
	// get scanner storage height
	currentPos, _ := b.scannerStorage.GetScanPos() // ignore error

	// 1. if we've configured a starting height, use that
	if b.cfg.StartBlockHeight > 0 {
		return b.cfg.StartBlockHeight, nil
	}
	// 2. attempt to find the height from thorchain
	// wait for thorchain to be caught up first
	if err := b.thorchainBridge.WaitToCatchUp(); err != nil {
		return 0, err
	}
	if b.thorchainBridge != nil {
		var height int64
		if b.cfg.ChainID.Equals(common.THORChain) {
			height, _ = b.thorchainBridge.GetBlockHeight()
		} else {
			height, _ = b.thorchainBridge.GetLastObservedInHeight(b.cfg.ChainID)
		}
		if height > 0 {
			return height, nil
		}
	}

	// 3. If we've already started scanning, begin where we left off
	if currentPos > 0 {
		return currentPos, nil
	}

	// 4. Start from latest height on the chain itself
	return b.chainScanner.GetHeight()
}

func (b *BlockScanner) Stop() {
	b.logger.Debug().Msg("receive stop request")
	defer b.logger.Debug().Msg("common block scanner stopped")
	close(b.stopChan)
	b.wg.Wait()
}
