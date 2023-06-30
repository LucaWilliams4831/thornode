package evm

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	ecommon "github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"

	evmtypes "gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/evm/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/constants"
)

// This is the number of THORChain blocks to wait before re-broadcasting a stuck tx with
// a higher gas price. 150 was chosen because the signing period for outbounds is 300
// blocks. After 300 blocks the tx will be re-assigned to a different vault, so we want
// to try re-broadcasting before then.
const TxWaitBlocks = 150

// unstuck should be called in a goroutine and runs until the client stop channel is
// closed. It ensures that stuck transactions are re-broadcast with higher gas price
// before being rescheduled to a different vault.
func (c *EVMClient) unstuck() {
	c.logger.Info().Msg("starting unstuck routine")
	defer c.logger.Info().Msg("stopping unstuck routine")
	defer c.wg.Done()

	for {
		select {
		case <-c.stopchan: // exit when stopchan is closed
			return
		case <-time.After(constants.ThorchainBlockTime):
			c.unstuckAction()
		}
	}
}

func (c *EVMClient) unstuckAction() {
	height, err := c.bridge.GetBlockHeight()
	if err != nil {
		c.logger.Err(err).Msg("failed to get THORChain block height")
		return
	}
	signedTxItems, err := c.evmScanner.blockMetaAccessor.GetSignedTxItems()
	if err != nil {
		c.logger.Err(err).Msg("failed to get all signed tx items")
		return
	}
	for _, item := range signedTxItems {
		// this should not possible, but just skip it
		if item.Height > height {
			c.logger.Warn().Msg("signed outbound height greater than current thorchain height")
			continue
		}

		if (height - item.Height) < TxWaitBlocks {
			// not time yet, continue to wait for this tx to commit
			continue
		}
		if err := c.unstuckTx(item.VaultPubKey, item.Hash); err != nil {
			c.logger.Err(err).
				Str("txid", item.Hash).
				Str("vault", item.VaultPubKey).
				Msg("failed to unstuck tx")
			continue
		}

		// remove it
		if err := c.evmScanner.blockMetaAccessor.RemoveSignedTxItem(item.Hash); err != nil {
			c.logger.Err(err).
				Str("txid", item.Hash).
				Str("vault", item.VaultPubKey).
				Msg("failed to remove signed tx item")
		}
	}
}

// unstuckTx will re-broadcast the transaction for the given txid with a higher gas price.
func (c *EVMClient) unstuckTx(vaultPubKey, txid string) error {
	ctx, cancel := c.getTimeoutContext()
	defer cancel()
	tx, pending, err := c.ethClient.TransactionByHash(ctx, ecommon.HexToHash(txid))
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			c.logger.Err(err).Str("txid", txid).Msg("transaction not found on chain")
			return nil
		}
		return fmt.Errorf("fail to get transaction by txid: %s, error: %w", txid, err)
	}

	// the transaction is no longer pending
	if !pending {
		c.logger.Info().Str("txid", txid).Msg("transaction already committed")
		return nil
	}

	pubKey, err := common.NewPubKey(vaultPubKey)
	if err != nil {
		c.logger.Err(err).Str("pubkey", vaultPubKey).Msg("public key is invalid")
		// this should not happen, and if it does there is no point to try again
		return nil
	}
	address, err := pubKey.GetAddress(c.cfg.ChainID)
	if err != nil {
		c.logger.Err(err).Msg("fail to get EVM address")
		return nil
	}

	c.logger.Info().Str("txid", txid).Uint64("nonce", tx.Nonce()).Msg("cancel tx with nonce")

	// double the current suggest gas price
	currentGasRate := big.NewInt(1).Mul(c.GetGasPrice(), big.NewInt(2))

	// inflate the originGasPrice by 10% as per EVM chain, the transaction to cancel an
	// existing tx in the mempool need to pay at least 10% more than the original price,
	// otherwise it will not allow it. the error will be "replacement transaction
	// underpriced" this is the way to get 110% of the original gas price
	originGasPrice := tx.GasPrice()
	inflatedOriginalGasPrice := big.NewInt(1).Div(big.NewInt(1).Mul(tx.GasPrice(), big.NewInt(11)), big.NewInt(10))
	if inflatedOriginalGasPrice.Cmp(currentGasRate) > 0 {
		currentGasRate = big.NewInt(1).Mul(originGasPrice, big.NewInt(2))
	}

	// create the cancel transaction
	canceltx := etypes.NewTransaction(
		tx.Nonce(),
		ecommon.HexToAddress(address.String()),
		big.NewInt(0),
		MaxContractGas,
		currentGasRate,
		nil,
	)
	rawBytes, err := c.kw.Sign(canceltx, pubKey)
	if err != nil {
		return fmt.Errorf("fail to sign tx for cancelling with nonce: %d,err: %w", tx.Nonce(), err)
	}
	broadcastTx := &etypes.Transaction{}
	if err := broadcastTx.UnmarshalJSON(rawBytes); err != nil {
		return fmt.Errorf("fail to unmarshal tx, err: %w", err)
	}

	// broadcast the cancel transaction
	ctx, cancel = c.getTimeoutContext()
	defer cancel()
	err = c.evmScanner.ethClient.SendTransaction(ctx, broadcastTx)
	if !isAcceptableError(err) {
		return fmt.Errorf("fail to broadcast the cancel transaction, hash:%s , err: %w", txid, err)
	}

	c.logger.Info().
		Str("old txid", txid).
		Uint64("nonce", tx.Nonce()).
		Stringer("new txid", broadcastTx.Hash()).
		Msg("broadcast new tx, old tx cancelled")

	return nil
}

// AddSignedTxItem add the transaction to key value store
func (c *EVMClient) AddSignedTxItem(hash string, height int64, vaultPubKey string) error {
	return c.evmScanner.blockMetaAccessor.AddSignedTxItem(evmtypes.SignedTxItem{
		Hash:        hash,
		Height:      height,
		VaultPubKey: vaultPubKey,
	})
}
