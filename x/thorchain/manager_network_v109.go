package thorchain

import (
	"errors"
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// NetworkMgrV109 is going to manage the vaults
type NetworkMgrV109 struct {
	k          keeper.Keeper
	txOutStore TxOutStore
	eventMgr   EventManager
}

// newNetworkMgrV109 create a new vault manager
func newNetworkMgrV109(k keeper.Keeper, txOutStore TxOutStore, eventMgr EventManager) *NetworkMgrV109 {
	return &NetworkMgrV109{
		k:          k,
		txOutStore: txOutStore,
		eventMgr:   eventMgr,
	}
}

func (vm *NetworkMgrV109) processGenesisSetup(ctx cosmos.Context) error {
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
			common.AVAXChain,
			common.GAIAChain,
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

func (vm *NetworkMgrV109) BeginBlock(ctx cosmos.Context, mgr Manager) error {
	return vm.spawnDerivedAssets(ctx, mgr)
}

func (vm *NetworkMgrV109) suspendVirtualPool(ctx cosmos.Context, mgr Manager, derivedAsset common.Asset, suspendReasonErr error) {
	// Ensure that derivedAsset is indeed a derived asset.
	derivedAsset = derivedAsset.GetDerivedAsset()

	if !mgr.Keeper().PoolExist(ctx, derivedAsset) {
		// pool doesn't exist, no need to suspend it
		return
	}

	derivedPool, err := mgr.Keeper().GetPool(ctx, derivedAsset)
	if err != nil {
		ctx.Logger().Error("failed to fetch derived pool", "asset", derivedAsset, "err", err)
		return
	}
	if derivedPool.Status != PoolSuspended {
		derivedPool.Status = PoolSuspended
		derivedPool.StatusSince = ctx.BlockHeight()

		poolEvt := NewEventPool(derivedPool.Asset, PoolSuspended)
		if err := mgr.EventMgr().EmitEvent(ctx, poolEvt); err != nil {
			ctx.Logger().Error("fail to emit pool event", "asset", derivedPool.Asset, "error", err)
		}
		telemetry.IncrCounterWithLabels(
			[]string{"thornode", "derived_asset", "suspended"},
			float32(1),
			[]metrics.Label{telemetry.NewLabel("pool", derivedPool.Asset.String())},
		)
		ctx.Logger().Error("derived virtual pool suspended", "asset", derivedPool.Asset, "error", suspendReasonErr)
	}
	if err := mgr.Keeper().SetPool(ctx, derivedPool); err != nil {
		ctx.Logger().Error("failed to set pool", "asset", derivedPool.Asset, "error", err)
	}
}

func (vm *NetworkMgrV109) calcAnchor(ctx cosmos.Context, mgr Manager, asset common.Asset) (cosmos.Uint, cosmos.Uint, cosmos.Uint) {
	anchors := getAnchors(ctx, mgr.Keeper(), asset)

	maxAnchorBlocks := mgr.Keeper().GetConfigInt64(ctx, constants.MaxAnchorBlocks)

	// sum anchor pool rune depths
	totalRuneDepth := cosmos.ZeroUint()
	availableAnchors := make([]common.Asset, 0)
	slippageCollector := make([]cosmos.Uint, 0)
	for _, anchorAsset := range anchors {
		// skip assets where trading isn't occurring (hence price is likely not correct)
		if isGlobalTradingHalted(ctx, mgr) || isChainTradingHalted(ctx, mgr, anchorAsset.Chain) {
			continue
		}
		if !mgr.Keeper().PoolExist(ctx, anchorAsset) {
			continue
		}
		p, err := mgr.Keeper().GetPool(ctx, anchorAsset)
		if err != nil {
			ctx.Logger().Error("failed to get anchor pool", "asset", anchorAsset, "error", err)
			continue
		}
		// skip assets that aren't available (hence price isn't likely to be correct)
		if p.Status != PoolAvailable {
			continue
		}
		if p.BalanceRune.IsZero() || p.BalanceAsset.IsZero() {
			continue
		}

		slip, err := mgr.Keeper().RollupSwapSlip(ctx, maxAnchorBlocks, anchorAsset)
		if err != nil {
			ctx.Logger().Error("failed to rollup swap slip", "asset", anchorAsset, "err", err)
			continue
		}

		totalRuneDepth = totalRuneDepth.Add(p.BalanceRune)
		availableAnchors = append(availableAnchors, anchorAsset)
		slippageCollector = append(slippageCollector, cosmos.NewUint(slip.Uint64()))
	}

	slippage := getMedian(slippageCollector)
	price := anchorMedian(ctx, mgr, availableAnchors)

	return totalRuneDepth, price, slippage
}

func (vm *NetworkMgrV109) spawnDerivedAssets(ctx cosmos.Context, mgr Manager) error {
	active, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return err
	}

	if len(active) == 0 {
		return fmt.Errorf("dev error: no active asgard vaults")
	}

	// TODO: if a gas asset is removed from the network, this pool needs to be
	// removed

	// get assets to create derived pools
	layer1Assets := []common.Asset{common.TOR}
	for _, chain := range active[0].GetChains() {
		if chain.IsTHORChain() {
			continue
		}
		layer1Assets = append(layer1Assets, chain.GetGasAsset())
	}

	for _, asset := range layer1Assets {
		vm.SpawnDerivedAsset(ctx, asset, mgr)
	}

	return nil
}

func (vm *NetworkMgrV109) SpawnDerivedAsset(ctx cosmos.Context, asset common.Asset, mgr Manager) {
	var err error
	layer1 := asset
	if layer1.IsDerivedAsset() && !asset.Equals(common.TOR) {
		// NOTE: if the symbol of a derived asset isn't the chain, this won't work
		// (ie TERRA.LUNA)
		layer1.Chain, err = common.NewChain(layer1.Symbol.String())
		if err != nil {
			return
		}
	}
	if !asset.Equals(common.TOR) && !layer1.IsGasAsset() {
		return
	}

	maxAnchorSlip := mgr.Keeper().GetConfigInt64(ctx, constants.MaxAnchorSlip)
	depthBasisPts := mgr.Keeper().GetConfigInt64(ctx, constants.DerivedDepthBasisPts)
	minDepthPts := mgr.Keeper().GetConfigInt64(ctx, constants.DerivedMinDepth)

	derivedAsset := asset.GetDerivedAsset()
	pool, err := mgr.Keeper().GetPool(ctx, layer1)
	if err != nil {
		vm.suspendVirtualPool(ctx, mgr, derivedAsset, err)
		ctx.Logger().Error("failed to fetch pool", "asset", asset, "err", err)
		return
	}
	// when gas pool is not ready yet
	if pool.IsEmpty() && !asset.Equals(common.TOR) {
		return
	}

	if depthBasisPts == 0 {
		vm.suspendVirtualPool(ctx, mgr, derivedAsset, fmt.Errorf("derived pools have been disabled"))
		return
	}

	totalRuneDepth, price, slippage := vm.calcAnchor(ctx, mgr, layer1)
	if totalRuneDepth.IsZero() {
		vm.suspendVirtualPool(ctx, mgr, derivedAsset, fmt.Errorf("no anchor pools available"))
		return
	}
	if price.IsZero() {
		vm.suspendVirtualPool(ctx, mgr, derivedAsset, fmt.Errorf("fail to get asset price (%s)", asset))
		return
	}

	// Get the derivedPool for Status-checking.
	derivedPool, err := mgr.Keeper().GetPool(ctx, derivedAsset)
	if err != nil {
		// Since unable to get the derivedAsset pool, unable to check its Status for suspension.
		ctx.Logger().Error("failed to fetch pool", "asset", derivedAsset, "err", err)
		return
	}

	// Now inherit all properties from the reference pool except for Asset, Status, StatusSince.
	derivedPoolStatus := derivedPool.Status
	derivedPoolStatusSince := derivedPool.StatusSince
	derivedPool = pool
	derivedPool.Asset = derivedAsset
	derivedPool.Status = derivedPoolStatus
	derivedPool.StatusSince = derivedPoolStatusSince

	// If the pool is newly created, it will start with status PoolAvailable and StatusSince 0,
	// and still warrants a status change event and StatusSince update.
	if derivedPool.Status != PoolAvailable || derivedPoolStatusSince == 0 {
		derivedPool.Status = PoolAvailable
		derivedPool.StatusSince = ctx.BlockHeight()

		poolEvt := NewEventPool(derivedPool.Asset, PoolAvailable)
		if err := mgr.EventMgr().EmitEvent(ctx, poolEvt); err != nil {
			ctx.Logger().Error("fail to emit pool event", "asset", asset, "err", err)
			return
		}
		telemetry.IncrCounterWithLabels(
			[]string{"thornode", "derived_asset", "available"},
			float32(1),
			[]metrics.Label{telemetry.NewLabel("pool", derivedPool.Asset.String())},
		)
	}

	minRuneDepth := common.GetSafeShare(cosmos.NewUint(uint64(minDepthPts)), cosmos.NewUint(10000), totalRuneDepth)
	runeDepth := common.GetUncappedShare(cosmos.NewUint(uint64(depthBasisPts)), cosmos.NewUint(10000), totalRuneDepth)
	// adjust rune depth by median slippage. This is so high volume trading
	// causes the derived virtual pool to become more shallow making price
	// manipulation profitability significantly harder
	reverseSlip := common.SafeSub(cosmos.NewUint(uint64(maxAnchorSlip)), slippage)
	runeDepth = common.GetSafeShare(reverseSlip, cosmos.NewUint(uint64(maxAnchorSlip)), runeDepth)
	if runeDepth.LT(minRuneDepth) {
		runeDepth = minRuneDepth
	}
	assetDepth := runeDepth.Mul(price).QuoUint64(uint64(constants.DollarMulti * common.One))

	// emit an event for midgard
	runeAmt := common.SafeSub(runeDepth, derivedPool.BalanceRune)
	assetAmt := common.SafeSub(assetDepth, derivedPool.BalanceAsset)
	assetAdd, runeAdd := true, true
	if derivedPool.BalanceAsset.GT(assetDepth) {
		assetAdd = false
		assetAmt = common.SafeSub(derivedPool.BalanceAsset, assetDepth)
	}
	if derivedPool.BalanceRune.GT(runeDepth) {
		runeAdd = false
		runeAmt = common.SafeSub(derivedPool.BalanceRune, runeDepth)
	}

	// Only emit an EventPoolBalanceChanged if there's a balance change.
	if !assetAmt.IsZero() || !runeAmt.IsZero() {
		mod := NewPoolMod(derivedPool.Asset, runeAmt, runeAdd, assetAmt, assetAdd)
		emitPoolBalanceChangedEvent(ctx, mod, "derived pool adjustment", mgr)

		derivedPool.BalanceAsset = assetDepth
		derivedPool.BalanceRune = runeDepth
	}

	if err := mgr.Keeper().SetPool(ctx, derivedPool); err != nil {
		// Since unable to SetPool here, presumably unable to SetPool in suspendVirtualPool either.
		ctx.Logger().Error("failed to set pool", "asset", derivedPool.Asset, "err", err)
		return
	}
}

// EndBlock move funds from retiring asgard vaults
func (vm *NetworkMgrV109) EndBlock(ctx cosmos.Context, mgr Manager) error {
	if ctx.BlockHeight() == genesisBlockHeight {
		return vm.processGenesisSetup(ctx)
	}
	controller := NewRouterUpgradeController(mgr)
	controller.Process(ctx)

	if err := vm.POLCycle(ctx, mgr); err != nil {
		ctx.Logger().Error("fail to process POL liquidity", "error", err)
	}

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

	migrationRounds := mgr.GetConstants().GetInt64Value(constants.ChurnMigrateRounds)

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
				if nth < migrationRounds { // migrate partial funds 4 times (migrationRounds is 5 in mainnet)
					// each round of migration, we are increasing the amount 20%.
					// Round 1 = 20%
					// Round 2 = 40%
					// Round 3 = 60%
					// Round 4 = 80%
					// Round 5 = 100%
					amt = amt.MulUint64(uint64(nth)).QuoUint64(uint64(migrationRounds))
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
					if amt.IsZero() && nth > migrationRounds {
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

// paySaverYield - takes a pool asset and total rune collected in yield to the pool, then pays out savers their proportion of yield based on its size (relative to dual side LPs) and the SynthYieldBasisPoints
func (vm *NetworkMgrV109) paySaverYield(ctx cosmos.Context, asset common.Asset, runeAmt cosmos.Uint) error {
	pool, err := vm.k.GetPool(ctx, asset.GetLayer1Asset())
	if err != nil {
		return err
	}

	// if saver's layer 1 pool is empty, skip
	// if the pool is not active, no need to pay synths for yield
	if pool.BalanceAsset.IsZero() || pool.Status != PoolAvailable {
		return nil
	}

	saver, err := vm.k.GetPool(ctx, asset.GetSyntheticAsset())
	if err != nil {
		return err
	}

	if saver.BalanceAsset.IsZero() || saver.LPUnits.IsZero() {
		return nil
	}

	basisPts, err := vm.k.GetMimir(ctx, constants.SynthYieldBasisPoints.String())
	if basisPts < 0 || err != nil {
		constAccessor := constants.GetConstantValues(vm.k.GetVersion())
		basisPts = constAccessor.GetInt64Value(constants.SynthYieldBasisPoints)
		if err != nil {
			ctx.Logger().Error("fail to fetch mimir value", "key", constants.SynthYieldBasisPoints.String(), "error", err)
			return err
		}
	}
	if basisPts <= 0 {
		return nil
	}

	assetAmt := pool.RuneValueInAsset(runeAmt)
	// get the portion of the assetAmt based on the pool depth (asset * 2) and
	// the saver asset balance
	earnings := common.GetSafeShare(saver.BalanceAsset, pool.BalanceAsset.MulUint64(2), assetAmt)
	earnings = common.GetSafeShare(cosmos.NewUint(uint64(basisPts)), cosmos.NewUint(10_000), earnings)
	if earnings.IsZero() {
		return nil
	}

	// Mint the corresponding amount of synths
	coin := common.NewCoin(saver.Asset.GetSyntheticAsset(), earnings)
	if err := vm.k.MintToModule(ctx, ModuleName, coin); err != nil {
		ctx.Logger().Error("fail to mint synth rewards", "error", err)
		return err
	}

	// send synths to asgard module
	if err := vm.k.SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(coin)); err != nil {
		ctx.Logger().Error("fail to move module synths", "error", err)
		return err
	}

	// update synthetic saver state with new synths
	saver.BalanceAsset = saver.BalanceAsset.Add(earnings)
	if err := vm.k.SetPool(ctx, saver); err != nil {
		ctx.Logger().Error("fail to save saver", "saver", saver.Asset, "error", err)
		return err
	}

	// emit event
	modAddress, err := vm.k.GetModuleAddress(ModuleName)
	if err != nil {
		return err
	}
	asgardAddress, err := vm.k.GetModuleAddress(AsgardName)
	if err != nil {
		return err
	}
	tx := common.NewTx(common.BlankTxID, modAddress, asgardAddress, common.NewCoins(coin), nil, "THOR-SAVERS-YIELD")
	donateEvt := NewEventDonate(saver.Asset, tx)
	if err := vm.eventMgr.EmitEvent(ctx, donateEvt); err != nil {
		return cosmos.Wrapf(errFailSaveEvent, "fail to save donate events: %w", err)
	}
	return nil
}

func (vm *NetworkMgrV109) POLCycle(ctx cosmos.Context, mgr Manager) error {
	maxDeposit := fetchConfigInt64(ctx, mgr, constants.POLMaxNetworkDeposit)
	movement := fetchConfigInt64(ctx, mgr, constants.POLMaxPoolMovement)
	target := fetchConfigInt64(ctx, mgr, constants.POLTargetSynthPerPoolDepth)
	buf := fetchConfigInt64(ctx, mgr, constants.POLBuffer)
	targetSynthPerPoolDepth := cosmos.NewUint(uint64(target))
	maxMovement := cosmos.NewUint(uint64(movement))
	buffer := cosmos.NewUint(uint64(buf))

	// if POLTargetSynthPerPoolDepth is zero, disable POL
	if target == 0 {
		return nil
	}

	pol, err := mgr.Keeper().GetPOL(ctx)
	if err != nil {
		return err
	}

	nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return err
	}
	if len(nodeAccounts) == 0 {
		return fmt.Errorf("dev err: no active node accounts")
	}
	signer := nodeAccounts[0].NodeAddress

	polAddress, err := mgr.Keeper().GetModuleAddress(ReserveName)
	if err != nil {
		return err
	}
	asgardAddress, err := mgr.Keeper().GetModuleAddress(AsgardName)
	if err != nil {
		return err
	}

	pools := vm.fetchPOLPools(ctx, mgr)

	if len(pools) == 0 {
		return fmt.Errorf("no POL pools")
	}

	pool := pools[int(ctx.BlockHeight()%int64(len(pools)))]

	// The POL key for the ETH.ETH pool would be POL-ETH-ETH .
	key := "POL-" + pool.Asset.MimirString()
	val, err := mgr.Keeper().GetMimir(ctx, key)
	if err != nil {
		ctx.Logger().Error("fail to manage POL in pool", "pool", pool.Asset.String(), "error", err)
		return nil
	}

	// if pool isn't available or mimir has it configured, force withdraw from the pool
	if val == 2 || pool.Status != PoolAvailable {
		targetSynthPerPoolDepth = cosmos.NewUint(10_000)
	}

	synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	pool.CalcUnits(mgr.GetVersion(), synthSupply)
	synthPerPoolDepth := common.GetUncappedShare(pool.SynthUnits, pool.GetPoolUnits(), cosmos.NewUint(10_000))

	// detect if we need to deposit rune
	if common.SafeSub(synthPerPoolDepth, buffer).GT(targetSynthPerPoolDepth) {
		if maxDeposit <= pol.CurrentDeposit().Int64() {
			ctx.Logger().Info("maximum rune deployed from POL")
			return nil
		}
		if err := vm.addPOLLiquidity(ctx, pool, polAddress, asgardAddress, signer, maxMovement, synthPerPoolDepth, targetSynthPerPoolDepth, mgr); err != nil {
			ctx.Logger().Error("fail to manage POL in pool", "pool", pool.Asset.String(), "error", err)
		}
		return nil
	}

	// detect if we need to withdraw rune
	if synthPerPoolDepth.Add(buffer).LT(targetSynthPerPoolDepth) {
		if err := vm.removePOLLiquidity(ctx, pool, polAddress, asgardAddress, signer, maxMovement, synthPerPoolDepth, targetSynthPerPoolDepth, mgr); err != nil {
			ctx.Logger().Error("fail to manage POL in pool", "pool", pool.Asset.String(), "error", err)
		}
	}

	return nil
}

// generated a filtered list of pools that the POL is active with
func (mv *NetworkMgrV109) fetchPOLPools(ctx cosmos.Context, mgr Manager) Pools {
	var pools Pools
	iterator := mgr.Keeper().GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &pool)
		if err != nil {
			ctx.Logger().Error("fail to unmarshal pool", "pool", pool.Asset.String(), "error", err)
			continue
		}

		if pool.Asset.IsSyntheticAsset() {
			continue
		}

		if pool.BalanceRune.IsZero() {
			continue
		}

		if pool.Status == PoolSuspended {
			continue
		}

		if isChainTradingHalted(ctx, mgr, pool.Asset.GetChain()) || isGlobalTradingHalted(ctx, mgr) {
			continue
		}

		// The POL key for the ETH.ETH pool would be POL-ETH-ETH .
		key := "POL-" + pool.Asset.MimirString()
		val, err := mgr.Keeper().GetMimir(ctx, key)
		if err != nil {
			ctx.Logger().Error("fail to manage POL in pool", "pool", pool.Asset.String(), "error", err)
			continue
		}

		// -1 is unset default behaviour; 0 is off (paused); 1 is on; 2 (elsewhere) is forced withdraw.
		switch val {
		case -1:
			continue // unset default behaviour:  pause POL movements
		case 0:
			continue // off behaviour:  pause POL movements
		case 1:
			// on behaviour:  POL is enabled
		}

		pools = append(pools, pool)
	}

	return pools
}

func (vm *NetworkMgrV109) addPOLLiquidity(
	ctx cosmos.Context,
	pool Pool,
	polAddress, asgardAddress common.Address,
	signer cosmos.AccAddress,
	maxMovement, synthPerPoolDepth, targetSynthPerPoolDepth cosmos.Uint,
	mgr Manager,
) error {
	handler := NewInternalHandler(mgr)

	// NOTE: move is in hundredths of a basis point
	move := synthPerPoolDepth.Sub(targetSynthPerPoolDepth).MulUint64(100)
	if move.GT(maxMovement) {
		move = maxMovement
	}

	runeAmt := common.GetSafeShare(move, cosmos.NewUint(1000_000), pool.BalanceRune)
	if runeAmt.IsZero() {
		return nil
	}
	coins := common.NewCoins(common.NewCoin(common.RuneAsset(), runeAmt))

	// check balance
	bal := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)
	if runeAmt.GT(bal) {
		return nil
	}
	if err := mgr.Keeper().SendFromModuleToModule(ctx, ReserveName, AsgardName, coins); err != nil {
		return err
	}

	tx := common.NewTx(common.BlankTxID, polAddress, asgardAddress, coins, nil, "THOR-POL-ADD")
	msg := NewMsgAddLiquidity(tx, pool.Asset, runeAmt, cosmos.ZeroUint(), polAddress, common.NoAddress, common.NoAddress, cosmos.ZeroUint(), signer)
	_, err := handler(ctx, msg)
	if err != nil {
		// revert the rune back to the reserve
		if err := mgr.Keeper().SendFromModuleToModule(ctx, AsgardName, ReserveName, coins); err != nil {
			return err
		}
	}
	return err
}

func (vm *NetworkMgrV109) removePOLLiquidity(
	ctx cosmos.Context,
	pool Pool,
	polAddress, asgardAddress common.Address,
	signer cosmos.AccAddress,
	maxMovement, synthPerPoolDepth, targetSynthPerPoolDepth cosmos.Uint,
	mgr Manager,
) error {
	handler := NewInternalHandler(mgr)

	lp, err := mgr.Keeper().GetLiquidityProvider(ctx, pool.Asset, polAddress)
	if err != nil {
		return err
	}
	if lp.Units.IsZero() {
		// no LP position to withdraw
		return nil
	}

	// NOTE: move is in hundredths of a basis point
	move := targetSynthPerPoolDepth.Sub(synthPerPoolDepth).MulUint64(100)
	if move.GT(maxMovement) {
		move = maxMovement
	}

	runeAmt := common.GetSafeShare(move, cosmos.NewUint(1000_000), pool.BalanceRune)
	if runeAmt.IsZero() {
		return nil
	}
	lpRune := common.GetSafeShare(lp.Units, pool.GetPoolUnits(), pool.BalanceRune).MulUint64(2)
	basisPts := common.GetSafeShare(runeAmt, lpRune, cosmos.NewUint(10_000))

	coins := common.NewCoins(common.NewCoin(common.RuneAsset(), cosmos.OneUint()))
	tx := common.NewTx(common.BlankTxID, polAddress, asgardAddress, coins, nil, "THOR-POL-REMOVE")
	msg := NewMsgWithdrawLiquidity(
		tx,
		polAddress,
		basisPts,
		pool.Asset,
		common.RuneAsset(),
		signer,
	)

	_, err = handler(ctx, msg)
	return err
}

// TriggerKeygen generate a record to instruct signer kick off keygen process
func (vm *NetworkMgrV109) TriggerKeygen(ctx cosmos.Context, nas NodeAccounts) error {
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
func (vm *NetworkMgrV109) RotateVault(ctx cosmos.Context, vault Vault) error {
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

	vault.UpdateStatus(ActiveVault, ctx.BlockHeight())
	if err := vm.k.SetVault(ctx, vault); err != nil {
		return err
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent(EventTypeActiveVault,
			cosmos.NewAttribute("add new asgard vault", vault.PubKey.String())))
	if err := vm.cleanupAsgardIndex(ctx); err != nil {
		ctx.Logger().Error("fail to clean up asgard index", "error", err)
	}
	return nil
}

func (vm *NetworkMgrV109) cleanupAsgardIndex(ctx cosmos.Context) error {
	asgards, err := vm.k.GetAsgardVaults(ctx)
	if err != nil {
		return fmt.Errorf("fail to get all asgards,err: %w", err)
	}
	for _, vault := range asgards {
		if vault.PubKey.IsEmpty() {
			continue
		}
		if !vault.IsAsgard() {
			continue
		}
		if vault.Status == InactiveVault {
			if err := vm.k.RemoveFromAsgardIndex(ctx, vault.PubKey); err != nil {
				ctx.Logger().Error("fail to remove inactive asgard from index", "error", err)
			}
		}
	}
	return nil
}

// manageChains - checks to see if we have any chains that we are ragnaroking,
// and ragnaroks them
func (vm *NetworkMgrV109) manageChains(ctx cosmos.Context, mgr Manager) error {
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
func (vm *NetworkMgrV109) findChainsToRetire(ctx cosmos.Context) (common.Chains, error) {
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
func (vm *NetworkMgrV109) RecallChainFunds(ctx cosmos.Context, chain common.Chain, mgr Manager, excludeNodes common.PubKeys) error {
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
func (vm *NetworkMgrV109) ragnarokChain(ctx cosmos.Context, chain common.Chain, nth int64, mgr Manager) error {
	nas, err := vm.k.ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("can't get active nodes", "error", err)
		return err
	}
	if chain.IsTHORChain() {
		return fmt.Errorf("can't ragnarok THORChain")
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
func (vm *NetworkMgrV109) withdrawLiquidity(ctx cosmos.Context, pool Pool, na NodeAccount, mgr Manager) error {
	if pool.Status == PoolSuspended {
		ctx.Logger().Info("cannot further withdraw liquidity from a suspended pool", "pool", pool.Asset)
		return nil
	}
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
func (vm *NetworkMgrV109) UpdateNetwork(ctx cosmos.Context, constAccessor constants.ConstantValues, gasManager GasManager, eventMgr EventManager) error {
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

	// Reserve-emitted block rewards (not liquidity fees) are based on totalReserve, thus the Reserve should always have enough for them.
	// The same does not go for liquidity fees; liquidity fees sent from pools to the Reserve (negative pool rewards)
	// are to be passed on as bond rewards, so pool reward transfers should be processed before the bond reward transfer.

	var evtPools []PoolAmt

	if !totalPoolRewards.IsZero() { // If Pool Rewards to hand out
		var rewardAmts []cosmos.Uint
		var rewardPools []Pool
		// Pool Rewards are based on Fee Share
		for _, pool := range pools {
			if !pool.IsAvailable() {
				continue
			}
			var amt, fees cosmos.Uint
			if totalLiquidityFees.IsZero() {
				amt = common.GetSafeShare(pool.BalanceRune, totalProvidedLiquidity, totalPoolRewards)
				fees = cosmos.ZeroUint()
			} else {
				var err error
				fees, err = vm.k.GetPoolLiquidityFees(ctx, currentHeight, pool.Asset)
				if err != nil {
					ctx.Logger().Error("fail to get fees", "error", err)
					continue
				}
				amt = common.GetSafeShare(fees, totalLiquidityFees, totalPoolRewards)
			}
			if err := vm.paySaverYield(ctx, pool.Asset, amt.Add(fees)); err != nil {
				return fmt.Errorf("fail to pay saver yield: %w", err)
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

	if !bondReward.IsZero() {
		coin := common.NewCoin(common.RuneNative, bondReward)
		if err := vm.k.SendFromModuleToModule(ctx, ReserveName, BondName, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to transfer funds from reserve to bond", "error", err)
			return fmt.Errorf("fail to transfer funds from reserve to bond: %w", err)
		}
	}
	network.BondRewardRune = network.BondRewardRune.Add(bondReward) // Add here for individual Node collection later

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

func (vm *NetworkMgrV109) getTotalProvidedLiquidityRune(ctx cosmos.Context) (Pools, cosmos.Uint, error) {
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
		if pool.Asset.IsNative() {
			continue
		}
		if !pool.BalanceRune.IsZero() {
			totalProvidedLiquidity = totalProvidedLiquidity.Add(pool.BalanceRune)
			pools = append(pools, pool)
		}
	}
	return pools, totalProvidedLiquidity, nil
}

func (vm *NetworkMgrV109) getTotalActiveBond(ctx cosmos.Context) (cosmos.Uint, error) {
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
func (vm *NetworkMgrV109) payPoolRewards(ctx cosmos.Context, poolRewards []cosmos.Uint, pools Pools) error {
	for i, reward := range poolRewards {
		if reward.IsZero() {
			continue
		}
		pools[i].BalanceRune = pools[i].BalanceRune.Add(reward)
		if err := vm.k.SetPool(ctx, pools[i]); err != nil {
			return fmt.Errorf("fail to set pool: %w", err)
		}
		coin := common.NewCoin(common.RuneNative, reward)
		if err := vm.k.SendFromModuleToModule(ctx, ReserveName, AsgardName, common.NewCoins(coin)); err != nil {
			return fmt.Errorf("fail to transfer funds from reserve to asgard: %w", err)
		}
	}
	return nil
}

// Calculate pool deficit based on the pool's accrued fees compared with total fees.
func (vm *NetworkMgrV109) calcPoolDeficit(lpDeficit, totalFees, poolFees cosmos.Uint) cosmos.Uint {
	return common.GetSafeShare(poolFees, totalFees, lpDeficit)
}

// Calculate the block rewards that bonders and liquidity providers should receive
func (vm *NetworkMgrV109) calcBlockRewards(totalProvidedLiquidity, totalBonded, totalReserve, totalLiquidityFees cosmos.Uint, emissionCurve, incentiveCurve, blocksPerYear int64) (cosmos.Uint, cosmos.Uint, cosmos.Uint, cosmos.Uint) {
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

func (vm *NetworkMgrV109) getPoolShare(incentiveCurve int64, totalProvidedLiquidity, totalBonded, totalRewards cosmos.Uint) cosmos.Uint {
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
func (vm *NetworkMgrV109) deductPoolRewardDeficit(ctx cosmos.Context, pools Pools, totalLiquidityFees, lpDeficit cosmos.Uint) ([]PoolAmt, error) {
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
		if err := vm.paySaverYield(ctx, pool.Asset, common.SafeSub(poolFees, poolDeficit)); err != nil {
			ctx.Logger().Error("fail to pay saver yield", "error", err)
		}

		// when pool deficit is zero , the pool doesn't pay deficit
		if poolDeficit.IsZero() {
			continue
		}
		coin := common.NewCoin(common.RuneNative, poolDeficit)
		if err := vm.k.SendFromModuleToModule(ctx, AsgardName, ReserveName, common.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to transfer funds from asgard to reserve", "error", err)
			return poolAmts, fmt.Errorf("fail to transfer funds from asgard to reserve: %w", err)
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
func (vm *NetworkMgrV109) checkPoolRagnarok(ctx cosmos.Context, mgr Manager) error {
	// check whether pool need to be ragnarok per constants.FundMigrationInterval
	if ctx.BlockHeight()%mgr.GetConstants().GetInt64Value(constants.FundMigrationInterval) > 0 {
		return nil
	}
	pools, err := vm.k.GetPools(ctx)
	if err != nil {
		return err
	}

	for _, pool := range pools {
		// The Ragnarok key for the TERRA.UST pool would be RAGNAROK-TERRA-UST .
		k := "RAGNAROK-" + pool.Asset.MimirString()
		v, err := vm.k.GetMimir(ctx, k)
		if err != nil {
			ctx.Logger().Error("fail to get mimir value", "mimir", k, "error", err)
			continue
		}
		if v < 1 {
			continue
		}
		if pool.Asset.IsGasAsset() && !vm.canRagnarokGasPool(ctx, pool.Asset.GetChain(), pools) {
			continue
		}
		if err := vm.ragnarokPool(ctx, mgr, pool); err != nil {
			ctx.Logger().Error("fail to ragnarok pool", "error", err)
		}
	}

	return nil
}

// canRagnarokGasPool check whether a gas pool can be ragnarok
// On blockchain that support multiple assets, make sure gas pool doesn't get ragnarok before none-gas asset pool
func (vm *NetworkMgrV109) canRagnarokGasPool(ctx cosmos.Context, c common.Chain, allPools Pools) bool {
	for _, pool := range allPools {
		if pool.Status == PoolSuspended {
			continue
		}
		if pool.Asset.GetChain().Equals(c) && !pool.Asset.IsGasAsset() {
			ctx.Logger().
				With("asset", pool.Asset.String()).
				Info("gas asset pool can't ragnarok when none-gas asset pool still exist")
			return false
		}
	}
	return true
}

func (vm *NetworkMgrV109) redeemSynthAssetToReserve(ctx cosmos.Context, p Pool) error {
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

func (vm *NetworkMgrV109) ragnarokPool(ctx cosmos.Context, mgr Manager, p Pool) error {
	if p.Status == PoolSuspended {
		ctx.Logger().Info("cannot further ragnarok a suspended pool", "pool", p.Asset)
		return nil
	}
	startBlockHeight, err := vm.k.GetPoolRagnarokStart(ctx, p.Asset)
	if err != nil || startBlockHeight == 0 {
		if err != nil {
			ctx.Logger().Error("fail to get pool ragnarok start block height", "error", err)
		}

		// redeem all synth asset from the pool , and send RUNE to reserve
		if err := vm.redeemSynthAssetToReserve(ctx, p); err != nil {
			ctx.Logger().Error("fail to redeem synth to reserve, continue to ragnarok", "error", err)
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
