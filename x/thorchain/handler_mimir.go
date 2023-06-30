package thorchain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

var (
	mimirValidKey    = regexp.MustCompile(`^[a-zA-Z-]+$`).MatchString
	mimirValidKeyV95 = regexp.MustCompile(constants.MimirKeyRegex).MatchString
)

// MimirHandler is to handle admin messages
type MimirHandler struct {
	mgr Manager
}

// NewMimirHandler create new instance of MimirHandler
func NewMimirHandler(mgr Manager) MimirHandler {
	return MimirHandler{
		mgr: mgr,
	}
}

// Run is the main entry point to execute mimir logic
func (h MimirHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgMimir)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg mimir failed validation", "error", err)
		return nil, err
	}
	if err := h.handle(ctx, *msg); err != nil {
		ctx.Logger().Error("fail to process msg set mimir", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h MimirHandler) validate(ctx cosmos.Context, msg MsgMimir) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.106.0")):
		return h.validateV106(ctx, msg)
	case version.GTE(semver.MustParse("1.95.0")):
		return h.validateV95(ctx, msg)
	case version.GTE(semver.MustParse("0.78.0")):
		return h.validateV78(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h MimirHandler) validateV106(ctx cosmos.Context, msg MsgMimir) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	if !mimirValidKeyV95(msg.Key) || len(msg.Key) > 64 {
		return cosmos.ErrUnknownRequest("invalid mimir key")
	}
	if isAdmin(msg.Signer) {
		// If the signer is an admin key, check the admin access controls for this mimir.
		if !isAdminAllowedForMimir(msg.Key) {
			return cosmos.ErrUnauthorized(fmt.Sprintf("%s cannot set this mimir key", msg.Signer))
		}
	} else if !isSignedByActiveNodeAccounts(ctx, h.mgr.Keeper(), msg.GetSigners()) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorized", msg.Signer))
	}
	return nil
}

func (h MimirHandler) handle(ctx cosmos.Context, msg MsgMimir) error {
	ctx.Logger().Info("handleMsgMimir request", "node", msg.Signer, "key", msg.Key, "value", msg.Value)
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		return h.handleV112(ctx, msg)
	case version.GTE(semver.MustParse("1.92.0")):
		return h.handleV92(ctx, msg)
	case version.GTE(semver.MustParse("1.87.0")):
		return h.handleV87(ctx, msg)
	case version.GTE(semver.MustParse("0.81.0")):
		return h.handleV81(ctx, msg)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return errBadVersion
}

func (h MimirHandler) handleV112(ctx cosmos.Context, msg MsgMimir) error {
	// Get the current Mimir key value if it exists.
	currentMimirValue, _ := h.mgr.Keeper().GetMimir(ctx, msg.Key)
	// Here, an error is assumed to mean the Mimir key is currently unset.
	if isAdmin(msg.Signer) {
		// If the Mimir key is already the submitted value, don't do anything further.
		if msg.Value == currentMimirValue {
			return nil
		}
		nodeMimirs, err := h.mgr.Keeper().GetNodeMimirs(ctx, msg.Key)
		if err != nil {
			ctx.Logger().Error("fail to get node mimirs", "error", err)
			return err
		}
		activeNodes, err := h.mgr.Keeper().ListActiveValidators(ctx)
		if err != nil {
			ctx.Logger().Error("fail to list active validators", "error", err)
			return err
		}
		currentSuperMajorityValue, currentlyHasSuperMajority := nodeMimirs.HasSuperMajority(msg.Key, activeNodes.GetNodeAddresses())
		if currentlyHasSuperMajority && (msg.Value != currentSuperMajorityValue) {
			ctx.Logger().With("key", msg.Key).
				With("consensus_value", currentMimirValue).
				Info("admin mimir should not be able to override node voted mimir value")
			return nil
		}
		// Deleting or setting Mimir key value, and emitting a SetMimir event.
		if msg.Value < 0 {
			_ = h.mgr.Keeper().DeleteMimir(ctx, msg.Key)
		} else {
			h.mgr.Keeper().SetMimir(ctx, msg.Key, msg.Value)
		}
		mimirEvent := NewEventSetMimir(strings.ToUpper(msg.Key), strconv.FormatInt(msg.Value, 10))
		if err := h.mgr.EventMgr().EmitEvent(ctx, mimirEvent); err != nil {
			ctx.Logger().Error("fail to emit set_mimir event", "error", err)
			return nil
		}

	} else {
		// Cost and emitting of SetNodeMimir, even if a duplicate
		// (for instance if needed to confirm a new supermajority after a node number decrease).
		nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.Signer)
		if err != nil {
			ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
			return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorized", msg.Signer))
		}
		cost := h.mgr.Keeper().GetNativeTxFee(ctx)
		nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, cost)
		if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
			ctx.Logger().Error("fail to save node account", "error", err)
			return fmt.Errorf("fail to save node account: %w", err)
		}
		// move set mimir cost from bond module to reserve
		coin := common.NewCoin(common.RuneNative, cost)
		if !cost.IsZero() {
			if err := h.mgr.Keeper().SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin)); err != nil {
				ctx.Logger().Error("fail to transfer funds from bond to reserve", "error", err)
				return err
			}
		}
		if err := h.mgr.Keeper().SetNodeMimir(ctx, msg.Key, msg.Value, msg.Signer); err != nil {
			ctx.Logger().Error("fail to save node mimir", "error", err)
			return err
		}
		nodeMimirEvent := NewEventSetNodeMimir(strings.ToUpper(msg.Key), strconv.FormatInt(msg.Value, 10), msg.Signer.String())
		if err := h.mgr.EventMgr().EmitEvent(ctx, nodeMimirEvent); err != nil {
			ctx.Logger().Error("fail to emit set_node_mimir event", "error", err)
			return err
		}
		tx := common.Tx{}
		tx.ID = common.BlankTxID
		tx.ToAddress = common.Address(nodeAccount.String())
		bondEvent := NewEventBond(cost, BondCost, tx)
		if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
			ctx.Logger().Error("fail to emit bond event", "error", err)
			return err
		}

		// If the Mimir key is already the submitted value, don't do anything further.
		if msg.Value == currentMimirValue {
			return nil
		}

		// Get the current Active Node supermajority Mimir key value if it exists.
		// This code needs to be duplicated, since run either for Admin or only after SetNodeMimir.
		nodeMimirs, err := h.mgr.Keeper().GetNodeMimirs(ctx, msg.Key)
		if err != nil {
			ctx.Logger().Error("fail to get node mimirs", "error", err)
			return err
		}
		activeNodes, err := h.mgr.Keeper().ListActiveValidators(ctx)
		if err != nil {
			ctx.Logger().Error("fail to list active validators", "error", err)
			return err
		}
		currentSuperMajorityValue, currentlyHasSuperMajority := nodeMimirs.HasSuperMajority(msg.Key, activeNodes.GetNodeAddresses())
		// if the given key doesn't have super majority , then it shall return
		if !currentlyHasSuperMajority {
			return nil
		}
		// Given that there is an active Node super majority,
		// a Node must only change the Mimir key value when changing it to the super majority value.
		if currentlyHasSuperMajority && (currentMimirValue == currentSuperMajorityValue) {
			return nil
		}
		// after this point , means node mimir reach consensus for the first time
		// set admin mimir to lock in the value
		// setting Mimir key value, and emitting a SetMimir event.
		// if admin override node mimir voted value , and node vote again , it will then reset the admin mimir
		h.mgr.Keeper().SetMimir(ctx, msg.Key, currentSuperMajorityValue)
		mimirEvent := NewEventSetMimir(strings.ToUpper(msg.Key), strconv.FormatInt(msg.Value, 10))
		if err := h.mgr.EventMgr().EmitEvent(ctx, mimirEvent); err != nil {
			ctx.Logger().Error("fail to emit set_mimir event", "error", err)
		}
	}
	return nil
}

// MimirAnteHandler called by the ante handler to gate mempool entry
// and also during deliver. Store changes will persist if this function
// succeeds, regardless of the success of the transaction.
func MimirAnteHandler(ctx cosmos.Context, v semver.Version, k keeper.Keeper, msg MsgMimir) error {
	return nil
}
