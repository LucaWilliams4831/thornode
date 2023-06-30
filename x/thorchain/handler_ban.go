package thorchain

import (
	"fmt"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// BanHandler is to handle Ban message
type BanHandler struct {
	mgr Manager
}

// NewBanHandler create new instance of BanHandler
func NewBanHandler(mgr Manager) BanHandler {
	return BanHandler{
		mgr: mgr,
	}
}

// Run is the main entry point to execute Ban logic
func (h BanHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgBan)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg ban failed validation", "error", err)
		return nil, err
	}
	return h.handle(ctx, *msg)
}

func (h BanHandler) validate(ctx cosmos.Context, msg MsgBan) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h BanHandler) validateV1(ctx cosmos.Context, msg MsgBan) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.mgr.Keeper(), msg.GetSigners()) {
		return cosmos.ErrUnauthorized(errNotAuthorized.Error())
	}

	return nil
}

func (h BanHandler) handle(ctx cosmos.Context, msg MsgBan) (*cosmos.Result, error) {
	ctx.Logger().Info("handleMsgBan request", "node address", msg.NodeAddress.String())
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return nil, errBadVersion
}

func (h BanHandler) handleV1(ctx cosmos.Context, msg MsgBan) (*cosmos.Result, error) {
	toBan, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		err = wrapError(ctx, err, "fail to get to ban node account")
		return nil, err
	}
	if err := toBan.Valid(); err != nil {
		return nil, err
	}
	if toBan.ForcedToLeave {
		// already ban, no need to ban again
		return &cosmos.Result{}, nil
	}

	switch toBan.Status {
	case NodeActive, NodeStandby:
		// we can ban an active or standby node
	default:
		return nil, se.Wrap(errInternal, "cannot ban a node account that is not currently active or standby")
	}

	banner, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		err = wrapError(ctx, err, "fail to get banner node account")
		return nil, err
	}
	if err := banner.Valid(); err != nil {
		return nil, err
	}

	active, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		err = wrapError(ctx, err, "fail to get list of active node accounts")
		return nil, err
	}

	voter, err := h.mgr.Keeper().GetBanVoter(ctx, msg.NodeAddress)
	if err != nil {
		return nil, err
	}

	if !voter.HasSigned(msg.Signer) && voter.BlockHeight == 0 {
		// take 0.1% of the minimum bond, and put it into the reserve
		minBond, err := h.mgr.Keeper().GetMimir(ctx, constants.MinimumBondInRune.String())
		if minBond < 0 || err != nil {
			minBond = h.mgr.GetConstants().GetInt64Value(constants.MinimumBondInRune)
		}
		slashAmount := cosmos.NewUint(uint64(minBond)).QuoUint64(1000)
		if slashAmount.GT(banner.Bond) {
			slashAmount = banner.Bond
		}
		banner.Bond = common.SafeSub(banner.Bond, slashAmount)

		coin := common.NewCoin(common.RuneNative, slashAmount)
		if err := h.mgr.Keeper().SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to transfer funds from bond to reserve", "error", err)
			return nil, err
		}

		if err := h.mgr.Keeper().SetNodeAccount(ctx, banner); err != nil {
			return nil, fmt.Errorf("fail to save node account: %w", err)
		}

		tx := common.Tx{}
		tx.ID = common.BlankTxID
		tx.FromAddress = banner.BondAddress
		bondEvent := NewEventBond(slashAmount, BondCost, tx)
		if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
			return nil, fmt.Errorf("fail to emit bond event: %w", err)
		}
	}

	voter.Sign(msg.Signer)
	h.mgr.Keeper().SetBanVoter(ctx, voter)
	// doesn't have consensus yet
	if !voter.HasConsensus(active) {
		ctx.Logger().Info("not having consensus yet, return")
		return &cosmos.Result{}, nil
	}

	if voter.BlockHeight > 0 {
		// ban already processed
		return &cosmos.Result{}, nil
	}

	voter.BlockHeight = ctx.BlockHeight()
	h.mgr.Keeper().SetBanVoter(ctx, voter)

	toBan.ForcedToLeave = true
	toBan.LeaveScore = 1 // Set Leave Score to 1, which means the nodes is bad
	if err := h.mgr.Keeper().SetNodeAccount(ctx, toBan); err != nil {
		err = fmt.Errorf("fail to save node account: %w", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

// BanAnteHandler called by the ante handler to gate mempool entry
// and also during deliver. Store changes will persist if this function
// succeeds, regardless of the success of the transaction.
func BanAnteHandler(ctx cosmos.Context, v semver.Version, k keeper.Keeper, msg MsgBan) error {
	return nil
}
