package thorchain

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h SolvencyHandler) handleV87(ctx cosmos.Context, msg MsgSolvency) (*cosmos.Result, error) {
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

	if !h.insolvencyCheckV79(ctx, vault, voter.Coins, voter.Chain) {
		// here doesn't override HaltChain when the vault is solvent
		// in some case even the vault is solvent , the network might need to halt by admin mimir
		// admin mimir halt chain usually set the value to 1
		if haltChain <= 1 {
			return &cosmos.Result{}, nil
		}
		// if the chain was halted by previous solvency checker, auto unhalt it
		ctx.Logger().Info("auto un-halt", "chain", voter.Chain, "previous halt height", haltChain, "current block height", ctx.BlockHeight())
		h.mgr.Keeper().SetMimir(ctx, haltChainKey, 0)
		mimirEvent := NewEventSetMimir(strings.ToUpper(haltChainKey), "0")
		if err := h.mgr.EventMgr().EmitEvent(ctx, mimirEvent); err != nil {
			ctx.Logger().Error("fail to emit set_mimir event", "error", err)
		}
	}

	if haltChain > 0 && haltChain < ctx.BlockHeight() {
		// Trading already halt
		return &cosmos.Result{}, nil
	}
	h.mgr.Keeper().SetMimir(ctx, haltChainKey, ctx.BlockHeight())
	mimirEvent := NewEventSetMimir(strings.ToUpper(haltChainKey), strconv.FormatInt(ctx.BlockHeight(), 10))
	if err := h.mgr.EventMgr().EmitEvent(ctx, mimirEvent); err != nil {
		ctx.Logger().Error("fail to emit set_mimir event", "error", err)
	}
	ctx.Logger().Info("chain is insolvent, halt until it is resolved", "chain", voter.Chain)
	return &cosmos.Result{}, nil
}

// handleCurrent is the logic to process MsgSolvency, the feature works like this
//  1. Bifrost report MsgSolvency to thornode , which is the balance of asgard wallet on each individual chain
//  2. once MsgSolvency reach consensus , then the network compare the wallet balance against wallet
//     if wallet has less fund than asgard vault , and the gap is more than 1% , then the chain
//     that is insolvent will be halt
//  3. When chain is halt , bifrost will not observe inbound , and will not sign outbound txs until the issue has been investigated , and enabled it again using mimir
func (h SolvencyHandler) handleV79(ctx cosmos.Context, msg MsgSolvency) (*cosmos.Result, error) {
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
	haltChainKey := fmt.Sprintf(`Halt%sChain`, voter.Chain)
	haltChain, err := h.mgr.Keeper().GetMimir(ctx, haltChainKey)
	if err != nil {
		ctx.Logger().Error("fail to get mimir", "error", err)
	}

	if !h.insolvencyCheckV79(ctx, vault, voter.Coins, voter.Chain) {
		// here doesn't override HaltChain when the vault is solvent
		// in some case even the vault is solvent , the network might need to halt by admin mimir
		// admin mimir halt chain usually set the value to 1
		if haltChain <= 1 {
			return &cosmos.Result{}, nil
		}
		// if the chain was halted by previous solvency checker, auto unhalt it
		ctx.Logger().Info("auto un-halt", "chain", voter.Chain, "previous halt height", haltChain, "current block height", ctx.BlockHeight())
		h.mgr.Keeper().SetMimir(ctx, haltChainKey, 0)
	}

	if haltChain > 0 && haltChain < ctx.BlockHeight() {
		// Trading already halt
		return &cosmos.Result{}, nil
	}
	h.mgr.Keeper().SetMimir(ctx, haltChainKey, ctx.BlockHeight())
	ctx.Logger().Info("chain is insolvent, halt until it is resolved", "chain", voter.Chain)
	return &cosmos.Result{}, nil
}
