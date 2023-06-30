package gaia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	ctypes "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	atypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	btypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tendermint/tendermint/crypto"
	tssp "gitlab.com/thorchain/tss/go-tss/tss"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/runners"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/signercache"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/bifrost/tss"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/config"
	"gitlab.com/thorchain/thornode/constants"
	memo "gitlab.com/thorchain/thornode/x/thorchain/memo"
)

// CosmosSuccessCodes a transaction is considered successful if it returns 0
// or if tx is unauthorized or already in the mempool (another Bifrost already sent it)
var CosmosSuccessCodes = map[uint32]bool{
	errortypes.SuccessABCICode:                true,
	errortypes.ErrTxInMempoolCache.ABCICode(): true,
	errortypes.ErrWrongSequence.ABCICode():    true,
}

// CosmosClient is a structure to sign and broadcast tx to Cosmos chain used by signer mostly
type CosmosClient struct {
	logger              zerolog.Logger
	cfg                 config.BifrostChainConfiguration
	chainID             string
	txConfig            client.TxConfig
	txClient            txtypes.ServiceClient
	bankClient          btypes.QueryClient
	accountClient       atypes.QueryClient
	accts               *CosmosMetaDataStore
	tssKeyManager       *tss.KeySign
	localKeyManager     *keyManager
	thorchainBridge     thorclient.ThorchainBridge
	storage             *blockscanner.BlockScannerStorage
	blockScanner        *blockscanner.BlockScanner
	signerCacheManager  *signercache.CacheManager
	cosmosScanner       *CosmosBlockScanner
	globalSolvencyQueue chan stypes.Solvency
	wg                  *sync.WaitGroup
	stopchan            chan struct{}
}

// NewCosmosClient creates a new instance of a Cosmos-based chain client
func NewCosmosClient(
	thorKeys *thorclient.Keys,
	cfg config.BifrostChainConfiguration,
	server *tssp.TssServer,
	thorchainBridge thorclient.ThorchainBridge,
	m *metrics.Metrics,
) (*CosmosClient, error) {
	logger := log.With().Str("module", cfg.ChainID.String()).Logger()

	tssKm, err := tss.NewKeySign(server, thorchainBridge)
	if err != nil {
		return nil, fmt.Errorf("fail to create tss signer: %w", err)
	}

	priv, err := thorKeys.GetPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("fail to get private key: %w", err)
	}

	temp, err := cryptocodec.ToTmPubKeyInterface(priv.PubKey())
	if err != nil {
		return nil, fmt.Errorf("fail to get tm pub key: %w", err)
	}
	pk, err := common.NewPubKeyFromCrypto(temp)
	if err != nil {
		return nil, fmt.Errorf("fail to get pub key: %w", err)
	}
	if thorchainBridge == nil {
		return nil, errors.New("thorchain bridge is nil")
	}

	localKm := &keyManager{
		privKey: priv,
		addr:    ctypes.AccAddress(priv.PubKey().Address()),
		pubkey:  pk,
	}

	grpcConn, err := getGRPCConn(cfg.CosmosGRPCHost, cfg.CosmosGRPCTLS)
	if err != nil {
		return nil, fmt.Errorf("fail to create grpc connection,err: %w", err)
	}

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	interfaceRegistry.RegisterImplementations((*ctypes.Msg)(nil), &btypes.MsgSend{})
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txConfig := tx.NewTxConfig(marshaler, []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})

	// CHANGEME: each THORNode network (e.g. mainnet, testnet, etc.) may connect to a Cosmos chain with a different chain ID
	// Implement the logic here for determinine which chain ID to use.
	chainID := ""
	switch os.Getenv("NET") {
	case "mainnet", "stagenet":
		chainID = "cosmoshub-4"
	case "mocknet":
		chainID = "localgaia"
	}

	c := &CosmosClient{
		chainID:         chainID,
		logger:          logger,
		cfg:             cfg,
		txConfig:        txConfig,
		txClient:        txtypes.NewServiceClient(grpcConn),
		bankClient:      btypes.NewQueryClient(grpcConn),
		accountClient:   atypes.NewQueryClient(grpcConn),
		accts:           NewCosmosMetaDataStore(),
		tssKeyManager:   tssKm,
		localKeyManager: localKm,
		thorchainBridge: thorchainBridge,
		wg:              &sync.WaitGroup{},
		stopchan:        make(chan struct{}),
	}

	var path string // if not set later, will in memory storage
	if len(c.cfg.BlockScanner.DBPath) > 0 {
		path = fmt.Sprintf("%s/%s", c.cfg.BlockScanner.DBPath, c.cfg.BlockScanner.ChainID)
	}
	c.storage, err = blockscanner.NewBlockScannerStorage(path, c.cfg.ScannerLevelDB)
	if err != nil {
		return nil, fmt.Errorf("fail to create scan storage: %w", err)
	}

	c.cosmosScanner, err = NewCosmosBlockScanner(
		c.cfg.BlockScanner,
		c.storage,
		c.thorchainBridge,
		m,
		c.ReportSolvency,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos scanner: %w", err)
	}

	c.blockScanner, err = blockscanner.NewBlockScanner(c.cfg.BlockScanner, c.storage, m, c.thorchainBridge, c.cosmosScanner)
	if err != nil {
		return nil, fmt.Errorf("failed to create block scanner: %w", err)
	}

	signerCacheManager, err := signercache.NewSignerCacheManager(c.storage.GetInternalDb())
	if err != nil {
		return nil, fmt.Errorf("fail to create signer cache manager")
	}
	c.signerCacheManager = signerCacheManager

	return c, nil
}

// Start Cosmos chain client
func (c *CosmosClient) Start(globalTxsQueue chan stypes.TxIn, globalErrataQueue chan stypes.ErrataBlock, globalSolvencyQueue chan stypes.Solvency) {
	c.globalSolvencyQueue = globalSolvencyQueue
	c.tssKeyManager.Start()
	c.blockScanner.Start(globalTxsQueue)
	c.wg.Add(1)
	go runners.SolvencyCheckRunner(c.GetChain(), c, c.thorchainBridge, c.stopchan, c.wg, constants.ThorchainBlockTime)
}

// Stop Cosmos chain client
func (c *CosmosClient) Stop() {
	c.tssKeyManager.Stop()
	c.blockScanner.Stop()
	c.cosmosScanner.grpc.Close()
	close(c.stopchan)
	c.wg.Wait()
}

// GetConfig return the configuration used by Cosmos chain client
func (c *CosmosClient) GetConfig() config.BifrostChainConfiguration {
	return c.cfg
}

func (c *CosmosClient) IsBlockScannerHealthy() bool {
	return c.blockScanner.IsHealthy()
}

func (c *CosmosClient) GetChain() common.Chain {
	return c.cfg.ChainID
}

func (c *CosmosClient) GetHeight() (int64, error) {
	return c.blockScanner.FetchLastHeight()
}

// GetAddress return current signer address, it will be bech32 encoded address
func (c *CosmosClient) GetAddress(poolPubKey common.PubKey) string {
	addr, err := poolPubKey.GetAddress(c.GetChain())
	if err != nil {
		c.logger.Err(err).Str("pool_pub_key", poolPubKey.String()).Msg("fail to get pool address")
		return ""
	}
	return addr.String()
}

func (c *CosmosClient) GetAccount(pkey common.PubKey, _ *big.Int) (common.Account, error) {
	addr, err := pkey.GetAddress(c.GetChain())
	if err != nil {
		return common.Account{}, fmt.Errorf("failed to convert address (%s) from bech32: %w", pkey, err)
	}
	return c.GetAccountByAddress(addr.String(), big.NewInt(0))
}

func (c *CosmosClient) GetAccountByAddress(address string, _ *big.Int) (common.Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	bankReq := &btypes.QueryAllBalancesRequest{
		Address: address,
	}
	balances, err := c.bankClient.AllBalances(ctx, bankReq)
	if err != nil {
		return common.Account{}, err
	}

	nativeCoins := make([]common.Coin, 0)
	for _, balance := range balances.Balances {
		coin, err := fromCosmosToThorchain(balance)
		if err != nil {
			c.logger.Err(err).Interface("balances", balances.Balances).Msg("wasn't able to convert coins that passed whitelist")
			continue
		}
		nativeCoins = append(nativeCoins, coin)
	}

	authReq := &atypes.QueryAccountRequest{
		Address: address,
	}

	acc, err := c.accountClient.Account(ctx, authReq)
	if err != nil {
		return common.Account{}, err
	}

	ba := new(atypes.BaseAccount)
	err = ba.Unmarshal(acc.GetAccount().Value)
	if err != nil {
		return common.Account{}, err
	}

	return common.Account{
		Sequence:      int64(ba.Sequence),
		AccountNumber: int64(ba.AccountNumber),
		Coins:         nativeCoins,
	}, nil
}

func (c *CosmosClient) processOutboundTx(tx stypes.TxOutItem, thorchainHeight int64) (*btypes.MsgSend, error) {
	fromAddr, err := tx.VaultPubKey.GetAddress(c.GetChain())
	if err != nil {
		return nil, fmt.Errorf("failed to convert address (%s) to bech32: %w", tx.VaultPubKey.String(), err)
	}

	var coins ctypes.Coins
	for _, coin := range tx.Coins {
		// Handle yggdrasil return. Leave enough coin to pay for gas
		if strings.HasPrefix(tx.Memo, "YGGDRASIL-:") {
			if coin.Asset == c.cfg.ChainID.GetGasAsset() {
				// CHANGEME: you may need to set aside for coins for Yggdrasil return if Thorchain
				// will support a large # of assets (all returned in a single MsgSend).
				// This subtractFee takes into account two assets being sent back. Test this thoroughly.
				subtractFee := c.cosmosScanner.averageFee().Mul(ctypes.NewUint(3)).Quo(ctypes.NewUint(2))
				if coin.Amount.LT(subtractFee) {
					// not enough gas to pay for transaction
					return &btypes.MsgSend{}, fmt.Errorf("not enough gas to pay for transaction, have %d want %d", coin.Amount.Uint64(), subtractFee.Uint64())
				}
				coin.Amount = coin.Amount.Sub(subtractFee)
			}
		}
		// convert to cosmos coin
		cosmosCoin, err := fromThorchainToCosmos(coin)
		if err != nil {
			c.logger.Warn().Err(err).Interface("tx", tx).Msg("unable to convert coin fromThorchainToCosmos")
			continue
		}

		coins = append(coins, cosmosCoin)
	}

	return &btypes.MsgSend{
		FromAddress: fromAddr.String(),
		ToAddress:   tx.ToAddress.String(),
		Amount:      coins.Sort(),
	}, nil
}

// SignTx sign the the given TxArrayItem
func (c *CosmosClient) SignTx(tx stypes.TxOutItem, thorchainHeight int64) (signedTx, checkpoint []byte, _ *stypes.TxInItem, err error) {
	defer func() {
		if err != nil {
			var keysignError tss.KeysignError
			if errors.As(err, &keysignError) {
				if len(keysignError.Blame.BlameNodes) == 0 {
					c.logger.Err(err).Msg("TSS doesn't know which node to blame")
					return
				}

				// key sign error forward the keysign blame to thorchain
				txID, err := c.thorchainBridge.PostKeysignFailure(keysignError.Blame, thorchainHeight, tx.Memo, tx.Coins, tx.VaultPubKey)
				if err != nil {
					c.logger.Err(err).Msg("fail to post keysign failure to THORChain")
					return
				}
				c.logger.Info().Str("tx_id", txID.String()).Msgf("post keysign failure to thorchain")
			}
			c.logger.Err(err).Msg("failed to sign tx")
			return
		}
	}()

	if c.signerCacheManager.HasSigned(tx.CacheHash()) {
		c.logger.Info().Interface("tx", tx).Msg("transaction already signed, ignoring...")
		return nil, nil, nil, nil
	}

	msg, err := c.processOutboundTx(tx, thorchainHeight)
	if err != nil {
		c.logger.Err(err).Msg("failed to process outbound tx")
		return nil, nil, nil, err
	}

	currentHeight, err := c.cosmosScanner.GetHeight()
	if err != nil {
		c.logger.Err(err).Msg("fail to get current block height")
		return nil, nil, nil, err
	}

	// the metadata is stored as the transaction checkpoint, if it is set deserialize it
	// so we only retry with the same account number and sequence to avoid double spend
	meta := CosmosMetadata{}
	if tx.Checkpoint != nil {
		if err := json.Unmarshal(tx.Checkpoint, &meta); err != nil {
			c.logger.Err(err).Msg("fail to unmarshal checkpoint")
			return nil, nil, nil, err
		}
	} else {
		// Check if we have CosmosMetadata for the current block height before
		// fetching it from the GRPC server
		meta = c.accts.Get(tx.VaultPubKey)
		if currentHeight > meta.BlockHeight {
			acc, err := c.GetAccount(tx.VaultPubKey, big.NewInt(0))
			if err != nil {
				return nil, nil, nil, fmt.Errorf("fail to get account info: %w", err)
			}
			// Only update local sequence # if it is less than what is on chain
			// When local sequence # is larger than on chain , that could be there are transactions in mempool not commit yet
			if meta.SeqNumber <= acc.Sequence {
				meta = CosmosMetadata{
					AccountNumber: acc.AccountNumber,
					SeqNumber:     acc.Sequence,
					BlockHeight:   currentHeight,
				}
				c.accts.Set(tx.VaultPubKey, meta)
			}
		}
	}

	// serialize the checkpoint for later
	checkpointBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fail to marshal checkpoint: %w", err)
	}

	gasCoins := tx.MaxGas.ToCoins()
	if len(gasCoins) != 1 {
		// CHANGEME: same as above, you may need to tweak this depending on the chain / # of assets
		if strings.HasPrefix(tx.Memo, "YGGDRASIL-:") {
			gasCoins = append(gasCoins, common.NewCoin(
				c.GetChain().GetGasAsset(),
				c.cosmosScanner.averageFee().Mul(ctypes.NewUint(3)).Quo(ctypes.NewUint(2)),
			))
		} else {
			err = errors.New("exactly one gas coin must be provided")
			c.logger.Err(err).Interface("fee", gasCoins).Msg(err.Error())
			return nil, nil, nil, err
		}
	}

	if !gasCoins[0].Asset.Equals(c.GetChain().GetGasAsset()) {
		err = errors.New("gas coin asset must match chain gas asset")
		c.logger.Err(err).Interface("coin", gasCoins[0]).Msg(err.Error())
		return nil, nil, nil, err
	}
	cCoin, err := fromThorchainToCosmos(gasCoins[0])
	if err != nil {
		err = errors.New("gas coin is not defined in cosmos_assets.go, unable to pay fee")
		c.logger.Err(err).Msg(err.Error())
		return nil, nil, nil, err
	}
	fee := ctypes.Coins{cCoin}

	txBuilder, err := buildUnsigned(
		c.txConfig,
		msg,
		tx.VaultPubKey,
		tx.Memo,
		fee,
		uint64(meta.AccountNumber),
		uint64(meta.SeqNumber),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to build unsigned tx: %w", err)
	}

	txBytes, err := c.signMsg(
		txBuilder,
		tx.VaultPubKey,
		uint64(meta.AccountNumber),
		uint64(meta.SeqNumber),
	)
	if err != nil {
		return nil, checkpointBytes, nil, fmt.Errorf("failed to sign message: %w", err)
	}

	return txBytes, nil, nil, nil
}

// signMsg takes an unsigned msg in a txBuilder and signs it using either private key or TSS.
func (c *CosmosClient) signMsg(
	txBuilder client.TxBuilder,
	pubkey common.PubKey,
	account uint64,
	sequence uint64,
) ([]byte, error) {
	cpk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeAccPub, pubkey.String())
	if err != nil {
		return nil, fmt.Errorf("unable to GetPubKeyFromBech32 from cosmos: %w", err)
	}

	modeHandler := c.txConfig.SignModeHandler()
	signingData := signing.SignerData{
		ChainID:       c.chainID,
		AccountNumber: account,
		Sequence:      sequence,
	}

	signBytes, err := modeHandler.GetSignBytes(signingtypes.SignMode_SIGN_MODE_DIRECT, signingData, txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("unable to GetSignBytes on modeHandler: %w", err)
	}

	sigData := &signingtypes.SingleSignatureData{
		SignMode: signingtypes.SignMode_SIGN_MODE_DIRECT,
	}
	sig := signingtypes.SignatureV2{
		PubKey:   cpk,
		Data:     sigData,
		Sequence: sequence,
	}

	if c.localKeyManager.Pubkey().Equals(pubkey) {
		sigData.Signature, err = c.localKeyManager.Sign(signBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to sign using localKeyManager: %w", err)
		}
	} else {
		hashedMsg := crypto.Sha256(signBytes)
		sigData.Signature, _, err = c.tssKeyManager.RemoteSign(hashedMsg, pubkey.String())
		if err != nil {
			return nil, err
		}
	}

	// Ensure the signature is valid
	if !cpk.VerifySignature(signBytes, sigData.Signature) {
		return nil, fmt.Errorf("unable to verify signature with secpPubKey")
	}

	err = txBuilder.SetSignatures(sig)
	if err != nil {
		return nil, fmt.Errorf("unable to final SetSignatures on txBuilder: %w", err)
	}

	txBytes, err := c.txConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("unable to encode tx: %w", err)
	}

	return txBytes, nil
}

// BroadcastTx is to broadcast the tx to cosmos chain
func (c *CosmosClient) BroadcastTx(tx stypes.TxOutItem, txBytes []byte) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	req := &txtypes.BroadcastTxRequest{
		TxBytes: txBytes,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	}

	broadcastRes, err := c.txClient.BroadcastTx(ctx, req)
	if err != nil {
		c.logger.Err(err).Msg("unable to broadcast tx")
		return "", err
	}

	c.logger.Info().Interface("broadcastRes", broadcastRes).Msg("BroadcastTx success")
	if success := CosmosSuccessCodes[broadcastRes.TxResponse.Code]; !success {
		c.logger.Error().Interface("response", broadcastRes).Msg("unsuccessful error code in transaction broadcast")
		return "", errors.New("broadcast msg failed")
	}

	c.accts.SeqInc(tx.VaultPubKey)
	// Only add the transaction to signer cache when it is sure the transaction has been broadcast successfully.
	// So for other scenario , like transaction already in mempool , invalid account sequence # , the transaction can be rescheduled , and retried
	if broadcastRes.TxResponse.Code == errortypes.SuccessABCICode {
		if err := c.signerCacheManager.SetSigned(tx.CacheHash(), broadcastRes.TxResponse.TxHash); err != nil {
			c.logger.Err(err).Msg("fail to set signer cache")
		}
	}
	return broadcastRes.TxResponse.TxHash, nil
}

// ConfirmationCountReady cosmos chain has almost instant finality, so doesn't need to wait for confirmation
func (c *CosmosClient) ConfirmationCountReady(txIn stypes.TxIn) bool {
	return true
}

// GetConfirmationCount determine how many confirmations are required
// NOTE: Cosmos chains are instant finality, so confirmations are not needed.
// If the transaction was successful, we know it is included in a block and thus immutable.
func (c *CosmosClient) GetConfirmationCount(txIn stypes.TxIn) int64 {
	return 0
}

func (c *CosmosClient) ReportSolvency(blockHeight int64) error {
	if !c.ShouldReportSolvency(blockHeight) {
		return nil
	}
	asgardVaults, err := c.thorchainBridge.GetAsgards()
	if err != nil {
		return fmt.Errorf("fail to get asgards,err: %w", err)
	}
	for _, asgard := range asgardVaults {
		acct, err := c.GetAccount(asgard.PubKey, big.NewInt(0))
		if err != nil {
			c.logger.Err(err).Msgf("fail to get account balance")
			continue
		}
		if runners.IsVaultSolvent(acct, asgard, c.cosmosScanner.lastFee) && c.IsBlockScannerHealthy() {
			continue
		}
		select {
		case c.globalSolvencyQueue <- stypes.Solvency{
			Height: blockHeight,
			Chain:  c.cfg.ChainID,
			PubKey: asgard.PubKey,
			Coins:  acct.Coins,
		}:
		case <-time.After(constants.ThorchainBlockTime):
			c.logger.Info().Msgf("fail to send solvency info to THORChain, timeout")
		}
	}
	return nil
}

func (c *CosmosClient) ShouldReportSolvency(height int64) bool {
	// Block time on Cosmos-based chains generally hovers around 6 seconds (10
	// blocks/min). Since the last fee is used as a buffer we also want to ensure that is
	// non-zero (enough blocks have been seen) before checking insolvency to avoid false
	// positives.
	return height%10 == 0 && !c.cosmosScanner.lastFee.IsZero()
}

// OnObservedTxIn update the signer cache (in case we haven't already)
func (c *CosmosClient) OnObservedTxIn(txIn stypes.TxInItem, blockHeight int64) {
	m, err := memo.ParseMemo(common.LatestVersion, txIn.Memo)
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
