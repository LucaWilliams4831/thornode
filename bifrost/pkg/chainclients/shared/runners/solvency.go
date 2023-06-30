package runners

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

// SolvencyCheckProvider methods that a SolvencyChecker implementation should have
type SolvencyCheckProvider interface {
	GetHeight() (int64, error)
	ShouldReportSolvency(height int64) bool
	ReportSolvency(height int64) error
}

// SolvencyCheckRunner when a chain get marked as insolvent , and then get halt automatically , the chain client will stop scanning blocks , as a result , solvency checker will
// not report current solvency status to THORNode anymore, this method is to ensure that the chain client will continue to do solvency check even when the chain has been halted
func SolvencyCheckRunner(chain common.Chain,
	provider SolvencyCheckProvider,
	bridge thorclient.ThorchainBridge,
	stopper <-chan struct{},
	wg *sync.WaitGroup,
	backOffDuration time.Duration,
) {
	logger := log.Logger.With().Str("chain", chain.String()).Logger()
	logger.Info().Msg("start solvency check runner")
	defer func() {
		wg.Done()
		logger.Info().Msg("finish  solvency check runner")
	}()
	if provider == nil {
		logger.Error().Msg("solvency checker provider is nil")
		return
	}
	if backOffDuration == 0 {
		backOffDuration = constants.ThorchainBlockTime
	}
	for {
		select {
		case <-stopper:
			return
		case <-time.After(backOffDuration):
			// check whether the chain is halted via mimir or not
			haltHeight, err := bridge.GetMimir(fmt.Sprintf("Halt%sChain", chain))
			if err != nil {
				logger.Err(err).Msg("fail to get chain halt height")
				continue
			}

			// check whether the chain is halted via solvency check
			solvencyHaltHeight, err := bridge.GetMimir(fmt.Sprintf("SolvencyHalt%sChain", chain))
			if err != nil {
				logger.Err(err).Msg("fail to get solvency halt height")
				continue
			}

			// when HaltHeight == 1 means admin halt the chain, no need to do solvency check
			// when Chain is not halted, the normal chain client will report solvency when it need to
			// But if SolvencyHalt<chain>Chain > 0 this means the chain is halted, and we need to report solvency here
			if haltHeight <= 1 && solvencyHaltHeight <= 0 {
				continue
			}

			currentBlockHeight, err := provider.GetHeight()
			if err != nil {
				logger.Err(err).Msg("fail to get current block height")
				break
			}
			if provider.ShouldReportSolvency(currentBlockHeight) {
				logger.Info().Msgf("current block height: %d, report solvency again", currentBlockHeight)
				if err := provider.ReportSolvency(currentBlockHeight); err != nil {
					logger.Err(err).Msg("fail to report solvency")
				}
			}
		}
	}
}

// IsVaultSolvent check whether the given vault is solvent or not , if it is not solvent , then it will need to report solvency to thornode
func IsVaultSolvent(account common.Account, vault types.Vault, currentGasFee cosmos.Uint) bool {
	logger := log.Logger
	for _, c := range account.Coins {
		asgardCoin := vault.GetCoin(c.Asset)

		// ETH.RUNE will be burned on the way in , so the wallet will not have any, thus exclude it from solvency check
		// (Even though also ignored later in handler_solvency.go, avoid chain bloat and disagreement-slash-points
		// for solvent situations.  Note that BNB.RUNE is not burnt on switching the way ETH.RUNE is.)
		if c.Asset.Equals(common.ERC20RuneAsset()) {
			continue
		}

		// when wallet has more coins or equal exactly as asgard , then the vault is solvent
		if c.Amount.GTE(asgardCoin.Amount) {
			continue
		}

		gap := asgardCoin.Amount.Sub(c.Amount)
		// thornode allow 10x of MaxGas as the gap
		if c.Asset.IsGasAsset() && gap.LT(currentGasFee.MulUint64(10)) {
			continue
		}
		logger.Info().
			Str("asset", c.Asset.String()).
			Str("asgard amount", asgardCoin.Amount.String()).
			Str("wallet amount", c.Amount.String()).
			Str("gap", gap.String()).
			Msg("insolvency detected")
		return false
	}
	return true
}
