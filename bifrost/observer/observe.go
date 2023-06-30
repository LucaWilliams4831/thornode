package observer

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients"
	"gitlab.com/thorchain/thornode/bifrost/pubkeymanager"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/config"
	"gitlab.com/thorchain/thornode/constants"
	mem "gitlab.com/thorchain/thornode/x/thorchain/memo"
	stypes "gitlab.com/thorchain/thornode/x/thorchain/types"
)

const maxTxArrayLen = 100

// signedTxOutCacheSize is the number of signed tx out observations to keep in memory
// to prevent duplicate observations. Based on historical data at the time of writing,
// the peak of Thorchain's L1 swaps was 10k per day.
const signedTxOutCacheSize = 10_000

// Observer observer service
type Observer struct {
	logger              zerolog.Logger
	chains              map[common.Chain]chainclients.ChainClient
	stopChan            chan struct{}
	pubkeyMgr           *pubkeymanager.PubKeyManager
	onDeck              []types.TxIn
	lock                *sync.Mutex
	globalTxsQueue      chan types.TxIn
	globalErrataQueue   chan types.ErrataBlock
	globalSolvencyQueue chan types.Solvency
	m                   *metrics.Metrics
	errCounter          *prometheus.CounterVec
	thorchainBridge     thorclient.ThorchainBridge
	storage             *ObserverStorage
	tssKeysignMetricMgr *metrics.TssKeysignMetricMgr

	// signedTxOutCache is a cache to keep track of observations for outbounds which were
	// manually observed after completion of signing and should be filtered from future
	// mempool and block observations.
	signedTxOutCache *lru.Cache
}

// NewObserver create a new instance of Observer for chain
func NewObserver(pubkeyMgr *pubkeymanager.PubKeyManager,
	chains map[common.Chain]chainclients.ChainClient,
	thorchainBridge thorclient.ThorchainBridge,
	m *metrics.Metrics, dataPath string,
	tssKeysignMetricMgr *metrics.TssKeysignMetricMgr,
) (*Observer, error) {
	logger := log.Logger.With().Str("module", "observer").Logger()

	cfg := config.GetBifrost()

	storage, err := NewObserverStorage(dataPath, cfg.ObserverLevelDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create observer storage: %w", err)
	}
	if tssKeysignMetricMgr == nil {
		return nil, fmt.Errorf("tss keysign manager is nil")
	}

	signedTxOutCache, err := lru.New(signedTxOutCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create signed tx out cache: %w", err)
	}

	return &Observer{
		logger:              logger,
		chains:              chains,
		stopChan:            make(chan struct{}),
		m:                   m,
		pubkeyMgr:           pubkeyMgr,
		lock:                &sync.Mutex{},
		globalTxsQueue:      make(chan types.TxIn),
		globalErrataQueue:   make(chan types.ErrataBlock),
		globalSolvencyQueue: make(chan types.Solvency),
		errCounter:          m.GetCounterVec(metrics.ObserverError),
		thorchainBridge:     thorchainBridge,
		storage:             storage,
		tssKeysignMetricMgr: tssKeysignMetricMgr,
		signedTxOutCache:    signedTxOutCache,
	}, nil
}

func (o *Observer) getChain(chainID common.Chain) (chainclients.ChainClient, error) {
	chain, ok := o.chains[chainID]
	if !ok {
		o.logger.Debug().Str("chain", chainID.String()).Msg("is not supported yet")
		return nil, errors.New("Not supported")
	}
	return chain, nil
}

func (o *Observer) Start() error {
	o.restoreDeck()
	for _, chain := range o.chains {
		chain.Start(o.globalTxsQueue, o.globalErrataQueue, o.globalSolvencyQueue)
	}
	go o.processTxIns()
	go o.processErrataTx()
	go o.processSolvencyQueue()
	go o.deck()
	return nil
}

// ObserveSigned is called when a tx is signed by the signer and returns an observation that should be immediately submitted. Observations passed to this method will be cached in memory and skipped if they are later observed in the mempool or block.
func (o *Observer) ObserveSigned(txIn types.TxIn) {
	// add all transaction ids to the signed tx out cache
	for _, tx := range txIn.TxArray {
		o.signedTxOutCache.Add(tx.Tx, nil)
	}

	o.globalTxsQueue <- txIn
}

func (o *Observer) restoreDeck() {
	onDeckTxs, err := o.storage.GetOnDeckTxs()
	if err != nil {
		o.logger.Error().Err(err).Msg("fail to restore ondeck txs")
	}
	o.lock.Lock()
	defer o.lock.Unlock()
	o.onDeck = onDeckTxs
}

func (o *Observer) deck() {
	for {
		select {
		case <-o.stopChan:
			o.sendDeck()
			return
		case <-time.After(constants.ThorchainBlockTime):
			o.sendDeck()
		}
	}
}

func (o *Observer) sendDeck() {
	o.lock.Lock()
	defer o.lock.Unlock()
	newDeck := make([]types.TxIn, 0)
	for _, deck := range o.onDeck {
		// check if chain client has OnObservedTxIn method then call it
		chainClient, err := o.getChain(deck.Chain)
		if err != nil {
			o.logger.Error().Err(err).Msg("fail to retrieve chain client")
			continue
		}
		// retried txIn will be filtered already, doesn't need to filter it again
		if !deck.Filtered {
			deck.TxArray = o.filterObservations(deck.Chain, deck.TxArray, deck.MemPool)
			deck.TxArray = o.filterBinanceMemoFlag(deck.Chain, deck.TxArray)
			deck.ConfirmationRequired = chainClient.GetConfirmationCount(deck)
		}
		newTxIn := types.TxIn{
			Chain:                deck.Chain,
			Filtered:             true,
			MemPool:              deck.MemPool,
			SentUnFinalised:      deck.SentUnFinalised,
			ConfirmationRequired: deck.ConfirmationRequired,
		}

		if !chainClient.ConfirmationCountReady(deck) {
			// TxIn doesn't have enough confirmation , add it back to queue, and try it later
			newTxIn.TxArray = append(newTxIn.TxArray, deck.TxArray...)
			// send not finalised tx to THORChain, so THORChain can aware this inbound tx
			if !deck.SentUnFinalised {
				result := o.chunkifyAndSendToThorchain(deck, chainClient, false)
				if len(result.TxArray) == 0 {
					// all had been sent to THORChain , no left
					newTxIn.SentUnFinalised = true
				}
			}
		} else {
			// here all the tx either don't need confirmation counting or it already have enough
			result := o.chunkifyAndSendToThorchain(deck, chainClient, true)
			if len(result.TxArray) > 0 {
				newTxIn.TxArray = append(newTxIn.TxArray, result.TxArray...)
			}
		}
		if len(newTxIn.TxArray) > 0 {
			newTxIn.Count = strconv.Itoa(len(newTxIn.TxArray))
			newDeck = append(newDeck, newTxIn)
		}
	}
	// filtered , but didn't send to thorchain yet, save to key value store
	// bifrost will trap exit signal , and when exit get triggered , it will call sendToDeck before it actually quit
	// thus it is fine to save the deck txin from here
	if err := o.storage.SetOnDeckTxs(newDeck); err != nil {
		o.logger.Error().Err(err).Msg("fail to save ondeck tx to key value store")
	}
	o.onDeck = newDeck
}

func (o *Observer) chunkifyAndSendToThorchain(deck types.TxIn, chainClient chainclients.ChainClient, finalised bool) types.TxIn {
	newTxIn := types.TxIn{
		Chain:                deck.Chain,
		Filtered:             true,
		MemPool:              deck.MemPool,
		SentUnFinalised:      deck.SentUnFinalised,
		ConfirmationRequired: deck.ConfirmationRequired,
	}
	deck.Finalised = finalised
	for _, txIn := range o.chunkify(deck) {
		if err := o.signAndSendToThorchain(txIn); err != nil {
			o.logger.Error().Err(err).Msg("fail to send to THORChain")
			// tx failed to be forward to THORChain will be added back to queue , and retry later
			newTxIn.TxArray = append(newTxIn.TxArray, txIn.TxArray...)
			continue
		}

		i, ok := chainClient.(interface {
			OnObservedTxIn(txIn types.TxInItem, blockHeight int64)
		})
		if ok {
			for _, item := range txIn.TxArray {
				i.OnObservedTxIn(item, item.BlockHeight)
			}
		}
	}
	return newTxIn
}

func (o *Observer) processTxIns() {
	for {
		select {
		case <-o.stopChan:
			return
		case txIn := <-o.globalTxsQueue:
			o.lock.Lock()
			found := false
			for i, in := range o.onDeck {
				if in.Chain != txIn.Chain {
					continue
				}
				if in.MemPool != txIn.MemPool {
					continue
				}
				if in.Filtered != txIn.Filtered {
					continue
				}
				// at the moment BNB chain has very short block time , so allow multiple BNB block to bundle together , but not BTC
				if !in.Chain.Equals(common.BNBChain) && len(in.TxArray) > 0 && len(txIn.TxArray) > 0 {
					if in.TxArray[0].BlockHeight != txIn.TxArray[0].BlockHeight {
						continue
					}
				}
				o.onDeck[i].TxArray = append(o.onDeck[i].TxArray, txIn.TxArray...)
				found = true
				break
			}
			if !found {
				o.onDeck = append(o.onDeck, txIn)
			}
			if err := o.storage.SetOnDeckTxs(o.onDeck); err != nil {
				o.logger.Err(err).Msg("fail to save ondeck tx")
			}
			o.lock.Unlock()
		}
	}
}

// chunkify  breaks the observations into 100 transactions per observation
func (o *Observer) chunkify(txIn types.TxIn) (result []types.TxIn) {
	// sort it by block height
	sort.SliceStable(txIn.TxArray, func(i, j int) bool {
		return txIn.TxArray[i].BlockHeight < txIn.TxArray[j].BlockHeight
	})
	for len(txIn.TxArray) > 0 {
		newTx := types.TxIn{
			Chain:                txIn.Chain,
			MemPool:              txIn.MemPool,
			Filtered:             txIn.Filtered,
			Finalised:            txIn.Finalised,
			SentUnFinalised:      txIn.SentUnFinalised,
			ConfirmationRequired: txIn.ConfirmationRequired,
		}
		if len(txIn.TxArray) > maxTxArrayLen {
			newTx.Count = fmt.Sprintf("%d", maxTxArrayLen)
			newTx.TxArray = txIn.TxArray[:maxTxArrayLen]
			txIn.TxArray = txIn.TxArray[maxTxArrayLen:]
		} else {
			newTx.Count = fmt.Sprintf("%d", len(txIn.TxArray))
			newTx.TxArray = txIn.TxArray
			txIn.TxArray = nil
		}
		result = append(result, newTx)
	}
	return result
}

func (o *Observer) filterObservations(chain common.Chain, items []types.TxInItem, memPool bool) (txs []types.TxInItem) {
	for _, txInItem := range items {
		// NOTE: the following could result in the same tx being added
		// twice, which is expected. We want to make sure we generate both
		// a inbound and outbound txn, if we both apply.

		isInternal := false
		// check if the from address is a valid pool
		if ok, cpi := o.pubkeyMgr.IsValidPoolAddress(txInItem.Sender, chain); ok {
			txInItem.ObservedVaultPubKey = cpi.PubKey
			isInternal = true

			// skip the outbound observation if we signed and manually observed
			if !o.signedTxOutCache.Contains(txInItem.Tx) {
				txs = append(txs, txInItem)
			}
		}
		// check if the to address is a valid pool address
		// for inbound message , if it is still in mempool , it will be ignored unless it is internal transaction
		// internal tx means both from & to addresses belongs to the network. for example migrate/yggdrasil+
		if ok, cpi := o.pubkeyMgr.IsValidPoolAddress(txInItem.To, chain); ok && (!memPool || isInternal) {
			txInItem.ObservedVaultPubKey = cpi.PubKey
			txs = append(txs, txInItem)
		}
	}
	return
}

// filterBinanceMemoFlag - on Binance chain , BEP12(https://github.com/binance-chain/BEPs/blob/master/BEP12.md#memo-check-script-for-transfer)
// it allow account to enable memo check flag, with the flag enabled , if a tx doesn't have memo, or doesn't have correct memo will be rejected by the chain ,
// unfortunately THORChain won't be able to deal with these accounts , as THORChain will not know what kind of memo it required to send the tx through
// given that Bifrost have to filter out those txes
// the logic has to be here as THORChain is chain agnostic , customer can swap from BTC/ETH to BNB
func (o *Observer) filterBinanceMemoFlag(chain common.Chain, items []types.TxInItem) (txs []types.TxInItem) {
	// finds the destination address, and supports THORNames
	fetchAddr := func(memo string, bridge thorclient.ThorchainBridge) common.Address {
		m, err := mem.ParseMemo(common.LatestVersion, memo)
		if err != nil {
			o.logger.Debug().Err(err).Msgf("fail to parse memo: %s", memo)
			// don't return yet, in case a thorname destination or affiliate caused the error
		}
		if !m.GetDestination().IsEmpty() {
			return m.GetDestination()
		}

		// could not find an address, check THORNames
		var raw string
		parts := strings.Split(memo, ":")
		switch m.(type) {
		case mem.AddLiquidityMemo:
			if len(parts) > 2 {
				raw = parts[2]
			}
		case mem.SwapMemo:
			if len(parts) > 2 {
				raw = parts[2]
			}
		}
		if len(raw) == 0 {
			return common.NoAddress
		}
		name, _ := bridge.GetTHORName(raw)
		return name.GetAlias(common.BNBChain)
	}

	bnbClient, ok := o.chains[common.BNBChain]
	if !ok {
		txs = items
		return
	}
	for _, txInItem := range items {
		var addressesToCheck []string
		addr := fetchAddr(txInItem.Memo, o.thorchainBridge)
		if !addr.IsEmpty() && addr.IsChain(common.BNBChain) {
			addressesToCheck = append(addressesToCheck, addr.String())
		}
		// if it BNB chain let's check the from address as well
		if chain.Equals(common.BNBChain) {
			addressesToCheck = append(addressesToCheck, txInItem.Sender)
		}
		skip := false
		for _, item := range addressesToCheck {
			account, err := bnbClient.GetAccountByAddress(item, nil)
			if err != nil {
				o.logger.Error().Err(err).Msgf("fail to check account for %s", item)
				continue
			}
			if account.HasMemoFlag {
				skip = true
				break
			}
		}
		if !skip {
			txs = append(txs, txInItem)
		}
	}
	return
}

func (o *Observer) processErrataTx() {
	for {
		select {
		case <-o.stopChan:
			return
		case errataBlock, more := <-o.globalErrataQueue:
			if !more {
				return
			}
			// filter
			o.filterErrataTx(errataBlock)
			o.logger.Info().Msgf("Received a errata block %+v from the Thorchain", errataBlock.Height)
			for _, errataTx := range errataBlock.Txs {
				if err := o.sendErrataTxToThorchain(errataBlock.Height, errataTx.TxID, errataTx.Chain); err != nil {
					o.errCounter.WithLabelValues("fail_to_broadcast_errata_tx", "").Inc()
					o.logger.Error().Err(err).Msg("fail to broadcast errata tx")
				}
			}
		}
	}
}

// filterErrataTx with confirmation counting logic in place, all inbound tx to asgard will be parked and waiting for confirmation count to reach
// the target threshold before it get forward to THORChain,  it is possible that when a re-org happened on BTC / ETH
// the transaction that has been re-org out ,still in bifrost memory waiting for confirmation, as such, it should be
// removed from ondeck tx queue, and not forward it to THORChain
func (o *Observer) filterErrataTx(block types.ErrataBlock) {
	o.lock.Lock()
	defer o.lock.Unlock()
	for _, tx := range block.Txs {
		for deckIdx, txIn := range o.onDeck {
			idx := -1
			for i, item := range txIn.TxArray {
				if item.Tx == tx.TxID.String() {
					idx = i
					break
				}
			}
			if idx != -1 {
				o.logger.Info().Msgf("drop tx (%s) from ondeck memory due to errata", tx.TxID)
				o.onDeck[deckIdx].TxArray = append(txIn.TxArray[:idx], txIn.TxArray[idx+1:]...) // nolint
			}
		}
	}
}

func (o *Observer) sendErrataTxToThorchain(height int64, txID common.TxID, chain common.Chain) error {
	errataMsg := o.thorchainBridge.GetErrataMsg(txID, chain)
	strHeight := strconv.FormatInt(height, 10)
	txID, err := o.thorchainBridge.Broadcast(errataMsg)
	if err != nil {
		o.errCounter.WithLabelValues("fail_to_send_to_thorchain", strHeight).Inc()
		return fmt.Errorf("fail to send the tx to thorchain: %w", err)
	}
	o.logger.Info().Int64("block", height).Str("thorchain hash", txID.String()).Msg("sign and send to thorchain successfully")
	return nil
}

func (o *Observer) sendSolvencyToThorchain(height int64, chain common.Chain, pubKey common.PubKey, coins common.Coins) error {
	nodeStatus, err := o.thorchainBridge.FetchNodeStatus()
	if err != nil {
		return fmt.Errorf("failed to get node status: %w", err)
	}

	if nodeStatus != stypes.NodeStatus_Active {
		return nil
	}

	msg := o.thorchainBridge.GetSolvencyMsg(height, chain, pubKey, coins)
	if msg == nil {
		return fmt.Errorf("fail to create solvency message")
	}
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	txID, err := o.thorchainBridge.Broadcast(msg)
	if err != nil {
		strHeight := strconv.FormatInt(height, 10)
		o.errCounter.WithLabelValues("fail_to_send_to_thorchain", strHeight).Inc()
		return fmt.Errorf("fail to send the MsgSolvency to thorchain: %w", err)
	}
	o.logger.Info().Int64("block", height).Str("chain", chain.String()).Str("thorchain hash", txID.String()).Msg("sign and send MsgSolvency to thorchain successfully")
	return nil
}

func (o *Observer) signAndSendToThorchain(txIn types.TxIn) error {
	nodeStatus, err := o.thorchainBridge.FetchNodeStatus()
	if err != nil {
		return fmt.Errorf("failed to get node status: %w", err)
	}
	if nodeStatus != stypes.NodeStatus_Active {
		return nil
	}
	txs, err := o.getThorchainTxIns(txIn)
	if err != nil {
		return fmt.Errorf("fail to convert txin to thorchain txin: %w", err)
	}
	msgs, err := o.thorchainBridge.GetObservationsStdTx(txs)
	if err != nil {
		return fmt.Errorf("fail to sign the tx: %w", err)
	}
	if len(msgs) == 0 {
		return nil
	}
	bf := backoff.NewExponentialBackOff()
	bf.MaxElapsedTime = constants.ThorchainBlockTime
	return backoff.Retry(func() error {
		txID, err := o.thorchainBridge.Broadcast(msgs...)
		if err != nil {
			return fmt.Errorf("fail to send the tx to thorchain: %w", err)
		}
		o.logger.Info().Str("thorchain hash", txID.String()).Msg("sign and send to thorchain successfully")
		return nil
	}, bf)
}

// getSaversMemo returns an add or withdraw memo for a Savers Vault
// If the tx is not a valid savers tx, an empty string will be returned
// Savers tx criteria:
// - Inbound amount must be gas asset
// - Inbound amount must be greater than the Dust Threshold of the tx chain (see chain.DustThreshold())
func (o *Observer) getSaversMemo(chain common.Chain, tx types.TxInItem) string {
	// Savers txs should have one Coin input
	if len(tx.Coins) > 1 || len(tx.Coins) == 0 {
		return ""
	}

	txAmt := tx.Coins[0].Amount
	dustThreshold := chain.DustThreshold()

	// Below dust threshold, ignore
	if txAmt.LT(dustThreshold) {
		return ""
	}

	asset := tx.Coins[0].Asset
	synthAsset := asset.GetSyntheticAsset()
	bps := txAmt.Sub(dustThreshold)

	switch {
	case bps.IsZero():
		// Amount is too low, ignore
		return ""
	case bps.LTE(cosmos.NewUint(10_000)):
		// Amount is within or includes dustThreshold + 10_000, generate withdraw memo
		return fmt.Sprintf("-:%s:%s", synthAsset.String(), bps.String())
	default:
		// Amount is above dustThreshold + 10_000, generate add memo
		return fmt.Sprintf("+:%s", synthAsset.String())
	}
}

// getThorchainTxIns convert to the type thorchain expected
// maybe in later THORNode can just refactor this to use the type in thorchain
func (o *Observer) getThorchainTxIns(txIn types.TxIn) (stypes.ObservedTxs, error) {
	txs := make(stypes.ObservedTxs, 0, len(txIn.TxArray))
	o.logger.Debug().Msgf("len %d", len(txIn.TxArray))
	for _, item := range txIn.TxArray {
		if item.Coins.IsEmpty() {
			o.logger.Info().Msgf("item(%+v) , coins are empty , so ignore", item)
			continue
		}
		if len([]byte(item.Memo)) > constants.MaxMemoSize {
			o.logger.Info().Msgf("tx (%s) memo (%s) too long", item.Tx, item.Memo)
			continue
		}

		// If memo is empty, see if it is a memo-less savers add or withdraw
		if strings.EqualFold(item.Memo, "") {
			memo := o.getSaversMemo(txIn.Chain, item)
			if !strings.EqualFold(memo, "") {
				o.logger.Info().Str("memo", memo).Str("txId", item.Tx).Msg("created savers memo")
				item.Memo = memo
			}
		}

		if len(item.To) == 0 {
			o.logger.Info().Msgf("tx (%s) to address is empty,ignore it", item.Tx)
			continue
		}
		o.logger.Debug().Str("tx-hash", item.Tx).Msg("txInItem")
		blockHeight := strconv.FormatInt(item.BlockHeight, 10)
		txID, err := common.NewTxID(item.Tx)
		if err != nil {
			o.errCounter.WithLabelValues("fail_to_parse_tx_hash", blockHeight).Inc()
			return nil, fmt.Errorf("fail to parse tx hash, %s is invalid: %w", item.Tx, err)
		}
		sender, err := common.NewAddress(item.Sender)
		if err != nil {
			o.errCounter.WithLabelValues("fail_to_parse_sender", item.Sender).Inc()
			// log the error , and ignore the transaction, since the address is not valid
			o.logger.Err(err).Msgf("fail to parse sender,%s is invalid sender address", item.Sender)
			return nil, nil
		}

		to, err := common.NewAddress(item.To)
		if err != nil {
			o.errCounter.WithLabelValues("fail_to_parse_sender", item.Sender).Inc()
			return nil, fmt.Errorf("fail to parse sender,%s is invalid sender address: %w", item.Sender, err)
		}

		o.logger.Debug().Msgf("pool pubkey %s", item.ObservedVaultPubKey)
		chainAddr, _ := item.ObservedVaultPubKey.GetAddress(txIn.Chain)
		o.logger.Debug().Msgf("%s address %s", txIn.Chain.String(), chainAddr)
		if err != nil {
			o.errCounter.WithLabelValues("fail to parse observed pool address", item.ObservedVaultPubKey.String()).Inc()
			return nil, fmt.Errorf("fail to parse observed pool address: %s: %w", item.ObservedVaultPubKey.String(), err)
		}
		height := item.BlockHeight
		if txIn.Finalised {
			height += txIn.ConfirmationRequired
		}
		tx := stypes.NewObservedTx(
			common.NewTx(txID, sender, to, item.Coins.NoneEmpty(), item.Gas, item.Memo),
			height,
			item.ObservedVaultPubKey,
			item.BlockHeight+txIn.ConfirmationRequired)
		tx.KeysignMs = o.tssKeysignMetricMgr.GetTssKeysignMetric(item.Tx)
		tx.Aggregator = item.Aggregator
		tx.AggregatorTarget = item.AggregatorTarget
		tx.AggregatorTargetLimit = item.AggregatorTargetLimit
		txs = append(txs, tx)
	}
	return txs, nil
}

func (o *Observer) processSolvencyQueue() {
	for {
		select {
		case <-o.stopChan:
			return
		case solvencyItem, more := <-o.globalSolvencyQueue:
			if !more {
				return
			}
			if solvencyItem.Chain.IsEmpty() || solvencyItem.Coins.IsEmpty() || solvencyItem.PubKey.IsEmpty() {
				continue
			}
			o.logger.Debug().Msgf("solvency:%+v", solvencyItem)
			if err := o.sendSolvencyToThorchain(solvencyItem.Height, solvencyItem.Chain, solvencyItem.PubKey, solvencyItem.Coins); err != nil {
				o.errCounter.WithLabelValues("fail_to_broadcast_solvency", "").Inc()
				o.logger.Error().Err(err).Msg("fail to broadcast solvency tx")
			}
		}
	}
}

// Stop the observer
func (o *Observer) Stop() error {
	o.logger.Debug().Msg("request to stop observer")
	defer o.logger.Debug().Msg("observer stopped")

	for _, chain := range o.chains {
		chain.Stop()
	}

	close(o.stopChan)
	if err := o.pubkeyMgr.Stop(); err != nil {
		o.logger.Error().Err(err).Msg("fail to stop pool address manager")
	}
	if err := o.storage.Close(); err != nil {
		o.logger.Err(err).Msg("fail to close observer storage")
	}
	return o.m.Stop()
}
