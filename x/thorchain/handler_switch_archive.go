package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h SwitchHandler) validateV1(ctx cosmos.Context, msg MsgSwitch) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// if we are getting a non-native asset, ensure its signed by an active
	// node account
	if !msg.Tx.Coins[0].IsNative() {
		if !isSignedByActiveNodeAccounts(ctx, h.mgr.Keeper(), msg.GetSigners()) {
			return cosmos.ErrUnauthorized(errNotAuthorized.Error())
		}
	}

	return nil
}

func (h SwitchHandler) handleV93(ctx cosmos.Context, msg MsgSwitch) (*cosmos.Result, error) {
	haltHeight, err := h.mgr.Keeper().GetMimir(ctx, "HaltTHORChain")
	if err != nil {
		return nil, fmt.Errorf("failed to get mimir setting: %w", err)
	}
	if haltHeight > 0 && ctx.BlockHeight() > haltHeight {
		return nil, fmt.Errorf("mimir has halted THORChain transactions")
	}

	if !msg.Tx.Coins[0].IsNative() && msg.Tx.Coins[0].Asset.IsRune() {
		return h.toNativeV93(ctx, msg)
	}

	return nil, fmt.Errorf("only non-native rune can be 'switched' to native rune")
}

func (h SwitchHandler) handleV87(ctx cosmos.Context, msg MsgSwitch) (*cosmos.Result, error) {
	haltHeight, err := h.mgr.Keeper().GetMimir(ctx, "HaltTHORChain")
	if err != nil {
		return nil, fmt.Errorf("failed to get mimir setting: %w", err)
	}
	if haltHeight > 0 && ctx.BlockHeight() > haltHeight {
		return nil, fmt.Errorf("mimir has halted THORChain transactions")
	}

	if !msg.Tx.Coins[0].IsNative() && msg.Tx.Coins[0].Asset.IsRune() {
		return h.toNativeV87(ctx, msg)
	}

	return nil, fmt.Errorf("only non-native rune can be 'switched' to native rune")
}

func (h SwitchHandler) handleV56(ctx cosmos.Context, msg MsgSwitch) (*cosmos.Result, error) {
	haltHeight, err := h.mgr.Keeper().GetMimir(ctx, "HaltTHORChain")
	if err != nil {
		return nil, fmt.Errorf("failed to get mimir setting: %w", err)
	}
	if haltHeight > 0 && ctx.BlockHeight() > haltHeight {
		return nil, fmt.Errorf("mimir has halted THORChain transactions")
	}

	if !msg.Tx.Coins[0].IsNative() && msg.Tx.Coins[0].Asset.IsRune() {
		return h.toNativeV56(ctx, msg)
	}

	return nil, fmt.Errorf("only non-native rune can be 'switched' to native rune")
}

func (h SwitchHandler) toNativeV87(ctx cosmos.Context, msg MsgSwitch) (*cosmos.Result, error) {
	coin := common.NewCoin(common.RuneNative, h.calcCoin(ctx, msg.Tx.Coins[0].Amount))

	addr, err := cosmos.AccAddressFromBech32(msg.Destination.String())
	if err != nil {
		return nil, ErrInternal(err, "fail to parse thor address")
	}
	if err := h.mgr.Keeper().MintAndSendToAccount(ctx, addr, coin); err != nil {
		return nil, ErrInternal(err, "fail to mint native rune coins")
	}

	// update network data
	network, err := h.mgr.Keeper().GetNetwork(ctx)
	if err != nil {
		// do not cause the transaction to fail
		ctx.Logger().Error("failed to get network", "error", err)
	}

	switch msg.Tx.Chain {
	case common.BNBChain:
		network.BurnedBep2Rune = network.BurnedBep2Rune.Add(msg.Tx.Coins[0].Amount)
	case common.ETHChain:
		network.BurnedErc20Rune = network.BurnedErc20Rune.Add(msg.Tx.Coins[0].Amount)
	}
	if err := h.mgr.Keeper().SetNetwork(ctx, network); err != nil {
		ctx.Logger().Error("failed to set network", "error", err)
	}

	switchEvent := NewEventSwitchV87(msg.Tx.FromAddress, addr, msg.Tx.Coins[0], msg.Tx.ID, coin.Amount)
	if err := h.mgr.EventMgr().EmitEvent(ctx, switchEvent); err != nil {
		ctx.Logger().Error("fail to emit switch event", "error", err)
	}

	return &cosmos.Result{}, nil
}

func (h SwitchHandler) toNativeV56(ctx cosmos.Context, msg MsgSwitch) (*cosmos.Result, error) {
	coin := common.NewCoin(common.RuneNative, msg.Tx.Coins[0].Amount)

	addr, err := cosmos.AccAddressFromBech32(msg.Destination.String())
	if err != nil {
		return nil, ErrInternal(err, "fail to parse thor address")
	}
	if err := h.mgr.Keeper().MintAndSendToAccount(ctx, addr, coin); err != nil {
		return nil, ErrInternal(err, "fail to mint native rune coins")
	}

	// update network data
	network, err := h.mgr.Keeper().GetNetwork(ctx)
	if err != nil {
		// do not cause the transaction to fail
		ctx.Logger().Error("failed to get network", "error", err)
	}

	switch msg.Tx.Chain {
	case common.BNBChain:
		network.BurnedBep2Rune = network.BurnedBep2Rune.Add(coin.Amount)
	case common.ETHChain:
		network.BurnedErc20Rune = network.BurnedErc20Rune.Add(coin.Amount)
	}
	if err := h.mgr.Keeper().SetNetwork(ctx, network); err != nil {
		ctx.Logger().Error("failed to set network", "error", err)
	}

	switchEvent := NewEventSwitch(msg.Tx.FromAddress, addr, msg.Tx.Coins[0], msg.Tx.ID)
	if err := h.mgr.EventMgr().EmitEvent(ctx, switchEvent); err != nil {
		ctx.Logger().Error("fail to emit switch event", "error", err)
	}

	return &cosmos.Result{}, nil
}

func (h SwitchHandler) calcCoinV56(ctx cosmos.Context, in cosmos.Uint) cosmos.Uint {
	killSwitchStart := fetchConfigInt64(ctx, h.mgr, constants.KillSwitchStart)
	killSwitchDuration := fetchConfigInt64(ctx, h.mgr, constants.KillSwitchDuration)
	if killSwitchStart > 0 {
		remainBlocks := (killSwitchStart + killSwitchDuration) - ctx.BlockHeight()
		if remainBlocks <= 0 {
			return cosmos.ZeroUint()
		}
		return common.GetUncappedShare(cosmos.NewUint(uint64(remainBlocks)), cosmos.NewUint(uint64(killSwitchDuration)), in)
	}
	return in
}

func (h SwitchHandler) handleV108(ctx cosmos.Context, msg MsgSwitch) (*cosmos.Result, error) {
	if isChainHalted(ctx, h.mgr, common.THORChain) {
		return nil, fmt.Errorf("unable to switch while THORChain is halted")
	}

	if !msg.Tx.Coins[0].IsNative() && msg.Tx.Coins[0].Asset.IsRune() {
		return h.toNativeV93(ctx, msg)
	}

	return nil, fmt.Errorf("only non-native rune can be 'switched' to native rune")
}
