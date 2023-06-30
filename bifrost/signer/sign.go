package signer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	tssp "gitlab.com/thorchain/tss/go-tss/tss"

	"gitlab.com/thorchain/thornode/app"
	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/observer"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients"
	"gitlab.com/thorchain/thornode/bifrost/pubkeymanager"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/bifrost/tss"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/config"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain"
	ttypes "gitlab.com/thorchain/thornode/x/thorchain/types"
)

// Signer will pull the tx out from thorchain and then forward it to chain
type Signer struct {
	logger                zerolog.Logger
	cfg                   config.BifrostSignerConfiguration
	wg                    *sync.WaitGroup
	thorchainBridge       thorclient.ThorchainBridge
	stopChan              chan struct{}
	blockScanner          *blockscanner.BlockScanner
	thorchainBlockScanner *ThorchainBlockScan
	chains                map[common.Chain]chainclients.ChainClient
	storage               SignerStorage
	m                     *metrics.Metrics
	errCounter            *prometheus.CounterVec
	tssKeygen             *tss.KeyGen
	pubkeyMgr             pubkeymanager.PubKeyValidator
	constantsProvider     *ConstantsProvider
	localPubKey           common.PubKey
	tssKeysignMetricMgr   *metrics.TssKeysignMetricMgr
	observer              *observer.Observer
}

// NewSigner create a new instance of signer
func NewSigner(cfg config.BifrostSignerConfiguration,
	thorchainBridge thorclient.ThorchainBridge,
	thorKeys *thorclient.Keys,
	pubkeyMgr pubkeymanager.PubKeyValidator,
	tssServer *tssp.TssServer,
	chains map[common.Chain]chainclients.ChainClient,
	m *metrics.Metrics,
	tssKeysignMetricMgr *metrics.TssKeysignMetricMgr,
	obs *observer.Observer,
) (*Signer, error) {
	storage, err := NewSignerStore(cfg.SignerDbPath, cfg.LevelDB, thorchainBridge.GetConfig().SignerPasswd)
	if err != nil {
		return nil, fmt.Errorf("fail to create thorchain scan storage: %w", err)
	}
	if tssKeysignMetricMgr == nil {
		return nil, fmt.Errorf("fail to create signer , tss keysign metric manager is nil")
	}
	var na *ttypes.NodeAccount
	for i := 0; i < 300; i++ { // wait for 5 min before timing out
		var err error
		na, err = thorchainBridge.GetNodeAccount(thorKeys.GetSignerInfo().GetAddress().String())
		if err != nil {
			return nil, fmt.Errorf("fail to get node account from thorchain,err:%w", err)
		}

		if !na.PubKeySet.Secp256k1.IsEmpty() {
			break
		}
		time.Sleep(constants.ThorchainBlockTime)
		fmt.Println("Waiting for node account to be registered...")
	}
	for _, item := range na.GetSignerMembership() {
		pubkeyMgr.AddPubKey(item, true)
	}
	if na.PubKeySet.Secp256k1.IsEmpty() {
		return nil, fmt.Errorf("unable to find pubkey for this node account. exiting... ")
	}
	pubkeyMgr.AddNodePubKey(na.PubKeySet.Secp256k1)

	cfg.BlockScanner.ChainID = common.THORChain // hard code to thorchain

	// Create pubkey manager and add our private key (Yggdrasil pubkey)
	thorchainBlockScanner, err := NewThorchainBlockScan(cfg.BlockScanner, storage, thorchainBridge, m, pubkeyMgr)
	if err != nil {
		return nil, fmt.Errorf("fail to create thorchain block scan: %w", err)
	}

	blockScanner, err := blockscanner.NewBlockScanner(cfg.BlockScanner, storage, m, thorchainBridge, thorchainBlockScanner)
	if err != nil {
		return nil, fmt.Errorf("fail to create block scanner: %w", err)
	}

	kg, err := tss.NewTssKeyGen(thorKeys, tssServer, thorchainBridge)
	if err != nil {
		return nil, fmt.Errorf("fail to create Tss Key gen,err:%w", err)
	}
	constantProvider := NewConstantsProvider(thorchainBridge)
	return &Signer{
		logger:                log.With().Str("module", "signer").Logger(),
		cfg:                   cfg,
		wg:                    &sync.WaitGroup{},
		stopChan:              make(chan struct{}),
		blockScanner:          blockScanner,
		thorchainBlockScanner: thorchainBlockScanner,
		chains:                chains,
		m:                     m,
		storage:               storage,
		errCounter:            m.GetCounterVec(metrics.SignerError),
		pubkeyMgr:             pubkeyMgr,
		thorchainBridge:       thorchainBridge,
		tssKeygen:             kg,
		constantsProvider:     constantProvider,
		localPubKey:           na.PubKeySet.Secp256k1,
		tssKeysignMetricMgr:   tssKeysignMetricMgr,
		observer:              obs,
	}, nil
}

func (s *Signer) getChain(chainID common.Chain) (chainclients.ChainClient, error) {
	chain, ok := s.chains[chainID]
	if !ok {
		s.logger.Debug().Str("chain", chainID.String()).Msg("is not supported yet")
		return nil, errors.New("not supported")
	}
	return chain, nil
}

// Start signer process
func (s *Signer) Start() error {
	s.wg.Add(1)
	go s.processTxnOut(s.thorchainBlockScanner.GetTxOutMessages(), 1)

	s.wg.Add(1)
	go s.processKeygen(s.thorchainBlockScanner.GetKeygenMessages())

	s.wg.Add(1)
	go s.signTransactions()

	s.blockScanner.Start(nil)
	return nil
}

func (s *Signer) shouldSign(tx types.TxOutItem) bool {
	return s.pubkeyMgr.HasPubKey(tx.VaultPubKey)
}

// signTransactions - looks for work to do by getting a list of all unsigned
// transactions stored in the storage
func (s *Signer) signTransactions() {
	s.logger.Info().Msg("start to sign transactions")
	defer s.logger.Info().Msg("stop to sign transactions")
	defer s.wg.Done()
	for {
		select {
		case <-s.stopChan:
			return
		default:
			// When THORChain is catching up , bifrost might get stale data from thornode , thus it shall pause signing
			catchingUp, err := s.thorchainBridge.IsCatchingUp()
			if err != nil {
				s.logger.Error().Err(err).Msg("fail to get thorchain sync status")
				time.Sleep(constants.ThorchainBlockTime)
				break // this will break select
			}
			if !catchingUp {
				s.processTransactions()
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func runWithContext(ctx context.Context, fn func() ([]byte, *types.TxInItem, error)) ([]byte, *types.TxInItem, error) {
	ch := make(chan error, 1)
	var checkpoint []byte
	var txIn *types.TxInItem
	go func() {
		var err error
		checkpoint, txIn, err = fn()
		ch <- err
	}()
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case err := <-ch:
		return checkpoint, txIn, err
	}
}

func (s *Signer) processTransactions() {
	wg := &sync.WaitGroup{}
	for _, items := range s.storage.OrderedLists() {
		wg.Add(1)

		go func(items []TxOutStoreItem) {
			chain := items[0].TxOutItem.Chain // all items in a batch should be the same chain

			// make all observations for the batch before returning
			observations := []types.TxInItem{}
			defer func() {
				if len(observations) > 0 {
					s.observer.ObserveSigned(types.TxIn{
						Count:                strconv.Itoa(len(observations)),
						Chain:                chain,
						TxArray:              observations,
						MemPool:              true,
						Filtered:             true,
						SentUnFinalised:      false,
						Finalised:            false,
						ConfirmationRequired: 0,
					})
				}

				wg.Done()
			}()

			// precondition: all transactions should be for the same chain
			for _, item := range items {
				if !item.TxOutItem.Chain.Equals(chain) {
					s.logger.Error().Msgf("tx out items for different chains in the same batch: %s, %s", item.TxOutItem.Chain, items[0].TxOutItem.Chain)
					return
				}
			}

			// if any tx out items are in broadcast or round 7 failure retry, only proceed with those
			retryItems := []TxOutStoreItem{}
			for _, item := range items {
				if item.Round7Retry || len(item.SignedTx) > 0 {
					retryItems = append(retryItems, item)
				}
			}
			if len(retryItems) > 0 {
				s.logger.Info().Msgf("found %d retry items", len(retryItems))
				items = retryItems
			}
			if len(retryItems) > 1 {
				s.logger.Error().Msgf("found %d retry items, there should only be one", len(retryItems))
			}

			for i, item := range items {
				select {
				case <-s.stopChan:
					return
				default:
					if item.Status == TxSpent { // don't rebroadcast spent transactions
						continue
					}

					s.logger.Info().Int("num", i).Int64("height", item.Height).Int("status", int(item.Status)).Interface("tx", item.TxOutItem).Msgf("Signing transaction")
					// a single keysign should not take longer than 5 minutes , regardless TSS or local
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					checkpoint, obs, err := runWithContext(ctx, func() ([]byte, *types.TxInItem, error) {
						return s.signAndBroadcast(item)
					})

					// if enabled and the observation is non-nil, add to this batch's observations
					if s.cfg.AutoObserve && obs != nil {
						observations = append(observations, *obs)
					}

					if err != nil {
						// mark the txout on round 7 failure to block other txs for the chain / pubkey
						ksErr := tss.KeysignError{}
						if errors.As(err, &ksErr) && ksErr.IsRound7() {
							s.logger.Error().Err(err).Interface("tx", item.TxOutItem).Msg("round 7 signing error")
							item.Round7Retry = true
							item.Checkpoint = checkpoint
							if err := s.storage.Set(item); err != nil {
								s.logger.Error().Err(err).Msg("fail to update tx out store item with round 7 retry")
							}
						}

						if errors.Is(err, context.DeadlineExceeded) {
							panic(fmt.Errorf("tx out item: %+v , keysign timeout : %w", item.TxOutItem, err))
						}
						s.logger.Error().Err(err).Msg("fail to sign and broadcast tx out store item")
						cancel()
						return
						// The 'item' for loop should not be items[0],
						// because problems which return 'nil, nil' should be skipped over instead of blocking others.
						// When signAndBroadcast returns an error (such as from a keysign timeout),
						// a 'return' and not a 'continue' should be used so that nodes can all restart the list,
						// for when the keysign failure was from a loss of list synchrony.
						// Otherwise, out-of-sync lists would cycle one timeout at a time, maybe never resynchronising.
					}
					cancel()

					// We have a successful broadcast! Remove the item from our store
					if err := s.storage.Remove(item); err != nil {
						s.logger.Error().Err(err).Msg("fail to update tx out store item")
					}
				}
			}
		}(items)
	}
	wg.Wait()
}

// processTxnOut processes outbound TxOuts and save them to storage
func (s *Signer) processTxnOut(ch <-chan types.TxOut, idx int) {
	s.logger.Info().Int("idx", idx).Msg("start to process tx out")
	defer s.logger.Info().Int("idx", idx).Msg("stop to process tx out")
	defer s.wg.Done()
	for {
		select {
		case <-s.stopChan:
			return
		case txOut, more := <-ch:
			if !more {
				return
			}
			s.logger.Info().Msgf("Received a TxOut Array of %v from the Thorchain", txOut)
			items := make([]TxOutStoreItem, 0, len(txOut.TxArray))

			for i, tx := range txOut.TxArray {
				items = append(items, NewTxOutStoreItem(txOut.Height, tx.TxOutItem(), int64(i)))
			}
			if err := s.storage.Batch(items); err != nil {
				s.logger.Error().Err(err).Msg("fail to save tx out items to storage")
			}
		}
	}
}

func (s *Signer) processKeygen(ch <-chan ttypes.KeygenBlock) {
	s.logger.Info().Msg("start to process keygen")
	defer s.logger.Info().Msg("stop to process keygen")
	defer s.wg.Done()
	for {
		select {
		case <-s.stopChan:
			return
		case keygenBlock, more := <-ch:
			if !more {
				return
			}
			s.logger.Info().Msgf("Received a keygen block %+v from the Thorchain", keygenBlock)
			for _, keygenReq := range keygenBlock.Keygens {
				keygenStart := time.Now()
				pubKey, blame, err := s.tssKeygen.GenerateNewKey(keygenBlock.Height, keygenReq.GetMembers())
				if !blame.IsEmpty() {
					err := fmt.Errorf("reason: %s, nodes %+v", blame.FailReason, blame.BlameNodes)
					s.logger.Error().Err(err).Msg("Blame")
				}
				keygenTime := time.Since(keygenStart).Milliseconds()

				if err != nil {
					s.errCounter.WithLabelValues("fail_to_keygen_pubkey", "").Inc()
					s.logger.Error().Err(err).Msg("fail to generate new pubkey")
				}
				if !pubKey.Secp256k1.IsEmpty() {
					s.pubkeyMgr.AddPubKey(pubKey.Secp256k1, true)
				}

				// Add pubkeys to pubkey manager for monitoring...
				// each member might become a yggdrasil pool
				for _, pk := range keygenReq.GetMembers() {
					s.pubkeyMgr.AddPubKey(pk, false)
				}

				if err := s.sendKeygenToThorchain(keygenBlock.Height, pubKey.Secp256k1, blame, keygenReq.GetMembers(), keygenReq.Type, keygenTime); err != nil {
					s.errCounter.WithLabelValues("fail_to_broadcast_keygen", "").Inc()
					s.logger.Error().Err(err).Msg("fail to broadcast keygen")
				}

			}
		}
	}
}

func (s *Signer) sendKeygenToThorchain(height int64, poolPk common.PubKey, blame ttypes.Blame, input common.PubKeys, keygenType ttypes.KeygenType, keygenTime int64) error {
	// collect supported chains in the configuration
	chains := common.Chains{
		common.THORChain,
	}
	for name, chain := range s.chains {
		if !chain.GetConfig().OptToRetire {
			chains = append(chains, name)
		}
	}

	// make a best effort to add encrypted keyshares to the message
	var keyshares []byte
	var err error
	if s.cfg.BackupKeyshares {
		keyshares, err = tss.EncryptKeyshares(
			filepath.Join(app.DefaultNodeHome(), fmt.Sprintf("localstate-%s.json", poolPk)),
			os.Getenv("SIGNER_SEED_PHRASE"),
		)
		if err != nil {
			s.logger.Error().Err(err).Msg("fail to encrypt keyshares")
		}
	}

	keygenMsg, err := s.thorchainBridge.GetKeygenStdTx(poolPk, keyshares, blame, input, keygenType, chains, height, keygenTime)
	if err != nil {
		return fmt.Errorf("fail to get keygen id: %w", err)
	}
	strHeight := strconv.FormatInt(height, 10)

	txID, err := s.thorchainBridge.Broadcast(keygenMsg)
	if err != nil {
		s.errCounter.WithLabelValues("fail_to_send_to_thorchain", strHeight).Inc()
		return fmt.Errorf("fail to send the tx to thorchain: %w", err)
	}
	s.logger.Info().Int64("block", height).Str("thorchain hash", txID.String()).Msg("sign and send to thorchain successfully")
	return nil
}

// signAndBroadcast will sign the tx and broadcast it to the corresponding chain. On
// SignTx error for the chain client, if we receive checkpoint bytes we also return them
// with the error so they can be set on the TxOutStoreItem and re-used on a subsequent
// retry to avoid double spend. The second returned value is an optional observation
// that should be submitted to THORChain.
func (s *Signer) signAndBroadcast(item TxOutStoreItem) ([]byte, *types.TxInItem, error) {
	height := item.Height
	tx := item.TxOutItem

	// set the checkpoint on the tx out item if it was stored
	if item.Checkpoint != nil {
		tx.Checkpoint = item.Checkpoint
	}

	blockHeight, err := s.thorchainBridge.GetBlockHeight()
	if err != nil {
		s.logger.Error().Err(err).Msgf("fail to get block height")
		return nil, nil, err
	}
	signingTransactionPeriod, err := s.constantsProvider.GetInt64Value(blockHeight, constants.SigningTransactionPeriod)
	s.logger.Debug().Msgf("signing transaction period:%d", signingTransactionPeriod)
	if err != nil {
		s.logger.Error().Err(err).Msgf("fail to get constant value for(%s)", constants.SigningTransactionPeriod)
		return nil, nil, err
	}

	// if not in round 7 retry, discard outbound if within configured blocks of reschedule
	if !item.Round7Retry && blockHeight-signingTransactionPeriod > height-s.cfg.RescheduleBufferBlocks {
		s.logger.Error().Msgf("tx was created at block height(%d), now it is (%d), it is older than (%d) blocks, skip it", height, blockHeight, signingTransactionPeriod)
		return nil, nil, nil
	}

	chain, err := s.getChain(tx.Chain)
	if err != nil {
		s.logger.Error().Err(err).Msgf("not supported %s", tx.Chain.String())
		return nil, nil, err
	}
	mimirKey := "HALTSIGNING"
	haltSigningGlobalMimir, err := s.thorchainBridge.GetMimir(mimirKey)
	if err != nil {
		s.logger.Err(err).Msgf("fail to get %s", mimirKey)
		return nil, nil, err
	}
	if haltSigningGlobalMimir > 0 {
		s.logger.Info().Msg("signing has been halted globally")
		return nil, nil, nil
	}
	mimirKey = fmt.Sprintf("HALTSIGNING%s", tx.Chain)
	haltSigningMimir, err := s.thorchainBridge.GetMimir(mimirKey)
	if err != nil {
		s.logger.Err(err).Msgf("fail to get %s", mimirKey)
		return nil, nil, err
	}
	if haltSigningMimir > 0 {
		s.logger.Info().Msgf("signing for %s is halted", tx.Chain)
		return nil, nil, nil
	}
	if !s.shouldSign(tx) {
		s.logger.Info().Str("signer_address", chain.GetAddress(tx.VaultPubKey)).Msg("different pool address, ignore")
		return nil, nil, nil
	}

	if len(tx.ToAddress) == 0 {
		s.logger.Info().Msg("To address is empty, THORNode don't know where to send the fund , ignore")
		return nil, nil, nil // return nil and discard item
	}

	// don't sign if the block scanner is unhealthy. This is because the
	// network may not be able to detect the outbound transaction, and
	// therefore reschedule the transaction to another signer. In a disaster
	// scenario, the network could broadcast a transaction several times,
	// bleeding funds.
	if !chain.IsBlockScannerHealthy() {
		return nil, nil, fmt.Errorf("the block scanner for chain %s is unhealthy, not signing transactions due to it", chain.GetChain())
	}

	// Check if we're sending all funds back , given we don't have memo in txoutitem anymore, so it rely on the coins field to be empty
	// In this scenario, we should chose the coins to send ourselves
	if tx.Coins.IsEmpty() {
		tx, err = s.handleYggReturn(height, tx)
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to handle yggdrasil return")
			return nil, nil, err
		}
	}

	start := time.Now()
	defer func() {
		s.m.GetHistograms(metrics.SignAndBroadcastDuration(chain.GetChain())).Observe(time.Since(start).Seconds())
	}()

	if !tx.OutHash.IsEmpty() {
		s.logger.Info().Str("OutHash", tx.OutHash.String()).Msg("tx had been sent out before")
		return nil, nil, nil // return nil and discard item
	}

	// We get the keysign object from thorchain again to ensure it hasn't
	// been signed already, and we can skip. This helps us not get stuck on
	// a task that we'll never sign, because 2/3rds already has and will
	// never be available to sign again.
	txOut, err := s.thorchainBridge.GetKeysign(height, tx.VaultPubKey.String())
	if err != nil {
		s.logger.Error().Err(err).Msg("fail to get keysign items")
		return nil, nil, err
	}
	for _, txArray := range txOut.TxArray {
		if txArray.TxOutItem().Equals(tx) && !txArray.OutHash.IsEmpty() {
			// already been signed, we can skip it
			s.logger.Info().Str("tx_id", tx.OutHash.String()).Msgf("already signed. skipping...")
			return nil, nil, nil
		}
	}

	// If SignedTx is set, we already signed and should only retry broadcast.
	var signedTx, checkpoint []byte
	var elapse time.Duration
	var observation *types.TxInItem
	if len(item.SignedTx) > 0 {
		s.logger.Info().Str("memo", tx.Memo).Msg("retrying broadcast of already signed tx")
		signedTx = item.SignedTx
	} else {
		startKeySign := time.Now()
		signedTx, checkpoint, observation, err = chain.SignTx(tx, height)
		if err != nil {
			s.logger.Error().Err(err).Msg("fail to sign tx")
			return checkpoint, nil, err
		}
		elapse = time.Since(startKeySign)
	}

	// looks like the transaction is already signed
	if len(signedTx) == 0 {
		s.logger.Warn().Msgf("signed transaction is empty")
		return nil, nil, nil
	}

	// broadcast the transaction
	hash, err := chain.BroadcastTx(tx, signedTx)
	if err != nil {
		s.logger.Error().Err(err).Str("memo", tx.Memo).Msg("fail to broadcast tx to chain")

		// store the signed tx for the next retry
		item.SignedTx = signedTx
		if err := s.storage.Set(item); err != nil {
			s.logger.Error().Err(err).Msg("fail to update tx out store item with signed tx")
		}

		return nil, observation, err
	}

	if s.isTssKeysign(tx.VaultPubKey) {
		s.tssKeysignMetricMgr.SetTssKeysignMetric(hash, elapse.Milliseconds())
	}

	return nil, observation, nil
}

func (s *Signer) handleYggReturn(height int64, tx types.TxOutItem) (types.TxOutItem, error) {
	chain, err := s.getChain(tx.Chain)
	if err != nil {
		s.logger.Error().Err(err).Msgf("not supported %s", tx.Chain.String())
		return tx, err
	}
	isValid, _ := s.pubkeyMgr.IsValidPoolAddress(tx.ToAddress.String(), tx.Chain)
	if !isValid {
		errInvalidPool := fmt.Errorf("yggdrasil return should return to a valid pool address,%s is not valid", tx.ToAddress.String())
		s.logger.Error().Err(errInvalidPool).Msg("invalid yggdrasil return address")
		return tx, errInvalidPool
	}
	// it is important to set the memo field to `yggdrasil-` , thus chain client can use it to decide leave some gas coin behind to pay the fees
	tx.Memo = thorchain.NewYggdrasilReturn(height).String()
	acct, err := chain.GetAccount(tx.VaultPubKey, nil)
	if err != nil {
		return tx, fmt.Errorf("fail to get chain account info: %w", err)
	}
	tx.Coins = make(common.Coins, 0)
	for _, coin := range acct.Coins {
		asset, err := common.NewAsset(coin.Asset.String())
		asset.Chain = tx.Chain
		if err != nil {
			return tx, fmt.Errorf("fail to parse asset: %w", err)
		}
		if coin.Amount.Uint64() > 0 {
			amount := coin.Amount
			tx.Coins = append(tx.Coins, common.NewCoin(asset, amount))
		}
	}
	// Yggdrasil return should pay whatever gas is necessary
	tx.MaxGas = common.Gas{}
	return tx, nil
}

func (s *Signer) isTssKeysign(pubKey common.PubKey) bool {
	return !s.localPubKey.Equals(pubKey)
}

// Stop the signer process
func (s *Signer) Stop() error {
	s.logger.Info().Msg("receive request to stop signer")
	defer s.logger.Info().Msg("signer stopped successfully")
	close(s.stopChan)
	s.wg.Wait()
	if err := s.m.Stop(); err != nil {
		s.logger.Error().Err(err).Msg("fail to stop metric server")
	}
	s.blockScanner.Stop()
	return s.storage.Close()
}
