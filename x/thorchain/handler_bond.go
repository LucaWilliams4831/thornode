package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// BondHandler a handler to process bond
type BondHandler struct {
	mgr Manager
}

// NewBondHandler create new BondHandler
func NewBondHandler(mgr Manager) BondHandler {
	return BondHandler{
		mgr: mgr,
	}
}

// Run execute the handler
func (h BondHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgBond)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgBond",
		"node address", msg.NodeAddress,
		"request hash", msg.TxIn.ID,
		"bond", msg.Bond)
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg bond fail validation", "error", err)
		return nil, err
	}

	err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process msg bond", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h BondHandler) validate(ctx cosmos.Context, msg MsgBond) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.96.0")):
		return h.validateV96(ctx, msg)
	case version.GTE(semver.MustParse("1.95.0")):
		return h.validateV95(ctx, msg)
	case version.GTE(semver.MustParse("1.88.0")):
		return h.validateV88(ctx, msg)
	case version.GTE(semver.MustParse("0.81.0")):
		return h.validateV81(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h BondHandler) validateV96(ctx cosmos.Context, msg MsgBond) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	// When RUNE is on thorchain , pay bond doesn't need to be active node
	// in fact , usually the node will not be active at the time it bond

	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	if nodeAccount.Status == NodeReady {
		return ErrInternal(err, "cannot add bond while node is ready status")
	}

	if h.mgr.GetVersion().GTE(semver.MustParse("1.88.1")) {
		if fetchConfigInt64(ctx, h.mgr, constants.PauseBond) > 0 {
			return ErrInternal(err, "bonding has been paused")
		}
	}

	bond := msg.Bond.Add(nodeAccount.Bond)
	maxBond, err := h.mgr.Keeper().GetMimir(ctx, "MaximumBondInRune")
	if maxBond > 0 && err == nil {
		maxValidatorBond := cosmos.NewUint(uint64(maxBond))
		if bond.GT(maxValidatorBond) {
			return cosmos.ErrUnknownRequest(fmt.Sprintf("too much bond, max validator bond (%s), bond(%s)", maxValidatorBond.String(), bond))
		}
	}

	if !msg.BondAddress.IsChain(common.THORChain) {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("bonding address is NOT a THORChain address: %s", msg.BondAddress.String()))
	}

	bp, err := h.mgr.Keeper().GetBondProviders(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", msg.NodeAddress))
	}

	// Attempting to set Operator Fee. If the Node has no bond address yet, it will have no fee set, continue
	if msg.OperatorFee > -1 && !nodeAccount.BondAddress.IsEmpty() {
		// Only Node Operator can set fee
		if !msg.BondAddress.Equals(nodeAccount.BondAddress) {
			return cosmos.ErrUnknownRequest("only node operator can set fee")
		}
	}

	// Validate bond address
	if msg.BondAddress.Equals(nodeAccount.BondAddress) {
		return nil
	}

	if nodeAccount.BondAddress.IsEmpty() {
		// no bond address yet, allow it to be bonded by any address
		return nil
	}

	from, err := msg.BondAddress.AccAddress()
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to parse bond address(%s)", msg.BondAddress))
	}
	if !bp.Has(from) {
		return cosmos.ErrUnknownRequest("bond address is not valid for node account")
	}

	return nil
}

func (h BondHandler) handle(ctx cosmos.Context, msg MsgBond) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.105.0")):
		return h.handleV105(ctx, msg)
	case version.GTE(semver.MustParse("1.103.0")):
		return h.handleV103(ctx, msg)
	case version.GTE(semver.MustParse("1.95.0")):
		return h.handleV95(ctx, msg)
	case version.GTE(semver.MustParse("1.88.0")):
		return h.handleV88(ctx, msg)
	case version.GTE(semver.MustParse("1.87.0")):
		return h.handleV87(ctx, msg)
	case version.GTE(semver.MustParse("1.86.0")):
		return h.handleV86(ctx, msg)
	case version.GTE(semver.MustParse("0.81.0")):
		return h.handleV81(ctx, msg)
	}
	return errBadVersion
}

func (h BondHandler) handleV105(ctx cosmos.Context, msg MsgBond) error {
	nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get node account(%s)", msg.NodeAddress))
	}

	if nodeAccount.Status == NodeUnknown {
		// THORNode will not have pub keys at the moment, so have to leave it empty
		emptyPubKeySet := common.PubKeySet{
			Secp256k1: common.EmptyPubKey,
			Ed25519:   common.EmptyPubKey,
		}
		// white list the given bep address
		nodeAccount = NewNodeAccount(msg.NodeAddress, NodeWhiteListed, emptyPubKeySet, "", cosmos.ZeroUint(), msg.BondAddress, ctx.BlockHeight())
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("new_node",
				cosmos.NewAttribute("address", msg.NodeAddress.String()),
			))
	}

	// Get the bond providers initially in order before adding the msg.Bond to the original bond.
	bp, err := h.mgr.Keeper().GetBondProviders(ctx, nodeAccount.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", msg.NodeAddress))
	}
	err = passiveBackfill(ctx, h.mgr, nodeAccount, &bp)
	if err != nil {
		return err
	}
	// Re-distribute current bond if needed
	bp.Adjust(h.mgr.GetVersion(), nodeAccount.Bond)

	nodeAccount.Bond = nodeAccount.Bond.Add(msg.Bond)

	acct := h.mgr.Keeper().GetAccount(ctx, msg.NodeAddress)

	// when node bond for the first time , send 1 RUNE to node address
	// so as the node address will be created on THORChain otherwise node account won't be able to send tx
	if acct == nil && nodeAccount.Bond.GTE(cosmos.NewUint(common.One)) {
		coin := common.NewCoin(common.RuneNative, cosmos.NewUint(common.One))
		if err := h.mgr.Keeper().SendFromModuleToAccount(ctx, BondName, msg.NodeAddress, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to send one RUNE to node address", "error", err)
			nodeAccount.Status = NodeUnknown
		}
		nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, cosmos.NewUint(common.One))
		msg.Bond = common.SafeSub(msg.Bond, cosmos.NewUint(common.One))
		tx := common.Tx{}
		tx.ID = common.BlankTxID
		tx.ToAddress = common.Address(nodeAccount.String())
		bondEvent := NewEventBond(cosmos.NewUint(common.One), BondCost, tx)
		if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
			ctx.Logger().Error("fail to emit bond event", "error", err)
		}
	}

	// if bonder is node operator, add additional bonding address
	if msg.BondAddress.Equals(nodeAccount.BondAddress) && !msg.BondProviderAddress.Empty() {
		max, err := h.mgr.Keeper().GetMimir(ctx, constants.MaxBondProviders.String())
		if err != nil || max < 0 {
			max = h.mgr.GetConstants().GetInt64Value(constants.MaxBondProviders)
		}
		if int64(len(bp.Providers)) >= max {
			return fmt.Errorf("additional bond providers are not allowed, maximum reached")
		}
		if !bp.Has(msg.BondProviderAddress) {
			bp.Providers = append(bp.Providers, NewBondProvider(msg.BondProviderAddress))
		}
	}

	from, err := msg.BondAddress.AccAddress()
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to parse bond address(%s)", msg.BondAddress))
	}
	if bp.Has(from) {
		bp.Bond(msg.Bond, from)
	}

	// Update operator fee (-1 means operator fee is not being set)
	if msg.OperatorFee > -1 && msg.OperatorFee <= 10000 {
		bp.NodeOperatorFee = cosmos.NewUint(uint64(msg.OperatorFee))
	}

	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAccount); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save node account(%s)", nodeAccount.String()))
	}

	if err := h.mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
	}

	bondEvent := NewEventBond(msg.Bond, BondPaid, msg.TxIn)
	if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
		ctx.Logger().Error("fail to emit bond event", "error", err)
	}

	return nil
}
