package thorchain

import (
	"context"
	"errors"
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

// YggdrasilHandler is to process yggdrasil messages
// When thorchain fund yggdrasil pool , observer should observe two transactions
// 1. outbound tx from asgard vault
// 2. inbound tx to yggdrasil vault
// when yggdrasil pool return fund , observer should observe two transactions as well
// 1. outbound tx from yggdrasil vault
// 2. inbound tx to asgard vault
type YggdrasilHandler struct {
	mgr Manager
}

// NewYggdrasilHandler create a new Yggdrasil handler
func NewYggdrasilHandler(mgr Manager) YggdrasilHandler {
	return YggdrasilHandler{
		mgr: mgr,
	}
}

// Run execute the logic in Yggdrasil Handler
func (h YggdrasilHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgYggdrasil)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgYggdrasil failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("failed to process MsgYggdrasil", "error", err)
		return nil, err
	}
	return result, nil
}

func (h YggdrasilHandler) validate(ctx cosmos.Context, msg MsgYggdrasil) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h YggdrasilHandler) validateV1(ctx cosmos.Context, msg MsgYggdrasil) error {
	return msg.ValidateBasic()
}

func (h YggdrasilHandler) handle(ctx cosmos.Context, msg MsgYggdrasil) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgYggdrasil", "pubkey", msg.PubKey.String(), "add_funds", msg.AddFunds, "coins", msg.Coins)
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.96.0")):
		return h.handleV96(ctx, msg)
	case version.GTE(semver.MustParse("0.1.0")):
		return h.handleV1(ctx, msg)
	default:
		return nil, errBadVersion
	}
}

func (h YggdrasilHandler) handleYggdrasilFundV1(ctx cosmos.Context, msg MsgYggdrasil, vault Vault) (*cosmos.Result, error) {
	switch vault.Type {
	case AsgardVault:
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("asgard_fund_yggdrasil",
				cosmos.NewAttribute("pubkey", vault.PubKey.String()),
				cosmos.NewAttribute("coins", msg.Coins.String()),
				cosmos.NewAttribute("tx", msg.Tx.ID.String())))
	case YggdrasilVault:
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("yggdrasil_receive_fund",
				cosmos.NewAttribute("pubkey", vault.PubKey.String()),
				cosmos.NewAttribute("coins", msg.Coins.String()),
				cosmos.NewAttribute("tx", msg.Tx.ID.String())))
	}
	// Yggdrasil usually comes from Asgard , Asgard --> Yggdrasil
	// It will be an outbound tx from Asgard pool , and it will be an Inbound tx form Yggdrasil pool
	// incoming fund will be added to Vault as part of ObservedTxInHandler
	// Yggdrasil handler doesn't need to do anything
	return &cosmos.Result{}, nil
}

func (h YggdrasilHandler) slashV96(ctx cosmos.Context, pk common.PubKey, tx common.Tx) error {
	toSlash := make(common.Coins, len(tx.Coins))
	copy(toSlash, tx.Coins)
	toSlash = toSlash.Adds(tx.Gas.ToCoins())
	return h.mgr.Slasher().SlashVault(ctx, pk, toSlash, h.mgr)
}

func (h YggdrasilHandler) handleV96(ctx cosmos.Context, msg MsgYggdrasil) (*cosmos.Result, error) {
	// update txOut record with our TxID that sent funds out of the pool
	txOut, err := h.mgr.Keeper().GetTxOut(ctx, msg.BlockHeight)
	if err != nil {
		return nil, ErrInternal(err, "unable to get txOut record")
	}

	shouldSlash := true

	// if ygg is returning funds, don't slash if they are sending to asgard vault
	if !msg.AddFunds {
		// check if the node account is active, it shouldn't be
		na, err := h.mgr.Keeper().GetNodeAccountByPubKey(ctx, msg.PubKey)
		if err != nil {
			ctx.Logger().Error("fail to get node account", "error", err)
		}
		if na.Status != NodeActive {
			active, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
			if err != nil {
				ctx.Logger().Error("fail to get vaults", "error", err)
			}
			retiring, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
			if err != nil {
				ctx.Logger().Error("fail to get vaults", "error", err)
			}
			for _, v := range append(active, retiring...) {
				addr, err := v.PubKey.GetAddress(msg.Tx.Chain)
				if err != nil {
					ctx.Logger().Error("fail to get address from pubkey", "error", err)
				}
				if !addr.IsEmpty() && addr.Equals(msg.Tx.ToAddress) {
					shouldSlash = false
					break
				}
			}
		}
	}

	for i, tx := range txOut.TxArray {
		// yggdrasil is the memo used by thorchain to identify fund migration
		// to a yggdrasil vault.
		// it use yggdrasil+/-:{block height} to mark a tx out caused by vault
		// rotation
		// this type of tx out is special , because it doesn't have relevant tx
		// in to trigger it, it is trigger by thorchain itself.
		fromAddress, _ := tx.VaultPubKey.GetAddress(tx.Chain)

		if tx.InHash.Equals(common.BlankTxID) &&
			tx.OutHash.IsEmpty() &&
			tx.ToAddress.Equals(msg.Tx.ToAddress) &&
			fromAddress.Equals(msg.Tx.FromAddress) {

			matchCoin := msg.Tx.Coins.Equals(common.Coins{tx.Coin})
			// when outbound is gas asset
			if !matchCoin && tx.Coin.Asset.Equals(tx.Chain.GetGasAsset()) {
				asset := tx.Chain.GetGasAsset()
				intendToSpend := tx.Coin.Amount.Add(tx.MaxGas.ToCoins().GetCoin(asset).Amount)
				actualSpend := msg.Tx.Coins.GetCoin(asset).Amount.Add(msg.Tx.Gas.ToCoins().GetCoin(asset).Amount)
				if intendToSpend.Equal(actualSpend) {
					maxGasAmt := tx.MaxGas.ToCoins().GetCoin(asset).Amount
					realGasAmt := msg.Tx.Gas.ToCoins().GetCoin(asset).Amount
					if maxGasAmt.GTE(realGasAmt) {
						ctx.Logger().Info("override match coin", "intend to spend", intendToSpend, "actual spend", actualSpend)
						matchCoin = true
					}
				}
			}

			// only need to check the coin if yggdrasil+
			if msg.AddFunds && !matchCoin {
				continue
			}

			txOut.TxArray[i].OutHash = msg.Tx.ID
			shouldSlash = false
			if err := h.mgr.Keeper().SetTxOut(ctx, txOut); nil != err {
				ctx.Logger().Error("fail to save tx out", "error", err)
			}
			break
		}
	}

	if shouldSlash {
		ctx.Logger().Info("slash node account, no matched tx out item", "outbound tx", msg.Tx)

		slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
			telemetry.NewLabel("reason", "failed_yggdrasil_return"),
			telemetry.NewLabel("chain", string(msg.Tx.Chain)),
		}))

		if err := h.slashV96(slashCtx, msg.PubKey, msg.Tx); err != nil {
			return nil, ErrInternal(err, "fail to slash account")
		}
	}

	vault, err := h.mgr.Keeper().GetVault(ctx, msg.PubKey)
	if err != nil && !errors.Is(err, kvTypes.ErrVaultNotFound) {
		return nil, fmt.Errorf("fail to get yggdrasil: %w", err)
	}
	if vault.IsType(UnknownVault) {
		vault.Status = ActiveVault
		vault.Type = YggdrasilVault
	}

	if err := h.mgr.Keeper().SetLastSignedHeight(ctx, msg.BlockHeight); err != nil {
		ctx.Logger().Info("fail to update last signed height", "error", err)
	}

	if msg.AddFunds {
		return h.handleYggdrasilFundV1(ctx, msg, vault)
	}
	return h.handleYggdrasilReturnV1(ctx, msg, vault)
}

func (h YggdrasilHandler) handleYggdrasilReturnV1(ctx cosmos.Context, msg MsgYggdrasil, vault Vault) (*cosmos.Result, error) {
	// observe an outbound tx from yggdrasil vault
	switch vault.Type {
	case YggdrasilVault:
		asgardVaults, err := h.mgr.Keeper().GetAsgardVaults(ctx)
		if err != nil {
			return nil, ErrInternal(err, "unable to get asgard vaults")
		}
		vaults := Vaults{}
		var contracts []ChainContract
		for _, v := range asgardVaults {
			// make sure vaults have both active asgard vault , and also retiring asgard vault
			if v.Status == ActiveVault || v.Status == RetiringVault {
				vaults = append(vaults, v)
			}
			if v.Status == ActiveVault {
				contracts = v.Routers
			}
		}

		isAsgardReceipient, err := vaults.HasAddress(msg.Tx.Chain, msg.Tx.ToAddress)
		if err != nil {
			return nil, ErrInternal(err, fmt.Sprintf("unable to determinate whether %s is an Asgard vault", msg.Tx.ToAddress))
		}

		if !isAsgardReceipient {
			ctx.Logger().Info("yggdrasil send fund to non-asgard address")
			// not sending to asgard , slash the node account
			slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
				telemetry.NewLabel("reason", "bad_yggdrasil_return_address"),
				telemetry.NewLabel("chain", string(msg.Tx.Chain)),
			}))
			if err := h.slashV96(slashCtx, msg.PubKey, msg.Tx); err != nil {
				return nil, ErrInternal(err, "fail to slash account for sending fund to a none asgard vault using yggdrasil-")
			}
		}

		// when yggdrasil return fund , check all the chains that support contract
		// once all the fund had been returned, then update the chain's contract to match asgard contract address
		// thus new yggdrasil will be fund with new contract
		for _, contract := range contracts {
			if vault.HasFundsForChain(contract.Chain) {
				var noneZeroCoins common.Coins
				for _, c := range vault.Coins {
					if !c.Asset.GetChain().Equals(contract.Chain) {
						continue
					}
					if c.Amount.IsZero() {
						continue
					}
					noneZeroCoins = append(noneZeroCoins, c)
				}
				// there are more than 1 coins that are not zero , that means yggdrasil vault doesn't send everything back yet
				if len(noneZeroCoins) > 1 {
					continue
				}
				// None zero coin is not gas coin,for example on ETH , the coin has outstanding balance is not ETH
				if !noneZeroCoins[0].Asset.Equals(contract.Chain.GetGasAsset()) {
					continue
				}
				// if the logic reach here , which means there might only have gas coin left with some balance in it
				// which is ok, because gas asset is not going to be hold by the smart contract , it will be hold by vault itself
				// thus the network can go ahead and update the vault's contract
				ctx.Logger().Info("only gas token left in the vault, continue to update contract", "gas token", contract.Chain.GetGasAsset().String(), "amount", noneZeroCoins[0].Amount.String())
			}
			vault.UpdateContract(contract)
			if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				return nil, fmt.Errorf("fail to save vault: %w", err)
			}
		}

		return &cosmos.Result{}, nil

	case AsgardVault:
		// when vault.Type is asgard, that means this tx is observed on an asgard pool and it is an inbound tx
		// Yggdrasil return fund back to Asgard
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("yggdrasil_return",
				cosmos.NewAttribute("pubkey", vault.PubKey.String()),
				cosmos.NewAttribute("coins", msg.Coins.String()),
				cosmos.NewAttribute("tx", msg.Tx.ID.String())))
	}
	return &cosmos.Result{}, nil
}
