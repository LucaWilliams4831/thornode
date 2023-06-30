//go:build !testnet && !stagenet && !mocknet
// +build !testnet,!stagenet,!mocknet

package thorchain

import (
	ctypes "github.com/cosmos/cosmos-sdk/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func migrateStoreV86(ctx cosmos.Context, mgr *Mgrs) {}

func importPreRegistrationTHORNames(ctx cosmos.Context, mgr Manager) error {
	oneYear := mgr.Keeper().GetConfigInt64(ctx, constants.BlocksPerYear)
	names, err := getPreRegisterTHORNames(ctx, ctx.BlockHeight()+oneYear)
	if err != nil {
		return err
	}

	for _, name := range names {
		mgr.Keeper().SetTHORName(ctx, name)
	}
	return nil
}

func migrateStoreV88(ctx cosmos.Context, mgr Manager) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v88", "error", err)
		}
	}()

	err := importPreRegistrationTHORNames(ctx, mgr)
	if err != nil {
		ctx.Logger().Error("fail to migrate store to v88", "error", err)
	}
}

// no op
func migrateStoreV102(ctx cosmos.Context, mgr Manager) {}

func migrateStoreV103(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v102", "error", err)
		}
	}()

	// MAINNET REFUND
	// A user sent two 4,500 RUNE swap out txs (to USDT), but the external asset matching had a conflict and the outbounds were dropped. Txs:

	// https://viewblock.io/thorchain/tx/B07A6B1B40ADBA2E404D9BCE1BEF6EDE6F70AD135E83806E4F4B6863CF637D0B
	// https://viewblock.io/thorchain/tx/4795A3C036322493A9692B5D44E7D4FF29C3E2C1E848637184E98FE8B05FD06E

	// The below methodology was tested on Stagenet, results are documented here: https://gitlab.com/thorchain/thornode/-/merge_requests/2596#note_1216814315

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
		inboundHash: "B07A6B1B40ADBA2E404D9BCE1BEF6EDE6F70AD135E83806E4F4B6863CF637D0B",
		gasAsset:    common.ETHAsset,
	}

	err := refundDroppedSwapOutFromRUNE(ctx, mgr, firstSwapOut)
	if err != nil {
		ctx.Logger().Error("fail to migrate store to v103 refund failed", "error", err, "tx hash", "B07A6B1B40ADBA2E404D9BCE1BEF6EDE6F70AD135E83806E4F4B6863CF637D0B")
	}

	secondSwapOut := DroppedSwapOutTx{
		inboundHash: "4795A3C036322493A9692B5D44E7D4FF29C3E2C1E848637184E98FE8B05FD06E",
		gasAsset:    common.ETHAsset,
	}

	err = refundDroppedSwapOutFromRUNE(ctx, mgr, secondSwapOut)
	if err != nil {
		ctx.Logger().Error("fail to migrate store to v103 refund failed", "error", err, "tx hash", "4795A3C036322493A9692B5D44E7D4FF29C3E2C1E848637184E98FE8B05FD06E")
	}
}

func migrateStoreV106(ctx cosmos.Context, mgr *Mgrs) {
	// refund tx stuck in pending state: https://thorchain.net/tx/BC12B3B715546053A2D5615ADB4B3C2C648613D44AA9E942F2BDE40AB09EAA86
	// pool module still contains 4884 synth eth/thor: https://thornode.ninerealms.com/cosmos/bank/v1beta1/balances/thor1g98cy3n9mmjrpn0sxmn63lztelera37n8n67c0?height=9221024
	// deduct 4884 from pool module, create 4884 to user address: thor1vlzlsjfx2l3anh6wsh293fv2e8yh9rwpg7u723
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v106", "error", err)
		}
	}()

	recipient, err := cosmos.AccAddressFromBech32("thor1vlzlsjfx2l3anh6wsh293fv2e8yh9rwpg7u723")
	if err != nil {
		ctx.Logger().Error("fail to create acc address from bech32", err)
		return
	}

	coins := cosmos.NewCoins(cosmos.NewCoin(
		"eth/thor-0xa5f2211b9b8170f694421f2046281775e8468044",
		cosmos.NewInt(488432852150),
	))
	if err := mgr.coinKeeper.SendCoinsFromModuleToAccount(ctx, AsgardName, recipient, coins); err != nil {
		ctx.Logger().Error("fail to SendCoinsFromModuleToAccount", err)
	}
}

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

func migrateStoreV109(ctx cosmos.Context, mgr *Mgrs) {
	// Requeue ETH-chain dangling actions swallowed in a chain halt.
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v109", "error", err)
		}
	}()

	danglingInboundTxIDs := []common.TxID{
		"91C72EFCCF18AE043D036E2A207CC03A063E60024899E050AA7070EF15956BD7",
		"8D17D78A9E3168B88EFDBC30C5ADB3B09459C981B784D8F63C931988295DFE3B",
		"AD88EC612C188E62352F6157B26B97D76BD981744CE4C5AAC672F6338737F011",
		"88FD1BE075C55F18E73DD176E82A870F93B0E4692D514C36C8BF23692B139DED",
		"037254E2534D979FA196EC7B42C62A121B7A46D6854F9EC6FBE33C24B237EF0C",
	}

	requeueDanglingActionsV108(ctx, mgr, danglingInboundTxIDs)
	createFakeTxInsAndMakeObservations(ctx, mgr)
}

// TXs
// - 1771d234f38e13fd9e4672fe469342fd598b6a2931f311d01b12dd4f35e9ce5c - 0.1 BTC - asg-9lf
// - 5c4ad18723fe385946288574760b2d563f52a8917cdaf850d66958cd472db07a - 0.1 BTC - asg-9lf
// - 96eca0eb4be36ac43fa2b2488fd3468aa2079ae02ae361ef5c08a4ace5070ed1 - 0.2 BTC - asg-9lf
// - 5338aa51f6a7ce8e7f7bc4c98ac06b47c50a3cf335d61e69cf06c0e11b945ea5 - 0.2 BTC - asg-9lf
// - 63d92b111b9dc1b09e030d5a853120917e6205ed43d536a25a335ae96930469d - 0.2 BTC - asg-9lf
// - 6a747fdf782fa87693183b865b261f39b32790df4b6959482c4c8d16c54c1273 - 0.2 BTC - asg-9lf
// - 4209f36cb89ff216fcf6b02f6badf22d64f1596a876c9805a9d6978c4e09190a - 0.2 BTC - asg-9lf
// - f09faaec7d3f84e89ef184bcf568e44b39296b2ad55d464743dd2a656720e6c1 - 0.2 BTC - asg-qev
// - ec7e201eda9313a434313376881cb61676b8407960df2d7cc9d17e65cbc8ba82 - 0.2 BTC - asg-qev

// Asgards
// - 9lf: 1.2 BTC (bc1q8my83gyjy95dya9e0j8vzsjz5hz786zll9w9lf) pubkey (thorpub1addwnpepqdlyqz7renj8u8hqsvynxwgwnfufcwmh7ttsx5n259cva8nctwre5qx29zu)
// - qev 0.4 BTC (bc1qe65v2vmxnplwfg8z0fwsps79sly2wrfn5tlqev) pubkey (thorpub1addwnpepqw6ckwjel98vpsfyd2cq6cvwdeqh6jfcshnsgdlpzng6uhg3f69ssawhg99)
func createFakeTxInsAndMakeObservations(ctx cosmos.Context, mgr *Mgrs) {
	userAddr, err := common.NewAddress("bc1qqfmzftwe7xtfjq5ukwar59yk9ts40u42mnznwr")
	if err != nil {
		ctx.Logger().Error("fail to create addr", "addr", userAddr.String(), "error", err)
		return
	}
	asg9lf, err := common.NewAddress("bc1q8my83gyjy95dya9e0j8vzsjz5hz786zll9w9lf")
	if err != nil {
		ctx.Logger().Error("fail to create addr", "addr", asg9lf.String(), "error", err)
		return
	}
	asg9lfPubKey, err := common.NewPubKey("thorpub1addwnpepqdlyqz7renj8u8hqsvynxwgwnfufcwmh7ttsx5n259cva8nctwre5qx29zu")
	if err != nil {
		ctx.Logger().Error("fail to create pubkey for vault", "addr", asg9lf.String(), "error", err)
		return
	}
	asgQev, err := common.NewAddress("bc1qe65v2vmxnplwfg8z0fwsps79sly2wrfn5tlqev")
	if err != nil {
		ctx.Logger().Error("fail to create addr", "addr", asgQev.String(), "error", err)
		return
	}
	asgQevPubKey, err := common.NewPubKey("thorpub1addwnpepqw6ckwjel98vpsfyd2cq6cvwdeqh6jfcshnsgdlpzng6uhg3f69ssawhg99")
	if err != nil {
		ctx.Logger().Error("fail to create pubkey for vault", "addr", asg9lf.String(), "error", err)
		return
	}

	// include savers add memo
	memo := "+:BTC/BTC"
	blockHeight := ctx.BlockHeight()

	unobservedTxs := ObservedTxs{
		NewObservedTx(common.Tx{
			ID:          "1771d234f38e13fd9e4672fe469342fd598b6a2931f311d01b12dd4f35e9ce5c",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.1 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "5c4ad18723fe385946288574760b2d563f52a8917cdaf850d66958cd472db07a",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.1 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "96eca0eb4be36ac43fa2b2488fd3468aa2079ae02ae361ef5c08a4ace5070ed1",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "5338aa51f6a7ce8e7f7bc4c98ac06b47c50a3cf335d61e69cf06c0e11b945ea5",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "63d92b111b9dc1b09e030d5a853120917e6205ed43d536a25a335ae96930469d",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "6a747fdf782fa87693183b865b261f39b32790df4b6959482c4c8d16c54c1273",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "4209f36cb89ff216fcf6b02f6badf22d64f1596a876c9805a9d6978c4e09190a",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "f09faaec7d3f84e89ef184bcf568e44b39296b2ad55d464743dd2a656720e6c1",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asgQev,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asgQevPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "ec7e201eda9313a434313376881cb61676b8407960df2d7cc9d17e65cbc8ba82",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asgQev,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asgQevPubKey, blockHeight),
	}

	err = makeFakeTxInObservation(ctx, mgr, unobservedTxs)
	if err != nil {
		ctx.Logger().Error("failed to migrate v109", "error", err)
	}
}

func migrateStoreV110(ctx cosmos.Context, mgr *Mgrs) {
	resetObservationHeights(ctx, mgr, 110, common.BTCChain, 788640)
}

func migrateStoreV111(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v111", "error", err)
		}
	}()

	// these were the node addresses missed in the last migration
	bech32Addrs := []string{
		"thor10rgvc7c44mq5vpcq07dx5fg942eykagm9p6gxh",
		"thor12espg8k5fxqmclx9vyte7cducmmvrtxll40q7z",
		"thor169fahg7x70vkv909h06c2mspphrzqgy7g6prr4",
		"thor1gukvqaag4vk2l3uq3kjme5x9xy8556pgv5rw4k",
		"thor1h6h54d7jutljwt46qzt2w7nnyuswwv045kmshl",
		"thor1raylctzthcvjc0a5pv5ckzjr3rgxk5qcwu7af2",
		"thor1s76zxv0kpr78za293kvj0eep4tfqljacknsjzc",
		"thor1w8mntay3xuk3c77j8fgvyyt0nfvl2sk398a3ww",
	}

	for _, addr := range bech32Addrs {
		// convert to cosmos address
		na, err := ctypes.AccAddressFromBech32(addr)
		if err != nil {
			ctx.Logger().Error("failed to convert bech32 address", "address", addr, "error", err)
			continue
		}

		// set observation height back
		mgr.Keeper().ForceSetLastObserveHeight(ctx, common.BTCChain, na, 788640)
	}
}

func migrateStoreV113(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v113", "error", err)
		}
	}()

	// block: 11227005, tx: 5AC64AC48219456C8701E67CB4E6ACA13495F8A8042EBC0E5B4E9DA9CF963A9B

	poolSlashRune := cosmos.NewUint(8101892874988)
	poolSlashBTC := cosmos.NewUint(297035619)

	// send coins from pool to bond module
	if err := mgr.Keeper().SendFromModuleToModule(ctx, AsgardName, BondName, common.Coins{common.NewCoin(common.RuneNative, poolSlashRune)}); err != nil {
		ctx.Logger().Error("fail to transfer coin from reserve to bond module", "error", err)
		return
	}

	// send coins from reserve to bond module
	if err := mgr.Keeper().SendFromModuleToModule(ctx, ReserveName, BondName, common.Coins{common.NewCoin(common.RuneNative, poolSlashRune)}); err != nil {
		ctx.Logger().Error("fail to transfer coin from reserve to bond module", "error", err)
		return
	}

	// revert pool slash
	pool, err := mgr.Keeper().GetPool(ctx, common.BTCAsset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return
	}
	pool.BalanceAsset = pool.BalanceAsset.Add(poolSlashBTC)
	pool.BalanceRune = common.SafeSub(pool.BalanceRune, poolSlashRune)

	// store updated pool
	if err := mgr.Keeper().SetPool(ctx, pool); err != nil {
		ctx.Logger().Error("fail to set pool", "error", err)
		return
	}

	// emit inverted slash event for midgard
	poolSlashAmt := []PoolAmt{
		{
			Asset:  common.BTCAsset,
			Amount: int64(poolSlashBTC.Uint64()),
		},
		{
			Asset:  common.RuneAsset(),
			Amount: 0 - int64(poolSlashRune.Uint64()),
		},
	}
	eventSlash := NewEventSlash(common.BTCAsset, poolSlashAmt)
	if err := mgr.EventMgr().EmitEvent(ctx, eventSlash); err != nil {
		ctx.Logger().Error("fail to emit slash event", "error", err)
	}

	// credits from node slashes (sum to 2x the RUNE amount from pool slash)
	credits := []struct {
		address string
		amount  cosmos.Uint
	}{
		{address: "thor10rgvc7c44mq5vpcq07dx5fg942eykagm9p6gxh", amount: cosmos.NewUint(956154881499)},
		{address: "thor1pt8zkvkccj4397kemxeq8sjcyl7y6vacaedpvx", amount: cosmos.NewUint(761044063699)},
		{address: "thor1nlsfq25y74u8qt2hqmuzh5wd9t4uv28ghc258g", amount: cosmos.NewUint(973107929821)},
		{address: "thor1u5pfv07xtxz6aj59pnejaxh2dy7ew5s79ds8cw", amount: cosmos.NewUint(1063814699290)},
		{address: "thor1ypjwdplx07vf42qdfkex39dp8zxqnaects270v", amount: cosmos.NewUint(917937526969)},
		{address: "thor1vt207wgvefjgk88mtfjuurcl3vw6z4d2gu5psw", amount: cosmos.NewUint(1000265002165)},
		{address: "thor1vp29289yyvfar0ektscjk08r0tufvl24tn6xf9", amount: cosmos.NewUint(1021124834581)},
		{address: "thor1u9dnzza6hpesrwq4p8j2f29v6jsyeq4le66j3c", amount: cosmos.NewUint(978832200788)},
		{address: "thor1xk362wwunmr0gzew05j3euvdkjcvfmfyhmzd82", amount: cosmos.NewUint(1010886872701)},
		{address: "thor183fwfzgdfxzf5338acw32kplscgltf28j9s68j", amount: cosmos.NewUint(966449181925)},
		{address: "thor170xscqs5d469chdt83fxatjntc79zucrygsfxj", amount: cosmos.NewUint(1083603612921)},
		{address: "thor12espg8k5fxqmclx9vyte7cducmmvrtxll40q7z", amount: cosmos.NewUint(996350776100)},
		{address: "thor18nlluv0zw5g8930sx3r5xn7tqpsvwd7axxfynv", amount: cosmos.NewUint(1027540783824)},
		{address: "thor1faa0c6sqryr0am6ls9u8y6zs22ju2y7yw8ju9g", amount: cosmos.NewUint(603270429244)},
		{address: "thor1dqlmsm67h363nuxpd68esg54kt2t7xw2xewqml", amount: cosmos.NewUint(973292135373)},
		{address: "thor1gukvqaag4vk2l3uq3kjme5x9xy8556pgv5rw4k", amount: cosmos.NewUint(986737992322)},
		{address: "thor10f40m6nv7ulc0fvhmt07szn3n7ajd7e8xhghc3", amount: cosmos.NewUint(883372826754)},
	}

	for _, credit := range credits {
		ctx.Logger().Info("credit", "node", credit.address, "amount", credit.amount)

		// get addresses
		addr, err := cosmos.AccAddressFromBech32(credit.address)
		if err != nil {
			ctx.Logger().Error("fail to parse node address", "error", err)
			return
		}

		// get node account
		na, err := mgr.Keeper().GetNodeAccount(ctx, addr)
		if err != nil {
			ctx.Logger().Error("fail to get node account", "error", err)
			return
		}

		// update node bond
		na.Bond = na.Bond.Add(credit.amount)

		// store updated records
		if err := mgr.Keeper().SetNodeAccount(ctx, na); err != nil {
			ctx.Logger().Error("fail to save node account", "error", err)
			return
		}
	}
}

func migrateStoreV114(ctx cosmos.Context, mgr *Mgrs) {}
