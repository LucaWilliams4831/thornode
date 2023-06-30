package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h ManageTHORNameHandler) validateV1(ctx cosmos.Context, msg MsgManageTHORName) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	exists := h.mgr.Keeper().THORNameExists(ctx, msg.Name)

	if !exists {
		// thorname doesn't appear to exist, let's validate the name
		if err := h.validateNameV1(msg.Name); err != nil {
			return err
		}
		registrationFee := h.mgr.GetConstants().GetInt64Value(constants.TNSRegisterFee)
		if msg.Coin.Amount.LTE(cosmos.NewUint(uint64(registrationFee))) {
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

func (h ManageTHORNameHandler) validateV110(ctx cosmos.Context, msg MsgManageTHORName) error {
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
		registrationFee := h.mgr.GetConstants().GetInt64Value(constants.TNSRegisterFee)
		if msg.Coin.Amount.LTE(cosmos.NewUint(uint64(registrationFee))) {
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

func (h ManageTHORNameHandler) handleV1(ctx cosmos.Context, msg MsgManageTHORName) (*cosmos.Result, error) {
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
			registrationFee := h.mgr.Keeper().GetConfigInt64(ctx, constants.TNSRegisterFee)
			msg.Coin.Amount = common.SafeSub(msg.Coin.Amount, cosmos.NewUint(uint64(registrationFee)))
			registrationFeePaid = cosmos.NewUint(uint64(registrationFee))
			addBlocks = h.mgr.GetConstants().GetInt64Value(constants.BlocksPerYear) // registration comes with 1 free year
		}
		feePerBlock := h.mgr.Keeper().GetConfigInt64(ctx, constants.TNSFeePerBlock)
		fundPaid = msg.Coin.Amount
		addBlocks += (int64(msg.Coin.Amount.Uint64()) / feePerBlock)
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
