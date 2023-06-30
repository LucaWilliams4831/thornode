package thorchain

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// SolvencyHandler is to process MsgSolvency message from bifrost
// Bifrost constantly monitor the account balance , and report to THORNode
// If it detect that wallet is short of fund , much less than vault, the network should automatically halt trading
type SolvencyHandler struct {
	mgr Manager
}

// NewSolvencyHandler create a new instance of solvency handler
func NewSolvencyHandler(mgr Manager) SolvencyHandler {
	return SolvencyHandler{
		mgr: mgr,
	}
}

// Run is the main entry point to process MsgSolvency
func (h SolvencyHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSolvency)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg solvency failed validation", "error", err)
		return nil, err
	}
	return h.handle(ctx, *msg)
}

func (h SolvencyHandler) validate(ctx cosmos.Context, msg MsgSolvency) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.70.0")) {
		return h.validateV70(ctx, msg)
	}
	return errBadVersion
}

func (h SolvencyHandler) validateV70(ctx cosmos.Context, msg MsgSolvency) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	m, err := NewMsgSolvency(msg.Chain, msg.PubKey, msg.Coins, msg.Height, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to reconstruct msg solvency", "error", err)
		return err
	}
	if !m.Id.Equals(msg.Id) {
		return cosmos.ErrUnknownRequest("invalid solvency message")
	}
	if !isSignedByActiveNodeAccounts(ctx, h.mgr.Keeper(), msg.GetSigners()) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%+v are not authorized", msg.GetSigners()))
	}
	return nil
}

func (h SolvencyHandler) handle(ctx cosmos.Context, msg MsgSolvency) (*cosmos.Result, error) {
	ctx.Logger().Debug("handle Solvency request", "id", msg.Id.String(), "signer", msg.Signer.String())
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.110.0")):
		return h.handleV110(ctx, msg)
	case version.GTE(semver.MustParse("1.87.0")):
		return h.handleV87(ctx, msg)
	case version.GTE(semver.MustParse("0.79.0")):
		return h.handleV79(ctx, msg)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return nil, errBadVersion
}

// handleCurrent is the logic to process MsgSolvency, the feature works like this
//  1. Bifrost report MsgSolvency to thornode , which is the balance of asgard wallet on each individual chain
//  2. once MsgSolvency reach consensus , then the network compare the wallet balance against wallet
//     if wallet has less fund than asgard vault , and the gap is more than 1% , then the chain
//     that is insolvent will be halt
//  3. When chain is halt , bifrost will not observe inbound , and will not sign outbound txs until the issue has been investigated , and enabled it again using mimir
func (h SolvencyHandler) handleV110(ctx cosmos.Context, msg MsgSolvency) (*cosmos.Result, error) {
	voter, err := h.mgr.Keeper().GetSolvencyVoter(ctx, msg.Id, msg.Chain)
	if err != nil {
		return &cosmos.Result{}, fmt.Errorf("fail to get solvency voter, err: %w", err)
	}
	observeSlashPoints := h.mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := h.mgr.GetConstants().GetInt64Value(constants.ObservationDelayFlexibility)

	slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
		telemetry.NewLabel("reason", "failed_observe_solvency"),
		telemetry.NewLabel("chain", string(msg.Chain)),
	}))
	h.mgr.Slasher().IncSlashPoints(slashCtx, observeSlashPoints, msg.Signer)

	if voter.Empty() {
		voter = NewSolvencyVoter(msg.Id, msg.Chain, msg.PubKey, msg.Coins, msg.Height, msg.Signer)
	} else if !voter.Sign(msg.Signer) {
		ctx.Logger().Info("signer already signed MsgSolvency", "signer", msg.Signer.String(), "id", msg.Id)
		return &cosmos.Result{}, nil
	}
	h.mgr.Keeper().SetSolvencyVoter(ctx, voter)
	active, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}
	if !voter.HasConsensus(active) {
		return &cosmos.Result{}, nil
	}

	// from this point , solvency reach consensus
	if voter.ConsensusBlockHeight > 0 {
		if (voter.ConsensusBlockHeight + observeFlex) >= ctx.BlockHeight() {
			h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, msg.Signer)
		}
		// solvency tx already processed
		return &cosmos.Result{}, nil
	}
	voter.ConsensusBlockHeight = ctx.BlockHeight()
	h.mgr.Keeper().SetSolvencyVoter(ctx, voter)
	// decrease the slash points
	h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, voter.GetSigners()...)
	vault, err := h.mgr.Keeper().GetVault(ctx, voter.PubKey)
	if err != nil {
		ctx.Logger().Error("fail to get vault", "error", err)
		return &cosmos.Result{}, fmt.Errorf("fail to get vault: %w", err)
	}
	const StopSolvencyCheckKey = `StopSolvencyCheck`
	stopSolvencyCheck, err := h.mgr.Keeper().GetMimir(ctx, StopSolvencyCheckKey)
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "key", StopSolvencyCheckKey, "error", err)
	}
	if stopSolvencyCheck > 0 && stopSolvencyCheck < ctx.BlockHeight() {
		return &cosmos.Result{}, nil
	}
	// stop solvency checker per chain
	// this allows the network to stop solvency checker for ETH chain for example , while other chains like BNB/BTC chains
	// their solvency checker are still active
	stopSolvencyCheckChain, err := h.mgr.Keeper().GetMimir(ctx, fmt.Sprintf(StopSolvencyCheckKey+voter.Chain.String()))
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "key", StopSolvencyCheckKey+voter.Chain.String(), "error", err)
	}
	if stopSolvencyCheckChain > 0 && stopSolvencyCheckChain < ctx.BlockHeight() {
		return &cosmos.Result{}, nil
	}
	haltChainKey := fmt.Sprintf(`SolvencyHalt%sChain`, voter.Chain)
	haltChain, err := h.mgr.Keeper().GetMimir(ctx, haltChainKey)
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "error", err)
	}

	// If the chain was halted this block, leave it halted without overriding.
	// (For instance if halted because of a different vault which is insolvent.)
	// Also don't unhalt if the chain was manually halted for a future height
	// or indefinitely ('1').
	if haltChain >= ctx.BlockHeight() || haltChain == 1 {
		return &cosmos.Result{}, nil
	}

	isInsolvent := h.insolvencyCheckV79(ctx, vault, voter.Coins, voter.Chain)

	// If insolvent and already halted, leave the Mimir key unchanged as a record of since when it's been insolvent.
	// If insolvent and unhalted, halt the chain.
	if isInsolvent && haltChain <= 0 {
		h.mgr.Keeper().SetMimir(ctx, haltChainKey, ctx.BlockHeight())
		mimirEvent := NewEventSetMimir(strings.ToUpper(haltChainKey), strconv.FormatInt(ctx.BlockHeight(), 10))
		if err := h.mgr.EventMgr().EmitEvent(ctx, mimirEvent); err != nil {
			ctx.Logger().Error("fail to emit set_mimir event", "error", err)
		}
		ctx.Logger().Info("chain is insolvent, halt until it is resolved", "chain", voter.Chain)
	}

	// If not insolvent and the chain is halted from an earlier block height, unhalt the chain.
	// Even if a different vault is still insolvent, it can re-halt the chain in this or a later block.
	// (An alternative approach would be for if an insolvent vault always updated a lower-height Mimir key to the current height.)
	if !isInsolvent && haltChain > 1 {
		// if the chain was halted by previous solvency checker, auto unhalt it
		ctx.Logger().Info("auto un-halt", "chain", voter.Chain, "previous halt height", haltChain, "current block height", ctx.BlockHeight())
		h.mgr.Keeper().SetMimir(ctx, haltChainKey, 0)
		mimirEvent := NewEventSetMimir(strings.ToUpper(haltChainKey), "0")
		if err := h.mgr.EventMgr().EmitEvent(ctx, mimirEvent); err != nil {
			ctx.Logger().Error("fail to emit set_mimir event", "error", err)
		}
	}

	return &cosmos.Result{}, nil
}

// insolvencyCheck compare the coins in vault against the coins report by solvency message
// insolvent usually means vault has more coins than wallet
// return true means the vault is insolvent , the network should halt , otherwise false
func (h SolvencyHandler) insolvencyCheckV79(ctx cosmos.Context, vault Vault, coins common.Coins, chain common.Chain) bool {
	adjustVault, err := h.excludePendingOutboundFromVault(ctx, vault)
	if err != nil {
		return false
	}
	permittedSolvencyGap, err := h.mgr.Keeper().GetMimir(ctx, constants.PermittedSolvencyGap.String())
	if err != nil || permittedSolvencyGap <= 0 {
		permittedSolvencyGap = h.mgr.GetConstants().GetInt64Value(constants.PermittedSolvencyGap)
	}
	// Use the coin in vault as baseline , wallet can have more coins than vault
	for _, c := range adjustVault.Coins {
		if !c.Asset.Chain.Equals(chain) {
			continue
		}
		// ETH.RUNE will be burned on the way in , so the wallet will not have any, thus exclude it from solvency check
		if c.Asset.IsRune() {
			continue
		}
		if c.IsEmpty() {
			continue
		}
		walletCoin := coins.GetCoin(c.Asset)
		if walletCoin.IsEmpty() {
			ctx.Logger().Info("asset exist in vault , but not in wallet, insolvent", "asset", c.Asset.String(), "amount", c.Amount.String())
			return true
		}
		if c.Asset.IsGasAsset() {
			gas, err := h.mgr.GasMgr().GetMaxGas(ctx, c.Asset.GetChain())
			if err != nil {
				ctx.Logger().Error("fail to get max gas", "error", err)
			} else if c.Amount.LTE(gas.Amount.MulUint64(10)) {
				// if the amount left in asgard vault is not enough for 10 * max gas, then skip it from solvency check
				continue
			}
		}

		if c.Amount.GT(walletCoin.Amount) {
			gap := c.Amount.Sub(walletCoin.Amount)
			permittedGap := walletCoin.Amount.MulUint64(uint64(permittedSolvencyGap)).QuoUint64(10000)
			if gap.GT(permittedGap) {
				ctx.Logger().Info("vault has more asset than wallet, insolvent", "asset", c.Asset.String(), "vault amount", c.Amount.String(), "wallet amount", walletCoin.Amount.String(), "gap", gap.String())
				return true
			}
		}
	}
	return false
}

func (h SolvencyHandler) excludePendingOutboundFromVault(ctx cosmos.Context, vault Vault) (Vault, error) {
	// go back SigningTransactionPeriod blocks to see whether there are outstanding tx, the vault need to send out
	// if there is , deduct it from their balance
	signingPeriod := h.mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := ctx.BlockHeight() - signingPeriod
	if startHeight < 1 {
		startHeight = 1
	}
	for i := startHeight; i < ctx.BlockHeight(); i++ {
		blockOut, err := h.mgr.Keeper().GetTxOut(ctx, i)
		if err != nil {
			ctx.Logger().Error("fail to get block tx out", "error", err)
			return vault, fmt.Errorf("fail to get block tx out, err: %w", err)
		}
		vault = h.deductVaultBlockPendingOutbound(vault, blockOut)
	}
	return vault, nil
}

func (h SolvencyHandler) deductVaultBlockPendingOutbound(vault Vault, block *TxOut) Vault {
	for _, txOutItem := range block.TxArray {
		if !txOutItem.VaultPubKey.Equals(vault.PubKey) {
			continue
		}
		// only still outstanding txout will be considered
		if !txOutItem.OutHash.IsEmpty() {
			continue
		}
		// deduct the gas asset from the vault as well
		var gasCoin common.Coin
		if !txOutItem.MaxGas.IsEmpty() {
			gasCoin = txOutItem.MaxGas.ToCoins().GetCoin(txOutItem.Chain.GetGasAsset())
		}
		for i, yggCoin := range vault.Coins {
			if yggCoin.Asset.Equals(txOutItem.Coin.Asset) {
				vault.Coins[i].Amount = common.SafeSub(vault.Coins[i].Amount, txOutItem.Coin.Amount)
			}
			if yggCoin.Asset.Equals(gasCoin.Asset) {
				vault.Coins[i].Amount = common.SafeSub(vault.Coins[i].Amount, gasCoin.Amount)
			}
		}
	}
	return vault
}

// SolvencyAnteHandler called by the ante handler to gate mempool entry
// and also during deliver. Store changes will persist if this function
// succeeds, regardless of the success of the transaction.
func SolvencyAnteHandler(ctx cosmos.Context, v semver.Version, k keeper.Keeper, msg MsgSolvency) error {
	return nil
}
