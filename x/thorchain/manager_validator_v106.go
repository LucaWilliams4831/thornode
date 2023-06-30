package thorchain

import (
	"errors"
	"fmt"
	"net"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// ValidatorMgrV106 is to manage a list of validators , and rotate them
type ValidatorMgrV106 struct {
	k                  keeper.Keeper
	networkMgr         NetworkManager
	txOutStore         TxOutStore
	eventMgr           EventManager
	existingValidators []string
}

// newValidatorMgrV106 create a new instance of ValidatorMgrV106
func newValidatorMgrV106(k keeper.Keeper, networkMgr NetworkManager, txOutStore TxOutStore, eventMgr EventManager) *ValidatorMgrV106 {
	return &ValidatorMgrV106{
		k:          k,
		networkMgr: networkMgr,
		txOutStore: txOutStore,
		eventMgr:   eventMgr,
	}
}

// BeginBlock when block begin
func (vm *ValidatorMgrV106) BeginBlock(ctx cosmos.Context, mgr Manager, existingValidators []string) error {
	constAccessor := mgr.GetConstants()

	vm.existingValidators = existingValidators
	height := ctx.BlockHeight()
	if height == genesisBlockHeight {
		if err := vm.setupValidatorNodes(ctx, height, constAccessor); err != nil {
			ctx.Logger().Error("fail to setup validator nodes", "error", err)
		}
	}
	if vm.k.RagnarokInProgress(ctx) {
		// ragnarok is in progress, no point to check node rotation
		return nil
	}
	minimumNodesForBFT := constAccessor.GetInt64Value(constants.MinimumNodesForBFT)
	totalActiveNodes, err := vm.k.TotalActiveValidators(ctx)
	if err != nil {
		return err
	}

	churnInterval, err := vm.k.GetMimir(ctx, constants.ChurnInterval.String())
	if churnInterval < 0 || err != nil {
		churnInterval = constAccessor.GetInt64Value(constants.ChurnInterval)
	}

	vaults, err := vm.k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("Failed to get Asgard vaults", "error", err)
		return err
	}

	lastChurnHeight := vm.getLastChurnHeight(ctx)

	// get constants
	desiredValidatorSet, err := vm.k.GetMimir(ctx, constants.DesiredValidatorSet.String())
	if desiredValidatorSet < 0 || err != nil {
		desiredValidatorSet = constAccessor.GetInt64Value(constants.DesiredValidatorSet)
	}
	churnRetryInterval := constAccessor.GetInt64Value(constants.ChurnRetryInterval)
	asgardSize, err := vm.k.GetMimir(ctx, constants.AsgardSize.String())
	if asgardSize < 0 || err != nil {
		asgardSize = constAccessor.GetInt64Value(constants.AsgardSize)
	}

	// calculate if we need to retry a churn because we are overdue for a
	// successful one
	nas, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		return err
	}
	expectedActiveVaults := int64(len(nas)) / asgardSize
	if int64(len(nas))%asgardSize > 0 {
		expectedActiveVaults++
	}
	incompleteChurnCheck := int64(len(vaults)) != expectedActiveVaults
	oldVaultCheck := ctx.BlockHeight()-lastChurnHeight > churnInterval
	onChurnTick := (ctx.BlockHeight()-lastChurnHeight-churnInterval)%churnRetryInterval == 0
	retryChurn := (oldVaultCheck || incompleteChurnCheck) && onChurnTick

	if lastChurnHeight+churnInterval == ctx.BlockHeight() || retryChurn {
		if retryChurn {
			ctx.Logger().Info("Checking for node account rotation... (retry)")
		} else {
			ctx.Logger().Info("Checking for node account rotation...")
		}

		// don't churn if we have retiring asgard vaults that still have funds
		retiringVaults, err := vm.k.GetAsgardVaultsByStatus(ctx, RetiringVault)
		if err != nil {
			return err
		}
		for _, vault := range retiringVaults {
			if vault.HasFunds() {
				ctx.Logger().Info("Skipping rotation due to retiring vaults still have funds.")
				return nil
			}
		}

		// Mark bad, old, low, and old version validators
		if minimumNodesForBFT+2 < int64(totalActiveNodes) {
			redline, err := vm.k.GetMimir(ctx, constants.BadValidatorRedline.String())
			if err != nil || redline < 0 {
				redline = constAccessor.GetInt64Value(constants.BadValidatorRedline)
			}
			minSlashPointsForBadValidator, err := vm.k.GetMimir(ctx, constants.MinSlashPointsForBadValidator.String())
			if err != nil || minSlashPointsForBadValidator < 0 {
				minSlashPointsForBadValidator = constAccessor.GetInt64Value(constants.MinSlashPointsForBadValidator)
			}
			if err := vm.markBadActor(ctx, minSlashPointsForBadValidator, redline); err != nil {
				return err
			}
			if !retryChurn { // Only mark old/low actors on initial churn
				if err := vm.markOldActor(ctx); err != nil {
					return err
				}
				// Only churn out lowest-bond Active node when there are already the maximum number of Active nodes.
				if int64(len(nas)) >= desiredValidatorSet {
					if err := vm.markLowBondActor(ctx); err != nil {
						return err
					}
				}

			}
			// when the active nodes didn't upgrade , boot them out one at a time
			if err := vm.markLowVersionValidators(ctx, constAccessor); err != nil {
				return err
			}
		}

		next, ok, err := vm.nextVaultNodeAccounts(ctx, int(desiredValidatorSet), constAccessor)
		if err != nil {
			return err
		}
		if ok {
			for _, nodeAccSet := range vm.splitNext(ctx, next, asgardSize) {
				if err := vm.networkMgr.TriggerKeygen(ctx, nodeAccSet); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// splits given list of node accounts into separate list of nas, for separate
// asgard vaults
func (vm *ValidatorMgrV106) splitNext(ctx cosmos.Context, nas NodeAccounts, asgardSize int64) []NodeAccounts {
	// calculate the number of asgard vaults we'll need to support the given
	// list of node accounts
	groupNum := int64(len(nas)) / asgardSize
	if int64(len(nas))%asgardSize > 0 {
		groupNum++
	}

	// sort by bond size, descending. This should help ensure that bond
	// distribution between asgard vaults is somewhat close to each other,
	// while still maintain that each asgard has the same number of members
	sort.SliceStable(nas, func(i, j int) bool {
		return nas[i].Bond.GT(nas[j].Bond)
	})

	groups := make([]NodeAccounts, groupNum)
	for i, na := range nas {
		groups[i%len(groups)] = append(groups[i%len(groups)], na)
	}

	// sanity checks
	for i, group := range groups {
		// ensure no group is more than the max
		if int64(len(group)) > asgardSize {
			ctx.Logger().Info("Skipping rotation due to an Asgard group is larger than the max size.")
			return nil
		}
		// ensure no group is less than the min
		if int64(len(group)) < 2 {
			ctx.Logger().Info("Skipping rotation due to an Asgard group is smaller than the min size.")
			return nil
		}
		// ensure a single group is significantly larger than another
		if i > 0 {
			diff := len(groups[i]) - len(groups[i-1])
			if diff < 0 {
				diff = -diff
			}
			if diff > 1 {
				ctx.Logger().Info("Skipping rotation due to an Asgard groups having dissimilar membership size.")
				return nil
			}
		}
	}

	return groups
}

// EndBlock when block commit
func (vm *ValidatorMgrV106) EndBlock(ctx cosmos.Context, mgr Manager) []abci.ValidatorUpdate {
	height := ctx.BlockHeight()
	activeNodes, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get all active nodes", "error", err)
		return nil
	}

	// when ragnarok is in progress, just process ragnarok
	if vm.k.RagnarokInProgress(ctx) {
		// process ragnarok
		if err := vm.processRagnarok(ctx, mgr); err != nil {
			ctx.Logger().Error("fail to process ragnarok protocol", "error", err)
		}
		return nil
	}

	newNodes, removedNodes, err := vm.getChangedNodes(ctx, activeNodes)
	if err != nil {
		ctx.Logger().Error("fail to get node changes", "error", err)
		return nil
	}

	artificialRagnarokBlockHeight, err := vm.k.GetMimir(ctx, constants.ArtificialRagnarokBlockHeight.String())
	if artificialRagnarokBlockHeight < 0 || err != nil {
		artificialRagnarokBlockHeight = mgr.GetConstants().GetInt64Value(constants.ArtificialRagnarokBlockHeight)
	}
	if artificialRagnarokBlockHeight > 0 {
		ctx.Logger().Info("Artificial Ragnarok is planned", "height", artificialRagnarokBlockHeight)
	}
	minimumNodesForBFT := mgr.GetConstants().GetInt64Value(constants.MinimumNodesForBFT)
	nodesAfterChange := len(activeNodes) + len(newNodes) - len(removedNodes)
	if (len(activeNodes) >= int(minimumNodesForBFT) && nodesAfterChange < int(minimumNodesForBFT)) ||
		(artificialRagnarokBlockHeight > 0 && ctx.BlockHeight() >= artificialRagnarokBlockHeight) {
		// THORNode don't have enough validators for BFT

		// Check we're not migrating funds
		retiring, err := vm.k.GetAsgardVaultsByStatus(ctx, RetiringVault)
		if err != nil {
			ctx.Logger().Error("fail to get retiring vaults", "error", err)
		}

		if len(retiring) == 0 { // wait until all funds are migrated before starting ragnarok
			if err := vm.processRagnarok(ctx, mgr); err != nil {
				ctx.Logger().Error("fail to process ragnarok protocol", "error", err)
			}
			return nil
		}
	}

	// If there's been a churn (the nodes have changed), continue; if there hasn't, end the function.
	if len(newNodes) == 0 && len(removedNodes) == 0 {
		return nil
	}

	// payout all active node accounts their rewards
	// This including nodes churning out, and takes place before changing the activity status below.
	if err := vm.distributeBondReward(ctx, mgr); err != nil {
		ctx.Logger().Error("fail to pay node bond rewards", "error", err)
	}

	validators := make([]abci.ValidatorUpdate, 0, len(newNodes)+len(removedNodes))
	for _, na := range newNodes {
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("UpdateNodeAccountStatus",
				cosmos.NewAttribute("Address", na.NodeAddress.String()),
				cosmos.NewAttribute("Former:", na.Status.String()),
				cosmos.NewAttribute("Current:", NodeActive.String())))
		na.UpdateStatus(NodeActive, height)
		na.LeaveScore = 0
		na.RequestedToLeave = false

		vm.k.ResetNodeAccountSlashPoints(ctx, na.NodeAddress)
		if err := vm.k.SetNodeAccount(ctx, na); err != nil {
			ctx.Logger().Error("fail to save node account", "error", err)
		}
		pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeConsPub, na.ValidatorConsPubKey)
		if err != nil {
			ctx.Logger().Error("fail to parse consensus public key", "key", na.ValidatorConsPubKey, "error", err)
			continue
		}
		validators = append(validators, abci.Ed25519ValidatorUpdate(pk.Bytes(), 100))
	}
	removedNodeKeys := common.PubKeys{}
	for _, na := range removedNodes {
		// retrieve the node from key value store again , as the node might get paid bond, thus the node properties has been changed
		nodeRemove, err := vm.k.GetNodeAccount(ctx, na.NodeAddress)
		if err != nil {
			ctx.Logger().Error("fail to get node account from key value store", "node address", na.NodeAddress)
			continue
		}

		status := NodeStandby
		if nodeRemove.ForcedToLeave {
			status = NodeDisabled
		}
		// if removed node requested to leave , unset it , so they can join back again
		if nodeRemove.RequestedToLeave {
			nodeRemove.RequestedToLeave = false
		}
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("UpdateNodeAccountStatus",
				cosmos.NewAttribute("Address", nodeRemove.NodeAddress.String()),
				cosmos.NewAttribute("Former:", nodeRemove.Status.String()),
				cosmos.NewAttribute("Current:", status.String())))
		nodeRemove.UpdateStatus(status, height)
		if err := vm.k.SetNodeAccount(ctx, nodeRemove); err != nil {
			ctx.Logger().Error("fail to save node account", "error", err)
		}

		// return yggdrasil funds
		if err := vm.RequestYggReturn(ctx, nodeRemove, mgr); err != nil {
			ctx.Logger().Error("fail to request yggdrasil funds return", "error", err)
		}

		pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeConsPub, nodeRemove.ValidatorConsPubKey)
		if err != nil {
			ctx.Logger().Error("fail to parse consensus public key", "key", nodeRemove.ValidatorConsPubKey, "error", err)
			continue
		}
		caddr := sdk.ValAddress(pk.Address()).String()
		removedNodeKeys = append(removedNodeKeys, nodeRemove.PubKeySet.Secp256k1)
		found := false
		for _, exist := range vm.existingValidators {
			if exist == caddr {
				validators = append(validators, abci.Ed25519ValidatorUpdate(pk.Bytes(), 0))
				found = true
				break
			}
		}
		if !found {
			ctx.Logger().Info("validator is not present, so can't be removed", "validator address", caddr)
		}

	}
	if err := vm.checkContractUpgrade(ctx, mgr, removedNodeKeys); err != nil {
		ctx.Logger().Error("fail to check contract upgrade", "error", err)
	}
	// reset all nodes in ready status back to standby status
	ready, err := vm.k.ListValidatorsByStatus(ctx, NodeReady)
	if err != nil {
		ctx.Logger().Error("fail to get list of ready node accounts", "error", err)
	}
	for _, na := range ready {
		na.UpdateStatus(NodeStandby, ctx.BlockHeight())
		if err := vm.k.SetNodeAccount(ctx, na); err != nil {
			ctx.Logger().Error("fail to set node account", "error", err)
		}
	}
	return validators
}

// checkContractUpgrade for those chains that support smart contract, it the contract get changed , then the network have to recall all
// the yggdrasil fund for chain, take ETH for example , if the smart contract used to process transactions on ETH chain get updated for some reason
// then the network has to recall all the fund on ETH(include both ETH and ERC20)
func (vm *ValidatorMgrV106) checkContractUpgrade(ctx cosmos.Context, mgr Manager, removedNodeKeys common.PubKeys) error {
	activeVaults, err := vm.k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return fmt.Errorf("fail to get active asgards: %w", err)
	}
	retiringVaults, err := vm.k.GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		return fmt.Errorf("fail to get retiring asgards: %w", err)
	}

	// no active asgard vault , not possible
	if len(activeVaults) == 0 {
		return nil
	}
	if len(retiringVaults) == 0 {
		return nil
	}
	oldChainRouters := retiringVaults[0].Routers
	newChainRouters := activeVaults[0].Routers
	chains := common.Chains{}
	for _, old := range oldChainRouters {
		found := false
		for _, n := range newChainRouters {
			if n.Chain.Equals(old.Chain) {
				found = true
				if !n.Router.Equals(old.Router) {
					// contract address get changed , need to recall funds
					chains = append(chains, n.Chain)
				}
			}
		}
		if !found {
			chains = append(chains, old.Chain)
		}
	}

	for _, c := range chains.Distinct() {
		if err := vm.networkMgr.RecallChainFunds(ctx, c, mgr, removedNodeKeys); err != nil {
			ctx.Logger().Error("fail to recall chain fund", "error", err, "chain", c.String())
		}
	}
	return nil
}

// getChangedNodes to identify which node had been removed ,and which one had been added
// newNodes , removed nodes,err
func (vm *ValidatorMgrV106) getChangedNodes(ctx cosmos.Context, activeNodes NodeAccounts) (NodeAccounts, NodeAccounts, error) {
	var newActive NodeAccounts    // store the list of new active users
	var removedNodes NodeAccounts // nodes that had been removed

	activeVaults, err := vm.k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active asgards", "error", err)
		return newActive, removedNodes, fmt.Errorf("fail to get active asgards: %w", err)
	}
	if len(activeVaults) == 0 {
		return newActive, removedNodes, errors.New("no active vault")
	}
	var membership common.PubKeys
	for _, vault := range activeVaults {
		membership = append(membership, vault.GetMembership()...)
	}

	// find active node accounts that are no longer active
	for _, na := range activeNodes {
		found := false
		for _, vault := range activeVaults {
			if vault.Contains(na.PubKeySet.Secp256k1) {
				found = true
				break
			}
		}
		if na.ForcedToLeave {
			found = false
		}
		if !found && len(membership) > 0 {
			removedNodes = append(removedNodes, na)
		}
	}

	// find ready nodes that change to active
	for _, pk := range membership {
		na, err := vm.k.GetNodeAccountByPubKey(ctx, pk)
		if err != nil {
			ctx.Logger().Error("fail to get node account", "error", err)
			continue
		}
		// Disabled account can't go back , it should not be include in the newActive
		if na.Status != NodeActive && na.Status != NodeDisabled {
			newActive = append(newActive, na)
		}
	}

	return newActive, removedNodes, nil
}

// payNodeAccountBondAward pay
func (vm *ValidatorMgrV106) payNodeAccountBondAward(ctx cosmos.Context, lastChurnHeight int64, na NodeAccount, totalBondReward, totalEffectiveBond, bondHardCap cosmos.Uint, mgr Manager) error {
	if na.ActiveBlockHeight == 0 || na.Bond.IsZero() {
		return nil
	}

	network, err := vm.k.GetNetwork(ctx)
	if err != nil {
		return fmt.Errorf("fail to get network: %w", err)
	}

	slashPts, err := vm.k.GetNodeAccountSlashPoints(ctx, na.NodeAddress)
	if err != nil {
		return fmt.Errorf("fail to get node slash points: %w", err)
	}

	// Find number of blocks since the last churn (the last bond reward payout)
	totalActiveBlocks := ctx.BlockHeight() - lastChurnHeight

	// find number of blocks they were well behaved (ie active - slash points)
	earnedBlocks := totalActiveBlocks - slashPts
	if earnedBlocks < 0 {
		earnedBlocks = 0
	}

	naEffectiveBond := na.Bond
	if naEffectiveBond.GT(bondHardCap) {
		naEffectiveBond = bondHardCap
	}

	// reward = totalBondReward * (naEffectiveBond / totalEffectiveBond) * (unslashed blocks since last churn / blocks since last churn)
	reward := common.GetUncappedShare(naEffectiveBond, totalEffectiveBond, totalBondReward)
	reward = common.GetUncappedShare(cosmos.NewUint(uint64(earnedBlocks)), cosmos.NewUint(uint64(totalActiveBlocks)), reward)

	// Record the node operator's bond (in bond provider form) before the reward
	bp, err := vm.k.GetBondProviders(ctx, na.NodeAddress)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get bond providers(%s)", na.NodeAddress))
	}
	nodeOperatorAccAddr, err := na.BondAddress.AccAddress()
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to parse bond address(%s)", na.BondAddress))
	}
	err = passiveBackfill(ctx, mgr, na, &bp)
	if err != nil {
		return err
	}
	// Ensure that previous rewards are already accounted for
	bp.Adjust(vm.k.GetVersion(), na.Bond)
	nodeOperatorProvider := bp.Get(nodeOperatorAccAddr)
	// Sanity check that the node operator is accounted for
	if nodeOperatorProvider.IsEmpty() {
		return ErrInternal(err, fmt.Sprintf("node operator address(%s) not listed in bond providers", na.BondAddress))
	}
	lastNodeOperatorProviderBond := nodeOperatorProvider.Bond

	// Add to their bond the amount rewarded
	na.Bond = na.Bond.Add(reward)

	// Minus the number of rune THORNode have awarded them
	network.BondRewardRune = common.SafeSub(network.BondRewardRune, reward)

	// Minus the number of units na has (do not include slash points)
	network.TotalBondUnits = common.SafeSub(
		network.TotalBondUnits,
		cosmos.NewUint(uint64(totalActiveBlocks)),
	)

	if err := vm.k.SetNetwork(ctx, network); err != nil {
		return fmt.Errorf("fail to save network data: %w", err)
	}

	// minus slash points used in this calculation
	vm.k.SetNodeAccountSlashPoints(ctx, na.NodeAddress, slashPts-totalActiveBlocks)

	// Distribute reward to bond providers and remove the NodeOperatorFee portion for node operator payout.
	// (This is the full fee from other bond providers' rewards, plus an equivalent proportion of the node operator's rewards.)
	bp.Adjust(vm.k.GetVersion(), na.Bond)
	nodeOperatorProvider = bp.Get(nodeOperatorAccAddr)
	nodeOperatorFees := common.GetSafeShare(bp.NodeOperatorFee, cosmos.NewUint(10000), reward)
	// Sanity check:  Fees to pay out should never exceed the increase of the node operator's bond.
	nodeOperatorBondIncrease := common.SafeSub(nodeOperatorProvider.Bond, lastNodeOperatorProviderBond)
	if nodeOperatorFees.GT(nodeOperatorBondIncrease) {
		nodeOperatorFees = nodeOperatorBondIncrease
	}
	if !nodeOperatorFees.IsZero() {
		na.Bond = common.SafeSub(na.Bond, nodeOperatorFees)
		bp.Unbond(nodeOperatorFees, nodeOperatorAccAddr)
	}

	// Set node account and bond providers, then emit BondReward event (for the full pre-payout reward)
	if err := vm.k.SetNodeAccount(ctx, na); err != nil {
		return fmt.Errorf("fail to save node account: %w", err)
	}
	if err := mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to save bond providers(%s)", bp.NodeAddress.String()))
	}
	tx := common.Tx{}
	tx.ID = common.BlankTxID
	tx.ToAddress = na.BondAddress
	bondRewardEvent := NewEventBond(reward, BondReward, tx)
	if err := mgr.EventMgr().EmitEvent(ctx, bondRewardEvent); err != nil {
		ctx.Logger().Error("fail to emit bond event", "error", err)
	}

	// Transfer node operator fees
	if !nodeOperatorFees.IsZero() {
		coin := common.NewCoin(common.RuneNative, nodeOperatorFees)
		sdkErr := vm.k.SendFromModuleToAccount(ctx, BondName, nodeOperatorAccAddr, common.NewCoins(coin))
		if sdkErr != nil {
			return errors.New(sdkErr.Error())
		}

		// The bond is being returned from the node to the node operator,
		// so reflect that (and unambiguously identify them) with the FromAddress and ToAddress.
		fromAddress, err := common.NewAddress(na.NodeAddress.String())
		if err != nil {
			return fmt.Errorf("fail to parse node address: %w", err)
		}
		// emit BondReturned event
		fakeTx := common.Tx{}
		fakeTx.ID = common.BlankTxID
		fakeTx.FromAddress = fromAddress
		fakeTx.ToAddress = na.BondAddress
		bondReturnedEvent := NewEventBond(nodeOperatorFees, BondReturned, fakeTx)
		if err := mgr.EventMgr().EmitEvent(ctx, bondReturnedEvent); err != nil {
			ctx.Logger().Error("fail to emit bond event", "error", err)
		}
	}

	return nil
}

// determines when/if to run each part of the ragnarok process
func (vm *ValidatorMgrV106) processRagnarok(ctx cosmos.Context, mgr Manager) error {
	// execute Ragnarok protocol, no going back
	// THORNode have to request the fund back now, because once it get to the rotate block height ,
	// THORNode won't have validators anymore
	ragnarokHeight, err := vm.k.GetRagnarokBlockHeight(ctx)
	if err != nil {
		return fmt.Errorf("fail to get ragnarok height: %w", err)
	}

	if ragnarokHeight == 0 {
		ragnarokHeight = ctx.BlockHeight()
		vm.k.SetRagnarokBlockHeight(ctx, ragnarokHeight)
		if err := vm.ragnarokProtocolStage1(ctx, mgr); err != nil {
			return fmt.Errorf("fail to execute ragnarok protocol step 1: %w", err)
		}
		if err := vm.distributeBondReward(ctx, mgr); err != nil {
			return fmt.Errorf("when ragnarok triggered, fail to give all active node bond reward %w", err)
		}
		return nil
	}

	nth, err := vm.k.GetRagnarokNth(ctx)
	if err != nil {
		return fmt.Errorf("fail to get ragnarok nth: %w", err)
	}

	position, err := vm.k.GetRagnarokWithdrawPosition(ctx)
	if err != nil {
		return fmt.Errorf("fail to get ragnarok position: %w", err)
	}
	if !position.IsEmpty() {
		if err := vm.ragnarokPools(ctx, nth, mgr); err != nil {
			ctx.Logger().Error("fail to ragnarok pools", "error", err)
		}
		return nil
	}

	// check if we have any pending ragnarok transactions
	pending, err := vm.k.GetRagnarokPending(ctx)
	if err != nil {
		return fmt.Errorf("fail to get ragnarok pending: %w", err)
	}
	if pending > 0 {
		txOutQueue, err := vm.getPendingTxOut(ctx, mgr.GetConstants())
		if err != nil {
			ctx.Logger().Error("fail to get pending tx out item", "error", err)
			return nil
		}
		if txOutQueue > 0 {
			ctx.Logger().Info("awaiting previous ragnarok transaction to clear before continuing", "nth", nth, "count", pending)
			return nil
		}
	}

	nth++ // increment by 1
	ctx.Logger().Info("starting next ragnarok iteration", "iteration", nth)
	err = vm.ragnarokProtocolStage2(ctx, nth, mgr)
	if err != nil {
		ctx.Logger().Error("fail to execute ragnarok protocol step 2", "error", err)
		return err
	}
	vm.k.SetRagnarokNth(ctx, nth)

	return nil
}

func (vm *ValidatorMgrV106) getPendingTxOut(ctx cosmos.Context, constAccessor constants.ConstantValues) (int64, error) {
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := ctx.BlockHeight() - signingTransactionPeriod
	count := int64(0)
	for height := startHeight; height <= ctx.BlockHeight(); height++ {
		txs, err := vm.k.GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			return 0, fmt.Errorf("fail to get tx out array from key value store: %w", err)
		}
		for _, tx := range txs.TxArray {
			if tx.OutHash.IsEmpty() {
				count++
			}
		}
	}
	return count, nil
}

// ragnarokProtocolStage1 - request all yggdrasil pool to return the fund
// when THORNode observe the node return fund successfully, the node's bound will be refund.
func (vm *ValidatorMgrV106) ragnarokProtocolStage1(ctx cosmos.Context, mgr Manager) error {
	return vm.recallYggFunds(ctx, mgr)
}

func (vm *ValidatorMgrV106) ragnarokProtocolStage2(ctx cosmos.Context, nth int64, mgr Manager) error {
	// Ragnarok Protocol
	// If THORNode can no longer be BFT, do a graceful shutdown of the entire network.
	// 1) THORNode will request all yggdrasil pool to return fund , if THORNode don't have yggdrasil pool THORNode will go to step 3 directly
	// 2) upon receiving the yggdrasil fund,  THORNode will refund the validator's bond
	// 3) once all yggdrasil fund get returned, return all fund to liquidity providers

	// refund bonders
	if err := vm.ragnarokBond(ctx, nth, mgr); err != nil {
		ctx.Logger().Error("fail to ragnarok bond", "error", err)
	}

	// refund liquidity providers. This is last to ensure there is likely gas for the
	// returning bond and reserve
	if err := vm.ragnarokPools(ctx, nth, mgr); err != nil {
		ctx.Logger().Error("fail to ragnarok pools", "error", err)
	}

	return nil
}

func (vm *ValidatorMgrV106) distributeBondReward(ctx cosmos.Context, mgr Manager) error {
	var resultErr error
	active, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		return fmt.Errorf("fail to get all active node account: %w", err)
	}

	// Note that unlike estimated CurrentAward distribution in querier.go ,
	// this estimate treats lastChurnHeight as the active_block_height of the youngest active node,
	// rather than the block_height of the first (oldest) Asgard vault.
	// As an example, note from the below URLs that these 5293733 and 5293728 respectively in block 5336942.
	// https://thornode.ninerealms.com/thorchain/nodes?height=5336942
	// (Nodes .cxmy and .uy3a .)
	// https://thornode.ninerealms.com/thorchain/vaults/asgard?height=5336942
	lastChurnHeight := int64(0)
	for _, node := range active {
		if node.ActiveBlockHeight > lastChurnHeight {
			lastChurnHeight = node.ActiveBlockHeight
		}
	}

	bondHardCap := getHardBondCap(active)

	totalEffectiveBond := cosmos.ZeroUint()
	for _, item := range active {
		b := item.Bond
		if item.Bond.GT(bondHardCap) {
			b = bondHardCap
		}

		totalEffectiveBond = totalEffectiveBond.Add(b)
	}

	network, err := vm.k.GetNetwork(ctx)
	if err != nil {
		return fmt.Errorf("fail to get network: %w", err)
	}

	for _, item := range active {
		if err := vm.payNodeAccountBondAward(ctx, lastChurnHeight, item, network.BondRewardRune, totalEffectiveBond, bondHardCap, mgr); err != nil {
			resultErr = err
			ctx.Logger().Error("fail to pay node account bond award", "node address", item.NodeAddress.String(), "error", err)
		}
	}
	return resultErr
}

func (vm *ValidatorMgrV106) ragnarokBond(ctx cosmos.Context, nth int64, mgr Manager) error {
	// bond should be returned on the back 10, not the first 10
	nth -= 10
	if nth < 1 {
		return nil
	}

	nas, err := vm.k.ListValidatorsWithBond(ctx)
	if err != nil {
		ctx.Logger().Error("can't get nodes", "error", err)
		return err
	}
	// nth * 10 == the amount of the bond we want to send
	for _, na := range nas {
		if na.Bond.IsZero() {
			continue
		}
		if vm.k.VaultExists(ctx, na.PubKeySet.Secp256k1) {
			ygg, err := vm.k.GetVault(ctx, na.PubKeySet.Secp256k1)
			if err != nil {
				return err
			}
			if ygg.HasFunds() {
				ctx.Logger().Info("skip bond refund due to remaining funds", "node address", na.NodeAddress)
				continue
			}
		}

		if nth >= 9 { // cap at 10
			nth = 10
		}
		amt := na.Bond.MulUint64(uint64(nth)).QuoUint64(10)

		// refund bond
		txOutItem := TxOutItem{
			Chain:      common.RuneAsset().Chain,
			ToAddress:  na.BondAddress,
			InHash:     common.BlankTxID,
			Coin:       common.NewCoin(common.RuneAsset(), amt),
			Memo:       NewRagnarokMemo(ctx.BlockHeight()).String(),
			ModuleName: BondName,
		}
		ok, err := vm.txOutStore.TryAddTxOutItem(ctx, mgr, txOutItem, cosmos.ZeroUint())
		if err != nil {
			if !errors.Is(err, ErrNotEnoughToPayFee) {
				return err
			}
			ok = true
		}
		if !ok {
			continue
		}

		// add a pending rangarok transaction
		pending, err := vm.k.GetRagnarokPending(ctx)
		if err != nil {
			return fmt.Errorf("fail to get ragnarok pending: %w", err)
		}
		vm.k.SetRagnarokPending(ctx, pending+1)

		na.Bond = common.SafeSub(na.Bond, amt)
		if err := vm.k.SetNodeAccount(ctx, na); err != nil {
			return err
		}

		tx := common.Tx{}
		tx.ID = common.BlankTxID
		tx.FromAddress = na.BondAddress
		bondEvent := NewEventBond(amt, BondCost, tx)
		if err := mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
			return fmt.Errorf("fail to emit bond event: %w", err)
		}
	}

	return nil
}

func (vm *ValidatorMgrV106) ragnarokPools(ctx cosmos.Context, nth int64, mgr Manager) error {
	nas, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		return fmt.Errorf("fail to get active nodes: %w", err)
	}
	if len(nas) == 0 {
		return fmt.Errorf("can't find any active nodes")
	}
	na := nas[0]

	position, err := vm.k.GetRagnarokWithdrawPosition(ctx)
	if err != nil {
		return fmt.Errorf("fail to get ragnarok position: %w", err)
	}
	basisPoints := MaxWithdrawBasisPoints
	// go through all the pools
	pools, err := vm.k.GetPools(ctx)
	if err != nil {
		return fmt.Errorf("fail to get pools: %w", err)
	}
	// set all pools to staged status
	for _, pool := range pools {
		if pool.Status != PoolStaged {
			poolEvent := NewEventPool(pool.Asset, PoolStaged)
			if err := vm.eventMgr.EmitEvent(ctx, poolEvent); err != nil {
				ctx.Logger().Error("fail to emit pool event", "error", err)
			}

			pool.Status = PoolStaged
			if err := vm.k.SetPool(ctx, pool); err != nil {
				return fmt.Errorf("fail to set pool %s to Stage status: %w", pool.Asset, err)
			}
		}
	}

	// the following line is pointless, granted. But in this case, removing it
	// would cause a consensus failure
	_ = vm.k.GetLowestActiveVersion(ctx)

	nextPool := false
	maxWithdrawsPerBlock := 20
	count := 0

	for i := len(pools) - 1; i >= 0; i-- { // iterate backwards
		pool := pools[i]

		if nextPool { // we've iterated to the next pool after our position pool
			position.Pool = pool.Asset
		}

		if !position.Pool.IsEmpty() && !pool.Asset.Equals(position.Pool) {
			continue
		}

		nextPool = true
		position.Pool = pool.Asset

		// withdraw gas asset pool on the back 10 nths
		if nth <= 10 && pool.Asset.IsGasAsset() {
			continue
		}

		j := int64(-1)
		iterator := vm.k.GetLiquidityProviderIterator(ctx, pool.Asset)
		for ; iterator.Valid(); iterator.Next() {
			j++
			if j == position.Number {
				position.Number++
				var lp LiquidityProvider
				if err := vm.k.Cdc().Unmarshal(iterator.Value(), &lp); err != nil {
					ctx.Logger().Error("fail to unmarshal liquidity provider", "error", err)
					continue
				}

				if lp.Units.IsZero() {
					continue
				}
				var withdrawAddr common.Address
				withdrawAsset := common.EmptyAsset
				if !lp.RuneAddress.IsEmpty() {
					withdrawAddr = lp.RuneAddress
					// if liquidity provider only add RUNE , then asset address will be empty
					if lp.AssetAddress.IsEmpty() {
						withdrawAsset = common.RuneAsset()
					}
				} else {
					// if liquidity provider only add Asset, then RUNE Address will be empty
					withdrawAddr = lp.AssetAddress
					withdrawAsset = lp.Asset
				}
				withdrawMsg := NewMsgWithdrawLiquidity(
					common.GetRagnarokTx(pool.Asset.Chain, withdrawAddr, withdrawAddr),
					withdrawAddr,
					cosmos.NewUint(uint64(basisPoints)),
					pool.Asset,
					withdrawAsset,
					na.NodeAddress,
				)

				handler := NewInternalHandler(mgr)
				_, err = handler(ctx, withdrawMsg)
				if err != nil {
					ctx.Logger().Error("fail to withdraw", "liquidity provider", lp.RuneAddress, "error", err)
				} else if !withdrawAsset.Equals(common.RuneAsset()) {
					// when withdraw asset is only RUNE , then it should process more , because RUNE asset doesn't leave THORChain
					count++
					pending, err := vm.k.GetRagnarokPending(ctx)
					if err != nil {
						return fmt.Errorf("fail to get ragnarok pending: %w", err)
					}
					vm.k.SetRagnarokPending(ctx, pending+1)
					if count >= maxWithdrawsPerBlock {
						break
					}
				}
			}
		}
		if err := iterator.Close(); err != nil {
			ctx.Logger().Error("fail to close iterator", "error", err)
		}
		if count >= maxWithdrawsPerBlock {
			break
		}
		position.Number = 0
	}

	if count < maxWithdrawsPerBlock { // we've completed all pools/liquidity providers, reset the position
		position = RagnarokWithdrawPosition{}
	}
	vm.k.SetRagnarokWithdrawPosition(ctx, position)

	return nil
}

// RequestYggReturn request the node that had been removed (yggdrasil) to return their fund
func (vm *ValidatorMgrV106) RequestYggReturn(ctx cosmos.Context, node NodeAccount, mgr Manager) error {
	if !vm.k.VaultExists(ctx, node.PubKeySet.Secp256k1) {
		return nil
	}
	ygg, err := vm.k.GetVault(ctx, node.PubKeySet.Secp256k1)
	if err != nil {
		return fmt.Errorf("fail to get yggdrasil: %w", err)
	}
	if ygg.IsAsgard() {
		return nil
	}
	if !ygg.HasFunds() {
		return nil
	}

	chains := make(common.Chains, 0)

	active, err := vm.k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return err
	}

	retiring, err := vm.k.GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		return err
	}

	for _, v := range append(active, retiring...) {
		chains = append(chains, v.GetChains()...)
	}
	chains = chains.Distinct()

	signingTransactionPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
	// select vault that is most secure
	vault := vm.k.GetMostSecure(ctx, active, signingTransactionPeriod)
	if vault.IsEmpty() {
		return fmt.Errorf("unable to determine asgard vault")
	}
	for _, chain := range chains {
		if chain.Equals(common.THORChain) {
			continue
		}
		if !ygg.HasFundsForChain(chain) {
			ctx.Logger().Info("there is not fund for chain, no need for yggdrasil return", "chain", chain)
			continue
		}
		toAddr, err := vault.PubKey.GetAddress(chain)
		if err != nil {
			return err
		}
		if !toAddr.IsEmpty() {
			txOutItem := TxOutItem{
				Chain:       chain,
				ToAddress:   toAddr,
				InHash:      common.BlankTxID,
				VaultPubKey: ygg.PubKey,
				Coin:        common.NewCoin(common.RuneAsset(), cosmos.ZeroUint()),
				Memo:        NewYggdrasilReturn(ctx.BlockHeight()).String(),
				GasRate:     int64(mgr.GasMgr().GetGasRate(ctx, chain).Uint64()),
				// DO NOT specify MaxGas , for yggdrasil return , should allow node to spend more on gas , for example ETH, return multiple
				// ERC20 token / ETH at the same time cost a lot gas
			}

			// yggdrasil- will not set coin field here, when signer see a TxOutItem that has memo "yggdrasil-" it will query the chain
			// and find out all the remaining assets , and fill in the field
			if err := vm.txOutStore.UnSafeAddTxOutItem(ctx, mgr, txOutItem); err != nil {
				return err
			}
		}
	}

	return nil
}

func (vm *ValidatorMgrV106) recallYggFunds(ctx cosmos.Context, mgr Manager) error {
	iter := vm.k.GetVaultIterator(ctx)
	defer iter.Close()
	vaults := Vaults{}
	for ; iter.Valid(); iter.Next() {
		var vault Vault
		if err := vm.k.Cdc().Unmarshal(iter.Value(), &vault); err != nil {
			return fmt.Errorf("fail to unmarshal vault, %w", err)
		}
		if vault.IsYggdrasil() && vault.HasFunds() {
			vaults = append(vaults, vault)
		}
	}

	if len(vaults) == 0 {
		return nil
	}

	for _, vault := range vaults {
		na, err := vm.k.GetNodeAccountByPubKey(ctx, vault.PubKey)
		if err != nil {
			ctx.Logger().Error("fail to get node account", "error", err)
			continue
		}
		if err := vm.RequestYggReturn(ctx, na, mgr); err != nil {
			return fmt.Errorf("fail to request yggdrasil fund back: %w", err)
		}
	}
	ctx.Logger().Info("some yggdrasil vaults (%d) still have funds", len(vaults))
	return nil
}

// setupValidatorNodes it is one off it only get called when genesis
func (vm *ValidatorMgrV106) setupValidatorNodes(ctx cosmos.Context, height int64, constAccessor constants.ConstantValues) error {
	if height != genesisBlockHeight {
		ctx.Logger().Info("only need to setup validator node when start up", "height", height)
		return nil
	}

	iter := vm.k.GetNodeAccountIterator(ctx)
	defer iter.Close()
	readyNodes := NodeAccounts{}
	activeCandidateNodes := NodeAccounts{}
	for ; iter.Valid(); iter.Next() {
		var na NodeAccount
		if err := vm.k.Cdc().Unmarshal(iter.Value(), &na); err != nil {
			return fmt.Errorf("fail to unmarshal node account, %w", err)
		}
		// when THORNode first start , THORNode only care about these two status
		switch na.Status {
		case NodeReady:
			readyNodes = append(readyNodes, na)
		case NodeActive:
			activeCandidateNodes = append(activeCandidateNodes, na)
		}
	}
	totalActiveValidators := len(activeCandidateNodes)
	totalNominatedValidators := len(readyNodes)
	if totalActiveValidators == 0 && totalNominatedValidators == 0 {
		return errors.New("no validators available")
	}

	sort.Sort(activeCandidateNodes)
	sort.Sort(readyNodes)
	activeCandidateNodes = append(activeCandidateNodes, readyNodes...)
	desiredValidatorSet, err := vm.k.GetMimir(ctx, constants.DesiredValidatorSet.String())
	if desiredValidatorSet < 0 || err != nil {
		desiredValidatorSet = constAccessor.GetInt64Value(constants.DesiredValidatorSet)
	}
	for idx, item := range activeCandidateNodes {
		if int64(idx) < desiredValidatorSet {
			item.UpdateStatus(NodeActive, ctx.BlockHeight())
		} else {
			item.UpdateStatus(NodeStandby, ctx.BlockHeight())
		}
		if err := vm.k.SetNodeAccount(ctx, item); err != nil {
			return fmt.Errorf("fail to save node account: %w", err)
		}
	}
	return nil
}

func (vm *ValidatorMgrV106) getLastChurnHeight(ctx cosmos.Context) int64 {
	vaults, err := vm.k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("Failed to get Asgard vaults", "error", err)
		return ctx.BlockHeight()
	}
	// calculate last churn block height
	var lastChurnHeight int64 // the last block height we had a successful churn
	for _, vault := range vaults {
		if vault.BlockHeight > lastChurnHeight {
			lastChurnHeight = vault.BlockHeight
		}
	}
	return lastChurnHeight
}

func (vm *ValidatorMgrV106) getScore(ctx cosmos.Context, slashPts, lastChurnHeight int64) cosmos.Uint {
	// get to the 8th decimal point, but keep numbers integers for safer math
	score := cosmos.NewUint(uint64((ctx.BlockHeight() - lastChurnHeight) * common.One))
	if slashPts == 0 {
		return score
	}
	return score.QuoUint64(uint64(slashPts))
}

// Iterate over active node accounts, finding bad actors with high slash points
func (vm *ValidatorMgrV106) findBadActors(ctx cosmos.Context, minSlashPointsForBadValidator, badValidatorRedline int64) (NodeAccounts, error) {
	badActors := make(NodeAccounts, 0)
	nas, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		return badActors, err
	}

	if len(nas) == 0 {
		return nil, nil
	}

	// NOTE: Our score gives a numerical representation of the behavior our a
	// node account. The lower the score, the worse behavior. The score is
	// determined by relative to how many slash points they have over how long
	// they have been an active node account.
	type badTracker struct {
		Score       cosmos.Uint
		NodeAccount NodeAccount
	}
	tracker := make([]badTracker, 0, len(nas))
	totalScore := cosmos.ZeroUint()

	// Find bad actor relative to age / slashpoints
	lastChurnHeight := vm.getLastChurnHeight(ctx)
	for _, na := range nas {
		slashPts, err := vm.k.GetNodeAccountSlashPoints(ctx, na.NodeAddress)
		if err != nil {
			ctx.Logger().Error("fail to get node slash points", "error", err)
		}

		if slashPts <= minSlashPointsForBadValidator {
			continue
		}

		score := vm.getScore(ctx, slashPts, lastChurnHeight)
		totalScore = totalScore.Add(score)

		tracker = append(tracker, badTracker{
			Score:       score,
			NodeAccount: na,
		})
	}

	if len(tracker) == 0 {
		// no offenders, exit nicely
		return nil, nil
	}

	sort.SliceStable(tracker, func(i, j int) bool {
		return tracker[i].Score.LT(tracker[j].Score)
	})

	// score lower is worse
	avgScore := totalScore.QuoUint64(uint64(len(nas)))

	// NOTE: our redline is a hard line in the sand to determine if a node
	// account is sufficiently bad that it should just be removed now. This
	// ensures that if we have multiple "really bad" node accounts, they all
	// can get removed in the same churn. It is important to note we shouldn't
	// be able to churn out more than 1/3rd of our node accounts in a single
	// churn, as that could threaten the security of the funds. This logic to
	// protect against this is not inside this function.
	redline := avgScore.QuoUint64(uint64(badValidatorRedline))

	// find any node accounts that have crossed the red line
	for _, track := range tracker {
		if redline.GTE(track.Score) {
			badActors = append(badActors, track.NodeAccount)
		}
	}

	// if no one crossed the redline, lets just grab the worse offender
	if len(badActors) == 0 {
		badActors = NodeAccounts{tracker[0].NodeAccount}
	}

	return badActors, nil
}

// Iterate over active node accounts, finding the one that has been active longest
func (vm *ValidatorMgrV106) findOldActor(ctx cosmos.Context) (NodeAccount, error) {
	na := NodeAccount{}
	nas, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		return na, err
	}

	na.StatusSince = ctx.BlockHeight() // set the start status age to "now"
	for _, n := range nas {
		if n.StatusSince < na.StatusSince {
			na = n
		}
	}

	return na, nil
}

// Iterate over active node accounts, finding the one that has the lowest bond
func (vm *ValidatorMgrV106) findLowBondActor(ctx cosmos.Context) (NodeAccount, error) {
	na := NodeAccount{}
	nas, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		return na, err
	}

	if len(nas) > 0 {
		bond := nas[0].Bond
		na = nas[0]
		for _, n := range nas {
			if n.Bond.LT(bond) {
				bond = n.Bond
				na = n
			}
		}
	}

	return na, nil
}

// Mark an old to be churned out
func (vm *ValidatorMgrV106) markActor(ctx cosmos.Context, na NodeAccount, reason string) error {
	if !na.IsEmpty() && na.LeaveScore == 0 {
		ctx.Logger().Info("marked Validator to be churned out", "node address", na.NodeAddress, "reason", reason)
		slashPts, err := vm.k.GetNodeAccountSlashPoints(ctx, na.NodeAddress)
		if err != nil {
			return fmt.Errorf("fail to get node account(%s) slash points: %w", na.NodeAddress, err)
		}
		na.LeaveScore = vm.getScore(ctx, slashPts, vm.getLastChurnHeight(ctx)).Uint64()
		return vm.k.SetNodeAccount(ctx, na)
	}
	return nil
}

// Mark an old actor to be churned out
func (vm *ValidatorMgrV106) markOldActor(ctx cosmos.Context) error {
	na, err := vm.findOldActor(ctx)
	if err != nil {
		return err
	}
	if err := vm.markActor(ctx, na, "for age"); err != nil {
		return err
	}
	return nil
}

// Mark an low bond actor to be churned out
func (vm *ValidatorMgrV106) markLowBondActor(ctx cosmos.Context) error {
	na, err := vm.findLowBondActor(ctx)
	if err != nil {
		return err
	}
	if err := vm.markActor(ctx, na, "for low bond"); err != nil {
		return err
	}
	return nil
}

// Mark a bad actor to be churned out
func (vm *ValidatorMgrV106) markBadActor(ctx cosmos.Context, minSlashPointsForBadValidator, redline int64) error {
	nas, err := vm.findBadActors(ctx, minSlashPointsForBadValidator, redline)
	if err != nil {
		return err
	}
	for _, na := range nas {
		if err := vm.markActor(ctx, na, "for bad behavior"); err != nil {
			return err
		}
	}
	return nil
}

// Mark up to `MaxNodeToChurnOutForLowVersion` nodes as low version
// This will slate them to churn out. `MaxNodeToChurnOutForLowVersion`
// is a Mimir setting that defaults in constants to 1
func (vm *ValidatorMgrV106) markLowVersionValidators(ctx cosmos.Context, constAccessor constants.ConstantValues) error {
	// Get max number of nodes to mark as low version
	maxNodes, err := vm.k.GetMimir(ctx, constants.MaxNodeToChurnOutForLowVersion.String())
	if maxNodes < 0 || err != nil {
		maxNodes = constAccessor.GetInt64Value(constants.MaxNodeToChurnOutForLowVersion)
	}

	nodeAccs, err := vm.findLowVersionValidators(ctx, maxNodes)
	if err != nil {
		return err
	}
	if len(nodeAccs) > 0 {
		for _, na := range nodeAccs {
			if err := vm.markActor(ctx, na, "for version lower than minimum join version"); err != nil {
				return err
			}
		}
	}
	return nil
}

// Finds up to `maxNodesToFind` active validators with version lower than the most "popular" version
func (vm *ValidatorMgrV106) findLowVersionValidators(ctx cosmos.Context, maxNodesToFind int64) (NodeAccounts, error) {
	minimumVersion := vm.k.GetMinJoinVersion(ctx)
	activeNodes, err := vm.k.ListValidatorsByStatus(ctx, NodeActive)
	if err != nil {
		return NodeAccounts{}, err
	}
	nodeAccs := NodeAccounts{}
	for _, na := range activeNodes {
		if na.GetVersion().LT(minimumVersion) {
			nodeAccs = append(nodeAccs, na)
		}
		if len(nodeAccs) == int(maxNodesToFind) {
			return nodeAccs, nil
		}
	}
	return nodeAccs, nil
}

// find any actor that are ready to become "ready" status
func (vm *ValidatorMgrV106) markReadyActors(ctx cosmos.Context, constAccessor constants.ConstantValues) error {
	standby, err := vm.k.ListValidatorsByStatus(ctx, NodeStandby)
	if err != nil {
		return err
	}
	ready, err := vm.k.ListValidatorsByStatus(ctx, NodeReady)
	if err != nil {
		return err
	}

	// check all ready and standby nodes are in "ready" state (upgrade/downgrade as needed)
	for _, na := range append(standby, ready...) {
		status, _ := vm.NodeAccountPreflightCheck(ctx, na, constAccessor)
		na.UpdateStatus(status, ctx.BlockHeight())

		if err := vm.k.SetNodeAccount(ctx, na); err != nil {
			return err
		}
	}

	return nil
}

// NodeAccountPreflightCheck preflight check to find out what the node account's next status will be
func (vm *ValidatorMgrV106) NodeAccountPreflightCheck(ctx cosmos.Context, na NodeAccount, constAccessor constants.ConstantValues) (NodeStatus, error) {
	// ensure banned nodes can't get churned in again
	if na.ForcedToLeave {
		return NodeDisabled, fmt.Errorf("node account has been banned")
	}

	// Check if they've requested to leave
	if na.RequestedToLeave {
		return NodeStandby, fmt.Errorf("node account has requested to leave")
	}

	// Check that the node account has an IP address
	if net.ParseIP(na.IPAddress) == nil {
		return NodeStandby, fmt.Errorf("node account has invalid registered IP address")
	}

	// Check that the node account has an pubkey set
	if na.PubKeySet.IsEmpty() {
		return NodeWhiteListed, fmt.Errorf("node account has not registered their pubkey set")
	}

	// check if node account is whitelisted. This is used for testnet/stagenet environments
	if len(VALIDATORS) > 0 {
		found := false
		for _, val := range VALIDATORS {
			acc, err := cosmos.AccAddressFromBech32(val)
			if err != nil {
				continue
			}
			if acc.Equals(na.NodeAddress) {
				found = true
				break
			}
		}
		if !found {
			return NodeStandby, fmt.Errorf("node account is not a whitelisted validator")
		}
	}

	// ensure we have enough rune
	minBond, err := vm.k.GetMimir(ctx, constants.MinimumBondInRune.String())
	if minBond < 0 || err != nil {
		minBond = constAccessor.GetInt64Value(constants.MinimumBondInRune)
	}
	if na.Bond.LT(cosmos.NewUint(uint64(minBond))) {
		return NodeStandby, fmt.Errorf("node account does not have minimum bond requirement: %d/%d", na.Bond.Uint64(), minBond)
	}

	minVersion := vm.k.GetMinJoinVersion(ctx)
	// Check version number is still supported
	if na.GetVersion().LT(minVersion) {
		return NodeStandby, fmt.Errorf("node account does not meet min version requirement: %s vs %s", na.Version, minVersion)
	}

	jail, err := vm.k.GetNodeAccountJail(ctx, na.NodeAddress)
	if err != nil {
		ctx.Logger().Error("fail to get node account jail", "error", err)
		return NodeStandby, fmt.Errorf("cannot fetch jail status: %w", err)
	}
	if jail.IsJailed(ctx) {
		return NodeStandby, fmt.Errorf("node account is jailed until block %d: %s", jail.ReleaseHeight, jail.Reason)
	}

	if vm.k.RagnarokInProgress(ctx) {
		return NodeStandby, fmt.Errorf("ragnarok is currently in progress: no churning")
	}

	return NodeReady, nil
}

// Returns a list of nodes to include in the next pool
func (vm *ValidatorMgrV106) nextVaultNodeAccounts(ctx cosmos.Context, targetCount int, constAccessor constants.ConstantValues) (NodeAccounts, bool, error) {
	rotation := false // track if are making any changes to the current active node accounts

	// update list of ready actors
	if err := vm.markReadyActors(ctx, constAccessor); err != nil {
		return nil, false, err
	}

	ready, err := vm.k.ListValidatorsByStatus(ctx, NodeReady)
	if err != nil {
		return nil, false, err
	}

	// sort by bond size, descending
	sort.SliceStable(ready, func(i, j int) bool {
		return ready[i].Bond.GT(ready[j].Bond)
	})

	active, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		return nil, false, err
	}

	// find out all the nodes that had been marked to leave , and update their score again , because even after a node has been marked
	// to be churn out , they can continue to accumulate slash points, in the scenario that an active node go offline , and consistently fail
	// keygen / keysign for a while , we would like to churn it out first
	lastChurnHeight := vm.getLastChurnHeight(ctx)
	for idx, item := range active {

		if item.LeaveScore == 0 {
			continue
		}
		slashPts, err := vm.k.GetNodeAccountSlashPoints(ctx, item.NodeAddress)
		if err != nil {
			ctx.Logger().Error("fail to get node account slash points", "error", err, "node address", item.NodeAddress.String())
			continue
		}
		newScore := vm.getScore(ctx, slashPts, lastChurnHeight)
		if !newScore.IsZero() {
			active[idx].LeaveScore = newScore.Uint64()
		}
	}

	// sort by LeaveScore ascending
	// giving preferential treatment to people who are forced to leave
	//  and then requested to leave
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].ForcedToLeave != active[j].ForcedToLeave {
			return active[i].ForcedToLeave
		}
		if active[i].RequestedToLeave != active[j].RequestedToLeave {
			return active[i].RequestedToLeave
		}
		// sort by LeaveHeight ascending , but exclude LeaveHeight == 0 , because that's the default value
		if active[i].LeaveScore == 0 && active[j].LeaveScore > 0 {
			return false
		}
		if active[i].LeaveScore > 0 && active[j].LeaveScore == 0 {
			return true
		}
		return active[i].LeaveScore < active[j].LeaveScore
	})

	toRemove := findCountToRemove(ctx.BlockHeight(), active)
	if toRemove > 0 {
		rotation = true
		active = active[toRemove:]
	}
	newNode, err := vm.k.GetMimir(ctx, constants.NumberOfNewNodesPerChurn.String())
	if err != nil || newNode <= 0 {
		newNode = 1
	}
	// add ready nodes to become active
	limit := toRemove + int(newNode) // Max limit of ready nodes to churn in
	minimumNodesForBFT := constAccessor.GetInt64Value(constants.MinimumNodesForBFT)
	if len(active)+limit < int(minimumNodesForBFT) {
		limit = int(minimumNodesForBFT) - len(active)
	}
	for i := 1; targetCount > len(active); i++ {
		if len(ready) >= i {
			rotation = true
			active = append(active, ready[i-1])
		}
		if i == limit { // limit adding ready accounts
			break
		}
	}

	return active, rotation, nil
}
