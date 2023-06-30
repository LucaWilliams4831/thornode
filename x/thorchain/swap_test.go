package thorchain

import (
	"errors"

	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	thorchaintypes "gitlab.com/thorchain/thornode/x/thorchain/types"
)

type TestSwapKeeper struct {
	keeper.KVStoreDummy
}

func (k *TestSwapKeeper) PoolExist(ctx cosmos.Context, asset common.Asset) bool {
	return !asset.Equals(common.Asset{Chain: common.BNBChain, Symbol: "NOTEXIST", Ticker: "NOTEXIST"})
}

func (k *TestSwapKeeper) GetPool(ctx cosmos.Context, asset common.Asset) (thorchaintypes.Pool, error) {
	if asset.Equals(common.Asset{Chain: common.BNBChain, Symbol: "NOTEXIST", Ticker: "NOTEXIST"}) {
		return thorchaintypes.Pool{}, nil
	}
	if asset.Equals(common.BCHAsset) {
		return thorchaintypes.Pool{
			BalanceRune:  cosmos.NewUint(100).MulUint64(common.One),
			BalanceAsset: cosmos.NewUint(100).MulUint64(common.One),
			LPUnits:      cosmos.NewUint(100).MulUint64(common.One),
			SynthUnits:   cosmos.ZeroUint(),
			Status:       PoolStaged,
			Asset:        asset,
		}, nil
	}
	return thorchaintypes.Pool{
		BalanceRune:  cosmos.NewUint(100).MulUint64(common.One),
		BalanceAsset: cosmos.NewUint(100).MulUint64(common.One),
		LPUnits:      cosmos.NewUint(100).MulUint64(common.One),
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
		Asset:        asset,
	}, nil
}

func (k *TestSwapKeeper) SetPool(ctx cosmos.Context, ps thorchaintypes.Pool) error { return nil }

func (k *TestSwapKeeper) GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (thorchaintypes.LiquidityProvider, error) {
	if asset.Equals(common.Asset{Chain: common.BNBChain, Symbol: "NOTEXISTSTICKER", Ticker: "NOTEXISTSTICKER"}) {
		return thorchaintypes.LiquidityProvider{}, errors.New("you asked for it")
	}
	return LiquidityProvider{
		Asset:        asset,
		RuneAddress:  addr,
		AssetAddress: addr,
		Units:        cosmos.NewUint(100),
		PendingRune:  cosmos.ZeroUint(),
	}, nil
}

func (k *TestSwapKeeper) SetLiquidityProvider(ctx cosmos.Context, ps thorchaintypes.LiquidityProvider) {
}

func (k *TestSwapKeeper) AddToLiquidityFees(ctx cosmos.Context, asset common.Asset, fs cosmos.Uint) error {
	return nil
}

func (k *TestSwapKeeper) AddToSwapSlip(ctx cosmos.Context, asset common.Asset, fs cosmos.Int) error {
	return nil
}

func (k *TestSwapKeeper) GetLowestActiveVersion(ctx cosmos.Context) semver.Version {
	return GetCurrentVersion()
}

func (k *TestSwapKeeper) AddPoolFeeToReserve(ctx cosmos.Context, fee cosmos.Uint) error { return nil }

func (k *TestSwapKeeper) GetGas(ctx cosmos.Context, _ common.Asset) ([]cosmos.Uint, error) {
	return []cosmos.Uint{cosmos.NewUint(37500), cosmos.NewUint(30000)}, nil
}

func (k *TestSwapKeeper) GetAsgardVaultsByStatus(ctx cosmos.Context, status VaultStatus) (Vaults, error) {
	vault := GetRandomVault()
	vault.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(10000*common.One)),
	}
	return Vaults{
		vault,
	}, nil
}

func (k *TestSwapKeeper) GetObservedTxInVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error) {
	return ObservedTxVoter{
		TxID: hash,
	}, nil
}

func (k *TestSwapKeeper) ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{}, nil
}

func (k *TestSwapKeeper) GetBlockOut(ctx cosmos.Context) (*TxOut, error) {
	return NewTxOut(ctx.BlockHeight()), nil
}

func (k *TestSwapKeeper) GetTxOut(ctx cosmos.Context, _ int64) (*TxOut, error) {
	return NewTxOut(ctx.BlockHeight()), nil
}

func (k *TestSwapKeeper) GetLeastSecure(ctx cosmos.Context, vaults Vaults, _ int64) Vault {
	return vaults[0]
}

func (k TestSwapKeeper) SortBySecurity(_ cosmos.Context, vaults Vaults, _ int64) Vaults {
	return vaults
}
func (k *TestSwapKeeper) AppendTxOut(_ cosmos.Context, _ int64, _ TxOutItem) error { return nil }
func (k *TestSwapKeeper) GetNetworkFee(ctx cosmos.Context, chain common.Chain) (NetworkFee, error) {
	if chain.Equals(common.BNBChain) {
		return NetworkFee{
			Chain:              common.BNBChain,
			TransactionSize:    1,
			TransactionFeeRate: 37500,
		}, nil
	}
	if chain.Equals(common.THORChain) {
		return NetworkFee{
			Chain:              common.THORChain,
			TransactionSize:    1,
			TransactionFeeRate: 1_00000000,
		}, nil
	}
	return NetworkFee{}, errKaboom
}

func (k *TestSwapKeeper) SendFromModuleToModule(ctx cosmos.Context, from, to string, coin common.Coins) error {
	return nil
}

func (k *TestSwapKeeper) BurnFromModule(ctx cosmos.Context, module string, coin common.Coin) error {
	return nil
}

func (k *TestSwapKeeper) MintToModule(ctx cosmos.Context, module string, coin common.Coin) error {
	return nil
}
