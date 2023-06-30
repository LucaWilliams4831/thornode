package thorchain

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

var IsValidTHORNameV1 = regexp.MustCompile(`^[a-zA-Z0-9+_-]+$`).MatchString

// ManageTHORNameHandler a handler to process MsgNetworkFee messages
type ManageTHORNameHandler struct {
	mgr Manager
}

// NewManageTHORNameHandler create a new instance of network fee handler
func NewManageTHORNameHandler(mgr Manager) ManageTHORNameHandler {
	return ManageTHORNameHandler{mgr: mgr}
}

// Run is the main entry point for network fee logic
func (h ManageTHORNameHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgManageTHORName)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgManageTHORName failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process MsgManageTHORName", "error", err)
	}
	return result, err
}

func (h ManageTHORNameHandler) validate(ctx cosmos.Context, msg MsgManageTHORName) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		return h.validateV112(ctx, msg)
	case version.GTE(semver.MustParse("1.110.0")):
		return h.validateV110(ctx, msg)
	case version.GTE(semver.MustParse("0.1.0")):
		return h.validateV1(ctx, msg)
	default:
		return errBadVersion
	}
}

func (h ManageTHORNameHandler) validateNameV1(n string) error {
	// validate THORName
	if len(n) > 30 {
		return errors.New("THORName cannot exceed 30 characters")
	}
	if !IsValidTHORNameV1(n) {
		return errors.New("invalid THORName")
	}
	return nil
}

func (h ManageTHORNameHandler) validateV112(ctx cosmos.Context, msg MsgManageTHORName) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// TODO on hard fork move network check to ValidateBasic
	if !common.CurrentChainNetwork.SoftEquals(msg.Address.GetNetwork(h.mgr.GetVersion(), msg.Address.GetChain())) {
		return fmt.Errorf("address(%s) is not same network", msg.Address)
	}

	exists := h.mgr.Keeper().THORNameExists(ctx, msg.Name)

	if !exists {
		// thorname doesn't appear to exist, let's validate the name
		if err := h.validateNameV1(msg.Name); err != nil {
			return err
		}
		registrationFee := h.mgr.Keeper().GetTHORNameRegisterFee(ctx)
		if msg.Coin.Amount.LTE(registrationFee) {
			return fmt.Errorf("not enough funds")
		}
	} else {
		name, err := h.mgr.Keeper().GetTHORName(ctx, msg.Name)
		if err != nil {
			return err
		}

		// if this thorname is already owned, check signer has ownership. If
		// expiration is past, allow different user to take ownership
		if !name.Owner.Equals(msg.Signer) && ctx.BlockHeight() <= name.ExpireBlockHeight {
			ctx.Logger().Error("no authorization", "owner", name.Owner)
			return fmt.Errorf("no authorization: owned by %s", name.Owner)
		}

		// ensure user isn't inflating their expire block height artificaially
		if name.ExpireBlockHeight < msg.ExpireBlockHeight {
			return errors.New("cannot artificially inflate expire block height")
		}
	}

	return nil
}

// handle process MsgManageTHORName
func (h ManageTHORNameHandler) handle(ctx cosmos.Context, msg MsgManageTHORName) (*cosmos.Result, error) {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		return h.handleV112(ctx, msg)
	case version.GTE(semver.MustParse("0.1.0")):
		return h.handleV1(ctx, msg)
	}
	return nil, errBadVersion
}

// handle process MsgManageTHORName
func (h ManageTHORNameHandler) handleV112(ctx cosmos.Context, msg MsgManageTHORName) (*cosmos.Result, error) {
	var err error

	enable, _ := h.mgr.Keeper().GetMimir(ctx, "THORNames")
	if enable == 0 {
		return nil, fmt.Errorf("THORNames are currently disabled")
	}

	tn := THORName{Name: msg.Name, Owner: msg.Signer, PreferredAsset: common.EmptyAsset}
	exists := h.mgr.Keeper().THORNameExists(ctx, msg.Name)
	if exists {
		tn, err = h.mgr.Keeper().GetTHORName(ctx, msg.Name)
		if err != nil {
			return nil, err
		}
	}

	registrationFeePaid := cosmos.ZeroUint()
	fundPaid := cosmos.ZeroUint()

	// check if user is trying to extend expiration
	if !msg.Coin.Amount.IsZero() {
		// check that THORName is still valid, can't top up an invalid THORName
		if err := h.validateNameV1(msg.Name); err != nil {
			return nil, err
		}
		var addBlocks int64
		// registration fee is for THORChain addresses only
		if !exists {
			// minus registration fee
			registrationFee := h.mgr.Keeper().GetTHORNameRegisterFee(ctx)
			msg.Coin.Amount = common.SafeSub(msg.Coin.Amount, registrationFee)
			registrationFeePaid = registrationFee
			addBlocks = h.mgr.GetConstants().GetInt64Value(constants.BlocksPerYear) // registration comes with 1 free year
		}
		feePerBlock := h.mgr.Keeper().GetTHORNamePerBlockFee(ctx)
		fundPaid = msg.Coin.Amount
		addBlocks += (int64(msg.Coin.Amount.Uint64()) / int64(feePerBlock.Uint64()))
		if tn.ExpireBlockHeight < ctx.BlockHeight() {
			tn.ExpireBlockHeight = ctx.BlockHeight() + addBlocks
		} else {
			tn.ExpireBlockHeight += addBlocks
		}
	}

	// check if we need to reduce the expire time, upon user request
	if msg.ExpireBlockHeight > 0 && msg.ExpireBlockHeight < tn.ExpireBlockHeight {
		tn.ExpireBlockHeight = msg.ExpireBlockHeight
	}

	// check if we need to update the preferred asset
	if !tn.PreferredAsset.Equals(msg.PreferredAsset) && !msg.PreferredAsset.IsEmpty() {
		tn.PreferredAsset = msg.PreferredAsset
	}

	tn.SetAlias(msg.Chain, msg.Address) // update address
	if !msg.Owner.Empty() {
		tn.Owner = msg.Owner // update owner
	}
	h.mgr.Keeper().SetTHORName(ctx, tn)

	evt := NewEventTHORName(tn.Name, msg.Chain, msg.Address, registrationFeePaid, fundPaid, tn.ExpireBlockHeight, tn.Owner)
	if err := h.mgr.EventMgr().EmitEvent(ctx, evt); nil != err {
		ctx.Logger().Error("fail to emit THORName event", "error", err)
	}

	return &cosmos.Result{}, nil
}
