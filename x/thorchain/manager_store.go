package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// StoreManager define the method as the entry point for store upgrade
type StoreManager interface {
	Iterator(_ cosmos.Context) error
}

// StoreMgr implement StoreManager interface
type StoreMgr struct {
	mgr *Mgrs
}

// newStoreMgr create a new instance of StoreMgr
func newStoreMgr(mgr *Mgrs) *StoreMgr {
	return &StoreMgr{
		mgr: mgr,
	}
}

// Iterator implement StoreManager interface decide whether it need to upgrade store
func (smgr *StoreMgr) Iterator(ctx cosmos.Context) error {
	version := smgr.mgr.GetVersion()

	if version.LT(semver.MustParse("1.90.0")) {
		version = smgr.mgr.Keeper().GetLowestActiveVersion(ctx) // TODO remove me on hard fork
	}

	if version.Major > constants.SWVersion.Major || version.Minor > constants.SWVersion.Minor {
		return fmt.Errorf("out of date software: have %s, network running %s", constants.SWVersion, version)
	}

	storeVer := smgr.mgr.Keeper().GetStoreVersion(ctx)
	if storeVer < 0 {
		return fmt.Errorf("unable to get store version: %d", storeVer)
	}
	if uint64(storeVer) < version.Minor {
		for i := uint64(storeVer + 1); i <= version.Minor; i++ {
			if err := smgr.migrate(ctx, i); err != nil {
				return err
			}
		}
	} else {
		ctx.Logger().Debug("No store migration needed")
	}
	return nil
}

func (smgr *StoreMgr) migrate(ctx cosmos.Context, i uint64) error {
	ctx.Logger().Info("Migrating store to new version", "version", i)
	// add the logic to migrate store here when it is needed

	switch i {
	case 84:
		migrateStoreV84(ctx, smgr.mgr)
	case 85:
		migrateStoreV85(ctx, smgr.mgr)
	case 86:
		migrateStoreV86(ctx, smgr.mgr)
	case 87:
		migrateStoreV87(ctx, smgr.mgr)
	case 88:
		migrateStoreV88(ctx, smgr.mgr)
	case 92:
		migrateStoreV92(ctx, smgr.mgr)
		migrateStoreV92_USDCBalance(ctx, smgr.mgr)
	case 94:
		migrateStoreV94(ctx, smgr.mgr)
	case 95:
		migrateStoreV95(ctx, smgr.mgr)
	case 102:
		migrateStoreV102(ctx, smgr.mgr)
	case 103:
		migrateStoreV103(ctx, smgr.mgr)
	case 106:
		migrateStoreV106(ctx, smgr.mgr)
	case 108:
		migrateStoreV108(ctx, smgr.mgr)
	case 109:
		migrateStoreV109(ctx, smgr.mgr)
	case 110:
		migrateStoreV110(ctx, smgr.mgr)
	case 111:
		migrateStoreV111(ctx, smgr.mgr)
	case 113:
		migrateStoreV113(ctx, smgr.mgr)
	case 114:
		migrateStoreV114(ctx, smgr.mgr)
	}

	smgr.mgr.Keeper().SetStoreVersion(ctx, int64(i))
	return nil
}

func migrateStoreV84(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v84", "error", err)
		}
	}()
	removeTransactions(ctx, mgr,
		"956AE0EDE6285E9125AE4AAC1ECB249FF327977DFE5792896FD866B1274F9BF8",
		"6D010D37AA436F48C06853F09E166DB74612DF02B532A775E813B6B20C1C3106")
}

func migrateStoreV85(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v84", "error", err)
		}
	}()
	removeTransactions(ctx, mgr,
		"DDE93247EAEF9B8DBC10605FA611AB2DC5E174C9099A319D6B0E6C7B2864CD5A")
}

func migrateStoreV87(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v87", "error", err)
		}
	}()
	if err := mgr.BeginBlock(ctx); err != nil {
		ctx.Logger().Error("fail to initialise manager", "error", err)
		return
	}
	opFee := cosmos.NewUint(uint64(fetchConfigInt64(ctx, mgr, constants.NodeOperatorFee)))

	bonded, err := mgr.Keeper().ListValidatorsWithBond(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get node accounts with bond", "error", err)
		return
	}
	for _, na := range bonded {
		bp, err := mgr.Keeper().GetBondProviders(ctx, na.NodeAddress)
		if err != nil {
			ctx.Logger().Error("fail to get bond provider", "error", err)
			return
		}
		bp.NodeOperatorFee = opFee

		if err := mgr.Keeper().SetBondProviders(ctx, bp); err != nil {
			ctx.Logger().Error("fail to save bond provider", "error", err)
			return
		}
	}
}

func migrateStoreV92(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v92", "error", err)
		}
	}()
	vaults, err := mgr.Keeper().GetAsgardVaults(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get asgard vaults", "error", err)
		return
	}
	ustAsset, err := common.NewAsset("TERRA.UST")
	if err != nil {
		ctx.Logger().Error("fail to parse UST asset", "error", err)
		return
	}
	for _, v := range vaults {
		if v.Status == InactiveVault || v.Status == InitVault {
			continue
		}
		coinsToSubtract := common.NewCoins(v.GetCoin(common.LUNAAsset), v.GetCoin(ustAsset))
		if coinsToSubtract.IsEmpty() {
			continue
		}
		v.SubFunds(coinsToSubtract)
		if err := mgr.Keeper().SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}
}

func migrateStoreV92_USDCBalance(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to correct USDC balance on v92", "error", err)
		}
	}()
	source := []struct {
		pubKey common.PubKey
		amount cosmos.Uint
	}{
		{
			pubKey: common.PubKey("thorpub1addwnpepqvxy3grgfsm6e9r7zdcfm5tuvmuefk6vcaen8x7mep4ywpa9jqeu6puyxnw"),
			amount: cosmos.NewUint(778569000000),
		},
		{
			pubKey: common.PubKey("thorpub1addwnpepqd6lgh7qjsfrkfxud68k7kc43f945x9s7wt2tzsttfde783u59mlyaql6cy"),
			amount: cosmos.NewUint(557933000000),
		},
		{
			pubKey: common.PubKey("thorpub1addwnpepq25tc6wckjrpnx2rc7l0lzz4vjsa8cseq8p39jyhm7jr9sk3hqjns80xmmm"),
			amount: cosmos.NewUint(1337164000000),
		},
	}
	usdcAsset, err := common.NewAsset("ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48")
	if err != nil {
		ctx.Logger().Error("fail to parse USDC asset", "error", err)
		return
	}
	pool, err := mgr.Keeper().GetPool(ctx, usdcAsset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return
	}
	for _, item := range source {
		v, err := mgr.Keeper().GetVault(ctx, item.pubKey)
		if err != nil {
			ctx.Logger().Error("fail to get vault", "error", err)
			continue
		}
		pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, item.amount)
		v.SubFunds(common.NewCoins(common.NewCoin(usdcAsset, item.amount)))
		if err := mgr.Keeper().SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
			continue
		}
	}
	if err := mgr.Keeper().SetPool(ctx, pool); err != nil {
		ctx.Logger().Error("fail to save pool balance", "error", err)
	}
}

func migrateStoreV94(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v94", "error", err)
		}
	}()
	iter := mgr.Keeper().GetVaultIterator(ctx)
	defer iter.Close()

	ustAsset, err := common.NewAsset("TERRA.UST")
	if err != nil {
		ctx.Logger().Error("fail to parse UST asset", "error", err)
		return
	}
	for ; iter.Valid(); iter.Next() {
		var v Vault
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &v); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
			continue
		}
		if v.Status == InactiveVault || v.Status == InitVault {
			continue
		}
		coinsToSubtract := common.NewCoins(v.GetCoin(common.LUNAAsset), v.GetCoin(ustAsset))
		if coinsToSubtract.IsEmpty() {
			continue
		}
		v.SubFunds(coinsToSubtract)
		if err := mgr.Keeper().SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}
	poolLuna, err := mgr.Keeper().GetPool(ctx, common.LUNAAsset)
	if err != nil {
		ctx.Logger().Error("fail to get LUNA pool", "error", err)
		return
	}
	if !poolLuna.BalanceRune.IsZero() {
		// move the remaining RUNE to reserve
		if err := mgr.Keeper().SendFromModuleToModule(ctx, AsgardName, ReserveName,
			common.NewCoins(common.NewCoin(common.RuneNative, poolLuna.BalanceRune))); err != nil {
			ctx.Logger().Error("fail to move remaining RUNE from asgard to reserve", "error", err)
			return
		}
	}
	// remove LUNA pool
	mgr.Keeper().RemovePool(ctx, common.LUNAAsset)
}

func migrateStoreV95(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v95", "error", err)
		}
	}()

	if err := mgr.Keeper().SetPOL(ctx, NewProtocolOwnedLiquidity()); err != nil {
		panic(err)
	}
}
