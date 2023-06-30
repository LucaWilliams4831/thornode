package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// SwitchHandler is to handle Switch message
// MsgSwitch is used to switch from bep2 RUNE to native RUNE
type SwitchHandler struct {
	mgr Manager
}

// NewSwitchHandler create new instance of SwitchHandler
func NewSwitchHandler(mgr Manager) SwitchHandler {
	return SwitchHandler{
		mgr: mgr,
	}
}

// Run it the main entry point to execute Switch logic
func (h SwitchHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSwitch)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg switch failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("failed to process msg switch", "error", err)
		return nil, err
	}
	return result, err
}

func (h SwitchHandler) validate(ctx cosmos.Context, msg MsgSwitch) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("1.87.0")) {
		return h.validateV87(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h SwitchHandler) validateV87(ctx cosmos.Context, msg MsgSwitch) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	killSwitchStart := h.mgr.Keeper().GetConfigInt64(ctx, constants.KillSwitchStart)
	killSwitchDuration := h.mgr.Keeper().GetConfigInt64(ctx, constants.KillSwitchDuration)

	if killSwitchStart > 0 && ctx.BlockHeight() > killSwitchStart+killSwitchDuration {
		return fmt.Errorf("switch is deprecated")
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

func (h SwitchHandler) handle(ctx cosmos.Context, msg MsgSwitch) (*cosmos.Result, error) {
	ctx.Logger().Info("handleMsgSwitch request", "destination address", msg.Destination.String())
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		return h.handleV112(ctx, msg)
	case version.GTE(semver.MustParse("1.108.0")):
		return h.handleV108(ctx, msg)
	case version.GTE(semver.MustParse("1.93.0")):
		return h.handleV93(ctx, msg)
	case version.GTE(semver.MustParse("1.87.0")):
		return h.handleV87(ctx, msg)
	case version.GTE(semver.MustParse("0.56.0")):
		return h.handleV56(ctx, msg)
	default:
		return nil, errBadVersion
	}
}

func (h SwitchHandler) handleV112(ctx cosmos.Context, msg MsgSwitch) (*cosmos.Result, error) {
	if h.mgr.Keeper().IsChainHalted(ctx, common.THORChain) {
		return nil, fmt.Errorf("unable to switch while THORChain is halted")
	}

	if !msg.Tx.Coins[0].IsNative() && msg.Tx.Coins[0].Asset.IsRune() {
		return h.toNativeV93(ctx, msg)
	}

	return nil, fmt.Errorf("only non-native rune can be 'switched' to native rune")
}

func (h SwitchHandler) toNativeV93(ctx cosmos.Context, msg MsgSwitch) (*cosmos.Result, error) {
	coin := common.NewCoin(common.RuneNative, h.calcCoinV93(ctx, msg.Tx.Coins[0].Amount))

	// sanity check
	if coin.Amount.GT(msg.Tx.Coins[0].Amount) {
		return nil, fmt.Errorf("improper switch calculation: %d/%d", coin.Amount.Uint64(), msg.Tx.Coins[0].Amount.Uint64())
	}

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

func (h SwitchHandler) calcCoin(ctx cosmos.Context, in cosmos.Uint) cosmos.Uint {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.93.0")):
		return h.calcCoinV93(ctx, in)
	case version.GTE(semver.MustParse("0.56.0")):
		return h.calcCoinV56(ctx, in)
	default:
		return cosmos.ZeroUint()
	}
}

func (h SwitchHandler) calcCoinV93(ctx cosmos.Context, in cosmos.Uint) cosmos.Uint {
	killSwitchStart := h.mgr.Keeper().GetConfigInt64(ctx, constants.KillSwitchStart)
	if killSwitchStart > 0 && ctx.BlockHeight() >= killSwitchStart {
		killSwitchDuration := h.mgr.Keeper().GetConfigInt64(ctx, constants.KillSwitchDuration)
		remainBlocks := (killSwitchStart + killSwitchDuration) - ctx.BlockHeight()
		if remainBlocks <= 0 {
			return cosmos.ZeroUint()
		}
		return common.GetSafeShare(cosmos.NewUint(uint64(remainBlocks)), cosmos.NewUint(uint64(killSwitchDuration)), in)
	}
	return in
}
