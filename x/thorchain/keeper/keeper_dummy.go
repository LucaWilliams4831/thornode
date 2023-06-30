package keeper

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/simapp"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	crisis "github.com/cosmos/cosmos-sdk/x/crisis/types"
	"github.com/tendermint/tendermint/libs/log"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

var kaboom = errors.New("Kaboom!!!")

type KVStoreDummy struct{}

func (k KVStoreDummy) Cdc() codec.BinaryCodec                  { return simapp.MakeTestEncodingConfig().Marshaler }
func (k KVStoreDummy) CoinKeeper() bankkeeper.Keeper           { return bankkeeper.BaseKeeper{} }
func (k KVStoreDummy) AccountKeeper() authkeeper.AccountKeeper { return authkeeper.AccountKeeper{} }
func (k KVStoreDummy) Logger(ctx cosmos.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", ModuleName))
}

func (k KVStoreDummy) GetVersion() semver.Version { return semver.MustParse("9999999.0.0") }
func (k KVStoreDummy) GetVersionWithCtx(ctx cosmos.Context) (semver.Version, bool) {
	return semver.MustParse("9999999.0.0"), true
}
func (k KVStoreDummy) SetVersionWithCtx(ctx cosmos.Context, v semver.Version) {}
func (k KVStoreDummy) GetMinJoinLast(ctx cosmos.Context) (semver.Version, int64) {
	return k.GetMinJoinVersion(ctx), 0
}
func (k KVStoreDummy) SetMinJoinLast(ctx cosmos.Context) {}

func (k KVStoreDummy) GetKey(_ cosmos.Context, prefix kvTypes.DbPrefix, key string) string {
	return fmt.Sprintf("%s/1/%s", prefix, key)
}

func (k KVStoreDummy) GetStoreVersion(ctx cosmos.Context) int64      { return 1 }
func (k KVStoreDummy) SetStoreVersion(ctx cosmos.Context, ver int64) {}

func (k KVStoreDummy) GetRuneBalanceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint {
	return cosmos.ZeroUint()
}

func (k KVStoreDummy) GetBalanceOfModule(ctx cosmos.Context, moduleName, denom string) cosmos.Uint {
	return cosmos.ZeroUint()
}

func (k KVStoreDummy) SendFromModuleToModule(ctx cosmos.Context, from, to string, coins common.Coins) error {
	return kaboom
}

func (k KVStoreDummy) SendCoins(ctx cosmos.Context, from, to cosmos.AccAddress, coins cosmos.Coins) error {
	return kaboom
}

func (k KVStoreDummy) AddCoins(ctx cosmos.Context, _ cosmos.AccAddress, coins cosmos.Coins) error {
	return kaboom
}

func (k KVStoreDummy) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coins common.Coins) error {
	return kaboom
}

func (k KVStoreDummy) SendFromModuleToAccount(ctx cosmos.Context, from string, to cosmos.AccAddress, coins common.Coins) error {
	return kaboom
}

func (k KVStoreDummy) MintToModule(ctx cosmos.Context, module string, coin common.Coin) error {
	return kaboom
}

func (k KVStoreDummy) BurnFromModule(ctx cosmos.Context, module string, coin common.Coin) error {
	return kaboom
}

func (k KVStoreDummy) MintAndSendToAccount(ctx cosmos.Context, to cosmos.AccAddress, coin common.Coin) error {
	return kaboom
}

func (k KVStoreDummy) GetModuleAddress(module string) (common.Address, error) {
	return "", kaboom
}

func (k KVStoreDummy) GetModuleAccAddress(module string) cosmos.AccAddress {
	return nil
}

func (k KVStoreDummy) GetAccount(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Account {
	return nil
}

func (k KVStoreDummy) GetBalance(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Coins {
	return nil
}

func (k KVStoreDummy) HasCoins(ctx cosmos.Context, addr cosmos.AccAddress, coins cosmos.Coins) bool {
	return false
}

func (k KVStoreDummy) SetLastSignedHeight(_ cosmos.Context, _ int64) error { return kaboom }
func (k KVStoreDummy) GetLastSignedHeight(_ cosmos.Context) (int64, error) {
	return 0, kaboom
}

func (k KVStoreDummy) SetLastChainHeight(_ cosmos.Context, _ common.Chain, _ int64) error {
	return kaboom
}

func (k KVStoreDummy) ForceSetLastChainHeight(_ cosmos.Context, _ common.Chain, _ int64) {}

func (k KVStoreDummy) GetLastChainHeight(_ cosmos.Context, _ common.Chain) (int64, error) {
	return 0, kaboom
}

func (k KVStoreDummy) GetLastChainHeights(ctx cosmos.Context) (map[common.Chain]int64, error) {
	return nil, kaboom
}

func (k KVStoreDummy) GetRagnarokBlockHeight(_ cosmos.Context) (int64, error) {
	return 0, kaboom
}
func (k KVStoreDummy) SetRagnarokBlockHeight(_ cosmos.Context, _ int64) {}
func (k KVStoreDummy) GetRagnarokNth(_ cosmos.Context) (int64, error) {
	return 0, kaboom
}
func (k KVStoreDummy) SetRagnarokNth(_ cosmos.Context, _ int64) {}
func (k KVStoreDummy) GetRagnarokPending(_ cosmos.Context) (int64, error) {
	return 0, kaboom
}
func (k KVStoreDummy) SetRagnarokPending(_ cosmos.Context, _ int64) {}
func (k KVStoreDummy) RagnarokInProgress(_ cosmos.Context) bool     { return false }
func (k KVStoreDummy) GetRagnarokWithdrawPosition(ctx cosmos.Context) (RagnarokWithdrawPosition, error) {
	return RagnarokWithdrawPosition{}, kaboom
}
func (k KVStoreDummy) SetRagnarokWithdrawPosition(_tx cosmos.Context, _ RagnarokWithdrawPosition) {}

// SetPoolRagnarokStart set pool ragnarok start block height
func (k KVStoreDummy) SetPoolRagnarokStart(ctx cosmos.Context, asset common.Asset) {}

// GetPoolRagnarokStart get pool ragnarok start block height
func (k KVStoreDummy) GetPoolRagnarokStart(ctx cosmos.Context, asset common.Asset) (int64, error) {
	return 0, kaboom
}

func (k KVStoreDummy) GetPoolBalances(_ cosmos.Context, _, _ common.Asset) (cosmos.Uint, cosmos.Uint) {
	return cosmos.ZeroUint(), cosmos.ZeroUint()
}

func (k KVStoreDummy) GetPoolIterator(_ cosmos.Context) cosmos.Iterator {
	return NewDummyIterator()
}
func (k KVStoreDummy) SetPoolData(_ cosmos.Context, _ common.Asset, _ PoolStatus) {}
func (k KVStoreDummy) GetPoolDataIterator(_ cosmos.Context) cosmos.Iterator {
	return NewDummyIterator()
}
func (k KVStoreDummy) EnableAPool(_ cosmos.Context) {}

func (k KVStoreDummy) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	return Pool{}, kaboom
}
func (k KVStoreDummy) GetPools(_ cosmos.Context) (Pools, error)        { return nil, kaboom }
func (k KVStoreDummy) SetPool(_ cosmos.Context, _ Pool) error          { return kaboom }
func (k KVStoreDummy) PoolExist(_ cosmos.Context, _ common.Asset) bool { return false }
func (k KVStoreDummy) RemovePool(_ cosmos.Context, _ common.Asset)     {}

func (k KVStoreDummy) SetPoolLUVI(ctx cosmos.Context, asset common.Asset, luvi cosmos.Uint) {}
func (k KVStoreDummy) GetPoolLUVI(ctx cosmos.Context, asset common.Asset) (cosmos.Uint, error) {
	return cosmos.ZeroUint(), kaboom
}

func (k KVStoreDummy) GetLoanIterator(ctx cosmos.Context, _ common.Asset) cosmos.Iterator {
	return nil
}

func (k KVStoreDummy) GetLoan(ctx cosmos.Context, asset common.Asset, addr common.Address) (Loan, error) {
	return Loan{}, kaboom
}
func (k KVStoreDummy) SetLoan(ctx cosmos.Context, _ Loan)                                 {}
func (k KVStoreDummy) RemoveLoan(ctx cosmos.Context, _ Loan)                              {}
func (k KVStoreDummy) SetTotalCollateral(_ cosmos.Context, _ common.Asset, _ cosmos.Uint) {}
func (k KVStoreDummy) GetTotalCollateral(_ cosmos.Context, _ common.Asset) (cosmos.Uint, error) {
	return cosmos.ZeroUint(), kaboom
}

func (k KVStoreDummy) GetLiquidityProviderIterator(_ cosmos.Context, _ common.Asset) cosmos.Iterator {
	return nil
}

func (k KVStoreDummy) GetLiquidityProvider(_ cosmos.Context, _ common.Asset, _ common.Address) (LiquidityProvider, error) {
	return LiquidityProvider{}, kaboom
}
func (k KVStoreDummy) SetLiquidityProvider(_ cosmos.Context, _ LiquidityProvider)    {}
func (k KVStoreDummy) RemoveLiquidityProvider(_ cosmos.Context, _ LiquidityProvider) {}
func (k KVStoreDummy) GetTotalSupply(ctx cosmos.Context, asset common.Asset) cosmos.Uint {
	return cosmos.ZeroUint()
}

func (k KVStoreDummy) TotalActiveValidators(_ cosmos.Context) (int, error) { return 0, kaboom }
func (k KVStoreDummy) ListValidatorsWithBond(_ cosmos.Context) (NodeAccounts, error) {
	return nil, kaboom
}

func (k KVStoreDummy) ListValidatorsByStatus(_ cosmos.Context, _ NodeStatus) (NodeAccounts, error) {
	return nil, kaboom
}

func (k KVStoreDummy) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	return nil, kaboom
}

func (k KVStoreDummy) GetLowestActiveVersion(_ cosmos.Context) semver.Version {
	return semver.Version{
		Major: 0,
		Minor: 1,
		Patch: 0,
	}
}
func (k KVStoreDummy) GetMinJoinVersion(_ cosmos.Context) semver.Version   { return semver.Version{} }
func (k KVStoreDummy) GetMinJoinVersionV1(_ cosmos.Context) semver.Version { return semver.Version{} }
func (k KVStoreDummy) GetNodeAccount(_ cosmos.Context, _ cosmos.AccAddress) (NodeAccount, error) {
	return NodeAccount{}, kaboom
}

func (k KVStoreDummy) GetNodeAccountByPubKey(_ cosmos.Context, _ common.PubKey) (NodeAccount, error) {
	return NodeAccount{}, kaboom
}

func (k KVStoreDummy) SetNodeAccount(_ cosmos.Context, _ NodeAccount) error { return kaboom }
func (k KVStoreDummy) EnsureNodeKeysUnique(_ cosmos.Context, _ string, _ common.PubKeySet) error {
	return kaboom
}
func (k KVStoreDummy) GetNodeAccountIterator(_ cosmos.Context) cosmos.Iterator { return nil }

func (k KVStoreDummy) GetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress) (int64, error) {
	return 0, kaboom
}
func (k KVStoreDummy) SetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64) {}
func (k KVStoreDummy) ResetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress)        {}
func (k KVStoreDummy) IncNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64) error {
	return kaboom
}

func (k KVStoreDummy) DecNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64) error {
	return kaboom
}

func (k KVStoreDummy) GetNodeAccountJail(ctx cosmos.Context, addr cosmos.AccAddress) (Jail, error) {
	return Jail{}, kaboom
}

func (k KVStoreDummy) SetNodeAccountJail(ctx cosmos.Context, addr cosmos.AccAddress, height int64, reason string) error {
	return kaboom
}

func (k KVStoreDummy) ReleaseNodeAccountFromJail(ctx cosmos.Context, addr cosmos.AccAddress) error {
	return kaboom
}
func (k KVStoreDummy) SetBondProviders(ctx cosmos.Context, _ BondProviders) error { return kaboom }
func (k KVStoreDummy) GetBondProviders(ctx cosmos.Context, _ cosmos.AccAddress) (BondProviders, error) {
	return BondProviders{}, kaboom
}

func (k KVStoreDummy) GetObservingAddresses(_ cosmos.Context) ([]cosmos.AccAddress, error) {
	return nil, kaboom
}

func (k KVStoreDummy) AddObservingAddresses(_ cosmos.Context, _ []cosmos.AccAddress) error {
	return kaboom
}
func (k KVStoreDummy) ClearObservingAddresses(_ cosmos.Context)                      {}
func (k KVStoreDummy) SetObservedTxInVoter(_ cosmos.Context, _ ObservedTxVoter)      {}
func (k KVStoreDummy) GetObservedTxInVoterIterator(_ cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) GetObservedTxInVoter(_ cosmos.Context, _ common.TxID) (ObservedTxVoter, error) {
	return ObservedTxVoter{}, kaboom
}
func (k KVStoreDummy) SetObservedTxOutVoter(_ cosmos.Context, _ ObservedTxVoter)      {}
func (k KVStoreDummy) GetObservedTxOutVoterIterator(_ cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) GetObservedTxOutVoter(_ cosmos.Context, _ common.TxID) (ObservedTxVoter, error) {
	return ObservedTxVoter{}, kaboom
}
func (k KVStoreDummy) SetObservedLink(ctx cosmos.Context, _, _ common.TxID) {}
func (k KVStoreDummy) GetObservedLink(ctx cosmos.Context, inhash common.TxID) []common.TxID {
	return nil
}
func (k KVStoreDummy) SetTssVoter(_ cosmos.Context, _ TssVoter)             {}
func (k KVStoreDummy) GetTssVoterIterator(_ cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) GetTssVoter(_ cosmos.Context, _ string) (TssVoter, error) {
	return TssVoter{}, kaboom
}

func (k KVStoreDummy) GetKeygenBlock(_ cosmos.Context, _ int64) (KeygenBlock, error) {
	return KeygenBlock{}, kaboom
}
func (k KVStoreDummy) SetKeygenBlock(_ cosmos.Context, _ KeygenBlock)          {}
func (k KVStoreDummy) GetKeygenBlockIterator(_ cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) GetTxOut(_ cosmos.Context, _ int64) (*TxOut, error)      { return nil, kaboom }
func (k KVStoreDummy) GetTxOutValue(_ cosmos.Context, _ int64) (cosmos.Uint, error) {
	return cosmos.ZeroUint(), kaboom
}
func (k KVStoreDummy) SetTxOut(_ cosmos.Context, _ *TxOut) error                { return kaboom }
func (k KVStoreDummy) AppendTxOut(_ cosmos.Context, _ int64, _ TxOutItem) error { return kaboom }
func (k KVStoreDummy) ClearTxOut(_ cosmos.Context, _ int64) error               { return kaboom }
func (k KVStoreDummy) GetTxOutIterator(_ cosmos.Context) cosmos.Iterator        { return nil }
func (k KVStoreDummy) AddToLiquidityFees(_ cosmos.Context, _ common.Asset, _ cosmos.Uint) error {
	return kaboom
}

func (k KVStoreDummy) GetTotalLiquidityFees(_ cosmos.Context, _ uint64) (cosmos.Uint, error) {
	return cosmos.ZeroUint(), kaboom
}

func (k KVStoreDummy) GetPoolLiquidityFees(_ cosmos.Context, _ uint64, _ common.Asset) (cosmos.Uint, error) {
	return cosmos.ZeroUint(), kaboom
}

func (k KVStoreDummy) GetRollingPoolLiquidityFee(ctx cosmos.Context, asset common.Asset) (uint64, error) {
	return 0, kaboom
}

func (k KVStoreDummy) ResetRollingPoolLiquidityFee(ctx cosmos.Context, asset common.Asset) {}

func (k KVStoreDummy) GetChains(_ cosmos.Context) (common.Chains, error) { return nil, kaboom }
func (k KVStoreDummy) SetChains(_ cosmos.Context, _ common.Chains)       {}
func (k KVStoreDummy) AddToSwapSlip(ctx cosmos.Context, asset common.Asset, amt cosmos.Int) error {
	return kaboom
}

func (k KVStoreDummy) RollupSwapSlip(ctx cosmos.Context, blockCount int64, _ common.Asset) (cosmos.Int, error) {
	return cosmos.ZeroInt(), kaboom
}

func (k KVStoreDummy) GetPoolSwapSlip(ctx cosmos.Context, height int64, asset common.Asset) (cosmos.Int, error) {
	return cosmos.ZeroInt(), kaboom
}
func (k KVStoreDummy) DeletePoolSwapSlip(ctx cosmos.Context, height int64, asset common.Asset) {}

func (k KVStoreDummy) GetVaultIterator(_ cosmos.Context) cosmos.Iterator  { return nil }
func (k KVStoreDummy) VaultExists(_ cosmos.Context, _ common.PubKey) bool { return false }
func (k KVStoreDummy) FindPubKeyOfAddress(_ cosmos.Context, _ common.Address, _ common.Chain) (common.PubKey, error) {
	return common.EmptyPubKey, kaboom
}
func (k KVStoreDummy) SetVault(_ cosmos.Context, _ Vault) error { return kaboom }
func (k KVStoreDummy) GetVault(_ cosmos.Context, _ common.PubKey) (Vault, error) {
	return Vault{}, kaboom
}
func (k KVStoreDummy) GetAsgardVaults(_ cosmos.Context) (Vaults, error) { return nil, kaboom }
func (k KVStoreDummy) GetAsgardVaultsByStatus(_ cosmos.Context, _ VaultStatus) (Vaults, error) {
	return nil, kaboom
}

func (k KVStoreDummy) RemoveFromAsgardIndex(ctx cosmos.Context, pubkey common.PubKey) error {
	return kaboom
}

func (k KVStoreDummy) GetLeastSecure(_ cosmos.Context, _ Vaults, _ int64) Vault  { return Vault{} }
func (k KVStoreDummy) GetMostSecure(_ cosmos.Context, _ Vaults, _ int64) Vault   { return Vault{} }
func (k KVStoreDummy) SortBySecurity(_ cosmos.Context, _ Vaults, _ int64) Vaults { return nil }
func (k KVStoreDummy) DeleteVault(_ cosmos.Context, _ common.PubKey) error       { return kaboom }

func (k KVStoreDummy) HasValidVaultPools(_ cosmos.Context) (bool, error)         { return false, kaboom }
func (k KVStoreDummy) AddPoolFeeToReserve(_ cosmos.Context, _ cosmos.Uint) error { return kaboom }
func (k KVStoreDummy) AddBondFeeToReserve(_ cosmos.Context, _ cosmos.Uint) error { return kaboom }
func (k KVStoreDummy) GetNetwork(_ cosmos.Context) (Network, error)              { return Network{}, kaboom }
func (k KVStoreDummy) SetNetwork(_ cosmos.Context, _ Network) error              { return kaboom }

func (k KVStoreDummy) GetPOL(_ cosmos.Context) (ProtocolOwnedLiquidity, error) {
	return ProtocolOwnedLiquidity{}, kaboom
}

func (k KVStoreDummy) SetPOL(_ cosmos.Context, _ ProtocolOwnedLiquidity) error {
	return kaboom
}

func (k KVStoreDummy) SetTssKeysignFailVoter(_ cosmos.Context, tss TssKeysignFailVoter) {
}

func (k KVStoreDummy) GetTssKeysignFailVoterIterator(_ cosmos.Context) cosmos.Iterator {
	return nil
}

func (k KVStoreDummy) GetTssKeysignFailVoter(_ cosmos.Context, _ string) (TssKeysignFailVoter, error) {
	return TssKeysignFailVoter{}, kaboom
}

func (k KVStoreDummy) GetGas(_ cosmos.Context, _ common.Asset) ([]cosmos.Uint, error) {
	return nil, kaboom
}
func (k KVStoreDummy) SetGas(_ cosmos.Context, _ common.Asset, _ []cosmos.Uint) {}
func (k KVStoreDummy) GetGasIterator(ctx cosmos.Context) cosmos.Iterator        { return nil }

func (k KVStoreDummy) SetErrataTxVoter(_ cosmos.Context, _ ErrataTxVoter)        {}
func (k KVStoreDummy) GetErrataTxVoterIterator(_ cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) GetErrataTxVoter(_ cosmos.Context, _ common.TxID, _ common.Chain) (ErrataTxVoter, error) {
	return ErrataTxVoter{}, kaboom
}
func (k KVStoreDummy) SetBanVoter(_ cosmos.Context, _ BanVoter) {}
func (k KVStoreDummy) GetBanVoter(_ cosmos.Context, _ cosmos.AccAddress) (BanVoter, error) {
	return BanVoter{}, kaboom
}

func (k KVStoreDummy) GetBanVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return nil
}
func (k KVStoreDummy) SetSwapQueueItem(ctx cosmos.Context, msg MsgSwap, i int) error { return kaboom }
func (k KVStoreDummy) GetSwapQueueIterator(ctx cosmos.Context) cosmos.Iterator       { return nil }
func (k KVStoreDummy) RemoveSwapQueueItem(ctx cosmos.Context, _ common.TxID, _ int)  {}
func (k KVStoreDummy) GetSwapQueueItem(ctx cosmos.Context, txID common.TxID, _ int) (MsgSwap, error) {
	return MsgSwap{}, kaboom
}

func (k KVStoreDummy) HasSwapQueueItem(ctx cosmos.Context, txID common.TxID, _ int) bool {
	return false
}

func (k KVStoreDummy) OrderBooksEnabled(ctx cosmos.Context) bool {
	return false
}

func (k KVStoreDummy) SetOrderBookItem(ctx cosmos.Context, msg MsgSwap) error      { return kaboom }
func (k KVStoreDummy) GetOrderBookItemIterator(ctx cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) RemoveOrderBookItem(ctx cosmos.Context, _ common.TxID) error {
	return kaboom
}

func (k KVStoreDummy) GetOrderBookItem(ctx cosmos.Context, txID common.TxID) (MsgSwap, error) {
	return MsgSwap{}, kaboom
}

func (k KVStoreDummy) HasOrderBookItem(ctx cosmos.Context, txID common.TxID) bool { return false }
func (k KVStoreDummy) GetOrderBookIndexIterator(_ cosmos.Context, _ types.OrderType, _, _ common.Asset) cosmos.Iterator {
	return nil
}

func (k KVStoreDummy) SetOrderBookIndex(_ cosmos.Context, _ MsgSwap) error {
	return kaboom
}

func (k KVStoreDummy) GetOrderBookIndex(_ cosmos.Context, _ MsgSwap) (common.TxIDs, error) {
	return nil, kaboom
}

func (k KVStoreDummy) HasOrderBookIndex(_ cosmos.Context, _ MsgSwap) (bool, error) {
	return false, kaboom
}

func (k KVStoreDummy) RemoveOrderBookIndex(_ cosmos.Context, _ MsgSwap) error {
	return kaboom
}

func (k KVStoreDummy) SetOrderBookProcessor(ctx cosmos.Context, record []bool) error {
	return kaboom
}

// GetOrderBookProcessor - get a list of asset pairs to process
func (k KVStoreDummy) GetOrderBookProcessor(ctx cosmos.Context) ([]bool, error) {
	return nil, kaboom
}

func (k KVStoreDummy) GetMimir(_ cosmos.Context, key string) (int64, error) { return 0, kaboom }
func (k KVStoreDummy) SetMimir(_ cosmos.Context, key string, value int64)   {}
func (k KVStoreDummy) GetNodeMimirs(ctx cosmos.Context, key string) (NodeMimirs, error) {
	return NodeMimirs{}, kaboom
}

func (k KVStoreDummy) SetNodeMimir(_ cosmos.Context, key string, value int64, acc cosmos.AccAddress) error {
	return kaboom
}
func (k KVStoreDummy) DeleteMimir(_ cosmos.Context, key string) error          { return kaboom }
func (k KVStoreDummy) GetMimirIterator(ctx cosmos.Context) cosmos.Iterator     { return nil }
func (k KVStoreDummy) GetNodeMimirIterator(ctx cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) GetNodePauseChain(ctx cosmos.Context, acc cosmos.AccAddress) int64 {
	return int64(-1)
}
func (k KVStoreDummy) SetNodePauseChain(ctx cosmos.Context, acc cosmos.AccAddress) {}

func (k KVStoreDummy) GetNetworkFee(ctx cosmos.Context, chain common.Chain) (NetworkFee, error) {
	return NetworkFee{}, kaboom
}

func (k KVStoreDummy) SaveNetworkFee(ctx cosmos.Context, chain common.Chain, networkFee NetworkFee) error {
	return kaboom
}

func (k KVStoreDummy) GetNetworkFeeIterator(ctx cosmos.Context) cosmos.Iterator {
	return nil
}

func (k KVStoreDummy) SetObservedNetworkFeeVoter(ctx cosmos.Context, networkFeeVoter ObservedNetworkFeeVoter) {
}

func (k KVStoreDummy) GetObservedNetworkFeeVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return nil
}

func (k KVStoreDummy) GetObservedNetworkFeeVoter(ctx cosmos.Context, height int64, chain common.Chain, rate int64) (ObservedNetworkFeeVoter, error) {
	return ObservedNetworkFeeVoter{}, nil
}

func (k KVStoreDummy) SetLastObserveHeight(ctx cosmos.Context, chain common.Chain, address cosmos.AccAddress, height int64) error {
	return kaboom
}

func (k KVStoreDummy) ForceSetLastObserveHeight(ctx cosmos.Context, chain common.Chain, address cosmos.AccAddress, height int64) {
}

func (k KVStoreDummy) GetLastObserveHeight(ctx cosmos.Context, address cosmos.AccAddress) (map[common.Chain]int64, error) {
	return nil, kaboom
}

func (k KVStoreDummy) SetTssKeygenMetric(_ cosmos.Context, metric *TssKeygenMetric) {
}

func (k KVStoreDummy) GetTssKeygenMetric(_ cosmos.Context, key common.PubKey) (*TssKeygenMetric, error) {
	return nil, kaboom
}

func (k KVStoreDummy) SetTssKeysignMetric(_ cosmos.Context, metric *TssKeysignMetric) {
}

func (k KVStoreDummy) GetTssKeysignMetric(_ cosmos.Context, txID common.TxID) (*TssKeysignMetric, error) {
	return nil, kaboom
}

func (k KVStoreDummy) GetLatestTssKeysignMetric(_ cosmos.Context) (*TssKeysignMetric, error) {
	return nil, kaboom
}
func (k KVStoreDummy) SetChainContract(ctx cosmos.Context, cc ChainContract) {}
func (k KVStoreDummy) GetChainContract(ctx cosmos.Context, chain common.Chain) (ChainContract, error) {
	return ChainContract{}, kaboom
}

func (k KVStoreDummy) GetChainContractIterator(ctx cosmos.Context) cosmos.Iterator {
	return nil
}

func (k KVStoreDummy) GetChainContracts(ctx cosmos.Context, chains common.Chains) []ChainContract {
	return nil
}
func (k KVStoreDummy) SetSolvencyVoter(_ cosmos.Context, _ SolvencyVoter) {}
func (k KVStoreDummy) GetSolvencyVoter(_ cosmos.Context, _ common.TxID, _ common.Chain) (SolvencyVoter, error) {
	return SolvencyVoter{}, kaboom
}

func (k KVStoreDummy) THORNameExists(ctx cosmos.Context, _ string) bool { return false }
func (k KVStoreDummy) GetTHORName(ctx cosmos.Context, _ string) (THORName, error) {
	return THORName{}, kaboom
}
func (k KVStoreDummy) SetTHORName(ctx cosmos.Context, name THORName)          {}
func (k KVStoreDummy) GetTHORNameIterator(ctx cosmos.Context) cosmos.Iterator { return nil }
func (k KVStoreDummy) DeleteTHORName(ctx cosmos.Context, _ string) error      { return kaboom }

func (k KVStoreDummy) InvariantRoutes() []crisis.InvarRoute {
	return nil
}

func (k KVStoreDummy) GetConstants() constants.ConstantValues {
	return constants.GetConstantValues(semver.MustParse("9999999.0.0"))
}

func (k KVStoreDummy) GetConfigInt64(ctx cosmos.Context, key constants.ConstantName) int64 {
	return -1
}

func (k KVStoreDummy) IsTradingHalt(ctx cosmos.Context, msg cosmos.Msg) bool            { return false }
func (k KVStoreDummy) IsGlobalTradingHalted(ctx cosmos.Context) bool                    { return false }
func (k KVStoreDummy) IsChainTradingHalted(ctx cosmos.Context, chain common.Chain) bool { return false }
func (k KVStoreDummy) IsChainHalted(ctx cosmos.Context, chain common.Chain) bool        { return false }
func (k KVStoreDummy) IsLPPaused(ctx cosmos.Context, chain common.Chain) bool           { return false }

func (k KVStoreDummy) GetAnchors(ctx cosmos.Context, asset common.Asset) []common.Asset { return nil }
func (k KVStoreDummy) AnchorMedian(ctx cosmos.Context, assets []common.Asset) cosmos.Uint {
	return cosmos.ZeroUint()
}
func (k KVStoreDummy) DollarsPerRune(ctx cosmos.Context) cosmos.Uint { return cosmos.ZeroUint() }
func (k KVStoreDummy) DollarInRune(ctx cosmos.Context) cosmos.Uint   { return cosmos.ZeroUint() } // TODO: remove me on hard fork
func (k KVStoreDummy) DollarConfigInRune(ctx cosmos.Context, key constants.ConstantName) cosmos.Uint {
	return cosmos.ZeroUint()
}

func (k KVStoreDummy) GetNativeTxFee(ctx cosmos.Context) cosmos.Uint {
	return cosmos.ZeroUint()
}

func (k KVStoreDummy) GetOutboundTxFee(ctx cosmos.Context) cosmos.Uint {
	return cosmos.ZeroUint()
}

func (k KVStoreDummy) GetTHORNameRegisterFee(ctx cosmos.Context) cosmos.Uint {
	return cosmos.ZeroUint()
}

func (k KVStoreDummy) GetTHORNamePerBlockFee(ctx cosmos.Context) cosmos.Uint {
	return cosmos.ZeroUint()
}

// a mock cosmos.Iterator implementation for testing purposes
type DummyIterator struct {
	cosmos.Iterator
	placeholder int
	keys        [][]byte
	values      [][]byte
	err         error
}

func NewDummyIterator() *DummyIterator {
	return &DummyIterator{
		keys:   make([][]byte, 0),
		values: make([][]byte, 0),
	}
}

func (iter *DummyIterator) AddItem(key, value []byte) {
	iter.keys = append(iter.keys, key)
	iter.values = append(iter.values, value)
}

func (iter *DummyIterator) Next() {
	iter.placeholder++
}

func (iter *DummyIterator) Valid() bool {
	return iter.placeholder < len(iter.keys)
}

func (iter *DummyIterator) Key() []byte {
	return iter.keys[iter.placeholder]
}

func (iter *DummyIterator) Value() []byte {
	return iter.values[iter.placeholder]
}

func (iter *DummyIterator) Close() error {
	iter.placeholder = 0
	return nil
}

func (iter *DummyIterator) Error() error {
	return iter.err
}

func (iter *DummyIterator) Domain() (start, end []byte) {
	return nil, nil
}
