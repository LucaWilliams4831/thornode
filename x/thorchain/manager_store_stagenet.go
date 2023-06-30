//go:build stagenet
// +build stagenet

package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func migrateStoreV86(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV88(ctx cosmos.Context, mgr Manager) {}

func migrateStoreV102(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v102", "error", err)
		}
	}()

	// STAGENET TESTING
	// Refund a 10 RUNE swap out tx that was eaten due to bad external asset matching:
	// https://stagenet-thornode.ninerealms.com/thorchain/tx/5FAAE55F9043580A1387E66CB9D749A5D262CED5F6F654640918149F71D8E4D6/signers

	// The RUNE was swapped to ETH, but the outbound swap out was dropped by Bifrost. This means RUNE was added, ETH was removed from
	// the pool. This must be reversed and the RUNE sent back to the user.
	// So:
	// 1. Credit the total ETH amount back the pool, this ETH is already in the pool since the outbound was dropped.
	// 2. Deduct the RUNE balance from the ETH pool, this will be sent back to the user.
	// 3. Send the user RUNE from Asgard.
	//
	// Note: the Asgard vault does not need to be credited the ETH since the outbound was never sent, thus never observed (which
	// is where Vault funds are subtracted)

	firstSwapOut := DroppedSwapOutTx{
		inboundHash: "5FAAE55F9043580A1387E66CB9D749A5D262CED5F6F654640918149F71D8E4D6",
		gasAsset:    common.ETHAsset,
	}

	err := refundDroppedSwapOutFromRUNE(ctx, mgr, firstSwapOut)
	if err != nil {
		ctx.Logger().Error("fail to migrate store to v102 refund failed", "error", err)
	}
}

// no op
func migrateStoreV103(ctx cosmos.Context, mgr *Mgrs) {}

// no op
func migrateStoreV106(ctx cosmos.Context, mgr *Mgrs) {}

// no op
func migrateStoreV108(ctx cosmos.Context, mgr *Mgrs) {}

// no op
func migrateStoreV109(ctx cosmos.Context, mgr *Mgrs) {}

// no op
func migrateStoreV110(ctx cosmos.Context, mgr *Mgrs) {}

// no op
func migrateStoreV111(ctx cosmos.Context, mgr *Mgrs) {}

// no op
func migrateStoreV113(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV114(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v114", "error", err)
		}
	}()

	// TWO PART MIGRATION
	// This migration includes a fix to the BNB pool and a requeues a MIGRATE tx
	// to allow stagenet to continue to churn.

	// PART 1: Fix BNB pool LP units

	// the LPs get zero'd out on the BNB pool in stagenet
	// we will set the LP units equal to the amount of RUNE in the pool
	// then we'll allocate all of those LP units to the stagenet funding address
	// this willl give 100% of the pool share to the stagenet funding address
	// and will make swapping through the BNB pool possible again

	bnbPool, err := mgr.Keeper().GetPool(ctx, common.BNBAsset)
	if err != nil {
		ctx.Logger().Error("fail to get BNB pool", "error", err)
		return
	}

	bnbRune := bnbPool.BalanceRune
	bnbPool.LPUnits = bnbRune

	stagenetFundingAddr := common.Address("bnb1laxspje9u0faauqh7j07p9x6ds8lg4ychhg5qh")
	bnbLP, err := mgr.Keeper().GetLiquidityProvider(ctx, common.BNBAsset, stagenetFundingAddr)
	if err != nil {
		ctx.Logger().Error("fail to get BNB LP", "error", err)
		return
	}
	bnbLP.Units = bnbRune

	mgr.Keeper().SetLiquidityProvider(ctx, bnbLP)
	err = mgr.Keeper().SetPool(ctx, bnbPool)
	if err != nil {
		ctx.Logger().Error("fail to set BNB pool", "error", err)
		return
	}

	// PART 2: Requeue MIGRATE transactions

	bscActiveVault, err := common.NewAddress("0xfef0090e45f13d1e49d8503d585c50dfab0892cc")
	dogeActiveVault, err := common.NewAddress("DMR1YegqDDNogd7wKojteLT5MQySAnmLhJ")

	height := ctx.BlockHeight()
	bscMigrate := TxOutItem{
		Chain:       common.BSCChain,
		ToAddress:   bscActiveVault,
		VaultPubKey: common.PubKey("sthorpub1addwnpepqtv607zqd3wt062hlzc8qakngkhn6jcmtzz0zecxvl82kz9fmehy2hvj6mz"),
		Coin:        common.NewCoin(common.BNBBEP20Asset, cosmos.NewUint(3421001000)),
		Memo:        fmt.Sprintf("MIGRATE:%d", height),
		InHash:      common.TxID(""),
	}

	dogeMigrate := TxOutItem{
		Chain:       common.DOGEChain,
		ToAddress:   dogeActiveVault,
		VaultPubKey: common.PubKey("sthorpub1addwnpepqtv607zqd3wt062hlzc8qakngkhn6jcmtzz0zecxvl82kz9fmehy2hvj6mz"),
		Coin:        common.NewCoin(common.DOGEAsset, cosmos.NewUint(4982159450950)),
		Memo:        fmt.Sprintf("MIGRATE:%d", height),
		InHash:      common.TxID(""),
	}

	err = mgr.txOutStore.UnSafeAddTxOutItem(ctx, mgr, bscMigrate)
	if err != nil {
		ctx.Logger().Error("fail to requeue BSC migrate tx", "error", err)
		return
	}
	err = mgr.txOutStore.UnSafeAddTxOutItem(ctx, mgr, dogeMigrate)
	if err != nil {
		ctx.Logger().Error("fail to requeue DOGE migrate tx", "error", err)
		return
	}
}
