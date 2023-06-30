package thorchain

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// NetworkMgrV91 is going to manage the vaults
type NetworkMgrV91 struct {
	k          keeper.Keeper
	txOutStore TxOutStore
	eventMgr   EventManager
}

// newNetworkMgrV91 create a new vault manager
func newNetworkMgrV91(k keeper.Keeper, txOutStore TxOutStore, eventMgr EventManager) *NetworkMgrV91 {
	return &NetworkMgrV91{
		k:          k,
		txOutStore: txOutStore,
		eventMgr:   eventMgr,
	}
}

func (vm *NetworkMgrV91) processGenesisSetup(ctx cosmos.Context) error {
	if ctx.BlockHeight() != genesisBlockHeight {
		return nil
	}
	vaults, err := vm.k.GetAsgardVaults(ctx)
	if err != nil {
		return fmt.Errorf("fail to get vaults: %w", err)
	}
	if len(vaults) > 0 {
		ctx.Logger().Info("already have vault, no need to generate at genesis")
		return nil
	}
	active, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		return fmt.Errorf("fail to get all active node accounts")
	}
	if len(active) == 0 {
		return errors.New("no active accounts,cannot proceed")
	}
	if len(active) == 1 {
		supportChains := common.Chains{
			common.THORChain,
			common.BTCChain,
			common.LTCChain,
			common.BCHChain,
			common.BNBChain,
			common.ETHChain,
			common.DOGEChain,
			common.TERRAChain,
		}
		vault := NewVault(0, ActiveVault, AsgardVault, active[0].PubKeySet.Secp256k1, supportChains.Strings(), vm.k.GetChainContracts(ctx, supportChains))
		vault.Membership = common.PubKeys{active[0].PubKeySet.Secp256k1}.Strings()
		if err := vm.k.SetVault(ctx, vault); err != nil {
			return fmt.Errorf("fail to save vault: %w", err)
		}
	} else {
		// Trigger a keygen ceremony
		err := vm.TriggerKeygen(ctx, active)
		if err != nil {
			return fmt.Errorf("fail to trigger a keygen: %w", err)
		}
	}
	return nil
}

func (vm *NetworkMgrV91) SpawnDerivedAsset(ctx cosmos.Context, asset common.Asset, mgr Manager) {}

func (vm *NetworkMgrV91) BeginBlock(ctx cosmos.Context, mgr Manager) error {
	return nil
}

// EndBlock move funds from retiring asgard vaults
func (vm *NetworkMgrV91) EndBlock(ctx cosmos.Context, mgr Manager) error {
	if ctx.BlockHeight() == genesisBlockHeight {
		return vm.processGenesisSetup(ctx)
	}
	controller := NewRouterUpgradeController(mgr)
	controller.Process(ctx)

	migrateInterval, err := vm.k.GetMimir(ctx, constants.FundMigrationInterval.String())
	if migrateInterval < 0 || err != nil {
		migrateInterval = mgr.GetConstants().GetInt64Value(constants.FundMigrationInterval)
	}

	retiring, err := vm.k.GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		return err
	}

	active, err := vm.k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return err
	}

	// if we have no active asgards to move funds to, don't move funds
	if len(active) == 0 {
		return nil
	}
	for _, av := range active {
		if av.Routers != nil {
			continue
		}
		av.Routers = vm.k.GetChainContracts(ctx, av.GetChains())
		if err := vm.k.SetVault(ctx, av); err != nil {
			ctx.Logger().Error("fail to update chain contract", "error", err)
		}
	}
	for _, vault := range retiring {
		if vault.LenPendingTxBlockHeights(ctx.BlockHeight(), mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)) > 0 {
			ctx.Logger().Info("Skipping the migration of funds while transactions are still pending")
			return nil
		}
	}

	for _, vault := range retiring {
		if !vault.HasFunds() {
			vault.Status = InactiveVault
			if err := vm.k.SetVault(ctx, vault); err != nil {
				ctx.Logger().Error("fail to set vault to inactive", "error", err)
			}
			continue
		}

		// move partial funds every 30 minutes
		if (ctx.BlockHeight()-vault.StatusSince)%migrateInterval == 0 {
			for _, coin := range vault.Coins {
				// non-native rune assets are no migrated, therefore they are
				// burned in each churn
				if coin.IsNative() {
					continue
				}
				// ERC20 RUNE will be burned when it reach router contract
				if coin.Asset.IsRune() && coin.Asset.GetChain().Equals(common.ETHChain) {
					continue
				}

				if coin.Amount.Equal(cosmos.ZeroUint()) {
					continue
				}
				var target Vault
				// when migrate assets from retiring vault to a new vault , if it is gas asset, like (BNB, BTC) , make
				// sure each new vault will get gas asset, take BNB for an example , it might get a lot of BEP2 asset
				// into the new vault , but without any BNB, which will make the vault unavailable , as it doesn't have BNB to
				// pay for gas. In a real production environment
				if coin.Asset.IsGasAsset() {
					for _, activeVault := range active {
						if activeVault.HasAsset(coin.Asset) {
							continue
						}
						target = activeVault
						break
					}
				}
				if target.IsEmpty() {
					// determine which active asgard vault to send funds to. Select
					// based on which has the most security
					signingTransactionPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
					target = vm.k.GetMostSecure(ctx, active, signingTransactionPeriod)
					if target.PubKey.Equals(vault.PubKey) {
						continue
					}
				}
				// get address of asgard pubkey
				addr, err := target.PubKey.GetAddress(coin.Asset.GetChain())
				if err != nil {
					return err
				}

				// figure the nth time, we've sent migration txs from this vault
				nth := (ctx.BlockHeight()-vault.StatusSince)/migrateInterval + 1

				// Default amount set to total remaining amount. Relies on the
				// signer, to successfully send these funds while respecting
				// gas requirements (so it'll actually send slightly less)
				amt := coin.Amount
				if nth < 5 { // migrate partial funds 4 times
					// each round of migration, we are increasing the amount 20%.
					// Round 1 = 20%
					// Round 2 = 40%
					// Round 3 = 60%
					// Round 4 = 80%
					// Round 5 = 100%
					amt = amt.MulUint64(uint64(nth)).QuoUint64(5)
				}
				amt = cosmos.RoundToDecimal(amt, coin.Decimals)

				// minus gas costs for our transactions
				gasAsset := coin.Asset.GetChain().GetGasAsset()
				if coin.Asset.Equals(gasAsset) {
					gasMgr := mgr.GasMgr()
					gas, err := gasMgr.GetMaxGas(ctx, coin.Asset.GetChain())
					if err != nil {
						ctx.Logger().Error("fail to get max gas: %w", err)
						return err
					}
					// if remainder is less than the gas amount, just send it all now
					if common.SafeSub(coin.Amount, amt).LTE(gas.Amount) {
						amt = coin.Amount
					}

					gasAmount := gas.Amount.MulUint64(uint64(vault.CoinLengthByChain(coin.Asset.GetChain())))
					amt = common.SafeSub(amt, gasAmount)

					// the left amount is not enough to pay for gas, likely only dust left, the network can't migrate it across
					// and this will only happen after 5th round
					if amt.IsZero() && nth > 5 {
						ctx.Logger().Info("left coin is not enough to pay for gas, thus burn it", "coin", coin, "gas", gasAmount)
						vault.SubFunds(common.Coins{
							coin,
						})
						// use reserve to subsidise the pool for the lost
						p, err := vm.k.GetPool(ctx, coin.Asset)
						if err != nil {
							return fmt.Errorf("fail to get pool for asset %s, err:%w", coin.Asset, err)
						}
						runeAmt := p.AssetValueInRune(coin.Amount)
						if !runeAmt.IsZero() {
							if err := vm.k.SendFromModuleToModule(ctx, ReserveName, AsgardName, common.NewCoins(common.NewCoin(common.RuneAsset(), runeAmt))); err != nil {
								return fmt.Errorf("fail to transfer RUNE from reserve to asgard,err:%w", err)
							}
						}
						p.BalanceRune = p.BalanceRune.Add(runeAmt)
						p.BalanceAsset = common.SafeSub(p.BalanceAsset, coin.Amount)
						if err := vm.k.SetPool(ctx, p); err != nil {
							return fmt.Errorf("fail to save pool: %w", err)
						}
						if err := vm.k.SetVault(ctx, vault); err != nil {
							return fmt.Errorf("fail to save vault: %w", err)
						}
						emitPoolBalanceChangedEvent(ctx,
							NewPoolMod(p.Asset, runeAmt, true, coin.Amount, false),
							"burn dust",
							mgr)
						continue
					}
				}
				if coin.Asset.Equals(common.BEP2RuneAsset()) {
					bepRuneOwnerAddr, err := common.NewAddress(BEP2RuneOwnerAddress)
					if err != nil {
						ctx.Logger().Error("fail to parse BEP2 RUNE owner address", "address", BEP2RuneOwnerAddress)
					} else {
						addr = bepRuneOwnerAddr
					}
				}
				toi := TxOutItem{
					Chain:       coin.Asset.GetChain(),
					InHash:      common.BlankTxID,
					ToAddress:   addr,
					VaultPubKey: vault.PubKey,
					Coin: common.Coin{
						Asset:  coin.Asset,
						Amount: amt,
					},
					Memo: NewMigrateMemo(ctx.BlockHeight()).String(),
				}
				ok, err := vm.txOutStore.TryAddTxOutItem(ctx, mgr, toi, cosmos.ZeroUint())
				if err != nil && !errors.Is(err, ErrNotEnoughToPayFee) {
					return err
				}
				if ok {
					vault.AppendPendingTxBlockHeights(ctx.BlockHeight(), mgr.GetConstants())
					if err := vm.k.SetVault(ctx, vault); err != nil {
						return fmt.Errorf("fail to save vault: %w", err)
					}
				}
			}
		}
	}
	if err := vm.checkPoolRagnarok(ctx, mgr); err != nil {
		ctx.Logger().Error("fail to process pool ragnarok", "error", err)
	}
	return nil
}

// TriggerKeygen generate a record to instruct signer kick off keygen process
func (vm *NetworkMgrV91) TriggerKeygen(ctx cosmos.Context, nas NodeAccounts) error {
	halt, err := vm.k.GetMimir(ctx, "HaltChurning")
	if halt > 0 && halt <= ctx.BlockHeight() && err == nil {
		ctx.Logger().Info("churn event skipped due to mimir has halted churning")
		return nil
	}
	var members []string
	for i := range nas {
		members = append(members, nas[i].PubKeySet.Secp256k1.String())
	}
	keygen, err := NewKeygen(ctx.BlockHeight(), members, AsgardKeygen)
	if err != nil {
		return fmt.Errorf("fail to create a new keygen: %w", err)
	}
	keygenBlock, err := vm.k.GetKeygenBlock(ctx, ctx.BlockHeight())
	if err != nil {
		return fmt.Errorf("fail to get keygen block from data store: %w", err)
	}

	if !keygenBlock.Contains(keygen) {
		keygenBlock.Keygens = append(keygenBlock.Keygens, keygen)
	}

	// check if we already have a an active vault with the same membership,
	// skip if we do
	active, err := vm.k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return fmt.Errorf("fail to get active vaults: %w", err)
	}
	for _, vault := range active {
		if vault.MembershipEquals(keygen.GetMembers()) {
			ctx.Logger().Info("skip keygen due to vault already existing")
			return nil
		}
	}

	vm.k.SetKeygenBlock(ctx, keygenBlock)
	// clear the init vault
	initVaults, err := vm.k.GetAsgardVaultsByStatus(ctx, InitVault)
	if err != nil {
		ctx.Logger().Error("fail to get init vault", "error", err)
		return nil
	}
	for _, v := range initVaults {
		if v.HasFunds() {
			continue
		}
		v.UpdateStatus(InactiveVault, ctx.BlockHeight())
		if err := vm.k.SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}
	return nil
}

// RotateVault update vault to Retiring and new vault to active
func (vm *NetworkMgrV91) RotateVault(ctx cosmos.Context, vault Vault) error {
	active, err := vm.k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return err
	}

	// find vaults the new vault conflicts with, mark them as inactive
	for _, asgard := range active {
		for _, member := range asgard.GetMembership() {
			if vault.Contains(member) {
				asgard.UpdateStatus(RetiringVault, ctx.BlockHeight())
				if err := vm.k.SetVault(ctx, asgard); err != nil {
					return err
				}

				ctx.EventManager().EmitEvent(
					cosmos.NewEvent(EventTypeInactiveVault,
						cosmos.NewAttribute("set asgard vault to inactive", asgard.PubKey.String())))
				break
			}
		}
	}

	// Update Node account membership
	for _, member := range vault.GetMembership() {
		na, err := vm.k.GetNodeAccountByPubKey(ctx, member)
		if err != nil {
			return err
		}
		na.TryAddSignerPubKey(vault.PubKey)
		if err := vm.k.SetNodeAccount(ctx, na); err != nil {
			return err
		}
	}

	if err := vm.k.SetVault(ctx, vault); err != nil {
		return err
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent(EventTypeActiveVault,
			cosmos.NewAttribute("add new asgard vault", vault.PubKey.String())))
	return nil
}

// manageChains - checks to see if we have any chains that we are ragnaroking,
// and ragnaroks them
func (vm *NetworkMgrV91) manageChains(ctx cosmos.Context, mgr Manager) error {
	chains, err := vm.findChainsToRetire(ctx)
	if err != nil {
		return err
	}

	active, err := vm.k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return err
	}
	vault := active.SelectByMinCoin(common.RuneAsset())
	if vault.IsEmpty() {
		return fmt.Errorf("unable to determine asgard vault")
	}

	migrateInterval, err := vm.k.GetMimir(ctx, constants.FundMigrationInterval.String())
	if migrateInterval < 0 || err != nil {
		migrateInterval = mgr.GetConstants().GetInt64Value(constants.FundMigrationInterval)
	}
	nth := (ctx.BlockHeight()-vault.StatusSince)/migrateInterval + 1
	if nth > 10 {
		nth = 10
	}

	for _, chain := range chains {
		// the first round to recall fund from yggdrasil
		if nth == 1 {
			if err := vm.RecallChainFunds(ctx, chain, mgr, common.PubKeys{}); err != nil {
				return err
			}
		}

		// only refund after the first nth. This gives yggs time to send funds
		// back to asgard
		if nth > 1 {
			if err := vm.ragnarokChain(ctx, chain, nth, mgr); err != nil {
				continue
			}
		}
	}
	return nil
}

// findChainsToRetire - evaluates the chains associated with active asgard
// vaults vs retiring asgard vaults to detemine if any chains need to be
// ragnarok'ed
func (vm *NetworkMgrV91) findChainsToRetire(ctx cosmos.Context) (common.Chains, error) {
	chains := make(common.Chains, 0)

	active, err := vm.k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return chains, err
	}
	retiring, err := vm.k.GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		return chains, err
	}

	// collect all chains for active vaults
	activeChains := make(common.Chains, 0)
	for _, v := range active {
		activeChains = append(activeChains, v.GetChains()...)
	}
	activeChains = activeChains.Distinct()

	// collect all chains for retiring vaults
	retiringChains := make(common.Chains, 0)
	for _, v := range retiring {
		retiringChains = append(retiringChains, v.GetChains()...)
	}
	retiringChains = retiringChains.Distinct()

	for _, chain := range retiringChains {
		// skip chain if its in active and retiring
		if activeChains.Has(chain) {
			continue
		}
		chains = append(chains, chain)
	}
	return chains, nil
}

// RecallChainFunds - sends a message to bifrost nodes to send back all funds
// associated with given chain
func (vm *NetworkMgrV91) RecallChainFunds(ctx cosmos.Context, chain common.Chain, mgr Manager, excludeNodes common.PubKeys) error {
	allNodes, err := vm.k.ListValidatorsWithBond(ctx)
	if err != nil {
		return fmt.Errorf("fail to list all node accounts: %w", err)
	}

	active, err := vm.k.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return err
	}

	signingTransactionPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
	vault := vm.k.GetMostSecure(ctx, active, signingTransactionPeriod)
	if vault.IsEmpty() {
		return fmt.Errorf("unable to determine asgard vault")
	}
	toAddr, err := vault.PubKey.GetAddress(chain)
	if err != nil {
		return err
	}

	// get yggdrasil to return funds back to asgard
	for _, node := range allNodes {
		if excludeNodes.Contains(node.PubKeySet.Secp256k1) {
			continue
		}
		if !vm.k.VaultExists(ctx, node.PubKeySet.Secp256k1) {
			continue
		}
		ygg, err := vm.k.GetVault(ctx, node.PubKeySet.Secp256k1)
		if err != nil {
			ctx.Logger().Error("fail to get ygg vault", "error", err)
			continue
		}
		if ygg.IsAsgard() {
			continue
		}

		if !ygg.HasFundsForChain(chain) {
			continue
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
			}
			// yggdrasil- will not set coin field here, when signer see a
			// TxOutItem that has memo "yggdrasil-" it will query the chain
			// and find out all the remaining assets , and fill in the
			// field
			if err := vm.txOutStore.UnSafeAddTxOutItem(ctx, mgr, txOutItem); err != nil {
				return err
			}
		}
	}

	return nil
}

// ragnarokChain - ends a chain by withdrawing all liquidity providers of any pool that's
// asset is on the given chain
func (vm *NetworkMgrV91) ragnarokChain(ctx cosmos.Context, chain common.Chain, nth int64, mgr Manager) error {
	nas, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("can't get active nodes", "error", err)
		return err
	}
	if len(nas) == 0 {
		return fmt.Errorf("can't find any active nodes")
	}
	na := nas[0]

	pools, err := vm.k.GetPools(ctx)
	if err != nil {
		return err
	}

	// rangarok this chain
	for _, pool := range pools {
		if !pool.Asset.GetChain().Equals(chain) || pool.LPUnits.IsZero() {
			continue
		}
		if err := vm.withdrawLiquidity(ctx, pool, na, mgr); err != nil {
			ctx.Logger().Error("fail to ragnarok liquidity", "error", err)
		}
	}

	return nil
}

// withdrawLiquidity will process a batch of LP per iteration, the batch size is defined by constants.RagnarokProcessNumOfLPPerIteration
// once the all LP get processed, none-gas pool will be removed , gas pool will be set to Suspended
func (vm *NetworkMgrV91) withdrawLiquidity(ctx cosmos.Context, pool Pool, na NodeAccount, mgr Manager) error {
	handler := NewInternalHandler(mgr)
	iterator := vm.k.GetLiquidityProviderIterator(ctx, pool.Asset)
	lpPerIteration := mgr.GetConstants().GetInt64Value(constants.RagnarokProcessNumOfLPPerIteration)
	totalCount := int64(0)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var lp LiquidityProvider
		if err := vm.k.Cdc().Unmarshal(iterator.Value(), &lp); err != nil {
			ctx.Logger().Error("fail to unmarshal liquidity provider", "error", err)
			vm.k.RemoveLiquidityProvider(ctx, lp)
			continue
		}
		if lp.Units.IsZero() && lp.PendingAsset.IsZero() && lp.PendingRune.IsZero() {
			vm.k.RemoveLiquidityProvider(ctx, lp)
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
			common.GetRagnarokTx(pool.Asset.GetChain(), withdrawAddr, withdrawAddr),
			withdrawAddr,
			cosmos.NewUint(uint64(MaxWithdrawBasisPoints)),
			pool.Asset,
			withdrawAsset,
			na.NodeAddress,
		)

		_, err := handler(ctx, withdrawMsg)
		if err != nil {
			ctx.Logger().Error("fail to withdraw, remove LP", "liquidity provider", lp.RuneAddress, "asset address", lp.AssetAddress, "error", err)
			// in a ragnarok scenario , try best to withdraw it  ,
			// if an LP failed to withdraw most likely it is due to not enough asset to pay for gas fee, then let's remove the LP record
			// write a log first , so we can grep the log to deal with it manually
			vm.k.RemoveLiquidityProvider(ctx, lp)
		}
		totalCount++
		if totalCount >= lpPerIteration {
			break
		}
	}
	// this means finished
	if totalCount < lpPerIteration {
		afterPool, err := vm.k.GetPool(ctx, pool.Asset)
		if err != nil {
			return fmt.Errorf("fail to get pool after ragnarok,err: %w", err)
		}
		poolEvent := NewEventPool(pool.Asset, PoolSuspended)
		if err := mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}
		if afterPool.Asset.IsGasAsset() {
			afterPool.Status = PoolSuspended
			return vm.k.SetPool(ctx, afterPool)
		} else {
			// remove the pool
			vm.k.RemovePool(ctx, pool.Asset)
		}
	}
	return nil
}

// UpdateNetwork Update the network data to reflect changing in this block
func (vm *NetworkMgrV91) UpdateNetwork(ctx cosmos.Context, constAccessor constants.ConstantValues, gasManager GasManager, eventMgr EventManager) error {
	network, err := vm.k.GetNetwork(ctx)
	if err != nil {
		return fmt.Errorf("fail to get existing network data: %w", err)
	}

	totalReserve := vm.k.GetRuneBalanceOfModule(ctx, ReserveName)

	// when total reserve is zero , can't pay reward
	if totalReserve.IsZero() {
		return nil
	}
	currentHeight := uint64(ctx.BlockHeight())
	pools, totalProvidedLiquidity, err := vm.getTotalProvidedLiquidityRune(ctx)
	if err != nil {
		return fmt.Errorf("fail to get available pools and total provided liquidity rune: %w", err)
	}

	// If no Rune is provided liquidity, then don't give out block rewards.
	if totalProvidedLiquidity.IsZero() {
		return nil // If no Rune is provided liquidity, then don't give out block rewards.
	}

	// get total liquidity fees
	totalLiquidityFees, err := vm.k.GetTotalLiquidityFees(ctx, currentHeight)
	if err != nil {
		return fmt.Errorf("fail to get total liquidity fee: %w", err)
	}

	// NOTE: if we continue to have remaining gas to pay off (which is
	// extremely unlikely), ignore it for now (attempt to recover in the next
	// block). This should be OK as the asset amount in the pool has already
	// been deducted so the balances are correct. Just operating at a deficit.
	totalBonded, err := vm.getTotalActiveBond(ctx)
	if err != nil {
		return fmt.Errorf("fail to get total active bond: %w", err)
	}

	emissionCurve, err := vm.k.GetMimir(ctx, constants.EmissionCurve.String())
	if emissionCurve < 0 || err != nil {
		emissionCurve = constAccessor.GetInt64Value(constants.EmissionCurve)
	}
	incentiveCurve, err := vm.k.GetMimir(ctx, constants.IncentiveCurve.String())
	if incentiveCurve < 0 || err != nil {
		incentiveCurve = constAccessor.GetInt64Value(constants.IncentiveCurve)
	}
	blocksPerYear := constAccessor.GetInt64Value(constants.BlocksPerYear)
	bondReward, totalPoolRewards, lpDeficit, lpShare := vm.calcBlockRewards(totalProvidedLiquidity, totalBonded, totalReserve, totalLiquidityFees, emissionCurve, incentiveCurve, blocksPerYear)

	network.LPIncomeSplit = int64(lpShare.Uint64())
	network.NodeIncomeSplit = int64(10_000) - network.LPIncomeSplit

	// given bondReward and toolPoolRewards are both calculated base on totalReserve, thus it should always have enough to pay the bond reward

	// Move Rune from the Reserve to the Bond and Pool Rewards
	coin := common.NewCoin(common.RuneNative, bondReward)
	if !bondReward.IsZero() {
		if err := vm.k.SendFromModuleToModule(ctx, ReserveName, BondName, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to transfer funds from reserve to bond", "error", err)
			return fmt.Errorf("fail to transfer funds from reserve to bond: %w", err)
		}
	}
	network.BondRewardRune = network.BondRewardRune.Add(bondReward) // Add here for individual Node collection later

	var evtPools []PoolAmt

	if !totalPoolRewards.IsZero() { // If Pool Rewards to hand out
		var rewardAmts []cosmos.Uint
		var rewardPools []Pool
		// Pool Rewards are based on Fee Share
		for _, pool := range pools {
			if !pool.IsAvailable() {
				continue
			}
			var amt cosmos.Uint
			if totalLiquidityFees.IsZero() {
				amt = common.GetSafeShare(pool.BalanceRune, totalProvidedLiquidity, totalPoolRewards)
			} else {
				fees, err := vm.k.GetPoolLiquidityFees(ctx, currentHeight, pool.Asset)
				if err != nil {
					ctx.Logger().Error("fail to get fees", "error", err)
					continue
				}
				amt = common.GetSafeShare(fees, totalLiquidityFees, totalPoolRewards)
			}
			rewardAmts = append(rewardAmts, amt)
			evtPools = append(evtPools, PoolAmt{Asset: pool.Asset, Amount: int64(amt.Uint64())})
			rewardPools = append(rewardPools, pool)
		}
		// Pay out
		if err := vm.payPoolRewards(ctx, rewardAmts, rewardPools); err != nil {
			return err
		}

	} else { // Else deduct pool deficit

		poolAmts, err := vm.deductPoolRewardDeficit(ctx, pools, totalLiquidityFees, lpDeficit)
		if err != nil {
			return err
		}
		evtPools = append(evtPools, poolAmts...)
	}

	rewardEvt := NewEventRewards(bondReward, evtPools)
	if err := eventMgr.EmitEvent(ctx, rewardEvt); err != nil {
		return fmt.Errorf("fail to emit reward event: %w", err)
	}
	i, err := getTotalActiveNodeWithBond(ctx, vm.k)
	if err != nil {
		return fmt.Errorf("fail to get total active node account: %w", err)
	}
	network.TotalBondUnits = network.TotalBondUnits.Add(cosmos.NewUint(uint64(i))) // Add 1 unit for each active Node

	return vm.k.SetNetwork(ctx, network)
}

func (vm *NetworkMgrV91) getTotalProvidedLiquidityRune(ctx cosmos.Context) (Pools, cosmos.Uint, error) {
	// First get active pools and total provided liquidity Rune
	totalProvidedLiquidity := cosmos.ZeroUint()
	var pools Pools
	iterator := vm.k.GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := vm.k.Cdc().Unmarshal(iterator.Value(), &pool); err != nil {
			return nil, cosmos.ZeroUint(), fmt.Errorf("fail to unmarhsl pool: %w", err)
		}
		if !pool.BalanceRune.IsZero() {
			totalProvidedLiquidity = totalProvidedLiquidity.Add(pool.BalanceRune)
			pools = append(pools, pool)
		}
	}
	return pools, totalProvidedLiquidity, nil
}

func (vm *NetworkMgrV91) getTotalActiveBond(ctx cosmos.Context) (cosmos.Uint, error) {
	totalBonded := cosmos.ZeroUint()
	nodes, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get all active accounts: %w", err)
	}
	for _, node := range nodes {
		totalBonded = totalBonded.Add(node.Bond)
	}
	return totalBonded, nil
}

// Pays out Rewards
func (vm *NetworkMgrV91) payPoolRewards(ctx cosmos.Context, poolRewards []cosmos.Uint, pools Pools) error {
	for i, reward := range poolRewards {
		if reward.IsZero() {
			continue
		}
		pools[i].BalanceRune = pools[i].BalanceRune.Add(reward)
		if err := vm.k.SetPool(ctx, pools[i]); err != nil {
			return fmt.Errorf("fail to set pool: %w", err)
		}
		coin := common.NewCoin(common.RuneNative, reward)
		if !reward.IsZero() {
			if err := vm.k.SendFromModuleToModule(ctx, ReserveName, AsgardName, common.NewCoins(coin)); err != nil {
				return fmt.Errorf("fail to transfer funds from reserve to asgard: %w", err)
			}
		}
	}
	return nil
}

// Calculate pool deficit based on the pool's accrued fees compared with total fees.
func (vm *NetworkMgrV91) calcPoolDeficit(lpDeficit, totalFees, poolFees cosmos.Uint) cosmos.Uint {
	return common.GetSafeShare(poolFees, totalFees, lpDeficit)
}

// Calculate the block rewards that bonders and liquidity providers should receive
func (vm *NetworkMgrV91) calcBlockRewards(totalProvidedLiquidity, totalBonded, totalReserve, totalLiquidityFees cosmos.Uint, emissionCurve, incentiveCurve, blocksPerYear int64) (cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint) {
	// Block Rewards will take the latest reserve, divide it by the emission
	// curve factor, then divide by blocks per year
	trD := cosmos.NewDec(int64(totalReserve.Uint64()))
	ecD := cosmos.NewDec(emissionCurve)
	bpyD := cosmos.NewDec(blocksPerYear)
	blockRewardD := trD.Quo(ecD).Quo(bpyD)
	blockReward := cosmos.NewUint(uint64((blockRewardD).RoundInt64()))

	systemIncome := blockReward.Add(totalLiquidityFees) // Get total system income for block

	lpSplit := vm.getPoolShare(incentiveCurve, totalProvidedLiquidity, totalBonded, systemIncome) // Get liquidity provider share
	bonderSplit := common.SafeSub(systemIncome, lpSplit)                                          // Remainder to Bonders
	lpShare := common.GetSafeShare(lpSplit, systemIncome, cosmos.NewUint(10_000))

	lpDeficit := cosmos.ZeroUint()
	poolReward := cosmos.ZeroUint()

	if lpSplit.GTE(totalLiquidityFees) {
		// Liquidity Providers have not been paid enough already, pay more
		poolReward = common.SafeSub(lpSplit, totalLiquidityFees) // Get how much to divert to add to liquidity provider split
	} else {
		// Liquidity Providers have been paid too much, calculate deficit
		lpDeficit = common.SafeSub(totalLiquidityFees, lpSplit) // Deduct existing income from split
	}

	return bonderSplit, poolReward, lpDeficit, lpShare
}

func (vm *NetworkMgrV91) getPoolShare(incentiveCurve int64, totalProvidedLiquidity, totalBonded, totalRewards cosmos.Uint) cosmos.Uint {
	/*
		Pooled : Share
		0 : 100%
		33% : 50% <- need to be 50% to match node rewards, when 33% is pooled
		50% : 0%
		https://gitlab.com/thorchain/thornode/-/issues/693
	*/
	if incentiveCurve <= 0 {
		incentiveCurve = 1
	}
	if totalProvidedLiquidity.GTE(totalBonded) { // Zero payments to liquidity providers when provided liquidity == bonded
		return cosmos.ZeroUint()
	}
	/*
		B = bondedRune
		P = pooledRune
		incentiveCurve = 33
		poolShareFactor = (B - P)/(B + P/incentiveCurve)
	*/

	var total cosmos.Uint
	if incentiveCurve >= 100 {
		total = totalBonded
	} else {
		inD := cosmos.NewDec(incentiveCurve)
		divi := cosmos.NewDecFromBigInt(totalProvidedLiquidity.BigInt()).Quo(inD)
		total = cosmos.NewUint(uint64((divi).RoundInt64())).Add(totalBonded)
	}
	part := common.SafeSub(totalBonded, totalProvidedLiquidity)
	return common.GetSafeShare(part, total, totalRewards)
}

// deductPoolRewardDeficit - When swap fees accrued by the pools surpass what
// the incentive pendulum dictates, the difference (lpDeficit) is deducted from
// the pools and sent to the reserve. The amount of RUNE deducted from each
// pool is in proportion to the amount of fees it accrued:
//
// deduction = (poolFees / totalLiquidityFees) * lpDeficit
func (vm *NetworkMgrV91) deductPoolRewardDeficit(ctx cosmos.Context, pools Pools, totalLiquidityFees, lpDeficit cosmos.Uint) ([]PoolAmt, error) {
	poolAmts := make([]PoolAmt, 0)
	for _, pool := range pools {
		if !pool.IsAvailable() {
			continue
		}
		poolFees, err := vm.k.GetPoolLiquidityFees(ctx, uint64(ctx.BlockHeight()), pool.Asset)
		if err != nil {
			return poolAmts, fmt.Errorf("fail to get liquidity fees for pool(%s): %w", pool.Asset, err)
		}
		if pool.BalanceRune.IsZero() || poolFees.IsZero() { // Safety checks
			continue
		}
		poolDeficit := vm.calcPoolDeficit(lpDeficit, totalLiquidityFees, poolFees)
		// when pool deficit is zero , the pool doesn't pay deficit
		if poolDeficit.IsZero() {
			continue
		}
		coin := common.NewCoin(common.RuneNative, poolDeficit)
		if !poolDeficit.IsZero() {
			if err := vm.k.SendFromModuleToModule(ctx, AsgardName, ReserveName, common.NewCoins(coin)); err != nil {
				ctx.Logger().Error("fail to transfer funds from asgard to reserve", "error", err)
				return poolAmts, fmt.Errorf("fail to transfer funds from asgard to reserve: %w", err)
			}
		}
		if poolDeficit.GT(pool.BalanceRune) {
			poolDeficit = pool.BalanceRune
		}
		pool.BalanceRune = common.SafeSub(pool.BalanceRune, poolDeficit)
		if err := vm.k.SetPool(ctx, pool); err != nil {
			return poolAmts, fmt.Errorf("fail to set pool: %w", err)
		}
		poolAmts = append(poolAmts, PoolAmt{
			Asset:  pool.Asset,
			Amount: 0 - int64(poolDeficit.Uint64()),
		})
	}
	return poolAmts, nil
}

// checkPoolRagnarok iterate through all the pools to see whether there are pools need to be ragnarok
// this function will only run in an interval , defined by constants.FundMigrationInterval
func (vm *NetworkMgrV91) checkPoolRagnarok(ctx cosmos.Context, mgr Manager) error {
	// check whether pool need to be ragnarok per constants.FundMigrationInterval
	if ctx.BlockHeight()%mgr.GetConstants().GetInt64Value(constants.FundMigrationInterval) > 0 {
		return nil
	}
	pools, err := vm.k.GetPools(ctx)
	if err != nil {
		return err
	}

	for _, pool := range pools {
		k := fmt.Sprintf("RAGNAROK-%s", strings.ReplaceAll(pool.Asset.String(), ".", "-"))
		v, err := vm.k.GetMimir(ctx, k)
		if err != nil {
			ctx.Logger().Error("fail to get mimir value", "mimir", k, "error", err)
			continue
		}
		if v < 1 {
			continue
		}
		if pool.Asset.IsGasAsset() && !canRagnarokGasPool(ctx, pool.Asset.GetChain(), pools) {
			continue
		}
		if err := vm.ragnarokPool(ctx, mgr, pool); err != nil {
			ctx.Logger().Error("fail to ragnarok pool", "error", err)
		}
	}

	return nil
}

func (vm *NetworkMgrV91) redeemSynthAssetToReserve(ctx cosmos.Context, p Pool) error {
	totalSupply := vm.k.GetTotalSupply(ctx, p.Asset.GetSyntheticAsset())
	if totalSupply.IsZero() {
		return nil
	}
	runeValue := p.AssetValueInRune(totalSupply)
	p.BalanceRune = common.SafeSub(p.BalanceRune, runeValue)
	// Here didn't set synth unit to zero , but `GetTotalSupply` will check pool ragnarok status
	// when Pool Ragnarok started , then the synth supply will return zero.
	if err := vm.k.SetPool(ctx, p); err != nil {
		return fmt.Errorf("fail to save pool,err: %w", err)
	}
	if err := vm.k.SendFromModuleToModule(ctx, AsgardName, ReserveName,
		common.NewCoins(common.NewCoin(common.RuneNative, runeValue))); err != nil {
		ctx.Logger().Error("fail to send redeemed synth RUNE to reserve", "error", err)
	}
	ctx.Logger().
		With("synth_supply", totalSupply.String()).
		With("rune_amount", runeValue).
		Info("sending synth redeem RUNE to Reserve")
	return nil
}

func (vm *NetworkMgrV91) ragnarokPool(ctx cosmos.Context, mgr Manager, p Pool) error {
	startBlockHeight, err := vm.k.GetPoolRagnarokStart(ctx, p.Asset)
	if err != nil || startBlockHeight == 0 {
		if err != nil {
			ctx.Logger().Error("fail to get pool ragnarok start block height", "error", err)
		}
		// set it to current block height
		vm.k.SetPoolRagnarokStart(ctx, p.Asset)
		startBlockHeight = ctx.BlockHeight()
	}
	nth := (ctx.BlockHeight()-startBlockHeight)/mgr.GetConstants().GetInt64Value(constants.FundMigrationInterval) + 1

	// set the pool status to stage , thus the network will not send asset to yggdrasil vault
	if p.Status != PoolStaged {
		p.Status = PoolStaged
		if err := vm.k.SetPool(ctx, p); err != nil {
			return fmt.Errorf("fail to set pool to stage,err: %w", err)
		}
		poolEvent := NewEventPool(p.Asset, PoolStaged)
		if err := mgr.EventMgr().EmitEvent(ctx, poolEvent); err != nil {
			ctx.Logger().Error("fail to emit pool event", "error", err)
		}

	}

	// first round , let's set the pool to stage , and recall yggdrasil fund
	// staged pool will not fund yggdrasil again
	if nth == 1 {
		// redeem all synth asset from the pool , and send RUNE to reserve
		if err := vm.redeemSynthAssetToReserve(ctx, p); err != nil {
			ctx.Logger().Error("fail to redeem synth to reserve, continue to ragnarok", "error", err)
		}
		return vm.RecallChainFunds(ctx, p.Asset.GetChain(), mgr, common.PubKeys{})
	}

	nas, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("can't get active nodes", "error", err)
		return err
	}
	if len(nas) == 0 {
		return fmt.Errorf("can't find any active nodes")
	}
	na := nas[0]

	return vm.withdrawLiquidity(ctx, p, na, mgr)
}
