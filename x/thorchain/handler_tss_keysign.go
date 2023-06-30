package thorchain

import (
	"context"
	"fmt"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"
	tssMessages "gitlab.com/thorchain/tss/go-tss/messages"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// TssKeysignHandler is design to process MsgTssKeysignFail
type TssKeysignHandler struct {
	mgr Manager
}

// NewTssKeysignHandler create a new instance of TssKeysignHandler
// when a signer fail to join tss keysign , thorchain need to slash the node account
func NewTssKeysignHandler(mgr Manager) TssKeysignHandler {
	return TssKeysignHandler{
		mgr: mgr,
	}
}

// Run is the main entry to process MsgTssKeysignFail
func (h TssKeysignHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgTssKeysignFail)
	if !ok {
		return nil, errInvalidMessage
	}
	err := h.validate(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("MsgTssKeysignFail failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("failed to process MsgTssKeysignFail", "error", err)
	}
	return result, err
}

func (h TssKeysignHandler) validate(ctx cosmos.Context, msg MsgTssKeysignFail) error {
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.109.0")):
		return h.validateV109(ctx, msg)
	case version.GTE(semver.MustParse("0.70.0")):
		return h.validateV70(ctx, msg)
	}
	return errBadVersion
}

func (h TssKeysignHandler) validateV109(ctx cosmos.Context, msg MsgTssKeysignFail) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	m, err := NewMsgTssKeysignFail(msg.Height, msg.Blame, msg.Memo, msg.Coins, msg.Signer, msg.PubKey)
	if err != nil {
		ctx.Logger().Error("fail to reconstruct keysign fail msg", "error", err)
		return err
	}
	if !strings.EqualFold(m.ID, msg.ID) {
		return cosmos.ErrUnknownRequest("invalid keysign fail message")
	}
	if !isSignedByActiveNodeAccounts(ctx, h.mgr.Keeper(), msg.GetSigners()) {
		shouldAccept := false
		vaults, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
		if err != nil {
			return ErrInternal(err, "fail to get retiring vaults")
		}
		if len(vaults) > 0 {
			for _, signer := range msg.GetSigners() {
				nodeAccount, err := h.mgr.Keeper().GetNodeAccount(ctx, signer)
				if err != nil {
					return ErrInternal(err, "fail to get node account")
				}

				for _, v := range vaults {
					if v.GetMembership().Contains(nodeAccount.PubKeySet.Secp256k1) {
						shouldAccept = true
						break
					}
				}
				if shouldAccept {
					break
				}
			}
		}
		if !shouldAccept {
			return cosmos.ErrUnauthorized("not authorized")
		}
		ctx.Logger().Info("keysign failure message from retiring vault member, should accept")
	}

	active, err := h.mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return wrapError(ctx, err, "fail to get list of active node accounts")
	}

	allowWideBlame := fetchConfigInt64(ctx, h.mgr, constants.AllowWideBlame)
	if allowWideBlame == 0 && !HasSimpleMajority(len(active)-len(msg.Blame.BlameNodes), len(active)) {
		ctx.Logger().Error("blame cast too wide", "blame", len(msg.Blame.BlameNodes))
		return fmt.Errorf("blame cast too wide: %d/%d", len(msg.Blame.BlameNodes), len(active))
	}

	return nil
}

func (h TssKeysignHandler) handle(ctx cosmos.Context, msg MsgTssKeysignFail) (*cosmos.Result, error) {
	ctx.Logger().Info("handle MsgTssKeysignFail request", "ID", msg.ID, "signer", msg.Signer, "pubkey", msg.PubKey, "blame", msg.Blame.String())
	version := h.mgr.GetVersion()
	switch {
	case version.GTE(semver.MustParse("1.110.0")):
		return h.handleV110(ctx, msg)
	case version.GTE(semver.MustParse("1.109.0")):
		return h.handleV109(ctx, msg)
	case version.GTE(semver.MustParse("0.1.0")):
		return h.handleV1(ctx, msg)
	}
	return nil, errBadVersion
}

func (h TssKeysignHandler) handleV110(ctx cosmos.Context, msg MsgTssKeysignFail) (*cosmos.Result, error) {
	voter, err := h.mgr.Keeper().GetTssKeysignFailVoter(ctx, msg.ID)
	if err != nil {
		return nil, err
	}
	observeSlashPoints := h.mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)

	// add labels to telemetry context
	labels := []metrics.Label{
		telemetry.NewLabel("reason", "failed_keysign"),
	}
	if len(msg.Coins) == 1 { // only label when slash is for single asset
		labels = append(
			labels,
			telemetry.NewLabel("chain", string(msg.Coins[0].Asset.Chain)),
			telemetry.NewLabel("symbol", string(msg.Coins[0].Asset.Symbol)),
		)
	}
	slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, labels))

	h.mgr.Slasher().IncSlashPoints(slashCtx, observeSlashPoints, msg.Signer)
	if !voter.Sign(msg.Signer) {
		ctx.Logger().Info("signer already signed MsgTssKeysignFail", "signer", msg.Signer.String(), "txid", msg.ID)
		return &cosmos.Result{}, nil
	}

	// track the count of round 7 failures
	if msg.Blame.Round == tssMessages.KEYSIGN7 {
		voter.Round7Count++
	}

	h.mgr.Keeper().SetTssKeysignFailVoter(ctx, voter)
	vault, err := h.mgr.Keeper().GetVault(ctx, msg.PubKey)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get vault")
	}
	if vault.IsEmpty() {
		return &cosmos.Result{}, nil
	}
	var vaultMemberNodes NodeAccounts
	for _, item := range vault.GetMembership() {
		addr, err := item.GetThorAddress()
		if err != nil {
			return nil, wrapError(ctx, err, "fail to get thor address for "+item.String())
		}
		na, err := h.mgr.Keeper().GetNodeAccount(ctx, addr)
		if err != nil {
			return nil, wrapError(ctx, err, "fail to get node account")
		}
		vaultMemberNodes = append(vaultMemberNodes, na)
	}

	// doesn't have consensus yet
	if !voter.HasConsensus(vaultMemberNodes) {
		ctx.Logger().Info("not having consensus yet, return")
		return &cosmos.Result{}, nil
	}
	violaters := make([]string, len(msg.Blame.BlameNodes))
	for i, node := range msg.Blame.BlameNodes {
		violaters[i] = node.Pubkey
	}
	ctx.Logger().Info(
		"has tss keysign consensus!!",
		"reason", msg.Blame.FailReason,
		"is_unicast", msg.Blame.IsUnicast,
		"round", msg.Blame.Round,
		"blame", strings.Join(violaters, ", "),
	)

	telemetry.IncrCounterWithLabels(
		[]string{"thornode", "tss", "keysign", "failure"},
		float32(1),
		[]metrics.Label{telemetry.NewLabel("pubkey", msg.PubKey.String()), telemetry.NewLabel("round", msg.Blame.Round)},
	)

	// If at least 2 nodes in the simple majority report round 7 failure freeze the vault.
	// There is a tradeoff here between the number of nodes required to maliciously freeze
	// the vault and the number of nodes required to maliciously prevent freeze - we err
	// on the side of over-freezing.
	if voter.Round7Count > 1 || (voter.Round7Count > 0 && len(voter.Signers) <= 2) {
		vault, err := h.mgr.Keeper().GetVault(ctx, msg.PubKey)
		if err != nil {
			ctx.Logger().Error("fail to fetch vault", "pubkey", msg.PubKey, "error", err)
		}
		// this will cause the vault to be "frozen" which causes the
		// rescheduler to NOT reschedule any outbound txns AND cause the tx out
		// manager to not assign new txns to this vault
		for _, coin := range msg.Coins {
			found := false
			for _, chain := range vault.Frozen {
				if chain == coin.Asset.GetChain().String() {
					found = true
					break
				}
			}
			if !found {
				vault.Frozen = append(vault.Frozen, coin.Asset.GetChain().String())
			}
		}
		if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "pubkey", msg.PubKey, "error", err)
		}
	}

	h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, voter.GetSigners()...)
	voter.Signers = nil
	voter.Round7Count = 0
	h.mgr.Keeper().SetTssKeysignFailVoter(ctx, voter)

	slashPoints := h.mgr.GetConstants().GetInt64Value(constants.FailKeysignSlashPoints)
	// fail to generate a new tss key let's slash the node account

	for _, node := range msg.Blame.BlameNodes {
		nodePubKey, err := common.NewPubKey(node.Pubkey)
		if err != nil {
			return nil, ErrInternal(err, "fail to parse pubkey")
		}
		na, err := h.mgr.Keeper().GetNodeAccountByPubKey(ctx, nodePubKey)
		if err != nil {
			return nil, ErrInternal(err, fmt.Sprintf("fail to get node account,pub key: %s", nodePubKey.String()))
		}
		if err := h.mgr.Keeper().IncNodeAccountSlashPoints(slashCtx, na.NodeAddress, slashPoints); err != nil {
			ctx.Logger().Error("fail to inc slash points", "error", err)
		}

		if err := h.mgr.EventMgr().EmitEvent(ctx, NewEventSlashPoint(na.NodeAddress, slashPoints, "fail keysign")); err != nil {
			ctx.Logger().Error("fail to emit slash point event")
		}
		// go to jail
		ctx.Logger().Info("jailing node", "pubkey", na.PubKeySet.Secp256k1)
		jailTime := h.mgr.GetConstants().GetInt64Value(constants.JailTimeKeysign)
		releaseHeight := ctx.BlockHeight() + jailTime
		reason := "failed to perform keysign"
		if err := h.mgr.Keeper().SetNodeAccountJail(ctx, na.NodeAddress, releaseHeight, reason); err != nil {
			ctx.Logger().Error("fail to set node account jail", "node address", na.NodeAddress, "reason", reason, "error", err)
		}
	}

	return &cosmos.Result{}, nil
}

// TssKeysignAnteHandler called by the ante handler to gate mempool entry
// and also during deliver. Store changes will persist if this function
// succeeds, regardless of the success of the transaction.
func TssKeysignFailAnteHandler(ctx cosmos.Context, v semver.Version, k keeper.Keeper, msg MsgTssKeysignFail) error {
	return nil
}
