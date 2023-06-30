package ethereum

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	ecommon "github.com/ethereum/go-ethereum/common"
	ecore "github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"
	etypes "github.com/ethereum/go-ethereum/core/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/constants"
)

type SignedTxItem struct {
	Hash        string `json:"hash,omitempty"`
	Height      int64  `json:"height,omitempty"`
	VaultPubKey string `json:"vault_pub_key,omitempty"`
}

const TxWaitBlocks = 150

// String implement fmt.Stringer
func (st SignedTxItem) String() string {
	return st.Hash
}

func (c *Client) unstuck() {
	c.logger.Info().Msg("start ETH chain unstuck process")
	defer c.logger.Info().Msg("stop ETH chain unstock process")
	defer c.wg.Done()
	for {
		select {
		case <-c.stopchan:
			// time to exit
			return
		case <-time.After(constants.ThorchainBlockTime):
			c.unstuckAction()
		}
	}
}

func (c *Client) unstuckAction() {
	height, err := c.bridge.GetBlockHeight()
	if err != nil {
		c.logger.Err(err).Msg("fail to get THORChain block height")
		return
	}
	signedTxItems, err := c.ethScanner.blockMetaAccessor.GetSignedTxItems()
	if err != nil {
		c.logger.Err(err).Msg("fail to get all signed tx items")
		return
	}
	for _, item := range signedTxItems {
		// this should not possible , but just skip it
		if item.Height > height {
			continue
		}

		if (height - item.Height) < TxWaitBlocks {
			// not time yet , continue to wait for this tx to commit
			continue
		}
		if err := c.unstuckTx(item.VaultPubKey, item.Hash); err != nil {
			c.logger.Err(err).Msgf("fail to unstuck tx with hash:%s vaultPubKey:%s", item.Hash, item.VaultPubKey)
			continue
		}
		// remove it
		if err := c.ethScanner.blockMetaAccessor.RemoveSignedTxItem(item.Hash); err != nil {
			c.logger.Err(err).Msgf("fail to remove signed tx item with hash:%s vaultPubKey:%s", item.Hash, item.VaultPubKey)
		}
	}
}

// unstuckTx is the method used to unstuck ETH address
// when unstuckTx return an err , then the same hash should retry otherwise it can be removed
func (c *Client) unstuckTx(vaultPubKey, hash string) error {
	ctx, cancel := c.getContext()
	defer cancel()
	tx, pending, err := c.client.TransactionByHash(ctx, ecommon.HexToHash(hash))
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			c.logger.Err(err).Msgf("Transaction with hash: %s doesn't exist on ETH chain anymore", hash)
			return nil
		}
		return fmt.Errorf("fail to get transaction by hash:%s, error:: %w", hash, err)
	}
	// the transaction is not pending any more
	if !pending {
		c.logger.Info().Msgf("transaction with hash %s already committed on block , don't need to unstuck, remove it", hash)
		return nil
	}

	pubKey, err := common.NewPubKey(vaultPubKey)
	if err != nil {
		c.logger.Err(err).Msgf("public key: %s is invalid", vaultPubKey)
		// this should not happen , and if it does , there is no point to try it again , just remove it
		return nil
	}
	address, err := pubKey.GetAddress(common.ETHChain)
	if err != nil {
		c.logger.Err(err).Msgf("fail to get ETH address")
		return nil
	}

	c.logger.Info().Msgf("cancel tx hash: %s, nonce: %d", hash, tx.Nonce())
	// double the current suggest gas price
	currentGasRate := big.NewInt(1).Mul(c.GetGasPrice(), big.NewInt(2))
	// inflate the originGasPrice by 10% as per ETH chain , the transaction to cancel an existing tx in the mempool
	// need to pay at least 10% more than the original price , otherwise it will not allow it.
	// the error will be "replacement transaction underpriced"
	// this is the way how to get 110% of the original gas price
	originGasPrice := tx.GasPrice()
	inflatedOriginalGasPrice := big.NewInt(1).Div(big.NewInt(1).Mul(tx.GasPrice(), big.NewInt(11)), big.NewInt(10))
	if inflatedOriginalGasPrice.Cmp(currentGasRate) > 0 {
		currentGasRate = big.NewInt(1).Mul(originGasPrice, big.NewInt(2))
	}
	canceltx := etypes.NewTransaction(tx.Nonce(), ecommon.HexToAddress(address.String()), big.NewInt(0), MaxContractGas, currentGasRate, nil)
	rawBytes, err := c.kw.Sign(canceltx, pubKey)
	if err != nil {
		return fmt.Errorf("fail to sign tx for cancelling with nonce: %d,err: %w", tx.Nonce(), err)
	}
	broadcastTx := &etypes.Transaction{}
	if err := broadcastTx.UnmarshalJSON(rawBytes); err != nil {
		return fmt.Errorf("fail to unmarshal tx, err: %w", err)
	}
	ctx, cancel = c.getContext()
	defer cancel()
	if err := c.client.SendTransaction(ctx, broadcastTx); err != nil && err.Error() != txpool.ErrAlreadyKnown.Error() && err.Error() != ecore.ErrNonceTooLow.Error() {
		return fmt.Errorf("fail to broadcast the cancel transaction,hash:%s , err: %w", hash, err)
	}
	c.logger.Info().Msgf("broadcast cancel transaction , tx hash: %s, nonce: %d , new tx hash:%s", hash, tx.Nonce(), broadcastTx.Hash().String())
	return nil
}

// AddSignedTxItem add the transaction to key value store
func (c *Client) AddSignedTxItem(hash string, height int64, vaultPubKey string) error {
	return c.ethScanner.blockMetaAccessor.AddSignedTxItem(SignedTxItem{
		Hash:        hash,
		Height:      height,
		VaultPubKey: vaultPubKey,
	})
}
