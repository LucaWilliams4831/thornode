package keeperv1

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	crisis "github.com/cosmos/cosmos-sdk/x/crisis/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// InvariantRoutes return the keeper's invariant routes
func (k KVStore) InvariantRoutes() []crisis.InvarRoute {
	return []crisis.InvarRoute{
		crisis.NewInvarRoute(ModuleName, "asgard", AsgardInvariant(k)),
		crisis.NewInvarRoute(ModuleName, "bond", BondInvariant(k)),
		crisis.NewInvarRoute(ModuleName, "thorchain", THORChainInvariant(k)),
	}
}

// AsgardInvariant the asgard module backs pool rune, savers synths, and native
// coins in queued swaps
func AsgardInvariant(k KVStore) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		pools, err := k.GetPools(ctx)
		if err != nil {
			return err.Error(), true
		}

		var expCoins common.Coins
		for _, pool := range pools {
			switch {
			case pool.Asset.IsSyntheticAsset():
				coin := common.NewCoin(
					pool.Asset,
					pool.BalanceAsset,
				)
				expCoins = expCoins.Add(coin)
			case !pool.Asset.IsDerivedAsset():
				coin := common.NewCoin(
					common.RuneAsset(),
					pool.BalanceRune.Add(pool.PendingInboundRune),
				)
				expCoins = expCoins.Add(coin)
			}
		}

		swapIter := k.GetSwapQueueIterator(ctx)
		defer swapIter.Close()
		for ; swapIter.Valid(); swapIter.Next() {
			var msg MsgSwap
			if err := k.Cdc().Unmarshal(swapIter.Value(), &msg); err != nil {
				continue
			}
			for _, coin := range msg.Tx.Coins {
				if coin.IsNative() {
					expCoins = expCoins.Add(coin)
				}
			}
		}

		expNative, err := expCoins.Native()
		if err != nil {
			return err.Error(), true
		}

		asgardAddr := k.GetModuleAccAddress(AsgardName)
		asgardCoins := k.GetBalance(ctx, asgardAddr)

		var msg string
		broken := false

		diffCoins, _ := asgardCoins.SafeSub(expNative.Sort())
		if !diffCoins.IsZero() {
			broken = true
			for _, coin := range diffCoins {
				if coin.IsPositive() {
					msg += fmt.Sprintf("oversolvent: %s\n", coin)
				} else {
					coin.Amount = coin.Amount.Neg()
					msg += fmt.Sprintf("insolvent: %s\n", coin)
				}
			}
		}

		return msg, broken
	}
}

// BondInvariant the bond module backs node bond and pending reward bond
func BondInvariant(k KVStore) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		bondedRune := cosmos.ZeroUint()
		naIterator := k.GetNodeAccountIterator(ctx)
		defer naIterator.Close()
		for ; naIterator.Valid(); naIterator.Next() {
			var na NodeAccount
			if err := k.Cdc().Unmarshal(naIterator.Value(), &na); err != nil {
				return fmt.Errorf("failed to unmarshal node account: %w", err).Error(), true
			}
			bondedRune = bondedRune.Add(na.Bond)
		}

		network, err := k.GetNetwork(ctx)
		if err != nil {
			return fmt.Errorf("failed to get network: %w", err).Error(), true
		}
		bondRewardRune := network.BondRewardRune

		bondModuleRune := k.GetBalanceOfModule(ctx, BondName, common.RuneAsset().Native())

		expectedRune := bondedRune.Add(bondRewardRune)

		if expectedRune.Equal(bondModuleRune) {
			return "", false
		}

		var msg string
		if expectedRune.GT(bondModuleRune) {
			diff := expectedRune.Sub(bondModuleRune)
			coin, _ := common.NewCoin(common.RuneAsset(), diff).Native()
			msg = fmt.Sprintf("insolvent: %s", coin)
		} else {
			diff := bondModuleRune.Sub(expectedRune)
			coin, _ := common.NewCoin(common.RuneAsset(), diff).Native()
			msg = fmt.Sprintf("oversolvent: %s", coin)
		}

		return msg, true
	}
}

// THORChainInvariant the thorchain module should never hold a balance
func THORChainInvariant(k KVStore) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		tcAddr := k.GetModuleAccAddress(ModuleName)
		tcCoins := k.GetBalance(ctx, tcAddr)

		var msg string
		broken := false

		if !tcCoins.Empty() {
			broken = true
			for _, coin := range tcCoins {
				msg += fmt.Sprintf("oversolvent: %s\n", coin)
			}
		}

		return msg, broken
	}
}
