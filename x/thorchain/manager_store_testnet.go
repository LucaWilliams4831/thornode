//go:build (testnet || mocknet) && !regtest
// +build testnet mocknet
// +build !regtest

package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// migrateStoreV86 remove all LTC asset from the retiring vault
func migrateStoreV86(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v86", "error", err)
		}
	}()
	vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		ctx.Logger().Error("fail to get retiring asgard vaults", "error", err)
		return
	}
	for _, v := range vaults {
		ltcCoin := v.GetCoin(common.LTCAsset)
		v.SubFunds(common.NewCoins(ltcCoin))
		if err := mgr.Keeper().SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}
}

func migrateStoreV88(ctx cosmos.Context, mgr Manager) {}

// no op
func migrateStoreV102(ctx cosmos.Context, mgr Manager) {}

// no op
func migrateStoreV103(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV106(ctx cosmos.Context, mgr *Mgrs) {
	// testing for migrateStoreV106 in chaosnet
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v106", "error", err)
		}
	}()

	recipient, err := cosmos.AccAddressFromBech32("tthor1zf3gsk7edzwl9syyefvfhle37cjtql35h6k85m")
	if err != nil {
		ctx.Logger().Error("fail to create acc address from bech32", err)
		return
	}

	coins := cosmos.NewCoins(cosmos.NewCoin(
		"btc/btc",
		cosmos.NewInt(488432852150),
	))
	if err := mgr.coinKeeper.SendCoinsFromModuleToAccount(ctx, AsgardName, recipient, coins); err != nil {
		ctx.Logger().Error("fail to SendCoinsFromModuleToAccount", err)
	}
}

// For v108-requeue.yaml regression test
func migrateStoreV108(ctx cosmos.Context, mgr *Mgrs) {
	// Requeue four BCH.BCH txout (dangling actions) items swallowed in a chain halt.
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v108", "error", err)
		}
	}()

	danglingInboundTxIDs := []common.TxID{
		"5840920B63CDB9A02028ABB844B28F0305C2B37ADA4F936B69EBEFA04E2F826B",
		"BFACE691A12E85083DD2E4E4ADFBE702299DA6F08A98E6B6F7CF95A9D1D71632",
		"395EBDADA6D0975CF4D3F2E2BD7E246037C672C9CAB97DBFB744CC0F2FFABE95",
		"5881692D0522D0D5221A61FC0704B018ED51A6C43475063ADF6AC912D748208D",
	}

	requeueDanglingActionsV108(ctx, mgr, danglingInboundTxIDs)
}

// For v109-fake-observation-in.yaml
func migrateStoreV109(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v109", "error", err)
		}
	}()

	userAddr, err := common.NewAddress("tb1qehshltuxerv4zt4ruzxufd8m6r5xll7rdwa2rq")
	if err != nil {
		ctx.Logger().Error("fail to create user addr", "error", err)
	}
	asg, err := common.NewAddress("bcrt1qzf3gsk7edzwl9syyefvfhle37cjtql35tlzesk")
	if err != nil {
		ctx.Logger().Error("fail to create asg9lf addr", "error", err)
	}
	asgPubKey, err := common.NewPubKey("tthorpub1addwnpepqfshsq2y6ejy2ysxmq4gj8n8mzuzyulk9wh4n946jv5w2vpwdn2yuyp6sp4")
	if err != nil {
		ctx.Logger().Error("fail to create asg9lf_Pk", "error", err)
	}

	// include savers add memo
	memo := "+:BTC/BTC"
	blockHeight := ctx.BlockHeight()

	unobservedTxs := ObservedTxs{
		NewObservedTx(common.Tx{
			ID:          "1771d234f38e13fd9e4672fe469342fd598b6a2931f311d01b12dd4f35e9ce5c",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.1 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asgPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "5c4ad18723fe385946288574760b2d563f52a8917cdaf850d66958cd472db07a",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.1 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asgPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "96eca0eb4be36ac43fa2b2488fd3468aa2079ae02ae361ef5c08a4ace5070ed1",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asgPubKey, blockHeight),
	}

	err = makeFakeTxInObservation(ctx, mgr, unobservedTxs)
	if err != nil {
		ctx.Logger().Error("failed to migrate v109", "error", err)
	}
}

func migrateStoreV110(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV111(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV113(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV114(ctx cosmos.Context, mgr *Mgrs) {}
