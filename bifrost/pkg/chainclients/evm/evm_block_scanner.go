package evm

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"

	_ "embed"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ecommon "github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	ethclient "github.com/ethereum/go-ethereum/ethclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/semaphore"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/evm"
	evmtypes "gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/evm/types"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/signercache"
	. "gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/types"
	"gitlab.com/thorchain/thornode/bifrost/pubkeymanager"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/common/tokenlist"
	"gitlab.com/thorchain/thornode/config"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/aggregators"
	memo "gitlab.com/thorchain/thornode/x/thorchain/memo"
)

////////////////////////////////////////////////////////////////////////////////////////
// EVMScanner
////////////////////////////////////////////////////////////////////////////////////////

type EVMScanner struct {
	cfg                  config.BifrostBlockScannerConfiguration
	logger               zerolog.Logger
	db                   blockscanner.ScannerStorage
	m                    *metrics.Metrics
	errCounter           *prometheus.CounterVec
	gasPriceChanged      bool
	gasPrice             *big.Int
	lastReportedGasPrice uint64
	ethClient            *ethclient.Client
	ethRpc               *evm.EthRPC
	blockMetaAccessor    evm.BlockMetaAccessor
	bridge               thorclient.ThorchainBridge
	pubkeyMgr            pubkeymanager.PubKeyValidator
	eipSigner            etypes.Signer
	currentBlockHeight   int64
	gasCache             []*big.Int
	solvencyReporter     SolvencyReporter
	whitelistTokens      []tokenlist.ERC20Token
	whitelistContracts   []common.Address
	signerCacheManager   *signercache.CacheManager
	tokenManager         *evm.TokenManager

	vaultABI *abi.ABI
	erc20ABI *abi.ABI
}

// NewEVMScanner create a new instance of EVMScanner.
func NewEVMScanner(cfg config.BifrostBlockScannerConfiguration,
	storage blockscanner.ScannerStorage,
	chainID *big.Int,
	ethClient *ethclient.Client,
	ethRpc *evm.EthRPC,
	bridge thorclient.ThorchainBridge,
	m *metrics.Metrics,
	pubkeyMgr pubkeymanager.PubKeyValidator,
	solvencyReporter SolvencyReporter,
	signerCacheManager *signercache.CacheManager,
) (*EVMScanner, error) {
	// check required arguments
	if storage == nil {
		return nil, errors.New("storage is nil")
	}
	if m == nil {
		return nil, errors.New("metrics manager is nil")
	}
	if ethClient == nil {
		return nil, errors.New("ETH RPC client is nil")
	}
	if pubkeyMgr == nil {
		return nil, errors.New("pubkey manager is nil")
	}

	// set storage prefixes
	prefixBlockMeta := fmt.Sprintf("%s-blockmeta-", strings.ToLower(cfg.ChainID.String()))
	prefixSignedMeta := fmt.Sprintf("%s-signedtx-", strings.ToLower(cfg.ChainID.String()))
	prefixTokenMeta := fmt.Sprintf("%s-tokenmeta-", strings.ToLower(cfg.ChainID.String()))

	// create block meta accessor
	blockMetaAccessor, err := evm.NewLevelDBBlockMetaAccessor(
		prefixBlockMeta, prefixSignedMeta, storage.GetInternalDb(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create block meta accessor: %w", err)
	}

	// load ABIs
	vaultABI, erc20ABI, err := evm.GetContractABI(routerContractABI, erc20ContractABI)
	if err != nil {
		return nil, fmt.Errorf("failed to create contract abi: %w", err)
	}

	// load token list
	allTokens := tokenlist.GetEVMTokenList(cfg.ChainID, common.LatestVersion).Tokens
	var whitelistTokens []tokenlist.ERC20Token
	for _, addr := range cfg.WhitelistTokens {
		// find matching token in token list
		found := false
		for _, tok := range allTokens {
			if strings.EqualFold(addr, tok.Address) {
				whitelistTokens = append(whitelistTokens, tok)
				found = true
				break
			}
		}

		// all whitelisted tokens must be in the chain token list
		if !found {
			return nil, fmt.Errorf("whitelist token %s not found in token list", addr)
		}
	}

	// create token manager - storage is scoped to chain so assets should not collide
	tokenManager, err := evm.NewTokenManager(
		storage.GetInternalDb(),
		prefixTokenMeta,
		cfg.ChainID.GetGasAsset(),
		defaultDecimals,
		cfg.HTTPRequestTimeout,
		whitelistTokens,
		ethClient,
		routerContractABI,
		erc20ContractABI,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create token helper: %w", err)
	}

	// store the token metadata for the chain gas asset
	err = tokenManager.SaveTokenMeta(
		cfg.ChainID.GetGasAsset().Symbol.String(), evm.NativeTokenAddr, defaultDecimals,
	)
	if err != nil {
		return nil, err
	}

	// load whitelist contracts for the chain
	whitelistContracts := []common.Address{}
	for _, agg := range aggregators.DexAggregators(common.LatestVersion) {
		if agg.Chain.Equals(cfg.ChainID) {
			whitelistContracts = append(whitelistContracts, common.Address(agg.Address))
		}
	}

	return &EVMScanner{
		cfg:                  cfg,
		logger:               log.Logger.With().Stringer("chain", cfg.ChainID).Logger(),
		errCounter:           m.GetCounterVec(metrics.BlockScanError(cfg.ChainID)),
		ethRpc:               ethRpc,
		db:                   storage,
		m:                    m,
		gasPrice:             big.NewInt(0),
		lastReportedGasPrice: 0,
		gasPriceChanged:      false,
		blockMetaAccessor:    blockMetaAccessor,
		bridge:               bridge,
		vaultABI:             vaultABI,
		erc20ABI:             erc20ABI,
		eipSigner:            etypes.NewLondonSigner(chainID),
		pubkeyMgr:            pubkeyMgr,
		gasCache:             make([]*big.Int, 0),
		solvencyReporter:     solvencyReporter,
		whitelistTokens:      whitelistTokens,
		whitelistContracts:   whitelistContracts,
		signerCacheManager:   signerCacheManager,
		tokenManager:         tokenManager,
	}, nil
}

// --------------------------------- exported ---------------------------------

// GetGasPrice returns the current gas price.
func (e *EVMScanner) GetGasPrice() *big.Int {
	return e.gasPrice
}

// GetHeight returns the current block height.
func (e *EVMScanner) GetHeight() (int64, error) {
	height, err := e.ethRpc.GetBlockHeight()
	if err != nil {
		return -1, err
	}
	return height, nil
}

// GetNonce returns the nonce for the given address.
func (e *EVMScanner) GetNonce(addr string) (uint64, error) {
	return e.ethRpc.GetNonce(addr)
}

// FetchMemPool returns all transactions in the mempool.
func (e *EVMScanner) FetchMemPool(_ int64) (stypes.TxIn, error) {
	return stypes.TxIn{}, nil
}

// GetTokens returns all token meta data.
func (e *EVMScanner) GetTokens() ([]*evmtypes.TokenMeta, error) {
	return e.tokenManager.GetTokens()
}

// FetchTxs extracts all relevant transactions from the block at the provided height.
func (e *EVMScanner) FetchTxs(height, chainHeight int64) (stypes.TxIn, error) {
	// log height every 100 blocks
	if height%100 == 0 {
		e.logger.Info().Int64("height", height).Msg("fetching txs for height")
	}

	// process all transactions in the block
	e.currentBlockHeight = height
	block, err := e.ethRpc.GetBlock(height)
	if err != nil {
		return stypes.TxIn{}, err
	}
	txIn, err := e.processBlock(block)
	if err != nil {
		e.logger.Error().Err(err).Int64("height", height).Msg("failed to search tx in block")
		return stypes.TxIn{}, fmt.Errorf("failed to process block: %d, err:%w", height, err)
	}

	// skip reporting network fee and solvency if block more than flexibility blocks from tip
	if chainHeight-height > e.cfg.ObservationFlexibilityBlocks {
		return txIn, nil
	}

	// report network fee and solvency
	e.reportNetworkFee(height)
	if e.solvencyReporter != nil {
		if err := e.solvencyReporter(height); err != nil {
			e.logger.Err(err).Msg("failed to report Solvency info to THORNode")
		}
	}

	return txIn, nil
}

// --------------------------------- extraction ---------------------------------

func (e *EVMScanner) processBlock(block *etypes.Block) (stypes.TxIn, error) {
	txIn := stypes.TxIn{
		Chain:           e.cfg.ChainID,
		TxArray:         nil,
		Filtered:        false,
		MemPool:         false,
		SentUnFinalised: false,
		Finalised:       false,
	}

	// skip empty blocks
	if block.Transactions().Len() == 0 {
		return txIn, nil
	}

	// collect gas prices of txs in current block
	var txsGas []*big.Int
	for _, tx := range block.Transactions() {
		txsGas = append(txsGas, tx.GasPrice())
	}
	e.updateGasPrice(txsGas)

	// collect all relevant transactions from the block
	txInBlock, err := e.getTxIn(block)
	if err != nil {
		return txIn, err
	}
	if len(txInBlock.TxArray) > 0 {
		txIn.TxArray = append(txIn.TxArray, txInBlock.TxArray...)
	}
	return txIn, nil
}

func (e *EVMScanner) getTxIn(block *etypes.Block) (stypes.TxIn, error) {
	txInbound := stypes.TxIn{
		Chain:    e.cfg.ChainID,
		Filtered: false,
		MemPool:  false,
	}

	sem := semaphore.NewWeighted(e.cfg.Concurrency)
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}

	processTx := func(tx *etypes.Transaction) {
		defer wg.Done()
		if err := sem.Acquire(context.Background(), 1); err != nil {
			e.logger.Err(err).Msg("failed to acquire semaphore")
			return
		}
		defer sem.Release(1)

		if tx.To() == nil {
			return
		}

		// best effort remove the tx from the signed txs (ok if it does not exist)
		if err := e.blockMetaAccessor.RemoveSignedTxItem(tx.Hash().String()); err != nil {
			e.logger.Err(err).Str("tx hash", tx.Hash().String()).Msg("failed to remove signed tx item")
		}

		txInItem, err := e.getTxInItem(tx)
		if err != nil {
			e.logger.Error().Err(err).Str("hash", tx.Hash().Hex()).Msg("failed to get one tx from server")
			return
		}
		if txInItem == nil {
			return
		}

		// sometimes if a transaction failed due to a gas problem it will have no `to` address
		if len(txInItem.To) == 0 {
			return
		}

		if len([]byte(txInItem.Memo)) > constants.MaxMemoSize {
			return
		}
		txInItem.BlockHeight = block.Number().Int64()
		mu.Lock()
		txInbound.TxArray = append(txInbound.TxArray, *txInItem)
		mu.Unlock()
	}

	// process txs in parallel
	for _, tx := range block.Transactions() {
		wg.Add(1)
		go processTx(tx)
	}
	wg.Wait()

	if len(txInbound.TxArray) == 0 {
		e.logger.Debug().Uint64("block", block.NumberU64()).Msg("no tx need to be processed in this block")
		return stypes.TxIn{}, nil
	}
	txInbound.Count = strconv.Itoa(len(txInbound.TxArray))
	return txInbound, nil
}

func (a *EVMScanner) getTxInItem(tx *etypes.Transaction) (*stypes.TxInItem, error) {
	if tx == nil || tx.To() == nil {
		return nil, nil
	}

	receipt, err := a.ethRpc.GetReceipt(tx.Hash().Hex())
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	if receipt.Status != 1 {
		a.logger.Debug().Stringer("txid", tx.Hash()).Uint64("status", receipt.Status).Msg("tx failed")

		// remove failed transactions from signer cache so they are retried
		if a.signerCacheManager != nil {
			a.signerCacheManager.RemoveSigned(tx.Hash().String())
		}

		return a.getTxInFromFailedTransaction(tx, receipt), nil
	}

	if a.isToValidContractAddress(tx.To(), true) {
		return a.getTxInFromSmartContract(tx, receipt)
	}

	a.logger.Debug().Stringer("txid", tx.Hash()).Stringer("to", tx.To()).Msg("not a valid contract")
	return a.getTxInFromTransaction(tx)
}

// --------------------------------- gas ---------------------------------

// updateGasPrice calculates and stores the current gas price to reported to thornode
func (e *EVMScanner) updateGasPrice(prices []*big.Int) {
	// skip empty blocks
	if len(prices) == 0 {
		return
	}

	// find the median gas price in the block
	sort.Slice(prices, func(i, j int) bool { return prices[i].Cmp(prices[j]) == -1 })
	gasPrice := prices[len(prices)/2]

	// add to the cache
	e.gasCache = append(e.gasCache, gasPrice)
	if len(e.gasCache) > e.cfg.GasCacheBlocks {
		e.gasCache = e.gasCache[(len(e.gasCache) - e.cfg.GasCacheBlocks):]
	}

	// skip update unless cache is full
	if len(e.gasCache) < e.cfg.GasCacheBlocks {
		return
	}

	// compute the median of the median prices in the cache
	medians := []*big.Int{}
	medians = append(medians, e.gasCache...)
	sort.Slice(medians, func(i, j int) bool { return medians[i].Cmp(medians[j]) == -1 })
	median := medians[len(medians)/2]

	// round the price up to avoid fee noise
	resolution := big.NewInt(e.cfg.GasPriceResolution)
	if median.Cmp(resolution) != 1 {
		e.gasPrice = resolution
	} else {
		median.Sub(median, big.NewInt(1))
		median.Quo(median, big.NewInt(e.cfg.GasPriceResolution))
		median.Add(median, big.NewInt(1))
		median.Mul(median, big.NewInt(e.cfg.GasPriceResolution))
		e.gasPrice = median
	}

	// record metrics
	gasPriceFloat, _ := new(big.Float).SetInt64(e.gasPrice.Int64()).Float64()
	e.m.GetGauge(metrics.GasPrice(e.cfg.ChainID)).Set(gasPriceFloat)
	e.m.GetCounter(metrics.GasPriceChange(e.cfg.ChainID)).Inc()
}

// reportNetworkFee reports current network fee to thornode
func (e *EVMScanner) reportNetworkFee(height int64) {
	gasPrice := e.GetGasPrice()

	// skip posting if there is not yet a fee
	if gasPrice.Cmp(big.NewInt(0)) == 0 {
		return
	}

	// skip fee if less than 1 resolution away from the last
	feeDelta := new(big.Int).Sub(gasPrice, big.NewInt(int64(e.lastReportedGasPrice)))
	feeDelta.Abs(feeDelta)
	if e.lastReportedGasPrice != 0 && feeDelta.Cmp(big.NewInt(e.cfg.GasPriceResolution)) != 1 {
		return
	}

	// gas price to 1e8
	tcGasPrice := new(big.Int).Div(gasPrice, big.NewInt(common.One*100))

	// post to thorchain
	if _, err := e.bridge.PostNetworkFee(height, e.cfg.ChainID, MaxContractGas, tcGasPrice.Uint64()); err != nil {
		e.logger.Err(err).Msg("failed to post EVM chain single transfer fee to THORNode")
	} else {
		e.lastReportedGasPrice = gasPrice.Uint64()
	}
}

// --------------------------------- parse transaction ---------------------------------

func (e *EVMScanner) getTxInFromTransaction(tx *etypes.Transaction) (*stypes.TxInItem, error) {
	txInItem := &stypes.TxInItem{
		Tx: tx.Hash().Hex()[2:], // drop the "0x" prefix
	}

	sender, err := e.eipSigner.Sender(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender: %w", err)
	}
	txInItem.Sender = strings.ToLower(sender.String())
	txInItem.To = strings.ToLower(tx.To().String())

	// on native transactions the memo is hex encoded in the data field
	data := tx.Data()
	if len(data) > 0 {
		memo, err := hex.DecodeString(string(data))
		if err != nil {
			txInItem.Memo = string(data)
		} else {
			txInItem.Memo = string(memo)
		}
	}

	nativeValue := e.tokenManager.ConvertAmount(evm.NativeTokenAddr, tx.Value())
	txInItem.Coins = append(txInItem.Coins, common.NewCoin(e.cfg.ChainID.GetGasAsset(), nativeValue))
	txGasPrice := tx.GasPrice()
	if txGasPrice.Cmp(big.NewInt(tenGwei)) < 0 {
		txGasPrice = big.NewInt(tenGwei)
	}
	txInItem.Gas = common.MakeEVMGas(e.cfg.ChainID, txGasPrice, tx.Gas())
	txInItem.Gas[0].Asset = e.cfg.ChainID.GetGasAsset()

	if txInItem.Coins.IsEmpty() {
		e.logger.Debug().Msg("there is no coin in this tx, ignore")
		return nil, nil
	}

	return txInItem, nil
}

// isToValidContractAddress this method make sure the transaction to address is to
// THORChain router or a whitelist address
func (e *EVMScanner) isToValidContractAddress(addr *ecommon.Address, includeWhiteList bool) bool {
	// get the smart contract used by thornode
	contractAddresses := e.pubkeyMgr.GetContracts(e.cfg.ChainID)
	if includeWhiteList {
		contractAddresses = append(contractAddresses, e.whitelistContracts...)
	}

	// combine the whitelist smart contract address
	for _, item := range contractAddresses {
		if strings.EqualFold(item.String(), addr.String()) {
			return true
		}
	}
	return false
}

// getTxInFromSmartContract returns txInItem
func (e *EVMScanner) getTxInFromSmartContract(tx *etypes.Transaction, receipt *etypes.Receipt) (*stypes.TxInItem, error) {
	txInItem := &stypes.TxInItem{
		Tx: tx.Hash().Hex()[2:], // drop the "0x" prefix
	}
	sender, err := e.eipSigner.Sender(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender: %w", err)
	}
	txInItem.Sender = strings.ToLower(sender.String())
	// 1 is Transaction success state
	if receipt.Status != 1 {
		e.logger.Debug().Stringer("txid", tx.Hash()).Uint64("status", receipt.Status).Msg("tx failed")
		return nil, nil
	}
	p := evm.NewSmartContractLogParser(e.isToValidContractAddress,
		e.tokenManager.GetAssetFromTokenAddress,
		e.tokenManager.GetTokenDecimalsForTHORChain,
		e.tokenManager.ConvertAmount,
		e.vaultABI,
		e.cfg.ChainID.GetGasAsset(),
	)

	// txInItem will be changed in p.getTxInItem function, so if the function return an
	// error txInItem should be abandoned
	isVaultTransfer, err := p.GetTxInItem(receipt.Logs, txInItem)
	if err != nil {
		return nil, fmt.Errorf("failed to parse logs, err: %w", err)
	}
	if isVaultTransfer {
		contractAddresses := e.pubkeyMgr.GetContracts(e.cfg.ChainID)
		isDirectlyToRouter := false
		for _, item := range contractAddresses {
			if strings.EqualFold(item.String(), tx.To().String()) {
				isDirectlyToRouter = true
				break
			}
		}
		if isDirectlyToRouter {
			// it is important to keep this part outside the above loop, as when we do router
			// upgrade, which might generate multiple deposit event, along with tx that has
			// native value in it
			nativeValue := cosmos.NewUintFromBigInt(tx.Value())
			if !nativeValue.IsZero() {
				nativeValue = e.tokenManager.ConvertAmount(evm.NativeTokenAddr, tx.Value())
				if txInItem.Coins.GetCoin(e.cfg.ChainID.GetGasAsset()).IsEmpty() && !nativeValue.IsZero() {
					txInItem.Coins = append(txInItem.Coins, common.NewCoin(e.cfg.ChainID.GetGasAsset(), nativeValue))
				}
			}
		}
	}
	e.logger.Info().Str("tx hash", txInItem.Tx).Str("gas price", tx.GasPrice().String()).Uint64("gas used", receipt.GasUsed).Uint64("tx status", receipt.Status).Msg("txInItem parsed from smart contract")

	// under no circumstance EVM gas price will be less than 1 Gwei, unless it is in dev environment
	txGasPrice := tx.GasPrice()
	if txGasPrice.Cmp(big.NewInt(tenGwei)) < 0 {
		txGasPrice = big.NewInt(tenGwei)
	}
	txInItem.Gas = common.MakeEVMGas(e.cfg.ChainID, txGasPrice, receipt.GasUsed)
	if txInItem.Coins.IsEmpty() {
		return nil, nil
	}
	return txInItem, nil
}

// getTxInFromFailedTransaction when a transaction failed due to out of gas, this method
// will check whether the transaction is an outbound it fake a txInItem if the failed
// transaction is an outbound , and report it back to thornode, thus the gas fee can be
// subsidised need to know that this will also cause the yggdrasil / asgard that send
// out the outbound to be slashed 1.5x gas it is for security purpose
func (e *EVMScanner) getTxInFromFailedTransaction(tx *etypes.Transaction, receipt *etypes.Receipt) *stypes.TxInItem {
	if receipt.Status == 1 {
		e.logger.Info().Str("hash", tx.Hash().String()).Msg("success transaction should not get into getTxInFromFailedTransaction")
		return nil
	}
	fromAddr, err := e.eipSigner.Sender(tx)
	if err != nil {
		e.logger.Err(err).Msg("failed to get from address")
		return nil
	}
	ok, cif := e.pubkeyMgr.IsValidPoolAddress(fromAddr.String(), e.cfg.ChainID)
	if !ok || cif.IsEmpty() {
		return nil
	}
	txGasPrice := tx.GasPrice()
	if txGasPrice.Cmp(big.NewInt(tenGwei)) < 0 {
		txGasPrice = big.NewInt(tenGwei)
	}
	txHash := tx.Hash().Hex()[2:]

	return &stypes.TxInItem{
		Tx:     txHash,
		Memo:   memo.NewOutboundMemo(common.TxID(txHash)).String(),
		Sender: strings.ToLower(fromAddr.String()),
		To:     strings.ToLower(tx.To().String()),
		Coins:  common.NewCoins(common.NewCoin(e.cfg.ChainID.GetGasAsset(), cosmos.NewUint(1))),
		Gas:    common.MakeEVMGas(e.cfg.ChainID, txGasPrice, tx.Gas()),
	}
}
