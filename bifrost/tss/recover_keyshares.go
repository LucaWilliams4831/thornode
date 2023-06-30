package tss

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/itchio/lzma"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/thornode/app"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/config"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

func RecoverKeyShares(conf config.Bifrost, thorchain thorclient.ThorchainBridge) error {
	tctx := thorchain.GetContext()

	// fetch the node account
	na, err := thorchain.GetNodeAccount(tctx.FromAddress.String())
	if err != nil {
		return fmt.Errorf("fail to get node account: %w", err)
	}

	// skip recovery if the current node is not active
	if na.Status != types.NodeStatus_Active {
		log.Info().Msgf("%s is not active, skipping key shares recovery", na.NodeAddress)
		return nil
	}

	// the current vault is the last pub key in the signer membership list
	membership := na.GetSignerMembership()
	if len(membership) == 0 {
		log.Info().Msgf("no signer membership for %s, skipping key shares recovery", na.NodeAddress)
		return fmt.Errorf("fail to get signer membership")
	}
	vault := membership[len(membership)-1]
	keysharesPath := filepath.Join(app.DefaultNodeHome(), fmt.Sprintf("localstate-%s.json", vault))

	// skip recovery if keyshares for the nodes current vault already exist
	if _, err = os.Stat(keysharesPath); !os.IsNotExist(err) {
		log.Info().Msgf("key shares for %s already exist, skipping recovery", vault)
		return nil
	}

	// get all vaults
	vaults, err := thorchain.GetAsgards()
	if err != nil {
		return fmt.Errorf("fail to get asgards: %w", err)
	}

	// get the creation height of the member vault
	var lastVaultHeight int64
	for _, v := range vaults {
		if v.PubKey.Equals(vault) {
			lastVaultHeight = v.BlockHeight
			break
		}
	}
	if lastVaultHeight == 0 {
		return fmt.Errorf("fail to get creation height of %s", vault)
	}

	// walk backward from the churn height until we find the TssPool message we sent
	var keysharesEncBytes []byte
	dec := tx.DefaultTxDecoder(thorclient.MakeCodec())
	for i := lastVaultHeight; i > lastVaultHeight-conf.TSS.MaxKeyshareRecoverScanBlocks; i-- {
		if i%1000 == 0 {
			log.Info().Msgf("scanning block %d for TssPool message to recover key shares", i)
		}

		b, err := thorchain.GetContext().Client.Block(context.Background(), &i)
		if err != nil {
			return fmt.Errorf("fail to get block: %w", err)
		}

		for _, txb := range b.Block.Txs {
			tx, err := dec(txb)
			if err != nil {
				return fmt.Errorf("fail to decode tx: %w", err)
			}
			for _, msg := range tx.GetMsgs() {
				switch m := msg.(type) {
				case *types.MsgTssPool:
					if m.Signer.Equals(na.NodeAddress) {
						if m.KeysharesBackup == nil {
							log.Warn().Msgf("key shares backup not saved for %s", na.NodeAddress)
						}
						keysharesEncBytes = m.KeysharesBackup
						goto finish
					}
				default:
				}
			}
		}
	}

finish:
	// open key shares file
	f, err := os.OpenFile(keysharesPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open keyshares file: %w", err)
	}
	defer f.Close()

	// decrypt and decompress into place
	decrypted, err := DecryptKeyshares(keysharesEncBytes, os.Getenv("SIGNER_SEED_PHRASE"))
	if err != nil {
		return fmt.Errorf("failed to decrypt key shares: %w", err)
	}
	cmpDec := lzma.NewReader(bytes.NewReader(decrypted))
	if _, err := io.Copy(f, cmpDec); err != nil {
		return fmt.Errorf("failed to decompress key shares: %w", err)
	}

	// success
	log.Info().Str("path", keysharesPath).Msgf("recovered key shares for %s", na.NodeAddress)
	return nil
}
