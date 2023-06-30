package thorchain

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type TssHandler = BaseHandler[*MsgTssPool]

// NewTssHandler create a new handler to process MsgTssPool
func NewTssHandler(mgr Manager) BaseHandler[*MsgTssPool] {
	return BaseHandler[*MsgTssPool]{
		mgr:    mgr,
		logger: MsgTssPoolLogger,
		validators: NewValidators[*MsgTssPool]().
			Register("0.71.0", MsgTssPoolValidateV71),
		handlers: NewHandlers[*MsgTssPool]().
			Register("1.93.0", MsgTssPoolHandleV93).
			Register("1.92.0", MsgTssPoolHandleV92).
			Register("0.73.0", MsgTssPoolHandleV73),
	}
}

func MsgTssPoolLogger(ctx cosmos.Context, msg *MsgTssPool) {
	ctx.Logger().Info("handleMsgTssPool request", "ID:", msg.ID)
}

func MsgTssPoolValidateV71(ctx cosmos.Context, mgr Manager, msg *MsgTssPool) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	newMsg, err := NewMsgTssPool(msg.PubKeys, msg.PoolPubKey, nil, msg.KeygenType, msg.Height, msg.Blame, msg.Chains, msg.Signer, msg.KeygenTime)
	if err != nil {
		return fmt.Errorf("fail to recreate MsgTssPool,err: %w", err)
	}
	if msg.ID != newMsg.ID {
		return cosmos.ErrUnknownRequest("invalid tss message")
	}

	churnRetryBlocks := mgr.GetConstants().GetInt64Value(constants.ChurnRetryInterval)
	if msg.Height <= ctx.BlockHeight()-churnRetryBlocks {
		return cosmos.ErrUnknownRequest("invalid keygen block")
	}

	keygenBlock, err := mgr.Keeper().GetKeygenBlock(ctx, msg.Height)
	if err != nil {
		return fmt.Errorf("fail to get keygen block from data store: %w", err)
	}

	for _, keygen := range keygenBlock.Keygens {
		keyGenMembers := keygen.GetMembers()
		if !msg.GetPubKeys().Equals(keyGenMembers) {
			continue
		}
		// Make sure the keygen type are consistent
		if msg.KeygenType != keygen.Type {
			continue
		}
		for _, member := range keygen.GetMembers() {
			addr, err := member.GetThorAddress()
			if err == nil && addr.Equals(msg.Signer) {
				return validateSigner(ctx, mgr, msg.Signer)
			}
		}
	}

	return cosmos.ErrUnauthorized("not authorized")
}

func validateSigner(ctx cosmos.Context, mgr Manager, signer cosmos.AccAddress) error {
	nodeSigner, err := mgr.Keeper().GetNodeAccount(ctx, signer)
	if err != nil {
		return fmt.Errorf("invalid signer")
	}
	if nodeSigner.IsEmpty() {
		return fmt.Errorf("invalid signer")
	}
	if nodeSigner.Status != NodeActive && nodeSigner.Status != NodeReady {
		return fmt.Errorf("invalid signer status(%s)", nodeSigner.Status)
	}
	// ensure we have enough rune
	minBond, err := mgr.Keeper().GetMimir(ctx, constants.MinimumBondInRune.String())
	if minBond < 0 || err != nil {
		minBond = mgr.GetConstants().GetInt64Value(constants.MinimumBondInRune)
	}
	if nodeSigner.Bond.LT(cosmos.NewUint(uint64(minBond))) {
		return fmt.Errorf("signer doesn't have enough rune")
	}
	return nil
}

func MsgTssPoolHandleV93(ctx cosmos.Context, mgr Manager, msg *MsgTssPool) (*cosmos.Result, error) {
	ctx.Logger().Info("handler tss", "current version", mgr.GetVersion())
	if !msg.Blame.IsEmpty() {
		blames := make([]string, len(msg.Blame.BlameNodes))
		for i := range msg.Blame.BlameNodes {
			pk, err := common.NewPubKey(msg.Blame.BlameNodes[i].Pubkey)
			if err != nil {
				ctx.Logger().Error("fail to get tss keygen pubkey", "pubkey", msg.Blame.BlameNodes[i].Pubkey, "error", err)
				continue
			}
			acc, err := pk.GetThorAddress()
			if err != nil {
				ctx.Logger().Error("fail to get tss keygen thor address", "pubkey", msg.Blame.BlameNodes[i].Pubkey, "error", err)
				continue
			}
			blames[i] = acc.String()
		}
		sort.Strings(blames)
		ctx.Logger().Info(
			"tss keygen results blame",
			"height", msg.Height,
			"id", msg.ID,
			"pubkey", msg.PoolPubKey,
			"round", msg.Blame.Round,
			"blames", strings.Join(blames, ", "),
			"reason", msg.Blame.FailReason,
			"blamer", msg.Signer,
		)
	}
	// only record TSS metric when keygen is success
	if msg.IsSuccess() && !msg.PoolPubKey.IsEmpty() {
		metric, err := mgr.Keeper().GetTssKeygenMetric(ctx, msg.PoolPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get keygen metric", "error", err)
		} else {
			ctx.Logger().Info("save keygen metric to db")
			metric.AddNodeTssTime(msg.Signer, msg.KeygenTime)
			mgr.Keeper().SetTssKeygenMetric(ctx, metric)
		}
	}
	voter, err := mgr.Keeper().GetTssVoter(ctx, msg.ID)
	if err != nil {
		return nil, fmt.Errorf("fail to get tss voter: %w", err)
	}

	// when PoolPubKey is empty , which means TssVoter with id(msg.ID) doesn't
	// exist before, this is the first time to create it
	// set the PoolPubKey to the one in msg, there is no reason voter.PubKeys
	// have anything in it either, thus override it with msg.PubKeys as well
	if voter.PoolPubKey.IsEmpty() {
		voter.PoolPubKey = msg.PoolPubKey
		voter.PubKeys = msg.PubKeys
	}
	// voter's pool pubkey is the same as the one in messasge
	if !voter.PoolPubKey.Equals(msg.PoolPubKey) {
		return nil, fmt.Errorf("invalid pool pubkey")
	}
	observeSlashPoints := mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := mgr.GetConstants().GetInt64Value(constants.ObservationDelayFlexibility)

	slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
		telemetry.NewLabel("reason", "failed_observe_tss_pool"),
	}))
	mgr.Slasher().IncSlashPoints(slashCtx, observeSlashPoints, msg.Signer)

	if !voter.Sign(msg.Signer, msg.Chains) {
		ctx.Logger().Info("signer already signed MsgTssPool", "signer", msg.Signer.String(), "txid", msg.ID)
		return &cosmos.Result{}, nil

	}
	mgr.Keeper().SetTssVoter(ctx, voter)

	// doesn't have 2/3 majority consensus yet
	if !voter.HasConsensus() {
		return &cosmos.Result{}, nil
	}

	// when keygen success
	if msg.IsSuccess() {
		judgeLateSigner(ctx, mgr, msg, voter)
		if !voter.HasCompleteConsensus() {
			return &cosmos.Result{}, nil
		}
	}

	if voter.BlockHeight == 0 {
		voter.BlockHeight = ctx.BlockHeight()
		mgr.Keeper().SetTssVoter(ctx, voter)
		mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, voter.GetSigners()...)
		if msg.IsSuccess() {
			ctx.Logger().Info(
				"tss keygen results success",
				"height", msg.Height,
				"id", msg.ID,
				"pubkey", msg.PoolPubKey,
			)
			vaultType := YggdrasilVault
			if msg.KeygenType == AsgardKeygen {
				vaultType = AsgardVault
			}
			chains := voter.ConsensusChains()
			vault := NewVault(ctx.BlockHeight(), InitVault, vaultType, voter.PoolPubKey, chains.Strings(), mgr.Keeper().GetChainContracts(ctx, chains))
			vault.Membership = voter.PubKeys

			if err := mgr.Keeper().SetVault(ctx, vault); err != nil {
				return nil, fmt.Errorf("fail to save vault: %w", err)
			}
			keygenBlock, err := mgr.Keeper().GetKeygenBlock(ctx, msg.Height)
			if err != nil {
				return nil, fmt.Errorf("fail to get keygen block, err: %w, height: %d", err, msg.Height)
			}
			initVaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, InitVault)
			if err != nil {
				return nil, fmt.Errorf("fail to get init vaults: %w", err)
			}

			metric, err := mgr.Keeper().GetTssKeygenMetric(ctx, msg.PoolPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get keygen metric", "error", err)
			} else {
				var total int64
				for _, item := range metric.NodeTssTimes {
					total += item.TssTime
				}
				evt := NewEventTssKeygenMetric(metric.PubKey, metric.GetMedianTime())
				if err := mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}

			if len(initVaults) == len(keygenBlock.Keygens) {
				ctx.Logger().Info("tss keygen results churn", "asgards", len(initVaults))
				for _, v := range initVaults {
					if err := mgr.NetworkMgr().RotateVault(ctx, v); err != nil {
						return nil, fmt.Errorf("fail to rotate vault: %w", err)
					}
				}
			} else {
				ctx.Logger().Info("not enough keygen yet", "expecting", len(keygenBlock.Keygens), "current", len(initVaults))
			}
		} else {
			// if a node fail to join the keygen, thus hold off the network
			// from churning then it will be slashed accordingly
			slashPoints := mgr.GetConstants().GetInt64Value(constants.FailKeygenSlashPoints)
			for _, node := range msg.Blame.BlameNodes {
				nodePubKey, err := common.NewPubKey(node.Pubkey)
				if err != nil {
					return nil, ErrInternal(err, fmt.Sprintf("fail to parse pubkey(%s)", node.Pubkey))
				}

				na, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, nodePubKey)
				if err != nil {
					return nil, fmt.Errorf("fail to get node from it's pub key: %w", err)
				}
				if na.Status == NodeActive {
					failedKeygenSlashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
						telemetry.NewLabel("reason", "failed_keygen"),
					}))
					if err := mgr.Keeper().IncNodeAccountSlashPoints(failedKeygenSlashCtx, na.NodeAddress, slashPoints); err != nil {
						ctx.Logger().Error("fail to inc slash points", "error", err)
					}

					if err := mgr.EventMgr().EmitEvent(ctx, NewEventSlashPoint(na.NodeAddress, slashPoints, "fail keygen")); err != nil {
						ctx.Logger().Error("fail to emit slash point event")
					}
				} else {
					// go to jail
					jailTime := mgr.GetConstants().GetInt64Value(constants.JailTimeKeygen)
					releaseHeight := ctx.BlockHeight() + jailTime
					reason := "failed to perform keygen"
					if err := mgr.Keeper().SetNodeAccountJail(ctx, na.NodeAddress, releaseHeight, reason); err != nil {
						ctx.Logger().Error("fail to set node account jail", "node address", na.NodeAddress, "reason", reason, "error", err)
					}

					network, err := mgr.Keeper().GetNetwork(ctx)
					if err != nil {
						return nil, fmt.Errorf("fail to get network: %w", err)
					}

					slashBond := network.CalcNodeRewards(cosmos.NewUint(uint64(slashPoints)))
					if slashBond.GT(na.Bond) {
						slashBond = na.Bond
					}
					ctx.Logger().Info("fail keygen , slash bond", "address", na.NodeAddress, "amount", slashBond.String())
					// take out bond from the node account and add it to the Reserve
					// thus good behaviour nodes and liquidity providers will get reward
					na.Bond = common.SafeSub(na.Bond, slashBond)
					coin := common.NewCoin(common.RuneNative, slashBond)
					if !coin.Amount.IsZero() {
						if err := mgr.Keeper().SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin)); err != nil {
							return nil, fmt.Errorf("fail to transfer funds from bond to reserve: %w", err)
						}
						slashFloat, _ := new(big.Float).SetInt(slashBond.BigInt()).Float32()
						telemetry.IncrCounterWithLabels(
							[]string{"thornode", "bond_slash"},
							slashFloat,
							[]metrics.Label{
								telemetry.NewLabel("address", na.NodeAddress.String()),
								telemetry.NewLabel("reason", "failed_keygen"),
							},
						)
					}

					tx := common.Tx{}
					tx.ID = common.BlankTxID
					tx.FromAddress = na.BondAddress
					bondEvent := NewEventBond(slashBond, BondCost, tx)
					if err := mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
						return nil, fmt.Errorf("fail to emit bond event: %w", err)
					}
				}
				if err := mgr.Keeper().SetNodeAccount(ctx, na); err != nil {
					return nil, fmt.Errorf("fail to save node account: %w", err)
				}
			}

		}
		return &cosmos.Result{}, nil
	}

	if (voter.BlockHeight + observeFlex) >= ctx.BlockHeight() {
		mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, msg.Signer)
	}

	return &cosmos.Result{}, nil
}

func judgeLateSigner(ctx cosmos.Context, mgr Manager, msg *MsgTssPool, voter TssVoter) {
	// if the voter doesn't reach 2/3 majority consensus , this method should not take any actions
	if !voter.HasConsensus() || !msg.IsSuccess() {
		return
	}
	slashPoints := mgr.GetConstants().GetInt64Value(constants.FailKeygenSlashPoints)
	slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
		telemetry.NewLabel("reason", "failed_observe_tss_pool"),
	}))

	// when voter already has 2/3 majority signers , restore current message signer's slash points
	if voter.MajorityConsensusBlockHeight > 0 {
		mgr.Slasher().DecSlashPoints(slashCtx, slashPoints, msg.Signer)
		if err := mgr.Keeper().ReleaseNodeAccountFromJail(ctx, msg.Signer); err != nil {
			ctx.Logger().Error("fail to release node account from jail", "node address", msg.Signer, "error", err)
		}
		return
	}

	voter.MajorityConsensusBlockHeight = ctx.BlockHeight()
	mgr.Keeper().SetTssVoter(ctx, voter)
	for _, member := range msg.PubKeys {
		pkey, err := common.NewPubKey(member)
		if err != nil {
			ctx.Logger().Error("fail to get pub key", "error", err)
			continue
		}
		thorAddr, err := pkey.GetThorAddress()
		if err != nil {
			ctx.Logger().Error("fail to get thor address", "error", err)
			continue
		}
		// whoever is in the keygen list , but didn't broadcast MsgTssPool
		if !voter.HasSigned(thorAddr) {
			mgr.Slasher().IncSlashPoints(slashCtx, slashPoints, thorAddr)
			// go to jail
			jailTime := mgr.GetConstants().GetInt64Value(constants.JailTimeKeygen)
			releaseHeight := ctx.BlockHeight() + jailTime
			reason := "failed to vote keygen in time"
			if err := mgr.Keeper().SetNodeAccountJail(ctx, thorAddr, releaseHeight, reason); err != nil {
				ctx.Logger().Error("fail to set node account jail", "node address", thorAddr, "reason", reason, "error", err)
			}
		}
	}
}

// TssAnteHandler called by the ante handler to gate mempool entry
// and also during deliver. Store changes will persist if this function
// succeeds, regardless of the success of the transaction.
func TssAnteHandler(ctx cosmos.Context, v semver.Version, k keeper.Keeper, msg MsgTssPool) error {
	return nil
}
