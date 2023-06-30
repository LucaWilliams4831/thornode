package bitcoincash

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/hashicorp/go-multierror"
	txscript "gitlab.com/thorchain/bifrost/bchd-txscript"

	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/utxo"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	mem "gitlab.com/thorchain/thornode/x/thorchain/memo"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

const (
	// SatsPervBytes it should be enough , this one will only be used if signer can't find any previous UTXO , and fee info from local storage.
	SatsPervBytes = 25
	// MinUTXOConfirmation UTXO that has less confirmation then this will not be spent , unless it is yggdrasil
	MinUTXOConfirmation  = 1
	defaultMaxBCHFeeRate = bchutil.SatoshiPerBitcoin / 10
	maxUTXOsToSpend      = 10
)

func getBCHPrivateKey(key cryptotypes.PrivKey) (*bchec.PrivateKey, error) {
	privateKey, _ := bchec.PrivKeyFromBytes(bchec.S256(), key.Bytes())
	return privateKey, nil
}

func (c *Client) getChainCfg() *chaincfg.Params {
	cn := common.CurrentChainNetwork
	switch cn {
	case common.MockNet:
		return &chaincfg.RegressionNetParams
	case common.TestNet:
		return &chaincfg.TestNet3Params
	case common.MainNet:
		return &chaincfg.MainNetParams
	case common.StageNet:
		return &chaincfg.MainNetParams
	}
	return nil
}

func (c *Client) getGasCoin(tx stypes.TxOutItem, vSize int64) common.Coin {
	gasRate := tx.GasRate
	// if the gas rate is zero , then try to get from last transaction fee
	if gasRate == 0 {
		fee, vBytes, err := c.temporalStorage.GetTransactionFee()
		if err != nil {
			c.logger.Error().Err(err).Msg("fail to get previous transaction fee from local storage")
			return common.NewCoin(common.BCHAsset, cosmos.NewUint(uint64(vSize*gasRate)))
		}
		if fee != 0.0 && vSize != 0 {
			amt, err := bchutil.NewAmount(fee)
			if err != nil {
				c.logger.Err(err).Msg("fail to convert amount from float64 to int64")
			} else {
				gasRate = int64(amt) / int64(vBytes) // sats per vbyte
			}
		}
	}
	// still empty , default to 25
	if gasRate == 0 {
		gasRate = int64(SatsPervBytes)
	}
	return common.NewCoin(common.BCHAsset, cosmos.NewUint(uint64(gasRate*vSize)))
}

// isYggdrasil - when the pubkey and node pubkey is the same that means it is signing from yggdrasil
func (c *Client) isYggdrasil(key common.PubKey) bool {
	return key.Equals(c.nodePubKey)
}

func (c *Client) getMaximumUtxosToSpend() int64 {
	const mimirMaxUTXOsToSpend = `MaxUTXOsToSpend`
	utxosToSpend, err := c.bridge.GetMimir(mimirMaxUTXOsToSpend)
	if err != nil {
		c.logger.Err(err).Msg("fail to get MaxUTXOsToSpend")
	}
	if utxosToSpend <= 0 {
		utxosToSpend = maxUTXOsToSpend
	}
	return utxosToSpend
}

// getAllUtxos go through all the block meta in the local storage, it will spend all UTXOs in  block that might be evicted from local storage soon
// it also try to spend enough UTXOs that can add up to more than the given total
func (c *Client) getUtxoToSpend(pubKey common.PubKey, total float64) ([]btcjson.ListUnspentResult, error) {
	var result []btcjson.ListUnspentResult
	minConfirmation := 0
	utxosToSpend := c.getMaximumUtxosToSpend()
	// Yggdrasil vault is funded by asgard , which will only spend UTXO that is older than 10 blocks, so yggdrasil doesn't need
	// to do the same logic
	isYggdrasil := c.isYggdrasil(pubKey)
	utxos, err := c.getUTXOs(minConfirmation, MaximumConfirmation, pubKey)
	if err != nil {
		return nil, fmt.Errorf("fail to get UTXOs: %w", err)
	}
	// spend UTXO older to younger
	sort.SliceStable(utxos, func(i, j int) bool {
		if utxos[i].Confirmations > utxos[j].Confirmations {
			return true
		} else if utxos[i].Confirmations < utxos[j].Confirmations {
			return false
		}
		return utxos[i].TxID < utxos[j].TxID
	})
	var toSpend float64
	minUTXOAmt := bchutil.Amount(c.chain.DustThreshold().Uint64()).ToBCH()
	for _, item := range utxos {
		if !c.isValidUTXO(item.ScriptPubKey) {
			c.logger.Info().Msgf("invalid UTXO , can't spent it")
			continue
		}
		isSelfTx := c.isSelfTransaction(item.TxID)
		if item.Confirmations == 0 {
			// pending tx that is still  in mempool, only count yggdrasil send to itself or from asgard
			if !c.isSelfTransaction(item.TxID) && !c.isAsgardAddress(item.Address) {
				continue
			}
		}
		// when the utxo is signed yggdrasil / asgard , even amount is less than DustThreshold
		// it is ok to spend it
		if item.Amount < minUTXOAmt && !isSelfTx && !isYggdrasil {
			continue
		}
		if isYggdrasil || item.Confirmations >= MinUTXOConfirmation || isSelfTx {
			result = append(result, item)
			toSpend += item.Amount
		}
		// in the scenario that there are too many unspent utxos available, make sure it doesn't spend too much
		// as too much UTXO will cause huge pressure on TSS, also make sure it will spend at least maxUTXOsToSpend
		// so the UTXOs will be consolidated
		if int64(len(result)) >= utxosToSpend && toSpend >= total {
			break
		}
	}
	return result, nil
}

// isSelfTransaction check the block meta to see whether the transactions is broadcast by ourselves
// if the transaction is broadcast by ourselves, then we should be able to spend the UTXO even it is still in mempool
// as such we could daisy chain the outbound transaction
func (c *Client) isSelfTransaction(txID string) bool {
	bms, err := c.temporalStorage.GetBlockMetas()
	if err != nil {
		c.logger.Err(err).Msg("fail to get block metas")
		return false
	}
	for _, item := range bms {
		for _, tx := range item.SelfTransactions {
			if strings.EqualFold(tx, txID) {
				c.logger.Debug().Msgf("%s is self transaction", txID)
				return true
			}
		}
	}
	return false
}

func (c *Client) getBlockHeight() (int64, error) {
	hash, err := c.client.GetBestBlockHash()
	if err != nil {
		return 0, fmt.Errorf("fail to get best block hash: %w", err)
	}
	blockInfo, err := c.client.GetBlockVerbose(hash, false)
	if err != nil {
		return 0, fmt.Errorf("fail to get the best block detail: %w", err)
	}

	return blockInfo.Height, nil
}

func (c *Client) getBCHPaymentAmount(tx stypes.TxOutItem) float64 {
	amtToPay := tx.Coins.GetCoin(common.BCHAsset).Amount.Uint64()
	amtToPayInBCH := bchutil.Amount(int64(amtToPay)).ToBCH()
	if !tx.MaxGas.IsEmpty() {
		gasAmt := tx.MaxGas.ToCoins().GetCoin(common.BCHAsset).Amount
		amtToPayInBCH += bchutil.Amount(int64(gasAmt.Uint64())).ToBCH()
	}
	return amtToPayInBCH
}

// getSourceScript retrieve pay to addr script from tx source
func (c *Client) getSourceScript(tx stypes.TxOutItem) ([]byte, error) {
	sourceAddr, err := tx.VaultPubKey.GetAddress(common.BCHChain)
	if err != nil {
		return nil, fmt.Errorf("fail to get source address: %w", err)
	}

	addr, err := bchutil.DecodeAddress(sourceAddr.String(), c.getChainCfg())
	if err != nil {
		return nil, fmt.Errorf("fail to decode source address(%s): %w", sourceAddr.String(), err)
	}
	return txscript.PayToAddrScript(addr)
}

// estimateTxSize will create a temporary MsgTx, and use it to estimate the final tx size
// the value in the temporary MsgTx is not real
// https://bitcoinops.org/en/tools/calc-size/
func (c *Client) estimateTxSize(memo string, txes []btcjson.ListUnspentResult) int64 {
	// overhead - 10
	// Per input - 148
	// Per output - 34 , we might have 1 / 2 output , depends on the circumstances , here we only count 1  output , would rather underestimate
	// so we won't hit absurd hight fee issue
	// overhead for NULL DATA - 9 , len(memo) is the size of memo
	return int64(10 + 148*len(txes) + 34 + 9 + len([]byte(memo)))
}

func (c *Client) buildTx(tx stypes.TxOutItem, sourceScript []byte) (*wire.MsgTx, map[string]int64, error) {
	txes, err := c.getUtxoToSpend(tx.VaultPubKey, c.getBCHPaymentAmount(tx))
	if err != nil {
		return nil, nil, fmt.Errorf("fail to get unspent UTXO")
	}
	redeemTx := wire.NewMsgTx(wire.TxVersion)
	totalAmt := float64(0)
	individualAmounts := make(map[string]int64, len(txes))
	for _, item := range txes {
		txID, err := chainhash.NewHashFromStr(item.TxID)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to parse txID(%s): %w", item.TxID, err)
		}
		// double check that the utxo is still valid
		outputPoint := wire.NewOutPoint(txID, item.Vout)
		sourceTxIn := wire.NewTxIn(outputPoint, nil)
		redeemTx.AddTxIn(sourceTxIn)
		totalAmt += item.Amount
		amt, err := bchutil.NewAmount(item.Amount)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to parse amount(%f): %w", item.Amount, err)
		}
		individualAmounts[fmt.Sprintf("%s-%d", txID, item.Vout)] = int64(amt)
	}

	outputAddr, err := bchutil.DecodeAddress(tx.ToAddress.String(), c.getChainCfg())
	if err != nil {
		return nil, nil, fmt.Errorf("fail to decode next address: %w", err)
	}
	buf, err := txscript.PayToAddrScript(outputAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to get pay to address script: %w", err)
	}

	total, err := bchutil.NewAmount(totalAmt)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to parse total amount(%f),err: %w", totalAmt, err)
	}
	coinToCustomer := tx.Coins.GetCoin(common.BCHAsset)
	totalSize := c.estimateTxSize(tx.Memo, txes)

	// bitcoind has a default rule max fee rate should less than 0.1 BCH / kb
	// the MaxGas coming from THORChain doesn't follow this rule , thus the MaxGas might be over the limit
	// as such , signer need to double check, if the MaxGas is over the limit , just pay the limit
	// the rest paid to customer to make sure the total doesn't change

	// maxFee in sats
	maxFeeSats := totalSize * defaultMaxBCHFeeRate / 1024
	gasCoin := c.getGasCoin(tx, totalSize)
	gasAmtSats := gasCoin.Amount.Uint64()

	// make sure the transaction fee is not more than 0.1 BCH / kb , otherwise it might reject the transaction
	if gasAmtSats > uint64(maxFeeSats) {
		diffSats := gasAmtSats - uint64(maxFeeSats) // in sats
		c.logger.Info().Msgf("gas amount: %d is larger than maximum fee: %d , diff: %d", gasAmtSats, uint64(maxFeeSats), diffSats)
		gasAmtSats = uint64(maxFeeSats)
	} else if gasAmtSats < c.minRelayFeeSats {
		c.logger.Info().Msgf("gas amount: %d is less than min relay fee: %d,use min relay fee instead", gasAmtSats, c.minRelayFeeSats)
		gasAmtSats = c.minRelayFeeSats
	}

	if !tx.MaxGas.IsEmpty() {
		maxGasCoin := tx.MaxGas.ToCoins().GetCoin(common.BCHAsset)
		if gasAmtSats > maxGasCoin.Amount.Uint64() {
			c.logger.Info().Msgf("max gas: %s, however estimated gas need %d", tx.MaxGas, gasAmtSats)
			gasAmtSats = maxGasCoin.Amount.Uint64()
		} else if gasAmtSats < maxGasCoin.Amount.Uint64() {
			// if the tx spend less gas then the estimated MaxGas , then the extra can be added to the coinToCustomer
			gap := maxGasCoin.Amount.Uint64() - gasAmtSats
			c.logger.Info().Msgf("max gas is: %s, however only: %d is required, gap: %d goes to customer", tx.MaxGas, gasAmtSats, gap)
			coinToCustomer.Amount = coinToCustomer.Amount.Add(cosmos.NewUint(gap))
		}
	} else {
		memo, err := mem.ParseMemo(common.LatestVersion, tx.Memo)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to parse memo: %w", err)
		}
		if memo.GetType() == mem.TxYggdrasilReturn || memo.GetType() == mem.TxConsolidate {
			gap := gasAmtSats
			c.logger.Info().Msgf("yggdrasil return asset or consolidate tx, need gas: %d", gap)
			coinToCustomer.Amount = common.SafeSub(coinToCustomer.Amount, cosmos.NewUint(gap))
		}
	}
	gasAmt := bchutil.Amount(gasAmtSats)
	if err := c.temporalStorage.UpsertTransactionFee(gasAmt.ToBCH(), int32(totalSize)); err != nil {
		c.logger.Err(err).Msg("fail to save gas info to UTXO storage")
	}

	// pay to customer
	redeemTxOut := wire.NewTxOut(int64(coinToCustomer.Amount.Uint64()), buf)
	redeemTx.AddTxOut(redeemTxOut)

	// balance to ourselves
	// add output to pay the balance back ourselves
	balance := int64(total) - redeemTxOut.Value - int64(gasAmt)

	c.logger.Info().Msgf("total: %d, to customer: %d, gas: %d", int64(total), redeemTxOut.Value, int64(gasAmt))
	if balance < 0 {
		return nil, nil, fmt.Errorf("not enough balance to pay customer: %d", balance)
	}

	// if the balance is dust , then just donate it to miners
	if balance > 0 {
		c.logger.Info().Msgf("send %d back to self", balance)
		redeemTx.AddTxOut(wire.NewTxOut(balance, sourceScript))
	}

	// memo
	if len(tx.Memo) != 0 {
		nullDataScript, err := txscript.NullDataScript([]byte(tx.Memo))
		if err != nil {
			return nil, nil, fmt.Errorf("fail to generate null data script: %w", err)
		}
		redeemTx.AddTxOut(wire.NewTxOut(0, nullDataScript))
	}

	return redeemTx, individualAmounts, nil
}

// SignTx builds and signs the outbound transaction. Returns the signed transaction, a
// serialized checkpoint on error, and an error.
func (c *Client) SignTx(tx stypes.TxOutItem, thorchainHeight int64) ([]byte, []byte, *stypes.TxInItem, error) {
	if !tx.Chain.Equals(common.BCHChain) {
		return nil, nil, nil, errors.New("not BCH chain")
	}

	// skip outbounds without coins
	if tx.Coins.IsEmpty() {
		return nil, nil, nil, nil
	}

	if !tx.ToAddress.IsValidBCHAddress() {
		c.logger.Error().Msgf("to address: %s is legacy not allowed ", tx.ToAddress)
		return nil, nil, nil, nil
	}

	// skip outbounds that have been signed
	if c.signerCacheManager.HasSigned(tx.CacheHash()) {
		c.logger.Info().Msgf("transaction(%+v), signed before , ignore", tx)
		return nil, nil, nil, nil
	}

	// only one keysign per chain at a time
	vaultSignerLock := c.getVaultSignerLock(tx.VaultPubKey.String())
	if vaultSignerLock == nil {
		c.logger.Error().Msgf("fail to get signer lock for vault pub key: %s", tx.VaultPubKey.String())
		return nil, nil, nil, fmt.Errorf("fail to get signer lock")
	}
	vaultSignerLock.Lock()
	defer vaultSignerLock.Unlock()

	sourceScript, err := c.getSourceScript(tx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fail to get source pay to address script: %w", err)
	}

	// verify output address
	outputAddr, err := bchutil.DecodeAddress(tx.ToAddress.String(), c.getChainCfg())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fail to decode next address: %w", err)
	}
	if !strings.EqualFold(outputAddr.String(), tx.ToAddress.String()) {
		c.logger.Info().Msgf("output address: %s, to address: %s can't roundtrip", outputAddr.String(), tx.ToAddress.String())
		return nil, nil, nil, nil
	}
	switch outputAddr.(type) {
	case *bchutil.AddressPubKey:
		c.logger.Info().Msgf("address: %s is address pubkey type, should not be used", outputAddr)
		return nil, nil, nil, nil
	default: // keep lint happy
	}

	// load from checkpoint if it exists
	checkpoint := utxo.SignCheckpoint{}
	redeemTx := &wire.MsgTx{}
	if tx.Checkpoint != nil {
		if err := json.Unmarshal(tx.Checkpoint, &checkpoint); err != nil {
			return nil, nil, nil, fmt.Errorf("fail to unmarshal checkpoint: %w", err)
		}
		if err := redeemTx.Deserialize(bytes.NewReader(checkpoint.UnsignedTx)); err != nil {
			return nil, nil, nil, fmt.Errorf("fail to deserialize tx: %w", err)
		}
	} else {
		redeemTx, checkpoint.IndividualAmounts, err = c.buildTx(tx, sourceScript)
		if err != nil {
			return nil, nil, nil, err
		}
		buf := bytes.NewBuffer([]byte{})
		err = redeemTx.Serialize(buf)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("fail to serialize tx: %w", err)
		}
		checkpoint.UnsignedTx = buf.Bytes()
	}

	// serialize the checkpoint for later
	checkpointBytes, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fail to marshal checkpoint: %w", err)
	}

	wg := &sync.WaitGroup{}
	var utxoErr error
	c.logger.Info().Msgf("UTXOs to sign: %d", len(redeemTx.TxIn))

	totalAmount := int64(0)
	for idx, txIn := range redeemTx.TxIn {
		key := fmt.Sprintf("%s-%d", txIn.PreviousOutPoint.Hash, txIn.PreviousOutPoint.Index)
		outputAmount := checkpoint.IndividualAmounts[key]
		totalAmount += outputAmount
		wg.Add(1)
		go func(i int, amount int64) {
			defer wg.Done()
			if err := c.signUTXO(redeemTx, tx, amount, sourceScript, i, thorchainHeight); err != nil {
				if nil == utxoErr {
					utxoErr = err
				} else {
					utxoErr = multierror.Append(utxoErr, err)
				}
			}
		}(idx, outputAmount)
	}
	wg.Wait()
	if utxoErr != nil {
		err = utxo.PostKeysignFailure(c.bridge, tx, c.logger, thorchainHeight, utxoErr)
		return nil, checkpointBytes, nil, fmt.Errorf("fail to sign the message: %w", err)
	}
	finalSize := redeemTx.SerializeSize()
	c.logger.Info().Msgf("final size: %d", finalSize)
	var signedTx bytes.Buffer
	if err := redeemTx.Serialize(&signedTx); err != nil {
		return nil, nil, nil, fmt.Errorf("fail to serialize tx to bytes: %w", err)
	}

	// create the observation to be sent by the signer before broadcast
	chainHeight, err := c.getBlockHeight()
	if err != nil { // fall back to the scanner height, thornode voter does not use height
		chainHeight = c.currentBlockHeight.Load()
	}
	amt := redeemTx.TxOut[0].Value // the first output is the outbound amount
	gas := totalAmount
	for _, txOut := range redeemTx.TxOut { // subtract all vouts to from vins to get the gas
		gas -= txOut.Value
	}
	var txIn *stypes.TxInItem
	sender, err := tx.VaultPubKey.GetAddress(tx.Chain)
	if err == nil {
		txIn = stypes.NewTxInItem(
			chainHeight+1,
			redeemTx.TxHash().String(),
			tx.Memo,
			sender.String(),
			tx.ToAddress.String(),
			common.NewCoins(
				common.NewCoin(c.chain.GetGasAsset(), cosmos.NewUint(uint64(amt))),
			),
			common.Gas(common.NewCoins(
				common.NewCoin(c.chain.GetGasAsset(), cosmos.NewUint(uint64(gas))),
			)),
			tx.VaultPubKey,
			"",
			"",
			nil,
		)
	}

	return signedTx.Bytes(), nil, txIn, nil
}

func (c *Client) signUTXO(redeemTx *wire.MsgTx, tx stypes.TxOutItem, amount int64, sourceScript []byte, idx int, thorchainHeight int64) error {
	signable := c.ksWrapper.GetSignable(tx.VaultPubKey)
	sigHashType := txscript.SigHashAll
	sig, err := txscript.RawTxInECDSASignature(redeemTx, idx, sourceScript, sigHashType, signable, amount)
	if err != nil {
		return fmt.Errorf("fail to get witness: %w", err)
	}

	pkData := signable.GetPubKey().SerializeCompressed()
	sigscript, err := txscript.NewScriptBuilder().AddData(sig).AddData(pkData).Script()
	if err != nil {
		return fmt.Errorf("fail to build signature script: %w", err)
	}
	redeemTx.TxIn[idx].SignatureScript = sigscript
	flag := txscript.StandardVerifyFlags
	engine, err := txscript.NewEngine(sourceScript, redeemTx, idx, flag, nil, nil, amount)
	if err != nil {
		return fmt.Errorf("fail to create engine: %w", err)
	}
	if err := engine.Execute(); err != nil {
		return fmt.Errorf("fail to execute the script: %w", err)
	}
	return nil
}

// BroadcastTx will broadcast the given payload to BCH chain
func (c *Client) BroadcastTx(txOut stypes.TxOutItem, payload []byte) (string, error) {
	redeemTx := wire.NewMsgTx(wire.TxVersion)
	buf := bytes.NewBuffer(payload)
	if err := redeemTx.Deserialize(buf); err != nil {
		return "", fmt.Errorf("fail to deserialize payload: %w", err)
	}

	height, err := c.getBlockHeight()
	if err != nil {
		return "", fmt.Errorf("fail to get block height: %w", err)
	}
	bm, err := c.temporalStorage.GetBlockMeta(height)
	if err != nil {
		c.logger.Err(err).Msgf("fail to get blockmeta for heigth: %d", height)
	}
	if bm == nil {
		bm = utxo.NewBlockMeta("", height, "")
	}
	defer func() {
		if err := c.temporalStorage.SaveBlockMeta(height, bm); err != nil {
			c.logger.Err(err).Msg("fail to save block metadata")
		}
	}()
	// broadcast tx
	txHash, err := c.client.SendRawTransaction(redeemTx, true)
	if txHash != nil {
		bm.AddSelfTransaction(txHash.String())
	}
	if err != nil {
		if rpcErr, ok := err.(*btcjson.RPCError); ok && rpcErr.Code == btcjson.ErrRPCTxAlreadyInChain {
			// this means the tx had been broadcast to chain, it must be another signer finished quicker then us
			// save tx id to block meta in case we need to errata later
			c.logger.Info().Str("hash", redeemTx.TxHash().String()).Msg("broadcast to BCH chain by another node")
			if err := c.signerCacheManager.SetSigned(txOut.CacheHash(), redeemTx.TxHash().String()); err != nil {
				c.logger.Err(err).Msgf("fail to mark tx out item (%+v) as signed", txOut)
			}
			return redeemTx.TxHash().String(), nil
		}

		return "", fmt.Errorf("fail to broadcast transaction to chain: %w", err)
	}
	// save tx id to block meta in case we need to errata later
	c.logger.Info().Str("hash", txHash.String()).Msg("broadcast to BCH chain successfully")
	if err := c.signerCacheManager.SetSigned(txOut.CacheHash(), txHash.String()); err != nil {
		c.logger.Err(err).Msgf("fail to mark tx out item (%+v) as signed", txOut)
	}
	return txHash.String(), nil
}

// consolidateUTXOs only required when there is a new block
func (c *Client) consolidateUTXOs() {
	defer func() {
		c.wg.Done()
		c.consolidateInProgress.Store(false)
	}()

	nodeStatus, err := c.bridge.FetchNodeStatus()
	if err != nil {
		c.logger.Err(err).Msg("fail to get node status")
		return
	}
	if nodeStatus != types.NodeStatus_Active {
		c.logger.Info().Msgf("node is not active , doesn't need to consolidate utxos")
		return
	}
	vaults, err := c.bridge.GetAsgards()
	if err != nil {
		c.logger.Err(err).Msg("fail to get current asgards")
		return
	}
	utxosTospend := c.getMaximumUtxosToSpend()
	for _, vault := range vaults {
		if !vault.Contains(c.nodePubKey) {
			// Not part of this vault , don't need to consolidate UTXOs for this Vault
			continue
		}
		// the amount used here doesn't matter , just to see whether there are more than 15 UTXO available or not
		utxos, err := c.getUtxoToSpend(vault.PubKey, 0.01)
		if err != nil {
			c.logger.Err(err).Msg("fail to get utxos to spend")
			continue
		}
		// doesn't have enough UTXOs , don't need to consolidate
		if int64(len(utxos)) < utxosTospend {
			continue
		}
		total := 0.0
		for _, item := range utxos {
			total += item.Amount
		}
		addr, err := vault.PubKey.GetAddress(common.BCHChain)
		if err != nil {
			c.logger.Err(err).Msgf("fail to get BCH address for pubkey:%s", vault.PubKey)
			continue
		}
		// THORChain usually pay 1.5 of the last observed fee rate
		feeRate := math.Ceil(float64(c.lastFeeRate) * 3 / 2)
		amt, err := bchutil.NewAmount(total)
		if err != nil {
			c.logger.Err(err).Msgf("fail to convert to BTC amount: %f", total)
			continue
		}

		txOutItem := stypes.TxOutItem{
			Chain:       common.BCHChain,
			ToAddress:   addr,
			VaultPubKey: vault.PubKey,
			Coins: common.Coins{
				common.NewCoin(common.BCHAsset, cosmos.NewUint(uint64(amt))),
			},
			Memo:    "consolidate",
			MaxGas:  nil,
			GasRate: int64(feeRate),
		}
		height, err := c.bridge.GetBlockHeight()
		if err != nil {
			c.logger.Err(err).Msg("fail to get THORChain block height")
			continue
		}
		rawTx, _, _, err := c.SignTx(txOutItem, height)
		if err != nil {
			c.logger.Err(err).Msg("fail to sign consolidate txout item")
			continue
		}
		txID, err := c.BroadcastTx(txOutItem, rawTx)
		if err != nil {
			c.logger.Err(err).Msg("fail to broadcast consolidate tx")
			continue
		}
		c.logger.Info().Msgf("broadcast consolidate tx successfully,hash:%s", txID)
	}
}
