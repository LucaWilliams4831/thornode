package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h NodePauseChainHandler) handleV1(ctx cosmos.Context, msg MsgNodePauseChain) error {
	// get block height of last churn
	active, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return err
	}
	lastChurn := int64(-1)
	for _, vault := range active {
		if vault.StatusSince > lastChurn {
			lastChurn = vault.StatusSince
		}
	}

	// check that node hasn't used this handler since the last churn already
	nodeHeight := h.mgr.Keeper().GetNodePauseChain(ctx, msg.Signer)
	if nodeHeight > lastChurn {
		return fmt.Errorf("node has already chosen pause/resume since the last churn")
	}

	// get the current block height set by node pause chain global
	pauseHeight, err := h.mgr.Keeper().GetMimir(ctx, "NodePauseChainGlobal")
	if err != nil {
		return err
	}

	blocks, err := h.mgr.Keeper().GetMimir(ctx, constants.NodePauseChainBlocks.String())
	if blocks < 0 || err != nil {
		blocks = h.mgr.GetConstants().GetInt64Value(constants.NodePauseChainBlocks)
	}

	if msg.Value > 0 { // node intends to pause chain
		if pauseHeight > ctx.BlockHeight() { // chain is paused
			pauseHeight += blocks
			h.mgr.Keeper().SetNodePauseChain(ctx, msg.Signer)
		} else { // chain isn't paused
			pauseHeight = ctx.BlockHeight() + blocks
			h.mgr.Keeper().SetNodePauseChain(ctx, msg.Signer)
		}
	} else if msg.Value < 0 { // node intends so resume chain
		if pauseHeight > ctx.BlockHeight() { // chain is paused
			h.mgr.Keeper().SetNodePauseChain(ctx, msg.Signer)
			pauseHeight -= blocks
		}
	}

	h.mgr.Keeper().SetMimir(ctx, "NodePauseChainGlobal", pauseHeight)

	return nil
}
