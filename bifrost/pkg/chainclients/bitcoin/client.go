package bitcoin

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	"github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/thorchain/bifrost/txscript"
	tssp "gitlab.com/thorchain/tss/go-tss/tss"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/runners"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/signercache"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/utxo"
	mem "gitlab.com/thorchain/thornode/x/thorchain/memo"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	btypes "gitlab.com/thorchain/thornode/bifrost/blockscanner/types"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/bifrost/tss"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/config"
	"gitlab.com/thorchain/thornode/constants"
)

// BlockCacheSize the number of block meta that get store in storage.
const (
	BlockCacheSize      = 144
	MaximumConfirmation = 99999999
	MaxAsgardAddresses  = 100
	// EstimateAverageTxSize for THORChain the estimate tx size is hard code to 1000 here , as most of time it will spend 1 input, have 3 output
	// which is average at 250 vbytes , however asgard will consolidate UTXOs , which will take up to 1000 vbytes
	EstimateAverageTxSize = 1000
	MaxMempoolScanPerTry  = 500
)

// Client observes bitcoin chain and allows to sign and broadcast tx
type Client struct {
	logger                  zerolog.Logger
	cfg                     config.BifrostChainConfiguration
	m                       *metrics.Metrics
	client                  *rpcclient.Client
	chain                   common.Chain
	privateKey              *btcec.PrivateKey
	blockScanner            *blockscanner.BlockScanner
	temporalStorage         *utxo.TemporalStorage
	ksWrapper               *KeySignWrapper
	bridge                  thorclient.ThorchainBridge
	globalErrataQueue       chan<- types.ErrataBlock
	globalSolvencyQueue     chan<- types.Solvency
	nodePubKey              common.PubKey
	currentBlockHeight      *atomic.Int64
	asgardAddresses         []common.Address
	lastAsgard              time.Time
	minRelayFeeSats         uint64
	tssKeySigner            *tss.KeySign
	wg                      *sync.WaitGroup
	lastFeeRate             int64
	signerLock              *sync.Mutex
	vaultSignerLocks        map[string]*sync.Mutex
	consolidateInProgress   *atomic.Bool
	signerCacheManager      *signercache.CacheManager
	stopchan                chan struct{}
	lastSolvencyCheckHeight int64
}

// NewClient generates a new Client
func NewClient(thorKeys *thorclient.Keys, cfg config.BifrostChainConfiguration, server *tssp.TssServer, bridge thorclient.ThorchainBridge, m *metrics.Metrics) (*Client, error) {
	client, err := rpcclient.New(&rpcclient.ConnConfig{
		Host:         cfg.RPCHost,
		User:         cfg.UserName,
		Pass:         cfg.Password,
		DisableTLS:   cfg.DisableTLS,
		HTTPPostMode: cfg.HTTPostMode,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("fail to create bitcoin rpc client: %w", err)
	}
	tssKm, err := tss.NewKeySign(server, bridge)
	if err != nil {
		return nil, fmt.Errorf("fail to create tss signer: %w", err)
	}
	thorPrivateKey, err := thorKeys.GetPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("fail to get THORChain private key: %w", err)
	}

	btcPrivateKey, err := getBTCPrivateKey(thorPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("fail to convert private key for BTC: %w", err)
	}
	ksWrapper, err := NewKeySignWrapper(btcPrivateKey, tssKm)
	if err != nil {
		return nil, fmt.Errorf("fail to create keysign wrapper: %w", err)
	}

	temp, err := codec.ToTmPubKeyInterface(thorPrivateKey.PubKey())
	if err != nil {
		return nil, fmt.Errorf("fail to get tm pub key: %w", err)
	}
	nodePubKey, err := common.NewPubKeyFromCrypto(temp)
	if err != nil {
		return nil, fmt.Errorf("fail to get the node pubkey: %w", err)
	}

	c := &Client{
		logger:                log.Logger.With().Str("module", "bitcoin").Logger(),
		cfg:                   cfg,
		m:                     m,
		chain:                 cfg.ChainID,
		client:                client,
		privateKey:            btcPrivateKey,
		ksWrapper:             ksWrapper,
		bridge:                bridge,
		nodePubKey:            nodePubKey,
		minRelayFeeSats:       1000, // 1000 sats is the default minimal relay fee
		tssKeySigner:          tssKm,
		wg:                    &sync.WaitGroup{},
		signerLock:            &sync.Mutex{},
		vaultSignerLocks:      make(map[string]*sync.Mutex),
		stopchan:              make(chan struct{}),
		consolidateInProgress: atomic.NewBool(false),
		currentBlockHeight:    atomic.NewInt64(0),
	}

	var path string // if not set later, will in memory storage
	if len(c.cfg.BlockScanner.DBPath) > 0 {
		path = fmt.Sprintf("%s/%s", c.cfg.BlockScanner.DBPath, c.cfg.BlockScanner.ChainID)
	}
	storage, err := blockscanner.NewBlockScannerStorage(path, c.cfg.ScannerLevelDB)
	if err != nil {
		return c, fmt.Errorf("fail to create blockscanner storage: %w", err)
	}

	c.blockScanner, err = blockscanner.NewBlockScanner(c.cfg.BlockScanner, storage, m, bridge, c)
	if err != nil {
		return c, fmt.Errorf("fail to create block scanner: %w", err)
	}

	c.temporalStorage, err = utxo.NewTemporalStorage(storage.GetInternalDb(), c.cfg.MempoolTxIDCacheSize)
	if err != nil {
		return c, fmt.Errorf("fail to create temporal storage: %w", err)
	}

	if err := c.registerAddressInWalletAsWatch(c.nodePubKey); err != nil {
		return nil, fmt.Errorf("fail to register (%s): %w", c.nodePubKey, err)
	}
	signerCacheManager, err := signercache.NewSignerCacheManager(storage.GetInternalDb())
	if err != nil {
		return nil, fmt.Errorf("fail to create signer cache manager,err: %w", err)
	}
	c.signerCacheManager = signerCacheManager
	c.updateNetworkInfo()
	return c, nil
}

// Start starts the block scanner
func (c *Client) Start(globalTxsQueue chan types.TxIn, globalErrataQueue chan types.ErrataBlock, globalSolvencyQueue chan types.Solvency) {
	c.globalErrataQueue = globalErrataQueue
	c.globalSolvencyQueue = globalSolvencyQueue
	c.tssKeySigner.Start()
	c.blockScanner.Start(globalTxsQueue)
	c.wg.Add(1)
	go runners.SolvencyCheckRunner(c.GetChain(), c, c.bridge, c.stopchan, c.wg, constants.ThorchainBlockTime)
}

// Stop stops the block scanner
func (c *Client) Stop() {
	c.tssKeySigner.Stop()
	c.blockScanner.Stop()
	close(c.stopchan)
	// wait for consolidate utxo to exit
	c.wg.Wait()
}

// GetConfig - get the chain configuration
func (c *Client) GetConfig() config.BifrostChainConfiguration {
	return c.cfg
}

// GetChain returns BTC Chain
func (c *Client) GetChain() common.Chain {
	return common.BTCChain
}

// GetHeight returns current block height
func (c *Client) GetHeight() (int64, error) {
	return c.client.GetBlockCount()
}

func (c *Client) IsBlockScannerHealthy() bool {
	return c.blockScanner.IsHealthy()
}

// GetAddress returns address from pubkey
func (c *Client) GetAddress(poolPubKey common.PubKey) string {
	addr, err := poolPubKey.GetAddress(common.BTCChain)
	if err != nil {
		c.logger.Error().Err(err).Str("pool_pub_key", poolPubKey.String()).Msg("fail to get pool address")
		return ""
	}
	return addr.String()
}

// getUTXOs send a request to bitcond RPC endpoint to query all the UTXO
func (c *Client) getUTXOs(minConfirm, maximumConfirm int, pkey common.PubKey) ([]btcjson.ListUnspentResult, error) {
	btcAddress, err := pkey.GetAddress(common.BTCChain)
	if err != nil {
		return nil, fmt.Errorf("fail to get BTC Address for pubkey(%s): %w", pkey, err)
	}
	addr, err := btcutil.DecodeAddress(btcAddress.String(), c.getChainCfg())
	if err != nil {
		return nil, fmt.Errorf("fail to decode BTC address(%s): %w", btcAddress.String(), err)
	}
	return c.client.ListUnspentMinMaxAddresses(minConfirm, maximumConfirm, []btcutil.Address{
		addr,
	})
}

// GetAccount returns account with balance for an address
func (c *Client) GetAccount(pkey common.PubKey, height *big.Int) (common.Account, error) {
	if height != nil {
		c.logger.Error().Msg("height was provided but will be ignored")
	}

	acct := common.Account{}
	if pkey.IsEmpty() {
		return acct, errors.New("pubkey can't be empty")
	}
	utxos, err := c.getUTXOs(0, MaximumConfirmation, pkey)
	if err != nil {
		return acct, fmt.Errorf("fail to get UTXOs: %w", err)
	}
	total := 0.0
	for _, item := range utxos {
		if !c.isValidUTXO(item.ScriptPubKey) {
			continue
		}
		if item.Confirmations == 0 {
			// pending tx that is still  in mempool, only count yggdrasil send to itself or from asgard
			if !c.isSelfTransaction(item.TxID) && !c.isAsgardAddress(item.Address) {
				continue
			}
		}
		total += item.Amount
	}
	totalAmt, err := btcutil.NewAmount(total)
	if err != nil {
		return acct, fmt.Errorf("fail to convert total amount: %w", err)
	}
	return common.NewAccount(0, 0,
		common.Coins{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(uint64(totalAmt))),
		}, false), nil
}

func (c *Client) GetAccountByAddress(string, *big.Int) (common.Account, error) {
	return common.Account{}, nil
}

func (c *Client) getAsgardAddress() ([]common.Address, error) {
	if time.Since(c.lastAsgard) < constants.ThorchainBlockTime && c.asgardAddresses != nil {
		return c.asgardAddresses, nil
	}
	newAddresses, err := utxo.GetAsgardAddress(c.chain, MaxAsgardAddresses, c.bridge)
	if err != nil {
		return nil, fmt.Errorf("fail to get asgards : %w", err)
	}
	if len(newAddresses) > 0 { // ensure we don't overwrite with empty list
		c.asgardAddresses = newAddresses
	}
	c.lastAsgard = time.Now()
	return c.asgardAddresses, nil
}

func (c *Client) isAsgardAddress(addressToCheck string) bool {
	asgards, err := c.getAsgardAddress()
	if err != nil {
		c.logger.Err(err).Msg("fail to get asgard addresses")
		return false
	}
	for _, addr := range asgards {
		if strings.EqualFold(addr.String(), addressToCheck) {
			return true
		}
	}
	return false
}

// OnObservedTxIn gets called from observer when we have a valid observation
// For bitcoin chain client we want to save the utxo we can spend later to sign
func (c *Client) OnObservedTxIn(txIn types.TxInItem, blockHeight int64) {
	hash, err := chainhash.NewHashFromStr(txIn.Tx)
	if err != nil {
		c.logger.Error().Err(err).Str("txID", txIn.Tx).Msg("fail to add spendable utxo to storage")
		return
	}
	blockMeta, err := c.temporalStorage.GetBlockMeta(blockHeight)
	if err != nil {
		c.logger.Err(err).Msgf("fail to get block meta on block height(%d)", blockHeight)
		return
	}
	if blockMeta == nil {
		blockMeta = utxo.NewBlockMeta("", blockHeight, "")
	}
	if _, err := c.temporalStorage.TrackObservedTx(txIn.Tx); err != nil {
		c.logger.Err(err).Msgf("fail to add hash (%s) to observed tx cache", txIn.Tx)
	}
	if c.isAsgardAddress(txIn.Sender) {
		c.logger.Debug().Msgf("add hash %s as self transaction,block height:%d", hash.String(), blockHeight)
		blockMeta.AddSelfTransaction(hash.String())
	} else {
		// add the transaction to block meta
		blockMeta.AddCustomerTransaction(hash.String())
	}
	if err := c.temporalStorage.SaveBlockMeta(blockHeight, blockMeta); err != nil {
		c.logger.Err(err).Msgf("fail to save block meta to storage,block height(%d)", blockHeight)
	}
	// update the signer cache
	m, err := mem.ParseMemo(common.LatestVersion, txIn.Memo)
	if err != nil {
		// Debug log only as ParseMemo error is expected for THORName inbounds.
		c.logger.Debug().Err(err).Msgf("fail to parse memo: %s", txIn.Memo)
		return
	}
	if !m.IsOutbound() {
		return
	}
	if m.GetTxID().IsEmpty() {
		return
	}
	if err := c.signerCacheManager.SetSigned(txIn.CacheHash(c.GetChain(), m.GetTxID().String()), txIn.Tx); err != nil {
		c.logger.Err(err).Msg("fail to update signer cache")
	}
}

func (c *Client) processReorg(block *btcjson.GetBlockVerboseTxResult) ([]types.TxIn, error) {
	previousHeight := block.Height - 1
	prevBlockMeta, err := c.temporalStorage.GetBlockMeta(previousHeight)
	if err != nil {
		return nil, fmt.Errorf("fail to get block meta of height(%d) : %w", previousHeight, err)
	}
	if prevBlockMeta == nil {
		return nil, nil
	}
	// the block's previous hash need to be the same as the block hash chain client recorded in block meta
	// blockMetas[PreviousHeight].BlockHash == Block.PreviousHash
	if strings.EqualFold(prevBlockMeta.BlockHash, block.PreviousHash) {
		return nil, nil
	}

	c.logger.Info().Msgf("re-org detected, current block height:%d ,previous block hash is : %s , however block meta at height: %d, block hash is %s", block.Height, block.PreviousHash, prevBlockMeta.Height, prevBlockMeta.BlockHash)
	blockHeights, err := c.reConfirmTx()
	if err != nil {
		c.logger.Err(err).Msgf("fail to reprocess all txs")
	}
	var txIns []types.TxIn
	for _, item := range blockHeights {
		c.logger.Info().Msgf("rescan block height: %d", item)
		b, err := c.getBlock(item)
		if err != nil {
			c.logger.Err(err).Msgf("fail to get block from RPC for height:%d", item)
			continue
		}
		txIn, err := c.extractTxs(b)
		if err != nil {
			c.logger.Err(err).Msgf("fail to extract txIn from block")
			continue
		}

		if len(txIn.TxArray) == 0 {
			continue
		}
		txIns = append(txIns, txIn)
	}
	return txIns, nil
}

// reConfirmTx will be kicked off only when chain client detected a re-org on bitcoin chain
// it will read through all the block meta data from local storage , and go through all the UTXOs.
// For each UTXO , it will send a RPC request to bitcoin chain , double check whether the TX exist or not
// if the tx still exist , then it is all good, if a transaction previous we detected , however doesn't exist anymore , that means
// the transaction had been removed from chain,  chain client should report to thorchain
func (c *Client) reConfirmTx() ([]int64, error) {
	blockMetas, err := c.temporalStorage.GetBlockMetas()
	if err != nil {
		return nil, fmt.Errorf("fail to get block metas from local storage: %w", err)
	}
	var rescanBlockHeights []int64
	for _, blockMeta := range blockMetas {
		var errataTxs []types.ErrataTx
		for _, tx := range blockMeta.CustomerTransactions {
			h, err := chainhash.NewHashFromStr(tx)
			if err != nil {
				c.logger.Info().Msgf("%s invalid transaction hash", tx)
				continue
			}
			if c.confirmTx(h) {
				c.logger.Info().Msgf("block height: %d, tx: %s still exist", blockMeta.Height, tx)
				continue
			}
			// this means the tx doesn't exist in chain ,thus should errata it
			errataTxs = append(errataTxs, types.ErrataTx{
				TxID:  common.TxID(tx),
				Chain: common.BTCChain,
			})
			blockMeta.RemoveCustomerTransaction(tx)
		}
		if len(errataTxs) > 0 {
			c.globalErrataQueue <- types.ErrataBlock{
				Height: blockMeta.Height,
				Txs:    errataTxs,
			}
		}
		// Let's get the block again to fix the block hash
		r, err := c.getBlock(blockMeta.Height)
		if err != nil {
			c.logger.Err(err).Msgf("fail to get block verbose tx result: %d", blockMeta.Height)
		}
		if !strings.EqualFold(blockMeta.BlockHash, r.Hash) {
			rescanBlockHeights = append(rescanBlockHeights, blockMeta.Height)
		}
		blockMeta.PreviousHash = r.PreviousHash
		blockMeta.BlockHash = r.Hash
		if err := c.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta); err != nil {
			c.logger.Err(err).Msgf("fail to save block meta of height: %d ", blockMeta.Height)
		}
	}
	return rescanBlockHeights, nil
}

// confirmTx check a tx is valid on chain post reorg
func (c *Client) confirmTx(txHash *chainhash.Hash) bool {
	// GetRawTransaction, it should check transaction in mempool as well
	_, err := c.client.GetRawTransaction(txHash)
	if err == nil {
		// exist , all good
		return true
	}
	c.logger.Err(err).Msgf("fail to get tx (%s) from chain", txHash)
	// double check mempool
	_, err = c.client.GetMempoolEntry(txHash.String())
	if err != nil {
		c.logger.Err(err).Msgf("fail to get tx(%s) from mempool", txHash)
		return false
	}
	return true
}

func (c *Client) removeFromMemPoolCache(hash string) {
	if err := c.temporalStorage.UntrackMempoolTx(hash); err != nil {
		c.logger.Err(err).Msgf("fail to remove %s from mempool cache", hash)
	}
}

func (c *Client) tryAddToMemPoolCache(hash string) bool {
	exist, err := c.temporalStorage.TrackMempoolTx(hash)
	if err != nil {
		c.logger.Err(err).Msgf("fail to add mempool hash to key value store")
	}
	return exist
}

func (c *Client) getMemPool(height int64) (types.TxIn, error) {
	hashes, err := c.client.GetRawMempool()
	if err != nil {
		return types.TxIn{}, fmt.Errorf("fail to get tx hashes from mempool: %w", err)
	}
	txIn := types.TxIn{
		Chain:   c.GetChain(),
		MemPool: true,
	}
	maxWorker := c.cfg.ParallelMempoolScan
	if maxWorker == 0 {
		// set default worker to 5
		maxWorker = 5
	}
	sem := semaphore.NewWeighted(int64(maxWorker))
	g, _ := errgroup.WithContext(context.Background())
	total := 0
	lock := &sync.Mutex{}
	for _, h := range hashes {
		// this hash had been processed before , ignore it
		if !c.tryAddToMemPoolCache(h.String()) {
			c.logger.Debug().Msgf("%s had been processed , ignore", h.String())
			continue
		}
		// closure
		txHash := h
		g.Go(func() error {
			ctx := context.TODO()
			if err := sem.Acquire(ctx, 1); err != nil {
				return fmt.Errorf("fail to acquire semaphore: %w", err)
			}
			defer sem.Release(1)
			defer func(startTime time.Time) {
				c.logger.Debug().Msgf("time to scan one tx in mempool: %s", time.Since(startTime).String())
			}(time.Now())
			c.logger.Debug().Msgf("process hash %s", txHash.String())
			result, err := c.client.GetRawTransactionVerbose(txHash)
			if err != nil {
				// if the client fail to get the transaction , it could be the transaction removed , of the local node fail to
				// respond with the transaction, it is safe to ignore it , and continue with the rest
				c.logger.Err(err).Msgf("fail to get raw transaction verbose with hash(%s)", txHash.String())
				return nil
			}
			txInItem, err := c.getTxIn(result, height, true)
			if err != nil {
				c.logger.Debug().Err(err).Msg("fail to get TxInItem")
				return nil
			}
			if txInItem.IsEmpty() {
				return nil
			}
			lock.Lock()
			defer lock.Unlock()
			txIn.TxArray = append(txIn.TxArray, txInItem)
			return nil
		})
		total++
		// even it didn't finish scan all the mempool tx, but still yield here , so as block scanner can continue to scan a committed block
		if total >= MaxMempoolScanPerTry {
			break
		}
	}
	if err := g.Wait(); err != nil {
		c.logger.Err(err).Msgf("fail to scan mempool")
	}
	txIn.Count = strconv.Itoa(len(txIn.TxArray))
	return txIn, nil
}

// FetchMemPool retrieves txs from mempool
func (c *Client) FetchMemPool(height int64) (types.TxIn, error) {
	return c.getMemPool(height)
}

// FetchTxs retrieves txs for a block height
func (c *Client) FetchTxs(height, chainHeight int64) (types.TxIn, error) {
	txIn := types.TxIn{
		Chain:   common.BTCChain,
		TxArray: nil,
	}
	block, err := c.getBlock(height)
	if err != nil {
		if rpcErr, ok := err.(*btcjson.RPCError); ok && rpcErr.Code == btcjson.ErrRPCInvalidParameter {
			return txIn, btypes.ErrUnavailableBlock
		}
		return txIn, fmt.Errorf("fail to get block: %w", err)
	}

	// if somehow the block is not valid
	if block.Hash == "" && block.PreviousHash == "" {
		return txIn, fmt.Errorf("fail to get block: %w", err)
	}
	c.currentBlockHeight.Store(height)
	reScannedTxs, err := c.processReorg(block)
	if err != nil {
		c.logger.Err(err).Msg("fail to process bitcoin re-org")
	}
	if len(reScannedTxs) > 0 {
		for _, item := range reScannedTxs {
			if len(item.TxArray) == 0 {
				continue
			}
			txIn.TxArray = append(txIn.TxArray, item.TxArray...)
		}
	}

	blockMeta, err := c.temporalStorage.GetBlockMeta(block.Height)
	if err != nil {
		return txIn, fmt.Errorf("fail to get block meta from storage: %w", err)
	}
	if blockMeta == nil {
		blockMeta = utxo.NewBlockMeta(block.PreviousHash, block.Height, block.Hash)
	} else {
		blockMeta.PreviousHash = block.PreviousHash
		blockMeta.BlockHash = block.Hash
	}

	if err := c.temporalStorage.SaveBlockMeta(block.Height, blockMeta); err != nil {
		return txIn, fmt.Errorf("fail to save block meta into storage: %w", err)
	}
	pruneHeight := height - BlockCacheSize
	if pruneHeight > 0 {
		defer func() {
			if err := c.temporalStorage.PruneBlockMeta(pruneHeight, c.canDeleteBlock); err != nil {
				c.logger.Err(err).Msgf("fail to prune block meta, height(%d)", pruneHeight)
			}
		}()
	}

	txInBlock, err := c.extractTxs(block)
	if err != nil {
		return types.TxIn{}, fmt.Errorf("fail to extract txIn from block: %w", err)
	}
	if len(txInBlock.TxArray) > 0 {
		txIn.TxArray = append(txIn.TxArray, txInBlock.TxArray...)
	}
	c.updateNetworkInfo()

	// report network fee and solvency if within flexibility blocks of tip
	if chainHeight-height <= c.cfg.BlockScanner.ObservationFlexibilityBlocks {
		if err := c.sendNetworkFee(height); err != nil {
			c.logger.Err(err).Msg("fail to send network fee")
		}
		if c.IsBlockScannerHealthy() {
			if err := c.ReportSolvency(height); err != nil {
				c.logger.Err(err).Msgf("fail to send solvency info to THORChain")
			}
		}
	}

	txIn.Count = strconv.Itoa(len(txIn.TxArray))
	if !c.consolidateInProgress.Load() {
		// try to consolidate UTXOs
		c.wg.Add(1)
		c.consolidateInProgress.Store(true)
		go c.consolidateUTXOs()
	}
	return txIn, nil
}

func (c *Client) ReportSolvency(bitcoinBlockHeight int64) error {
	if !c.ShouldReportSolvency(bitcoinBlockHeight) {
		return nil
	}
	asgardVaults, err := c.bridge.GetAsgards()
	if err != nil {
		return fmt.Errorf("fail to get asgards,err: %w", err)
	}
	for _, asgard := range asgardVaults {
		acct, err := c.GetAccount(asgard.PubKey, nil)
		if err != nil {
			c.logger.Err(err).Msgf("fail to get account balance")
			continue
		}

		if runners.IsVaultSolvent(acct, asgard, cosmos.NewUint(3*EstimateAverageTxSize*uint64(c.lastFeeRate))) && c.IsBlockScannerHealthy() {
			// when vault is solvent , don't need to report solvency
			continue
		}

		select {
		case c.globalSolvencyQueue <- types.Solvency{
			Height: bitcoinBlockHeight,
			Chain:  common.BTCChain,
			PubKey: asgard.PubKey,
			Coins:  acct.Coins,
		}:
		case <-time.After(constants.ThorchainBlockTime):
			c.logger.Info().Msgf("fail to send solvency info to THORChain, timeout")
		}
	}
	c.lastSolvencyCheckHeight = bitcoinBlockHeight
	return nil
}

// ShouldReportSolvency based on the given block height , should the client report solvency to THORNode
func (c *Client) ShouldReportSolvency(height int64) bool {
	return height-c.lastSolvencyCheckHeight > 1
}

func (c *Client) canDeleteBlock(blockMeta *utxo.BlockMeta) bool {
	if blockMeta == nil {
		return true
	}
	for _, tx := range blockMeta.SelfTransactions {
		if result, err := c.client.GetMempoolEntry(tx); err == nil && result != nil {
			c.logger.Info().Msgf("tx: %s still in mempool , block can't be deleted", tx)
			return false
		}
	}
	return true
}

func (c *Client) updateNetworkInfo() {
	networkInfo, err := c.client.GetNetworkInfo()
	if err != nil {
		c.logger.Err(err).Msg("fail to get network info")
		return
	}
	amt, err := btcutil.NewAmount(networkInfo.RelayFee)
	if err != nil {
		c.logger.Err(err).Msg("fail to get minimum relay fee")
		return
	}
	c.minRelayFeeSats = uint64(amt.ToUnit(btcutil.AmountSatoshi))
}

func (c *Client) sendNetworkFee(height int64) error {
	result, err := c.client.GetBlockStats(height, nil)
	if err != nil {
		return fmt.Errorf("fail to get block stats")
	}
	// fee rate and tx size should not be 0
	if result.AverageFeeRate == 0 {
		return nil
	}
	feeRate := result.AverageFeeRate
	if EstimateAverageTxSize*uint64(feeRate) < c.minRelayFeeSats {
		feeRate = int64(c.minRelayFeeSats) / EstimateAverageTxSize
		if uint64(feeRate)*EstimateAverageTxSize < c.minRelayFeeSats {
			feeRate++
		}
	}

	c.m.GetGauge(metrics.GasPrice(common.BTCChain)).Set(float64(feeRate))
	if c.lastFeeRate != feeRate {
		c.m.GetCounter(metrics.GasPriceChange(common.BTCChain)).Inc()
	}

	c.lastFeeRate = feeRate
	txid, err := c.bridge.PostNetworkFee(height, common.BTCChain, uint64(EstimateAverageTxSize), uint64(feeRate))
	if err != nil {
		return fmt.Errorf("fail to post network fee to thornode: %w", err)
	}
	c.logger.Debug().Str("txid", txid.String()).Msg("send network fee to THORNode successfully")
	return nil
}

// getBlock retrieves block from chain for a block height
func (c *Client) getBlock(height int64) (*btcjson.GetBlockVerboseTxResult, error) {
	hash, err := c.client.GetBlockHash(height)
	if err != nil {
		return &btcjson.GetBlockVerboseTxResult{}, err
	}
	return c.client.GetBlockVerboseTx(hash)
}

func (c *Client) isValidUTXO(hexPubKey string) bool {
	buf, err := hex.DecodeString(hexPubKey)
	if err != nil {
		c.logger.Err(err).Msgf("fail to decode hex string,%s", hexPubKey)
		return false
	}
	scriptType, addresses, requireSigs, err := txscript.ExtractPkScriptAddrs(buf, c.getChainCfg())
	if err != nil {
		c.logger.Err(err).Msg("fail to extract pub key script")
		return false
	}
	switch scriptType {
	case txscript.MultiSigTy:
		return false

	default:
		return len(addresses) == 1 && requireSigs == 1
	}
}

func (c *Client) isRBFEnabled(tx *btcjson.TxRawResult) bool {
	for _, vin := range tx.Vin {
		if vin.Sequence < (0xffffffff - 1) {
			return true
		}
	}
	return false
}

func (c *Client) getTxIn(tx *btcjson.TxRawResult, height int64, isMemPool bool) (types.TxInItem, error) {
	if c.ignoreTx(tx, height) {
		c.logger.Debug().Int64("height", height).Str("tx", tx.Hash).Msg("ignore tx not matching format")
		return types.TxInItem{}, nil
	}
	// RBF enabled transaction will not be observed until it get committed to block
	if c.isRBFEnabled(tx) && isMemPool {
		return types.TxInItem{}, nil
	}
	sender, err := c.getSender(tx)
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("fail to get sender from tx: %w", err)
	}
	memo, err := c.getMemo(tx)
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("fail to get memo from tx: %w", err)
	}
	if len([]byte(memo)) > constants.MaxMemoSize {
		return types.TxInItem{}, fmt.Errorf("memo (%s) longer than max allow length(%d)", memo, constants.MaxMemoSize)
	}
	m, err := mem.ParseMemo(common.LatestVersion, memo)
	if err != nil {
		c.logger.Debug().Msgf("fail to parse memo: %s,err : %s", memo, err)
	}
	output, err := c.getOutput(sender, tx, m.IsType(mem.TxConsolidate))
	if err != nil {
		if errors.Is(err, btypes.ErrFailOutputMatchCriteria) {
			c.logger.Debug().Int64("height", height).Str("tx", tx.Hash).Msg("ignore tx not matching format")
			return types.TxInItem{}, nil
		}
		return types.TxInItem{}, fmt.Errorf("fail to get output from tx: %w", err)
	}
	addresses := c.getAddressesFromScriptPubKey(output.ScriptPubKey)
	if len(addresses) == 0 {
		return types.TxInItem{}, fmt.Errorf("fail to get addresses from script pub key")
	}
	toAddr := addresses[0]
	// If a UTXO is outbound , there is no need to validate the UTXO against mutisig
	if c.isAsgardAddress(toAddr) {
		if !c.isValidUTXO(output.ScriptPubKey.Hex) {
			return types.TxInItem{}, fmt.Errorf("invalid utxo")
		}
	}

	amount, err := btcutil.NewAmount(output.Value)
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("fail to parse float64: %w", err)
	}
	amt := uint64(amount.ToUnit(btcutil.AmountSatoshi))

	gas, err := c.getGas(tx)
	if err != nil {
		return types.TxInItem{}, fmt.Errorf("fail to get gas from tx: %w", err)
	}
	return types.TxInItem{
		BlockHeight: height,
		Tx:          tx.Txid,
		Sender:      sender,
		To:          toAddr,
		Coins: common.Coins{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(amt)),
		},
		Memo: memo,
		Gas:  gas,
	}, nil
}

// extractTxs extracts txs from a block to type TxIn
func (c *Client) extractTxs(block *btcjson.GetBlockVerboseTxResult) (types.TxIn, error) {
	txIn := types.TxIn{
		Chain:   c.GetChain(),
		MemPool: false,
	}
	var txItems []types.TxInItem
	for idx, tx := range block.Tx {
		// mempool transaction get committed to block , thus remove it from mempool cache
		c.removeFromMemPoolCache(tx.Hash)
		txInItem, err := c.getTxIn(&block.Tx[idx], block.Height, false)
		if err != nil {
			c.logger.Debug().Err(err).Msg("fail to get TxInItem")
			continue
		}
		if txInItem.IsEmpty() {
			continue
		}
		if txInItem.Coins.IsEmpty() {
			continue
		}
		if txInItem.Coins[0].Amount.LT(c.chain.DustThreshold()) {
			continue
		}
		exist, err := c.temporalStorage.TrackObservedTx(txInItem.Tx)
		if err != nil {
			c.logger.Err(err).Msgf("fail to determinate whether hash(%s) had been observed before", txInItem.Tx)
		}
		if !exist {
			c.logger.Info().Msgf("tx: %s had been report before, ignore", txInItem.Tx)
			if err := c.temporalStorage.UntrackObservedTx(txInItem.Tx); err != nil {
				c.logger.Err(err).Msgf("fail to remove observed tx from cache: %s", txInItem.Tx)
			}
			continue
		}
		txItems = append(txItems, txInItem)
	}
	txIn.TxArray = txItems
	txIn.Count = strconv.Itoa(len(txItems))
	return txIn, nil
}

// ignoreTx checks if we can already ignore a tx according to preset rules
//
// we expect array of "vout" for a BTC to have this format
// OP_RETURN is mandatory only on inbound tx
// vout:0 is our vault
// vout:1 is any any change back to themselves
// vout:2 is OP_RETURN (first 80 bytes)
// vout:3 is OP_RETURN (next 80 bytes)
//
// Rules to ignore a tx are:
// - count vouts > 4
// - count vouts with coins (value) > 2
func (c *Client) ignoreTx(tx *btcjson.TxRawResult, height int64) bool {
	if len(tx.Vin) == 0 || len(tx.Vout) == 0 || len(tx.Vout) > 4 {
		return true
	}
	// LockTime <= current height doesn't affect spendability,
	// and most wallets for users doing Memoless Savers deposits automatically set LockTime to the current height.
	if tx.LockTime > uint32(height) {
		return true
	}
	if tx.Vin[0].Txid == "" {
		return true
	}

	countWithOutput := 0
	for idx, vout := range tx.Vout {
		if vout.Value > 0 {
			countWithOutput++
		}
		addresses := c.getAddressesFromScriptPubKey(vout.ScriptPubKey)
		// check we have one address on the first 2 outputs
		// TODO check what we do if get multiple addresses
		if idx < 2 && vout.ScriptPubKey.Type != "nulldata" && len(addresses) != 1 {
			return true
		}
	}
	// none of the output has any value
	if countWithOutput == 0 {
		return true
	}
	// there are more than two output with value in it, not THORChain format
	if countWithOutput > 2 {
		return true
	}
	return false
}

func (c *Client) getAddressesFromScriptPubKey(scriptPubKey btcjson.ScriptPubKeyResult) []string {
	addresses := scriptPubKey.Addresses
	if len(addresses) > 0 {
		return addresses
	}

	if len(scriptPubKey.Hex) == 0 {
		return nil
	}
	buf, err := hex.DecodeString(scriptPubKey.Hex)
	if err != nil {
		c.logger.Err(err).Msg("fail to hex decode script pub key")
		return nil
	}
	_, extractedAddresses, _, err := txscript.ExtractPkScriptAddrs(buf, c.getChainCfg())
	if err != nil {
		c.logger.Err(err).Msg("fail to extract addresses from script pub key")
		return nil
	}
	for _, item := range extractedAddresses {
		addresses = append(addresses, item.String())
	}
	return addresses
}

// getOutput retrieve the correct output for both inbound
// outbound tx.
// logic is if FROM == TO then its an outbound change output
// back to the vault and we need to select the other output
// as Bifrost already filtered the txs to only have here
// txs with max 2 outputs with values
// an exception need to be made for consolidate tx , because consolidate tx will be send from asgard back asgard itself
func (c *Client) getOutput(sender string, tx *btcjson.TxRawResult, consolidate bool) (btcjson.Vout, error) {
	for _, vout := range tx.Vout {
		if strings.EqualFold(vout.ScriptPubKey.Type, "nulldata") {
			continue
		}
		addresses := c.getAddressesFromScriptPubKey(vout.ScriptPubKey)
		if len(addresses) != 1 {
			return btcjson.Vout{}, fmt.Errorf("no vout address available")
		}
		if vout.Value > 0 {
			if consolidate && addresses[0] == sender {
				return vout, nil
			}
			if !consolidate && addresses[0] != sender {
				return vout, nil
			}
		}
	}
	return btcjson.Vout{}, btypes.ErrFailOutputMatchCriteria
}

// getSender returns sender address for a btc tx, using vin:0
func (c *Client) getSender(tx *btcjson.TxRawResult) (string, error) {
	if len(tx.Vin) == 0 {
		return "", fmt.Errorf("no vin available in tx")
	}
	txHash, err := chainhash.NewHashFromStr(tx.Vin[0].Txid)
	if err != nil {
		return "", fmt.Errorf("fail to get tx hash from tx id string,err: %w", err)
	}
	vinTx, err := c.client.GetRawTransactionVerbose(txHash)
	if err != nil {
		return "", fmt.Errorf("fail to query raw tx from btcd,err: %w", err)
	}
	vout := vinTx.Vout[tx.Vin[0].Vout]
	addresses := c.getAddressesFromScriptPubKey(vout.ScriptPubKey)
	if len(addresses) == 0 {
		return "", fmt.Errorf("no address available in vout")
	}
	return addresses[0], nil
}

// getMemo returns memo for a btc tx, using vout OP_RETURN
func (c *Client) getMemo(tx *btcjson.TxRawResult) (string, error) {
	var opReturns string
	for _, vOut := range tx.Vout {
		if !strings.EqualFold(vOut.ScriptPubKey.Type, "nulldata") {
			continue
		}
		buf, err := hex.DecodeString(vOut.ScriptPubKey.Hex)
		if err != nil {
			c.logger.Err(err).Msg("fail to hex decode scriptPubKey")
			continue
		}
		asm, err := txscript.DisasmString(buf)
		if err != nil {
			c.logger.Err(err).Msg("fail to disasm script pubkey")
			continue
		}
		opReturnFields := strings.Fields(asm)
		if len(opReturnFields) == 2 {
			decoded, err := hex.DecodeString(opReturnFields[1])
			if err != nil {
				c.logger.Err(err).Msgf("fail to decode OP_RETURN string: %s", opReturnFields[1])
				continue
			}
			opReturns += string(decoded)
		}
	}
	return opReturns, nil
}

// getGas returns gas for a btc tx (sum vin - sum vout)
func (c *Client) getGas(tx *btcjson.TxRawResult) (common.Gas, error) {
	var sumVin uint64
	for _, vin := range tx.Vin {
		txHash, err := chainhash.NewHashFromStr(vin.Txid)
		if err != nil {
			return common.Gas{}, fmt.Errorf("fail to get tx hash from tx id string")
		}
		vinTx, err := c.client.GetRawTransactionVerbose(txHash)
		if err != nil {
			return common.Gas{}, fmt.Errorf("fail to query raw tx from bitcoin node")
		}

		amount, err := btcutil.NewAmount(vinTx.Vout[vin.Vout].Value)
		if err != nil {
			return nil, err
		}
		sumVin += uint64(amount.ToUnit(btcutil.AmountSatoshi))
	}
	var sumVout uint64
	for _, vout := range tx.Vout {
		amount, err := btcutil.NewAmount(vout.Value)
		if err != nil {
			return nil, err
		}
		sumVout += uint64(amount.ToUnit(btcutil.AmountSatoshi))
	}
	totalGas := sumVin - sumVout
	return common.Gas{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(totalGas)),
	}, nil
}

// registerAddressInWalletAsWatch make a RPC call to import the address relevant to the given pubkey
// in wallet as watch only , so as when bifrost call ListUnspent , it will return appropriate result
func (c *Client) registerAddressInWalletAsWatch(pkey common.PubKey) error {
	addr, err := pkey.GetAddress(common.BTCChain)
	if err != nil {
		return fmt.Errorf("fail to get BTC address from pubkey(%s): %w", pkey, err)
	}
	err = c.createWallet("")
	if err != nil {
		return err
	}
	c.logger.Info().Msgf("import address: %s", addr.String())
	return c.client.ImportAddressRescan(addr.String(), "", false)
}

func (c *Client) createWallet(name string) error {
	walletNameJSON, err := json.Marshal(name)
	if err != nil {
		return err
	}
	falseJSON, err := json.Marshal(false)
	if err != nil {
		return err
	}

	_, err = c.client.RawRequest("createwallet", []json.RawMessage{
		walletNameJSON,
		falseJSON,
		falseJSON,
		json.RawMessage([]byte("\"\"")),
		falseJSON,
		falseJSON,
	})
	if err != nil {
		// ignore code -4 which means wallet already exists
		if strings.HasPrefix(err.Error(), "-4") {
			return nil
		}
		return err
	}
	return nil
}

// RegisterPublicKey register the given pubkey to bitcoin wallet
func (c *Client) RegisterPublicKey(pkey common.PubKey) error {
	return c.registerAddressInWalletAsWatch(pkey)
}

func (c *Client) getCoinbaseValue(blockHeight int64) (int64, error) {
	hash, err := c.client.GetBlockHash(blockHeight)
	if err != nil {
		return 0, fmt.Errorf("fail to get block hash:%w", err)
	}
	result, err := c.client.GetBlockVerboseTx(hash)
	if err != nil {
		return 0, fmt.Errorf("fail to get block verbose tx: %w", err)
	}
	for _, tx := range result.Tx {
		if len(tx.Vin) == 1 && tx.Vin[0].IsCoinBase() {
			total := float64(0)
			for _, opt := range tx.Vout {
				total += opt.Value
			}
			amt, err := btcutil.NewAmount(total)
			if err != nil {
				return 0, fmt.Errorf("fail to parse amount: %w", err)
			}
			return int64(amt), nil
		}
	}
	return 0, fmt.Errorf("fail to get coinbase value")
}

// getBlockRequiredConfirmation find out how many confirmation the given txIn need to have before it can be send to THORChain
func (c *Client) getBlockRequiredConfirmation(txIn types.TxIn, height int64) (int64, error) {
	totalTxValue := txIn.GetTotalTransactionValue(common.BTCAsset, c.asgardAddresses)
	totalFeeAndSubsidy, err := c.getCoinbaseValue(height)
	if err != nil {
		c.logger.Err(err).Msg("fail to get coinbase value")
	}
	if totalFeeAndSubsidy == 0 {
		cbValue, err := btcutil.NewAmount(c.chain.DefaultCoinbase())
		if err != nil {
			return 0, fmt.Errorf("fail to get default coinbase value: %w", err)
		}
		totalFeeAndSubsidy = int64(cbValue)
	}
	confirm := totalTxValue.QuoUint64(uint64(totalFeeAndSubsidy)).Uint64()
	c.logger.Info().Msgf("totalTxValue:%s,total fee and Subsidy:%d,confirmation:%d", totalTxValue, totalFeeAndSubsidy, confirm)
	return int64(confirm), nil
}

// GetConfirmationCount return the number of blocks the tx need to wait before processing in THORChain
func (c *Client) GetConfirmationCount(txIn types.TxIn) int64 {
	if len(txIn.TxArray) == 0 {
		return 0
	}
	// MemPool items doesn't need confirmation
	if txIn.MemPool {
		return 0
	}
	blockHeight := txIn.TxArray[0].BlockHeight
	confirm, err := c.getBlockRequiredConfirmation(txIn, blockHeight)
	c.logger.Info().Msgf("confirmation required: %d", confirm)
	if err != nil {
		c.logger.Err(err).Msg("fail to get block confirmation ")
		return 0
	}
	return confirm
}

// ConfirmationCountReady will be called by observer before send the txIn to thorchain
// confirmation counting is on block level , refer to https://medium.com/coinmonks/1confvalue-a-simple-pow-confirmation-rule-of-thumb-a8d9c6c483dd for detail
func (c *Client) ConfirmationCountReady(txIn types.TxIn) bool {
	if len(txIn.TxArray) == 0 {
		return true
	}
	// MemPool items doesn't need confirmation
	if txIn.MemPool {
		return true
	}
	blockHeight := txIn.TxArray[0].BlockHeight
	confirm := txIn.ConfirmationRequired
	c.logger.Info().Msgf("confirmation required: %d", confirm)
	// every tx in txIn already have at least 1 confirmation
	return (c.currentBlockHeight.Load() - blockHeight) >= confirm
}

// getVaultSignerLock , with consolidate UTXO process add into bifrost , there are two entry points for SignTx , one is from signer , signing the outbound tx
// from state machine, the other one will be consolidate utxo process
// this keep a lock per vault pubkey , the goal is each vault we only have one key sign in flight at a time, however different vault can do key sign in parallel
// assume there are multiple asgards(A,B) , and local yggdrasil vault , when A is signing , B and local yggdrasil vault should be able to sign as well
// however if A already has a key sign in flight , bifrost should not kick off another key sign in parallel, otherwise we might double spend some UTXOs
func (c *Client) getVaultSignerLock(vaultPubKey string) *sync.Mutex {
	c.signerLock.Lock()
	defer c.signerLock.Unlock()
	l, ok := c.vaultSignerLocks[vaultPubKey]
	if !ok {
		newLock := &sync.Mutex{}
		c.vaultSignerLocks[vaultPubKey] = newLock
		return newLock
	}
	return l
}
