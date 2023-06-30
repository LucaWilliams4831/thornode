package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

var mimirStopFundYggdrasil = `StopFundYggdrasil`

// YggMgrV112 is an implementation of YggManager
type YggMgrV112 struct {
	keeper keeper.Keeper
}

// newYggMgrV112 create a new instance of YggMgrV112 which implement YggManager interface
func newYggMgrV112(keeper keeper.Keeper) *YggMgrV112 {
	return &YggMgrV112{
		keeper: keeper,
	}
}

// Fund is a method to fund yggdrasil pool
func (ymgr YggMgrV112) Fund(ctx cosmos.Context, mgr Manager) error {
	// Check if we have triggered the ragnarok protocol
	ragnarokHeight, err := ymgr.keeper.GetRagnarokBlockHeight(ctx)
	if err != nil {
		return fmt.Errorf("fail to get ragnarok height: %w", err)
	}
	if ragnarokHeight > 0 {
		return nil
	}
	stopFundYggdrasil, err := mgr.Keeper().GetMimir(ctx, mimirStopFundYggdrasil)
	if err == nil && stopFundYggdrasil > 0 {
		ctx.Logger().Debug("mimir stop fund yggdrasil")
		return nil
	}

	// Check we're not migrating funds
	retiring, err := ymgr.keeper.GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		ctx.Logger().Error("fail to get retiring vaults", "error", err)
		return err
	}
	if len(retiring) > 0 {
		// skip yggdrasil funding while a migration is in progress
		return nil
	}

	// find total bonded
	totalBond := cosmos.ZeroUint()
	nodeAccs, err := ymgr.keeper.ListActiveValidators(ctx)
	if err != nil {
		return err
	}
	minimumNodesForYggdrasil := mgr.GetConstants().GetInt64Value(constants.MinimumNodesForYggdrasil)
	if int64(len(nodeAccs)) < minimumNodesForYggdrasil {
		return nil
	}

	// check abandon yggdrasil
	if err := ymgr.abandonYggdrasilVaults(ctx, mgr); err != nil {
		ctx.Logger().Error("fail to check whether need to abandon yggdrasil vault", "error", err)
	}
	// Gather list of all pools
	pools, err := ymgr.keeper.GetPools(ctx)
	if err != nil {
		return err
	}

	for _, na := range nodeAccs {
		totalBond = totalBond.Add(na.Bond)
	}

	// We don't want to check all Yggdrasil pools every time THORNode run this
	// function. So THORNode use modulus to determine which Ygg THORNode process. This
	// should behave as a "round robin" approach checking one Ygg per block.
	// With 100 Ygg pools, THORNode should check each pool every 8.33 minutes.
	na := nodeAccs[ctx.BlockHeight()%int64(len(nodeAccs))]

	// check that we have enough bond
	minBond, err := ymgr.keeper.GetMimir(ctx, constants.MinimumBondInRune.String())
	if minBond < 0 || err != nil {
		minBond = mgr.GetConstants().GetInt64Value(constants.MinimumBondInRune)
	}
	if na.Bond.LT(cosmos.NewUint(uint64(minBond))) {
		return nil
	}

	// check that node account is not forced to leave
	if na.ForcedToLeave {
		return nil
	}

	// when node account request to leave , stop sending more yggdrasil fund to them
	if na.RequestedToLeave {
		return nil
	}

	// figure out if THORNode need to send them assets.
	// get a list of coin/amounts this yggdrasil pool should have, ideally.
	// TODO: We are assuming here that the pub key is Secp256K1
	ygg, err := ymgr.keeper.GetVault(ctx, na.PubKeySet.Secp256k1)
	if err != nil {
		if !errors.Is(err, kvTypes.ErrVaultNotFound) {
			return fmt.Errorf("fail to get yggdrasil: %w", err)
		}
		// get what chain asgard currently support
		asgards, err := ymgr.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			return fmt.Errorf("fail to get active asgards: %w", err)
		}
		if len(asgards) == 0 {
			return fmt.Errorf("can't find current active asgard: %w", err)
		}
		supportChains := asgards[0].GetChains()
		// supported chain for yggdrasil vault will be set at the time when it get created
		ygg = NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, na.PubKeySet.Secp256k1, supportChains.Strings(), ymgr.keeper.GetChainContracts(ctx, supportChains))
		ygg.Membership = append(ygg.Membership, na.PubKeySet.Secp256k1.String())

		if err := ymgr.keeper.SetVault(ctx, ygg); err != nil {
			return fmt.Errorf("fail to create yggdrasil pool: %w", err)
		}
	}
	if !ygg.IsYggdrasil() {
		return nil
	}
	maxBlock, err := ymgr.keeper.GetMimir(ctx, constants.YggFundRetry.String())
	if maxBlock < 0 || err != nil {
		maxBlock = mgr.GetConstants().GetInt64Value(constants.YggFundRetry)
	}
	pendingTxCount := ygg.LenPendingTxBlockHeights(ctx.BlockHeight(), maxBlock)
	if pendingTxCount > 0 {
		return fmt.Errorf("cannot send more yggdrasil funds while transactions are pending (%s: %d)", ygg.PubKey, pendingTxCount)
	}

	yggFundLimit, err := ymgr.keeper.GetMimir(ctx, constants.YggFundLimit.String())
	if yggFundLimit < 0 || err != nil {
		yggFundLimit = mgr.GetConstants().GetInt64Value(constants.YggFundLimit)
	}

	minRuneDepth, err := ymgr.keeper.GetMimir(ctx, constants.PoolDepthForYggFundingMin.String())
	if minRuneDepth < 0 || err != nil {
		minRuneDepth = mgr.GetConstants().GetInt64Value(constants.PoolDepthForYggFundingMin)
	}

	targetCoins, err := ymgr.calcTargetYggCoins(pools, ygg, na.Bond, totalBond, cosmos.NewUint(uint64(yggFundLimit)), cosmos.NewUint(uint64(minRuneDepth)))
	if err != nil {
		return err
	}

	var sendCoins common.Coins
	// iterate over each target coin amount and figure if THORNode need to reimburse
	// a Ygg pool of this particular asset.
	for _, targetCoin := range targetCoins {
		yggCoin := ygg.GetCoin(targetCoin.Asset)
		// check if the amount the ygg pool has is less that 50% of what
		// they are suppose to have, ideally. We refill them if they drop
		// below this line
		if yggCoin.Amount.LT(targetCoin.Amount.QuoUint64(2)) {
			sendCoins = append(
				sendCoins,
				common.NewCoin(
					targetCoin.Asset,
					common.SafeSub(targetCoin.Amount, yggCoin.Amount),
				),
			)
		}
	}

	if len(sendCoins) > 0 {
		count, err := ymgr.sendCoinsToYggdrasil(ctx, sendCoins, ygg, mgr)
		if err != nil {
			return err
		}
		for i := 0; i < count; i++ {
			ygg.AppendPendingTxBlockHeights(ctx.BlockHeight(), mgr.GetConstants())
		}
		if err := ymgr.keeper.SetVault(ctx, ygg); err != nil {
			return fmt.Errorf("fail to create yggdrasil pool: %w", err)
		}
	}

	return nil
}

// sendCoinsToYggdrasil - adds outbound txs to send the given coins to a
// yggdrasil pool
func (ymgr YggMgrV112) sendCoinsToYggdrasil(ctx cosmos.Context, coins common.Coins, ygg Vault, mgr Manager) (int, error) {
	var count int

	active, err := ymgr.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return count, err
	}

	for i := 1; i <= 2; i++ {
		// First iteration (1), we add gas assets. This is to ensure the vault
		// has gas to send transactions as it needs to
		// Second iteration (2), we add non-gas assets
		for _, coin := range coins {
			if i == 1 && !coin.Asset.GetChain().GetGasAsset().Equals(coin.Asset) {
				continue
			}
			if i == 2 && coin.Asset.GetChain().GetGasAsset().Equals(coin.Asset) {
				continue
			}

			// ignore amount 0
			if coin.Amount.Equal(cosmos.ZeroUint()) {
				continue
			}

			gasCoin, err := mgr.GasMgr().GetMaxGas(ctx, coin.Asset.GetChain())
			if err != nil {
				ctx.Logger().Error("fail to get max gas coin", "error", err)
				continue
			}

			if !ymgr.shouldFundYggdrasil(ctx, active[0], ygg, coin.Asset.GetChain()) {
				addr, _ := ygg.PubKey.GetThorAddress()
				ctx.Logger().Error("yggdrasil didn't upgrade contract, should not be funded", "address", addr)
				continue
			}
			if mgr.Keeper().IsChainHalted(ctx, coin.Asset.GetChain()) {
				ctx.Logger().Info("chain is halt , stop funding yggdrasil", "chain", coin.Asset.GetChain().String())
				continue
			}
			// when the coin need to be send to yggdrasil is gas coin , for example BNB(Binance) / BTC (Bitcoin)
			// the gas cost need to be count in , as the gas cost will be paid by the chosen vault
			totalAmount := coin.Amount
			if coin.Asset.Equals(gasCoin.Asset) {
				totalAmount = totalAmount.Add(gasCoin.Amount)
			}
			// select active vault to send funds from
			filterVaults := make(Vaults, 0)
			for _, v := range active {

				// when  vault doesn't have enough gas coin should be ignored from funding yggdrasil
				// for example, if the network need to send BUSD to yggdrasil vault , however
				// the least secure vault doesn't have enough BNB in it to pay for the fee, like less than (0.00375) in it
				if gasCoin.Amount.GT(v.GetCoin(gasCoin.Asset).Amount) {
					continue
				}

				// when vault doesn't have enough balance
				if totalAmount.GT(v.GetCoin(coin.Asset).Amount) {
					continue
				}

				filterVaults = append(filterVaults, v)
			}
			signingTransactionPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
			vault := ymgr.keeper.GetLeastSecure(ctx, filterVaults, signingTransactionPeriod)
			if vault.IsEmpty() {
				continue
			}

			to, err := ygg.PubKey.GetAddress(coin.Asset.GetChain())
			if err != nil {
				ctx.Logger().Error("fail to get address from pub key", "pub key", ygg.PubKey, "chain", coin.Asset.GetChain(), "error", err)
				continue
			}

			toi := TxOutItem{
				Chain:       coin.Asset.GetChain(),
				ToAddress:   to,
				InHash:      common.BlankTxID,
				Memo:        NewYggdrasilFund(ctx.BlockHeight()).String(),
				Coin:        coin,
				VaultPubKey: vault.PubKey,
				MaxGas: common.Gas{
					gasCoin,
				},
				GasRate: int64(mgr.GasMgr().GetGasRate(ctx, coin.Asset.GetChain()).Uint64()),
			}
			if err := mgr.TxOutStore().UnSafeAddTxOutItem(ctx, mgr, toi); err != nil {
				return count, err
			}
			count++
		}
	}

	return count, nil
}

// shoudFundYggdrasil  make sure asgard and the yggdrasil is using the same contract for the given chain.
// in a scenario that when contract get updated , all yggdrasil vaults will have to return their fund from old contract to
// new contract , once that happen and detected by THORChain, yggdrasil vault's smart contract will be updated to the new address
// if there are different , means yggdrasil didn't transfer their fund from old control to new one
// thus asgard should not send yggdrasil fund for the chain
func (ymgr YggMgrV112) shouldFundYggdrasil(ctx cosmos.Context, asgard, ygg Vault, chain common.Chain) bool {
	asgardContract := asgard.GetContract(chain)
	if asgardContract.IsEmpty() {
		// the request chain doesn't support contract
		return true
	}
	yggContract := ygg.GetContract(chain)
	return asgardContract.Router.Equals(yggContract.Router)
}

// calcTargetYggCoins - calculate the amount of coins of each pool a yggdrasil
// pool should have, relative to how much they have bonded (which should be
// target == bond * yggFundLimit / 100).
func (ymgr YggMgrV112) calcTargetYggCoins(pools []Pool, ygg Vault, yggBond, totalBond, yggFundLimit, minRuneDepth cosmos.Uint) (common.Coins, error) {
	var coins common.Coins

	// calculate total liquidity provided rune in our pools
	totalLiquidityRune := cosmos.ZeroUint()
	for _, pool := range pools {
		if pool.Asset.IsNative() {
			continue
		}
		totalLiquidityRune = totalLiquidityRune.Add(pool.BalanceRune)
	}
	if totalLiquidityRune.IsZero() {
		// if nothing is liquidity provided, no coins should be issued
		return nil, nil
	}

	// if we're under bonded, calculate as if we're not. Otherwise, we'll try
	// to send too much funds to ygg vaults
	bondVal := totalBond.MulUint64(2)
	if bondVal.LT(totalLiquidityRune.MulUint64(4)) {
		bondVal = totalLiquidityRune.MulUint64(4)
	}
	// figure out what percentage of the bond this yggdrasil pool has. They
	// should get half of that value.
	targetRune := common.GetSafeShare(yggBond, bondVal, totalLiquidityRune)
	// check if more rune would be allocated to this pool than their bond allows
	if targetRune.GT(yggBond.Mul(yggFundLimit).QuoUint64(100)) {
		targetRune = yggBond.Mul(yggFundLimit).QuoUint64(100)
	}

	// track how much value (in rune) we've associated with this ygg pool. This
	// is here just to be absolutely sure THORNode never send too many assets to the
	// ygg by accident.
	counter := cosmos.ZeroUint()
	for _, pool := range pools {
		if !pool.IsAvailable() {
			continue
		}
		if pool.Asset.IsNative() {
			continue
		}
		// Don't send an asset out to Yggs if the pool has less than PoolDepthForYggFundingMin
		if pool.BalanceRune.LT(minRuneDepth) {
			continue
		}
		runeAmt := common.GetSafeShare(targetRune, totalLiquidityRune, pool.BalanceRune)
		assetAmt := common.GetSafeShare(targetRune, totalLiquidityRune, pool.BalanceAsset)
		// add rune amt (not asset since the two are considered to be equal)
		// in a single pool X, the value of 1% asset X in RUNE ,equals the 1% RUNE in the same pool
		yggCoin := ygg.GetCoin(pool.Asset)
		coin := common.NewCoin(pool.Asset, cosmos.RoundToDecimal(common.SafeSub(assetAmt, yggCoin.Amount), pool.Decimals))
		if !coin.IsEmpty() {
			counter = counter.Add(runeAmt)
			if !coin.IsNative() {
				coins = append(coins, coin)
			}
		}
	}

	// ensure THORNode don't send too much value in coins to the ygg pool
	if counter.GT(yggBond.Mul(yggFundLimit).QuoUint64(100)) {
		return nil, fmt.Errorf("exceeded safe amounts of assets for given Yggdrasil pool (%d/%d)", counter.Uint64(), yggBond.QuoUint64(2).Uint64())
	}

	return coins, nil
}

// abandonYggdrasilVaults is going to find out those yggdrasil pool
func (ymgr YggMgrV112) abandonYggdrasilVaults(ctx cosmos.Context, mgr Manager) error {
	activeVaults, err := ymgr.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return fmt.Errorf("fail to get active asgard vaults: %w", err)
	}
	retiringAsgards, err := ymgr.keeper.GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		return fmt.Errorf("fail to get retiring asgard vaults: %w", err)
	}
	allVaults := append(activeVaults, retiringAsgards...) // nolint

	slasher := mgr.Slasher()
	vaultIter := ymgr.keeper.GetVaultIterator(ctx)
	defer vaultIter.Close()
	for ; vaultIter.Valid(); vaultIter.Next() {
		var v Vault
		if err := ymgr.keeper.Cdc().Unmarshal(vaultIter.Value(), &v); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
			continue
		}
		if !v.IsYggdrasil() {
			continue
		}
		if !v.HasFunds() {
			continue
		}
		na, err := ymgr.keeper.GetNodeAccountByPubKey(ctx, v.PubKey)
		if err != nil {
			ctx.Logger().Error("fail to get node account by pub key", "error", err, "pubkey", v.PubKey)
			continue
		}
		if na.Status != NodeDisabled {
			continue
		}
		if na.Bond.IsZero() {
			continue
		}

		// check whether the disabled node is part of the active vault / retiring vault
		// when the node is still belongs to the retiring vault means , it has just been churned out
		// thus give it more time to return yggdrasil fund
		shouldSlash := true
		for _, vault := range allVaults {
			if vault.Contains(na.PubKeySet.Secp256k1) {
				shouldSlash = false
				break
			}
		}
		if !shouldSlash {
			continue
		}

		if err := ymgr.slash(ctx, slasher, mgr, na.PubKeySet.Secp256k1, v); err != nil {
			ctx.Logger().Error("fail to slash node account", "key", na.PubKeySet.Secp256k1, "error", err)
			continue
		}

		// assume slash finished successfully, delete the yggdrasil vault
		if err := ymgr.keeper.DeleteVault(ctx, na.PubKeySet.Secp256k1); err != nil {
			ctx.Logger().Error("fail to delete yggdrasil vault", "key", na.PubKeySet.Secp256k1, "error", err)
		}
	}
	return nil
}

func (ymgr YggMgrV112) slash(ctx cosmos.Context, slasher Slasher, mgr Manager, pk common.PubKey, ygg Vault) error {
	ctx.Logger().Info(fmt.Sprintf("slash, node account %s churned out , but fail to return yggdrasil fund", pk.String()), "coins", ygg.Coins.String())
	err := slasher.SlashVault(ctx, pk, ygg.Coins, mgr)
	ygg.SubFunds(ygg.Coins)
	if err := ymgr.keeper.SetVault(ctx, ygg); err != nil {
		return fmt.Errorf("fail to save yggdrasil vault: %w", err)
	}
	return err
}
