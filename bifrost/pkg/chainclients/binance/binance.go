package binance

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/thorchain/binance-sdk/common/types"
	ctypes "gitlab.com/thorchain/binance-sdk/common/types"
	ttypes "gitlab.com/thorchain/binance-sdk/types"
	"gitlab.com/thorchain/binance-sdk/types/msg"
	btx "gitlab.com/thorchain/binance-sdk/types/tx"
	tssp "gitlab.com/thorchain/tss/go-tss/tss"

	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/runners"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/signercache"
	"gitlab.com/thorchain/thornode/constants"
	memo "gitlab.com/thorchain/thornode/x/thorchain/memo"

	"gitlab.com/thorchain/thornode/bifrost/blockscanner"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/bifrost/tss"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/config"
	"gitlab.com/thorchain/thornode/x/thorchain"
)

// Binance is a structure to sign and broadcast tx to binance chain used by signer mostly
type Binance struct {
	logger                  zerolog.Logger
	cfg                     config.BifrostChainConfiguration
	cdc                     *codec.LegacyAmino
	chainID                 string
	isTestNet               bool
	client                  *http.Client
	accts                   *BinanceMetaDataStore
	tssKeyManager           *tss.KeySign
	localKeyManager         *keyManager
	thorchainBridge         thorclient.ThorchainBridge
	storage                 *blockscanner.BlockScannerStorage
	blockScanner            *blockscanner.BlockScanner
	bnbScanner              *BinanceBlockScanner
	globalSolvencyQueue     chan stypes.Solvency
	signerCacheManager      *signercache.CacheManager
	wg                      *sync.WaitGroup
	stopchan                chan struct{}
	lastSolvencyCheckHeight int64
}

// NewBinance create new instance of binance client
func NewBinance(thorKeys *thorclient.Keys, cfg config.BifrostChainConfiguration, server *tssp.TssServer, thorchainBridge thorclient.ThorchainBridge, m *metrics.Metrics) (*Binance, error) {
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

	b := &Binance{
		logger:          log.With().Str("module", "binance").Logger(),
		cfg:             cfg,
		cdc:             thorclient.MakeLegacyCodec(),
		accts:           NewBinanceMetaDataStore(),
		client:          &http.Client{},
		tssKeyManager:   tssKm,
		localKeyManager: localKm,
		thorchainBridge: thorchainBridge,
		stopchan:        make(chan struct{}),
		wg:              &sync.WaitGroup{},
	}

	if err := b.checkIsTestNet(); err != nil {
		b.logger.Error().Err(err).Msg("fail to check if is testnet")
		return b, err
	}

	var path string // if not set later, will in memory storage
	if len(b.cfg.BlockScanner.DBPath) > 0 {
		path = fmt.Sprintf("%s/%s", b.cfg.BlockScanner.DBPath, b.cfg.BlockScanner.ChainID)
	}
	b.storage, err = blockscanner.NewBlockScannerStorage(path, b.cfg.ScannerLevelDB)
	if err != nil {
		return nil, fmt.Errorf("fail to create scan storage: %w", err)
	}

	b.bnbScanner, err = NewBinanceBlockScanner(b.cfg.BlockScanner, b.storage, b.isTestNet, b.thorchainBridge, m, b.ReportSolvency)
	if err != nil {
		return nil, fmt.Errorf("fail to create block scanner: %w", err)
	}

	b.blockScanner, err = blockscanner.NewBlockScanner(b.cfg.BlockScanner, b.storage, m, b.thorchainBridge, b.bnbScanner)
	if err != nil {
		return nil, fmt.Errorf("fail to create block scanner: %w", err)
	}
	signerCacheManager, err := signercache.NewSignerCacheManager(b.storage.GetInternalDb())
	if err != nil {
		return nil, fmt.Errorf("fail to create signer cache manager")
	}
	b.signerCacheManager = signerCacheManager
	return b, nil
}

// Start Binance chain client
func (b *Binance) Start(globalTxsQueue chan stypes.TxIn, globalErrataQueue chan stypes.ErrataBlock, globalSolvencyQueue chan stypes.Solvency) {
	b.globalSolvencyQueue = globalSolvencyQueue
	b.tssKeyManager.Start()
	b.blockScanner.Start(globalTxsQueue)
	b.wg.Add(1)
	// Binance has 300ms block time , which is way faster than THORChain block time , thus , discover block height in solvency runner should be more aggressive
	go runners.SolvencyCheckRunner(b.GetChain(), b, b.thorchainBridge, b.stopchan, b.wg, b.GetConfig().BlockScanner.BlockHeightDiscoverBackoff)
}

// Stop Binance chain client
func (b *Binance) Stop() {
	b.tssKeyManager.Stop()
	b.blockScanner.Stop()
	close(b.stopchan)
	b.wg.Wait()
}

// GetConfig return the configuration used by Binance chain client
func (b *Binance) GetConfig() config.BifrostChainConfiguration {
	return b.cfg
}

func (b *Binance) IsBlockScannerHealthy() bool {
	return b.blockScanner.IsHealthy()
}

// checkIsTestNet determinate whether we are running on test net by checking the status
func (b *Binance) checkIsTestNet() error {
	// Cached data after first call
	if b.isTestNet {
		return nil
	}

	u, err := url.Parse(b.cfg.RPCHost)
	if err != nil {
		return fmt.Errorf("unable to parse rpc host: %s: %w", b.cfg.RPCHost, err)
	}

	u.Path = "/status"

	resp, err := b.client.Get(u.String())
	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			b.logger.Error().Err(err).Msg("fail to close resp body")
		}
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to read body") // nolint
	}

	type Status struct {
		Jsonrpc string `json:"jsonrpc"`
		ID      string `json:"id"`
		Result  struct {
			NodeInfo struct {
				Network string `json:"network"`
			} `json:"node_info"`
		} `json:"result"`
	}

	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		return fmt.Errorf("fail to unmarshal body: %w", err)
	}

	b.chainID = status.Result.NodeInfo.Network
	b.isTestNet = b.chainID == "Binance-Chain-Ganges"

	if b.isTestNet {
		types.Network = types.TestNetwork
	} else {
		types.Network = types.ProdNetwork
	}

	return nil
}

func (b *Binance) GetChain() common.Chain {
	return common.BNBChain
}

func (b *Binance) GetHeight() (int64, error) {
	return b.bnbScanner.GetHeight()
}

func (b *Binance) input(addr types.AccAddress, coins types.Coins) msg.Input {
	return msg.Input{
		Address: addr,
		Coins:   coins,
	}
}

func (b *Binance) output(addr types.AccAddress, coins types.Coins) msg.Output {
	return msg.Output{
		Address: addr,
		Coins:   coins,
	}
}

func (b *Binance) msgToSend(in []msg.Input, out []msg.Output) msg.SendMsg {
	return msg.SendMsg{Inputs: in, Outputs: out}
}

func (b *Binance) createMsg(from types.AccAddress, fromCoins types.Coins, transfers []msg.Transfer) msg.SendMsg {
	input := b.input(from, fromCoins)
	output := make([]msg.Output, 0, len(transfers))
	for _, t := range transfers {
		t.Coins = t.Coins.Sort()
		output = append(output, b.output(t.ToAddr, t.Coins))
	}
	return b.msgToSend([]msg.Input{input}, output)
}

func (b *Binance) parseTx(fromAddr string, transfers []msg.Transfer) msg.SendMsg {
	addr, err := types.AccAddressFromBech32(fromAddr)
	if err != nil {
		b.logger.Error().Str("address", fromAddr).Err(err).Msg("fail to parse address")
	}
	fromCoins := types.Coins{}
	for _, t := range transfers {
		t.Coins = t.Coins.Sort()
		fromCoins = fromCoins.Plus(t.Coins)
	}
	return b.createMsg(addr, fromCoins, transfers)
}

// GetAddress return current signer address, it will be bech32 encoded address
func (b *Binance) GetAddress(poolPubKey common.PubKey) string {
	addr, err := poolPubKey.GetAddress(common.BNBChain)
	if err != nil {
		b.logger.Error().Err(err).Str("pool_pub_key", poolPubKey.String()).Msg("fail to get pool address")
		return ""
	}
	return addr.String()
}

func (b *Binance) getGasFee(count uint64) common.Gas {
	coins := make(common.Coins, count)
	gasInfo := []cosmos.Uint{
		cosmos.NewUint(b.bnbScanner.singleFee),
		cosmos.NewUint(b.bnbScanner.multiFee),
	}
	return common.CalcBinanceGasPrice(common.Tx{Coins: coins}, common.BNBAsset, gasInfo)
}

func (b *Binance) checkAccountMemoFlag(addr string) bool {
	acct, _ := b.GetAccountByAddress(addr, nil)
	return acct.HasMemoFlag
}

// SignTx sign the the given TxArrayItem
func (b *Binance) SignTx(tx stypes.TxOutItem, thorchainHeight int64) ([]byte, []byte, *stypes.TxInItem, error) {
	var payload []msg.Transfer
	if b.signerCacheManager.HasSigned(tx.CacheHash()) {
		b.logger.Info().Msgf("transaction(%+v), signed before , ignore", tx)
		return nil, nil, nil, nil
	}
	toAddr, err := types.AccAddressFromBech32(tx.ToAddress.String())
	if err != nil {
		b.logger.Error().Err(err).Msgf("fail to parse account address(%s)", tx.ToAddress.String())
		// if we fail to parse the to address , then we log an error and move on
		return nil, nil, nil, nil
	}
	if b.checkAccountMemoFlag(toAddr.String()) {
		b.logger.Info().Msgf("address: %s has memo flag set , ignore tx", tx.ToAddress)
		return nil, nil, nil, nil
	}
	var gasCoin common.Coins

	// for yggdrasil, need to left some coin to pay for fee, this logic is per chain, given different chain charge fees differently
	if strings.EqualFold(tx.Memo, thorchain.NewYggdrasilReturn(thorchainHeight).String()) {
		gas := b.getGasFee(uint64(len(tx.Coins)))
		gasCoin = gas.ToCoins()
	}
	var coins types.Coins
	for _, coin := range tx.Coins {
		// deduct gas coin
		for _, gc := range gasCoin {
			if coin.Asset.Equals(gc.Asset) {
				coin.Amount = common.SafeSub(coin.Amount, gc.Amount)
			}
		}

		coins = append(coins, types.Coin{
			Denom:  coin.Asset.Symbol.String(),
			Amount: int64(coin.Amount.Uint64()),
		})
	}

	payload = append(payload, msg.Transfer{
		ToAddr: toAddr,
		Coins:  coins,
	})

	if len(payload) == 0 {
		b.logger.Error().Msg("payload is empty , this should not happen")
		return nil, nil, nil, nil
	}
	fromAddr := b.GetAddress(tx.VaultPubKey)
	sendMsg := b.parseTx(fromAddr, payload)
	if err := sendMsg.ValidateBasic(); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid send msg: %w", err)
	}

	currentHeight, err := b.bnbScanner.GetHeight()
	if err != nil {
		b.logger.Error().Err(err).Msg("fail to get current binance block height")
		return nil, nil, nil, err
	}

	// the metadata is stored as the transaction checkpoint, if it is set deserialize it
	// so we only retry with the same account number and sequence to avoid double spend
	meta := BinanceMetadata{}
	if tx.Checkpoint != nil {
		if err := json.Unmarshal(tx.Checkpoint, &meta); err != nil {
			b.logger.Error().Err(err).Msg("fail to unmarshal checkpoint")
			return nil, nil, nil, err
		}
	} else {
		meta = b.accts.Get(tx.VaultPubKey)
		if currentHeight > meta.BlockHeight {
			acc, err := b.GetAccount(tx.VaultPubKey, nil)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("fail to get account info: %w", err)
			}
			meta = BinanceMetadata{
				AccountNumber: acc.AccountNumber,
				SeqNumber:     acc.Sequence,
				BlockHeight:   currentHeight,
			}
			b.accts.Set(tx.VaultPubKey, meta)
		}
	}
	b.logger.Info().Int64("account_number", meta.AccountNumber).Int64("sequence_number", meta.SeqNumber).Int64("block height", meta.BlockHeight).Msg("account info")
	signMsg := btx.StdSignMsg{
		ChainID:       b.chainID,
		Memo:          tx.Memo,
		Msgs:          []msg.Msg{sendMsg},
		Source:        btx.Source,
		Sequence:      meta.SeqNumber,
		AccountNumber: meta.AccountNumber,
	}

	// serialize the checkpoint for later
	checkpointBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fail to marshal checkpoint: %w", err)
	}

	rawBz, err := b.signMsg(signMsg, fromAddr, tx.VaultPubKey, thorchainHeight, tx)
	if err != nil {
		return nil, checkpointBytes, nil, fmt.Errorf("fail to sign message: %w", err)
	}

	if len(rawBz) == 0 {
		b.logger.Warn().Msg("this should not happen, the message is empty")
		// the transaction was already signed
		return nil, nil, nil, nil
	}

	hexTx := []byte(hex.EncodeToString(rawBz))
	return hexTx, nil, nil, nil
}

func (b *Binance) sign(signMsg btx.StdSignMsg, poolPubKey common.PubKey) ([]byte, error) {
	if b.localKeyManager.Pubkey().Equals(poolPubKey) {
		return b.localKeyManager.Sign(signMsg)
	}
	return b.tssKeyManager.SignWithPool(signMsg, poolPubKey)
}

// signMsg is design to sign a given message until it success or the same message had been send out by other signer
func (b *Binance) signMsg(signMsg btx.StdSignMsg, from string, poolPubKey common.PubKey, thorchainHeight int64, txOutItem stypes.TxOutItem) ([]byte, error) {
	rawBytes, err := b.sign(signMsg, poolPubKey)
	if err == nil && rawBytes != nil {
		return rawBytes, nil
	}
	var keysignError tss.KeysignError
	if errors.As(err, &keysignError) {
		// don't know which node to blame , so just return
		if len(keysignError.Blame.BlameNodes) == 0 {
			return nil, err
		}
		// fail to sign a message , forward keysign failure to THORChain , so the relevant party can be blamed
		txID, errPostKeysignFail := b.thorchainBridge.PostKeysignFailure(keysignError.Blame, thorchainHeight, txOutItem.Memo, txOutItem.Coins, poolPubKey)
		if errPostKeysignFail != nil {
			b.logger.Error().Err(errPostKeysignFail).Msg("fail to post keysign failure to thorchain")
			return nil, multierror.Append(err, errPostKeysignFail)
		}
		b.logger.Info().Str("tx_id", txID.String()).Msgf("post keysign failure to thorchain")
	}
	b.logger.Error().Err(err).Msgf("fail to sign msg with memo: %s", signMsg.Memo)
	return nil, err
}

func (b *Binance) GetAccount(pkey common.PubKey, height *big.Int) (common.Account, error) {
	if height != nil {
		b.logger.Error().Msg("height was provided but will be ignored")
	}

	addr := b.GetAddress(pkey)
	address, err := types.AccAddressFromBech32(addr)
	if err != nil {
		b.logger.Error().Err(err).Msgf("fail to get parse address: %s", addr)
		return common.Account{}, err
	}
	return b.GetAccountByAddress(address.String(), nil)
}

func (b *Binance) GetAccountByAddress(address string, height *big.Int) (common.Account, error) {
	if height != nil {
		b.logger.Error().Msg("height was provided but will be ignored")
	}

	u, err := url.Parse(b.cfg.RPCHost)
	if err != nil {
		log.Fatal().Msgf("Error parsing rpc (%s): %s", b.cfg.RPCHost, err)
		return common.Account{}, err
	}
	u.Path = "/abci_query"
	v := u.Query()
	v.Set("path", fmt.Sprintf("\"/account/%s\"", address))
	u.RawQuery = v.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return common.Account{}, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			b.logger.Error().Err(err).Msg("fail to close response body")
		}
	}()

	type queryResult struct {
		Jsonrpc string `json:"jsonrpc"`
		ID      string `json:"id"`
		Result  struct {
			Response struct {
				Key         string `json:"key"`
				Value       string `json:"value"`
				BlockHeight string `json:"height"`
			} `json:"response"`
		} `json:"result"`
	}

	var result queryResult
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return common.Account{}, err
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return common.Account{}, err
	}

	data, err := base64.StdEncoding.DecodeString(result.Result.Response.Value)
	if err != nil {
		return common.Account{}, err
	}

	cdc := ttypes.NewCodec()
	var acc types.AppAccount
	err = cdc.UnmarshalBinaryBare(data, &acc)
	if err != nil {
		return common.Account{}, err
	}
	coins, err := common.GetCoins(common.BNBChain, acc.BaseAccount.Coins)
	if err != nil {
		return common.Account{}, err
	}
	account := common.NewAccount(acc.BaseAccount.Sequence, acc.BaseAccount.AccountNumber, coins, acc.Flags > 0)
	return account, nil
}

// BroadcastTx is to broadcast the tx to binance chain
func (b *Binance) BroadcastTx(tx stypes.TxOutItem, hexTx []byte) (string, error) {
	u, err := url.Parse(b.cfg.RPCHost)
	if err != nil {
		log.Error().Msgf("Error parsing rpc (%s): %s", b.cfg.RPCHost, err)
		return "", err
	}
	u.Path = "broadcast_tx_commit"
	values := u.Query()
	values.Set("tx", "0x"+string(hexTx))
	u.RawQuery = values.Encode()
	resp, err := http.Post(u.String(), "", nil)
	if err != nil {
		// Binance daemon sometimes get into trouble and restart , if bifrost happens to broadcast a tx at the time , it will return an err
		// which cause bifrost to retry the same tx , thus double spend.
		// log the error , and then move on , this might cause the node to accumulate some slash points , but at least not double spend and cause bond to be slashed.
		// 300 blocks later, if the tx has not been sent , it will be rescheduled by the network
		b.logger.Err(err).Msg("fail to broadcast tx to binance chain")
		return "", nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// the same reason mentioned above
		b.logger.Err(err).Msg("fail to read response body")
		return "", nil
	}

	// NOTE: we can actually see two different json responses for the same end.
	// This complicates things pretty well.
	// Sample 1: { "height": "0", "txhash": "D97E8A81417E293F5B28DDB53A4AD87B434CA30F51D683DA758ECC2168A7A005", "raw_log": "[{\"msg_index\":0,\"success\":true,\"log\":\"\",\"events\":[{\"type\":\"message\",\"attributes\":[{\"key\":\"action\",\"value\":\"set_observed_txout\"}]}]}]", "logs": [ { "msg_index": 0, "success": true, "log": "", "events": [ { "type": "message", "attributes": [ { "key": "action", "value": "set_observed_txout" } ] } ] } ] }
	// Sample 2: { "height": "0", "txhash": "6A9AA734374D567D1FFA794134A66D3BF614C4EE5DDF334F21A52A47C188A6A2", "code": 4, "raw_log": "{\"codespace\":\"sdk\",\"code\":4,\"message\":\"signature verification failed; verify correct account sequence and chain-id\"}" }
	// Sample 3: {\"jsonrpc\": \"2.0\",\"id\": \"\",\"result\": {  \"check_tx\": {    \"code\": 65541,    \"log\": \"{\\\"codespace\\\":1,\\\"code\\\":5,\\\"abci_code\\\":65541,\\\"message\\\":\\\"insufficient fund. you got 29602BNB,351873676FSN-F1B,1094620960FTM-585,10119750400LOK-3C0,191723639522RUNE-67C,13629773TATIC-E9C,4169469575TCAN-014,10648250188TOMOB-1E1,1155074377TUSDB-000, but 37500BNB fee needed.\\\"}\",    \"events\": [      {}    ]  },  \"deliver_tx\": {},  \"hash\": \"406A3F68B17544F359DF8C94D4E28A626D249BC9C4118B51F7B4CE16D45AF616\",  \"height\": \"0\"}\n}

	b.logger.Info().Str("body", string(body)).Msgf("broadcast response from Binance Chain,memo:%s", tx.Memo)
	var commit stypes.BroadcastResult
	err = b.cdc.UnmarshalJSON(body, &commit)
	if err != nil {
		b.logger.Error().Err(err).Msgf("fail unmarshal commit: %s", string(body))
		return "", fmt.Errorf("fail to unmarshal commit: %w", err)
	}
	// check for any failure logs
	// Error code 4 is used for bad account sequence number. We expect to
	// see this often because in TSS, multiple nodes will broadcast the
	// same sequence number but only one will be successful. We can just
	// drop and ignore in these scenarios. In 1of1 signing, we can also
	// drop and ignore. The reason being, thorchain will attempt to again
	// later.
	checkTx := commit.Result.CheckTx
	if checkTx.Code > 0 && checkTx.Code != cosmos.CodeUnauthorized {
		err := errors.New(checkTx.Log)
		b.logger.Info().Str("body", string(body)).Msg("broadcast response from Binance Chain")
		b.logger.Error().Err(err).Msg("fail to broadcast")
		return "", fmt.Errorf("fail to broadcast: %w", err)
	}

	deliverTx := commit.Result.DeliverTx
	if deliverTx.Code > 0 {
		err := errors.New(deliverTx.Log)
		b.logger.Error().Err(err).Msg("fail to broadcast")
		return "", fmt.Errorf("fail to broadcast: %w", err)
	}

	// increment sequence number
	b.accts.SeqInc(tx.VaultPubKey)
	if err := b.signerCacheManager.SetSigned(tx.CacheHash(), commit.Result.Hash.String()); err != nil {
		b.logger.Err(err).Msg("fail to set signer cache")
	}
	return commit.Result.Hash.String(), nil
}

// ConfirmationCountReady binance chain has almost instant finality , so doesn't need to wait for confirmation
func (b *Binance) ConfirmationCountReady(txIn stypes.TxIn) bool {
	return true
}

// GetConfirmationCount determinate how many confirmation it required
func (b *Binance) GetConfirmationCount(txIn stypes.TxIn) int64 {
	return 0
}

func (b *Binance) ReportSolvency(bnbBlockHeight int64) error {
	if !b.ShouldReportSolvency(bnbBlockHeight) {
		return nil
	}
	// blockchain scanner is catching up , no solvency check messages
	if !b.IsBlockScannerHealthy() && bnbBlockHeight == b.bnbScanner.currentScanningHeight {
		return nil
	}

	asgardVaults, err := b.thorchainBridge.GetAsgards()
	if err != nil {
		return fmt.Errorf("fail to get asgards,err: %w", err)
	}
	for _, asgard := range asgardVaults {
		acct, err := b.GetAccount(asgard.PubKey, nil)
		if err != nil {
			b.logger.Err(err).Msgf("fail to get account balance")
			continue
		}
		if runners.IsVaultSolvent(acct, asgard, cosmos.NewUint(3*b.bnbScanner.singleFee)) && b.IsBlockScannerHealthy() {
			// when vault is solvent , don't need to report solvency
			continue
		}
		select {
		case b.globalSolvencyQueue <- stypes.Solvency{
			Height: bnbBlockHeight,
			Chain:  common.BNBChain,
			PubKey: asgard.PubKey,
			Coins:  acct.Coins,
		}:
		case <-time.After(constants.ThorchainBlockTime):
			b.logger.Info().Msgf("fail to send solvency info to THORChain, timeout")
		}
	}
	b.lastSolvencyCheckHeight = bnbBlockHeight
	return nil
}

func (b *Binance) OnObservedTxIn(txIn stypes.TxInItem, blockHeight int64) {
	m, err := memo.ParseMemo(common.LatestVersion, txIn.Memo)
	if err != nil {
		// Debug log only as ParseMemo error is expected for THORName inbounds.
		b.logger.Debug().Err(err).Msgf("fail to parse memo: %s", txIn.Memo)
		return
	}
	if !m.IsOutbound() {
		return
	}
	if m.GetTxID().IsEmpty() {
		return
	}
	if err := b.signerCacheManager.SetSigned(txIn.CacheHash(b.GetChain(), m.GetTxID().String()), txIn.Tx); err != nil {
		b.logger.Err(err).Msg("fail to update signer cache")
	}
}

// ShouldReportSolvency given block height , should chain client report solvency to THORNode
func (b *Binance) ShouldReportSolvency(height int64) bool {
	return height%900 == 0
}
