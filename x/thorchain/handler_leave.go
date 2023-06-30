package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// LeaveHandler a handler to process leave request
// if an operator of THORChain node would like to leave and get their bond back , they have to
// send a Leave request through Binance Chain
type LeaveHandler struct {
	mgr Manager
}

// NewLeaveHandler create a new LeaveHandler
func NewLeaveHandler(mgr Manager) LeaveHandler {
	return LeaveHandler{
		mgr: mgr,
	}
}

// Run execute the handler
func (h LeaveHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgLeave)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgLeave",
		"sender", msg.Tx.FromAddress.String(),
		"request tx hash", msg.Tx.ID)
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg leave fail validation", "error", err)
		return nil, err
	}

	if err := h.handle(ctx, *msg); err != nil {
		ctx.Logger().Error("fail to process msg leave", "error", err)
		return nil, err
	}
	return &cosmos.Result{}, nil
}

func (h LeaveHandler) validate(ctx cosmos.Context, msg MsgLeave) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h LeaveHandler) validateV1(ctx cosmos.Context, msg MsgLeave) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	jail, err := h.mgr.Keeper().GetNodeAccountJail(ctx, msg.NodeAddress)
	if err != nil {
		// ignore this error and carry on. Don't want a jail bug causing node
		// accounts to not be able to get their funds out
		ctx.Logger().Error("fail to get node account jail", "error", err)
	}
	if jail.IsJailed(ctx) {
		return fmt.Errorf("failed to leave due to jail status: (release height %d) %s", jail.ReleaseHeight, jail.Reason)
	}

	return nil
}

func (h LeaveHandler) handle(ctx cosmos.Context, msg MsgLeave) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.76.0")) {
		return h.handleV76(ctx, msg)
	}
	return errBadVersion
}

func (h LeaveHandler) handleV76(ctx cosmos.Context, msg MsgLeave) error {
	nodeAcc, err := h.mgr.Keeper().GetNodeAccount(ctx, msg.NodeAddress)
	if err != nil {
		return ErrInternal(err, "fail to get node account by bond address")
	}
	if nodeAcc.IsEmpty() {
		return cosmos.ErrUnknownRequest("node account doesn't exist")
	}
	if !nodeAcc.BondAddress.Equals(msg.Tx.FromAddress) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s are not authorized to manage %s", msg.Tx.FromAddress, msg.NodeAddress))
	}
	// THORNode add the node to leave queue

	coin := msg.Tx.Coins.GetCoin(common.RuneAsset())
	if !coin.IsEmpty() {
		nodeAcc.Bond = nodeAcc.Bond.Add(coin.Amount)
	}
	bondAddr, err := nodeAcc.BondAddress.AccAddress()
	if err != nil {
		return ErrInternal(err, "fail to refund bond")
	}

	if nodeAcc.Status == NodeActive {
		if nodeAcc.LeaveScore == 0 {
			// get to the 8th decimal point, but keep numbers integers for safer math
			age := cosmos.NewUint(uint64((ctx.BlockHeight() - nodeAcc.StatusSince) * common.One))
			slashPts, err := h.mgr.Keeper().GetNodeAccountSlashPoints(ctx, nodeAcc.NodeAddress)
			if err != nil || slashPts == 0 {
				ctx.Logger().Error("fail to get node account slash points", "error", err)
				nodeAcc.LeaveScore = age.Uint64()
			} else {
				nodeAcc.LeaveScore = age.QuoUint64(uint64(slashPts)).Uint64()
			}
		}
	} else {
		bondLockPeriod, err := h.mgr.Keeper().GetMimir(ctx, constants.BondLockupPeriod.String())
		if err != nil || bondLockPeriod < 0 {
			bondLockPeriod = h.mgr.GetConstants().GetInt64Value(constants.BondLockupPeriod)
		}
		if ctx.BlockHeight()-nodeAcc.StatusSince < bondLockPeriod {
			return fmt.Errorf("node can not unbond before %d", nodeAcc.StatusSince+bondLockPeriod)
		}
		vaults, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
		if err != nil {
			return ErrInternal(err, "fail to get retiring vault")
		}
		isMemberOfRetiringVault := false
		for _, v := range vaults {
			if v.GetMembership().Contains(nodeAcc.PubKeySet.Secp256k1) {
				isMemberOfRetiringVault = true
				ctx.Logger().Info("node account is still part of the retiring vault,can't return bond yet")
				break
			}
		}
		if !isMemberOfRetiringVault {
			// NOTE: there is an edge case, where the first node doesn't have a
			// vault (it was destroyed when we successfully migrated funds from
			// their address to a new TSS vault
			if !h.mgr.Keeper().VaultExists(ctx, nodeAcc.PubKeySet.Secp256k1) {
				if err := refundBond(ctx, msg.Tx, bondAddr, cosmos.ZeroUint(), &nodeAcc, h.mgr); err != nil {
					return ErrInternal(err, "fail to refund bond")
				}
				nodeAcc.UpdateStatus(NodeDisabled, ctx.BlockHeight())
			} else {
				// given the node is not active, they should not have Yggdrasil pool either
				// but let's check it anyway just in case
				vault, err := h.mgr.Keeper().GetVault(ctx, nodeAcc.PubKeySet.Secp256k1)
				if err != nil {
					return ErrInternal(err, "fail to get vault pool")
				}
				if vault.IsYggdrasil() {
					if !vault.HasFunds() {
						// node is not active , they are free to leave , refund them
						if err := refundBond(ctx, msg.Tx, bondAddr, cosmos.ZeroUint(), &nodeAcc, h.mgr); err != nil {
							return ErrInternal(err, "fail to refund bond")
						}
						nodeAcc.UpdateStatus(NodeDisabled, ctx.BlockHeight())
					} else {
						if h.canAbandonVault(ctx, vault, nodeAcc) {
							if err := refundBond(ctx, msg.Tx, bondAddr, cosmos.ZeroUint(), &nodeAcc, h.mgr); err != nil {
								return ErrInternal(err, "fail to refund bond")
							}
						} else {
							if err := h.mgr.ValidatorMgr().RequestYggReturn(ctx, nodeAcc, h.mgr); err != nil {
								return ErrInternal(err, "fail to request yggdrasil return fund")
							}
						}
					}
				}
			}
		}
	}
	nodeAcc.RequestedToLeave = true
	if err := h.mgr.Keeper().SetNodeAccount(ctx, nodeAcc); err != nil {
		return ErrInternal(err, "fail to save node account to key value store")
	}
	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("validator_request_leave",
			cosmos.NewAttribute("signer bnb address", msg.Tx.FromAddress.String()),
			cosmos.NewAttribute("destination", nodeAcc.BondAddress.String()),
			cosmos.NewAttribute("tx", msg.Tx.ID.String())))

	return nil
}

// canAbandonVault do some sanity check & validation to make sure the vault can be abandoned
//
//	the vault will be abandoned only when it meet the following conditions
//
// 1. Only Gas asset left in vault. for Binance chain , it should not have any BEP2 left , for ETH it should not have any ERC20 left
// 2. The Gas asset is less than 10x MaxGas
// 3. The total value in RUNE left in the yggdrasil vault should be less than 100 RUNE
// Note: This method doesn't actually slash the vault / node account , if this method return true , then the logic will continue to
// call refundBond , which will then slash node accordingly
func (h LeaveHandler) canAbandonVault(ctx cosmos.Context, vault Vault, account NodeAccount) bool {
	totalRuneValue := cosmos.ZeroUint()
	for _, c := range vault.Coins {
		if c.Amount.IsZero() {
			continue
		}
		if !c.Asset.IsGasAsset() {
			// None gas asset has not been sent back to asgard in full
			ctx.Logger().Info("yggdrasil vault contains none gas asset", "asset", c.Asset.String(), "amount", c.Amount.String())
			return false
		}
		chain := c.Asset.GetChain()
		maxGas, err := h.mgr.GasMgr().GetMaxGas(ctx, chain)
		if err != nil {
			ctx.Logger().Error("fail to get max gas", "chain", chain, "error", err)
			return false
		}
		// 10x the maxGas , if the amount of gas asset left in the yggdrasil vault is larger than 10x of the MaxGas , then we don't allow node to unbond
		if c.Amount.GT(maxGas.Amount.MulUint64(10)) {
			ctx.Logger().Info("Amount left in yggdrasil vault is more than 10x of max gas", "asset", c.Asset.String(), "amount", c.Amount.String())
			return false
		}
		pool, err := h.mgr.Keeper().GetPool(ctx, c.Asset)
		if err != nil {
			ctx.Logger().Error("fail to get pool", "asset", c.Asset, "error", err)
			return false
		}
		totalRuneValue = totalRuneValue.Add(pool.AssetValueInRune(c.Amount))
	}
	// make sure the total value left in yggdrasil vault is less than 100 RUNE
	if totalRuneValue.GT(cosmos.NewUint(100 * common.One)) {
		ctx.Logger().Error("total rune value left in yggdrasil is larger than 100 RUNE, can't abandon vault", "total rune value", totalRuneValue.String())
		return false
	}
	ctx.Logger().Info("node leave , abandon yggdrasil vault",
		"node address", account.NodeAddress.String(),
		"coins", vault.Coins.String())
	return true
}
