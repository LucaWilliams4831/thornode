package keeper

import (
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	crisis "github.com/cosmos/cosmos-sdk/x/crisis/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
	kv1 "gitlab.com/thorchain/thornode/x/thorchain/keeper/v1"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type Keeper interface {
	Cdc() codec.BinaryCodec
	GetVersion() semver.Version
	GetVersionWithCtx(ctx cosmos.Context) (semver.Version, bool)
	SetVersionWithCtx(ctx cosmos.Context, v semver.Version)
	GetMinJoinLast(ctx cosmos.Context) (semver.Version, int64)
	SetMinJoinLast(ctx cosmos.Context)
	GetKey(ctx cosmos.Context, prefix kvTypes.DbPrefix, key string) string
	GetStoreVersion(ctx cosmos.Context) int64
	SetStoreVersion(ctx cosmos.Context, ver int64)
	GetRuneBalanceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint
	GetBalanceOfModule(ctx cosmos.Context, moduleName, denom string) cosmos.Uint
	SendFromModuleToModule(ctx cosmos.Context, from, to string, coin common.Coins) error
	SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coin common.Coins) error
	SendFromModuleToAccount(ctx cosmos.Context, from string, to cosmos.AccAddress, coin common.Coins) error
	MintToModule(ctx cosmos.Context, module string, coin common.Coin) error
	BurnFromModule(ctx cosmos.Context, module string, coin common.Coin) error
	MintAndSendToAccount(ctx cosmos.Context, to cosmos.AccAddress, coin common.Coin) error
	GetModuleAddress(module string) (common.Address, error)
	GetModuleAccAddress(module string) cosmos.AccAddress
	GetBalance(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Coins
	HasCoins(ctx cosmos.Context, addr cosmos.AccAddress, coins cosmos.Coins) bool
	GetAccount(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Account

	// passthrough funcs
	SendCoins(ctx cosmos.Context, from, to cosmos.AccAddress, coins cosmos.Coins) error
	AddCoins(ctx cosmos.Context, addr cosmos.AccAddress, coins cosmos.Coins) error

	InvariantRoutes() []crisis.InvarRoute

	GetConstants() constants.ConstantValues
	GetConfigInt64(ctx cosmos.Context, key constants.ConstantName) int64
	DollarConfigInRune(ctx cosmos.Context, key constants.ConstantName) cosmos.Uint

	GetNativeTxFee(ctx cosmos.Context) cosmos.Uint
	GetOutboundTxFee(ctx cosmos.Context) cosmos.Uint
	GetTHORNameRegisterFee(ctx cosmos.Context) cosmos.Uint
	GetTHORNamePerBlockFee(ctx cosmos.Context) cosmos.Uint

	// Keeper Interfaces
	KeeperConfig
	KeeperPool
	KeeperLastHeight
	KeeperLiquidityProvider
	KeeperLoan
	KeeperNodeAccount
	KeeperObserver
	KeeperObservedTx
	KeeperTxOut
	KeeperLiquidityFees
	KeeperSwapSlip
	KeeperVault
	KeeperReserveContributors
	KeeperNetwork
	KeeperTss
	KeeperTssKeysignFail
	KeeperKeygen
	KeeperRagnarok
	KeeperErrataTx
	KeeperBanVoter
	KeeperSwapQueue
	KeeperOrderBooks
	KeeperMimir
	KeeperNetworkFee
	KeeperObservedNetworkFeeVoter
	KeeperChainContract
	KeeperSolvencyVoter
	KeeperTHORName
	KeeperHalt
	KeeperAnchors
}

type KeeperConfig interface {
	GetConstants() constants.ConstantValues
	GetConfigInt64(ctx cosmos.Context, key constants.ConstantName) int64
}

type KeeperPool interface {
	GetPoolIterator(ctx cosmos.Context) cosmos.Iterator
	GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error)
	GetPools(ctx cosmos.Context) (Pools, error)
	SetPool(ctx cosmos.Context, pool Pool) error
	PoolExist(ctx cosmos.Context, asset common.Asset) bool
	RemovePool(ctx cosmos.Context, asset common.Asset)
	SetPoolLUVI(ctx cosmos.Context, asset common.Asset, luvi cosmos.Uint)
	GetPoolLUVI(ctx cosmos.Context, asset common.Asset) (cosmos.Uint, error)
}

type KeeperLastHeight interface {
	SetLastSignedHeight(ctx cosmos.Context, height int64) error
	GetLastSignedHeight(ctx cosmos.Context) (int64, error)
	SetLastChainHeight(ctx cosmos.Context, chain common.Chain, height int64) error
	ForceSetLastChainHeight(ctx cosmos.Context, chain common.Chain, height int64)
	GetLastChainHeight(ctx cosmos.Context, chain common.Chain) (int64, error)
	GetLastChainHeights(ctx cosmos.Context) (map[common.Chain]int64, error)
	SetLastObserveHeight(ctx cosmos.Context, chain common.Chain, address cosmos.AccAddress, height int64) error
	ForceSetLastObserveHeight(ctx cosmos.Context, chain common.Chain, address cosmos.AccAddress, height int64)
	GetLastObserveHeight(ctx cosmos.Context, address cosmos.AccAddress) (map[common.Chain]int64, error)
}

type KeeperLoan interface {
	GetLoanIterator(ctx cosmos.Context, _ common.Asset) cosmos.Iterator
	GetLoan(ctx cosmos.Context, asset common.Asset, addr common.Address) (Loan, error)
	SetLoan(ctx cosmos.Context, _ Loan)
	RemoveLoan(ctx cosmos.Context, _ Loan)
	SetTotalCollateral(_ cosmos.Context, _ common.Asset, _ cosmos.Uint)
	GetTotalCollateral(_ cosmos.Context, _ common.Asset) (cosmos.Uint, error)
}

type KeeperLiquidityProvider interface {
	GetLiquidityProviderIterator(ctx cosmos.Context, _ common.Asset) cosmos.Iterator
	GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error)
	SetLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider)
	RemoveLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider)
	GetTotalSupply(ctx cosmos.Context, asset common.Asset) cosmos.Uint
}

type KeeperNodeAccount interface {
	TotalActiveValidators(ctx cosmos.Context) (int, error)
	ListValidatorsWithBond(ctx cosmos.Context) (NodeAccounts, error)
	ListValidatorsByStatus(ctx cosmos.Context, status NodeStatus) (NodeAccounts, error)
	ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error)
	GetLowestActiveVersion(ctx cosmos.Context) semver.Version
	GetMinJoinVersion(ctx cosmos.Context) semver.Version
	GetNodeAccount(ctx cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error)
	GetNodeAccountByPubKey(ctx cosmos.Context, pk common.PubKey) (NodeAccount, error)
	SetNodeAccount(ctx cosmos.Context, na NodeAccount) error
	EnsureNodeKeysUnique(ctx cosmos.Context, consensusPubKey string, pubKeys common.PubKeySet) error
	GetNodeAccountIterator(ctx cosmos.Context) cosmos.Iterator
	GetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress) (int64, error)
	SetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64)
	IncNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64) error
	DecNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress, _ int64) error
	ResetNodeAccountSlashPoints(_ cosmos.Context, _ cosmos.AccAddress)
	GetNodeAccountJail(ctx cosmos.Context, addr cosmos.AccAddress) (Jail, error)
	SetNodeAccountJail(ctx cosmos.Context, addr cosmos.AccAddress, height int64, reason string) error
	ReleaseNodeAccountFromJail(ctx cosmos.Context, addr cosmos.AccAddress) error
	SetBondProviders(ctx cosmos.Context, _ BondProviders) error
	GetBondProviders(ctx cosmos.Context, add cosmos.AccAddress) (BondProviders, error)
}

type KeeperObserver interface {
	GetObservingAddresses(ctx cosmos.Context) ([]cosmos.AccAddress, error)
	AddObservingAddresses(ctx cosmos.Context, inAddresses []cosmos.AccAddress) error
	ClearObservingAddresses(ctx cosmos.Context)
}

type KeeperObservedTx interface {
	SetObservedTxInVoter(ctx cosmos.Context, tx ObservedTxVoter)
	GetObservedTxInVoterIterator(ctx cosmos.Context) cosmos.Iterator
	GetObservedTxInVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error)
	SetObservedTxOutVoter(ctx cosmos.Context, tx ObservedTxVoter)
	GetObservedTxOutVoterIterator(ctx cosmos.Context) cosmos.Iterator
	GetObservedTxOutVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error)
	SetObservedLink(ctx cosmos.Context, _, _ common.TxID)
	GetObservedLink(ctx cosmos.Context, inhash common.TxID) []common.TxID
}

type KeeperTxOut interface {
	SetTxOut(ctx cosmos.Context, blockOut *TxOut) error
	AppendTxOut(ctx cosmos.Context, height int64, item TxOutItem) error
	ClearTxOut(ctx cosmos.Context, height int64) error
	GetTxOutIterator(ctx cosmos.Context) cosmos.Iterator
	GetTxOut(ctx cosmos.Context, height int64) (*TxOut, error)
	GetTxOutValue(ctx cosmos.Context, height int64) (cosmos.Uint, error)
}

type KeeperLiquidityFees interface {
	AddToLiquidityFees(ctx cosmos.Context, asset common.Asset, fee cosmos.Uint) error
	GetTotalLiquidityFees(ctx cosmos.Context, height uint64) (cosmos.Uint, error)
	GetPoolLiquidityFees(ctx cosmos.Context, height uint64, asset common.Asset) (cosmos.Uint, error)
	GetRollingPoolLiquidityFee(ctx cosmos.Context, asset common.Asset) (uint64, error)
	ResetRollingPoolLiquidityFee(ctx cosmos.Context, asset common.Asset)
}

type KeeperSwapSlip interface {
	AddToSwapSlip(ctx cosmos.Context, asset common.Asset, amt cosmos.Int) error
	RollupSwapSlip(ctx cosmos.Context, blockCount int64, _ common.Asset) (cosmos.Int, error)
	GetPoolSwapSlip(ctx cosmos.Context, height int64, asset common.Asset) (cosmos.Int, error)
	DeletePoolSwapSlip(ctx cosmos.Context, height int64, asset common.Asset)
}

type KeeperVault interface {
	GetVaultIterator(ctx cosmos.Context) cosmos.Iterator
	VaultExists(ctx cosmos.Context, pk common.PubKey) bool
	SetVault(ctx cosmos.Context, vault Vault) error
	GetVault(ctx cosmos.Context, pk common.PubKey) (Vault, error)
	HasValidVaultPools(ctx cosmos.Context) (bool, error)
	GetAsgardVaults(ctx cosmos.Context) (Vaults, error)
	GetAsgardVaultsByStatus(_ cosmos.Context, _ VaultStatus) (Vaults, error)
	GetLeastSecure(_ cosmos.Context, _ Vaults, _ int64) Vault
	GetMostSecure(_ cosmos.Context, _ Vaults, _ int64) Vault
	SortBySecurity(_ cosmos.Context, _ Vaults, _ int64) Vaults
	DeleteVault(ctx cosmos.Context, pk common.PubKey) error
	RemoveFromAsgardIndex(ctx cosmos.Context, pubkey common.PubKey) error
}

type KeeperReserveContributors interface {
	AddPoolFeeToReserve(ctx cosmos.Context, fee cosmos.Uint) error
	AddBondFeeToReserve(ctx cosmos.Context, fee cosmos.Uint) error
}

// KeeperNetwork func to access network data in key value store
type KeeperNetwork interface {
	GetNetwork(ctx cosmos.Context) (Network, error)
	SetNetwork(ctx cosmos.Context, data Network) error
	GetPOL(ctx cosmos.Context) (ProtocolOwnedLiquidity, error)
	SetPOL(ctx cosmos.Context, data ProtocolOwnedLiquidity) error
}

type KeeperTss interface {
	SetTssVoter(_ cosmos.Context, tss TssVoter)
	GetTssVoterIterator(_ cosmos.Context) cosmos.Iterator
	GetTssVoter(_ cosmos.Context, _ string) (TssVoter, error)
	SetTssKeygenMetric(_ cosmos.Context, metric *TssKeygenMetric)
	GetTssKeygenMetric(_ cosmos.Context, key common.PubKey) (*TssKeygenMetric, error)
	SetTssKeysignMetric(_ cosmos.Context, metric *TssKeysignMetric)
	GetTssKeysignMetric(_ cosmos.Context, txID common.TxID) (*TssKeysignMetric, error)
	GetLatestTssKeysignMetric(_ cosmos.Context) (*TssKeysignMetric, error)
}

type KeeperTssKeysignFail interface {
	SetTssKeysignFailVoter(_ cosmos.Context, tss TssKeysignFailVoter)
	GetTssKeysignFailVoterIterator(_ cosmos.Context) cosmos.Iterator
	GetTssKeysignFailVoter(_ cosmos.Context, _ string) (TssKeysignFailVoter, error)
}

type KeeperKeygen interface {
	SetKeygenBlock(ctx cosmos.Context, keygenBlock KeygenBlock)
	GetKeygenBlockIterator(ctx cosmos.Context) cosmos.Iterator
	GetKeygenBlock(ctx cosmos.Context, height int64) (KeygenBlock, error)
}

type KeeperBanVoter interface {
	SetBanVoter(_ cosmos.Context, _ BanVoter)
	GetBanVoter(_ cosmos.Context, _ cosmos.AccAddress) (BanVoter, error)
	GetBanVoterIterator(_ cosmos.Context) cosmos.Iterator
}

type KeeperRagnarok interface {
	RagnarokInProgress(_ cosmos.Context) bool
	GetRagnarokBlockHeight(_ cosmos.Context) (int64, error)
	SetRagnarokBlockHeight(_ cosmos.Context, _ int64)
	GetRagnarokNth(_ cosmos.Context) (int64, error)
	SetRagnarokNth(_ cosmos.Context, _ int64)
	GetRagnarokPending(_ cosmos.Context) (int64, error)
	SetRagnarokPending(_ cosmos.Context, _ int64)
	GetRagnarokWithdrawPosition(ctx cosmos.Context) (RagnarokWithdrawPosition, error)
	SetRagnarokWithdrawPosition(ctx cosmos.Context, position RagnarokWithdrawPosition)
	SetPoolRagnarokStart(ctx cosmos.Context, asset common.Asset)
	GetPoolRagnarokStart(ctx cosmos.Context, asset common.Asset) (int64, error)
}

type KeeperErrataTx interface {
	SetErrataTxVoter(_ cosmos.Context, _ ErrataTxVoter)
	GetErrataTxVoterIterator(_ cosmos.Context) cosmos.Iterator
	GetErrataTxVoter(_ cosmos.Context, _ common.TxID, _ common.Chain) (ErrataTxVoter, error)
}

type KeeperSwapQueue interface {
	SetSwapQueueItem(ctx cosmos.Context, msg MsgSwap, i int) error
	GetSwapQueueIterator(ctx cosmos.Context) cosmos.Iterator
	GetSwapQueueItem(ctx cosmos.Context, txID common.TxID, i int) (MsgSwap, error)
	HasSwapQueueItem(ctx cosmos.Context, txID common.TxID, i int) bool
	RemoveSwapQueueItem(ctx cosmos.Context, txID common.TxID, i int)
}

type KeeperOrderBooks interface {
	OrderBooksEnabled(ctx cosmos.Context) bool
	SetOrderBookItem(ctx cosmos.Context, msg MsgSwap) error
	GetOrderBookItemIterator(ctx cosmos.Context) cosmos.Iterator
	GetOrderBookItem(ctx cosmos.Context, txID common.TxID) (MsgSwap, error)
	HasOrderBookItem(ctx cosmos.Context, txID common.TxID) bool
	RemoveOrderBookItem(ctx cosmos.Context, txID common.TxID) error
	GetOrderBookIndexIterator(_ cosmos.Context, _ types.OrderType, _, _ common.Asset) cosmos.Iterator
	SetOrderBookIndex(_ cosmos.Context, _ MsgSwap) error
	GetOrderBookIndex(_ cosmos.Context, _ MsgSwap) (common.TxIDs, error)
	HasOrderBookIndex(_ cosmos.Context, _ MsgSwap) (bool, error)
	RemoveOrderBookIndex(_ cosmos.Context, _ MsgSwap) error
	SetOrderBookProcessor(_ cosmos.Context, _ []bool) error
	GetOrderBookProcessor(_ cosmos.Context) ([]bool, error)
}

type KeeperMimir interface {
	GetMimir(_ cosmos.Context, key string) (int64, error)
	SetMimir(_ cosmos.Context, key string, value int64)
	GetNodeMimirs(ctx cosmos.Context, key string) (NodeMimirs, error)
	SetNodeMimir(_ cosmos.Context, key string, value int64, acc cosmos.AccAddress) error
	GetMimirIterator(ctx cosmos.Context) cosmos.Iterator
	GetNodeMimirIterator(ctx cosmos.Context) cosmos.Iterator
	DeleteMimir(_ cosmos.Context, key string) error
	GetNodePauseChain(ctx cosmos.Context, acc cosmos.AccAddress) int64
	SetNodePauseChain(ctx cosmos.Context, acc cosmos.AccAddress)
}

type KeeperNetworkFee interface {
	GetNetworkFee(ctx cosmos.Context, chain common.Chain) (NetworkFee, error)
	SaveNetworkFee(ctx cosmos.Context, chain common.Chain, networkFee NetworkFee) error
	GetNetworkFeeIterator(ctx cosmos.Context) cosmos.Iterator
}

type KeeperObservedNetworkFeeVoter interface {
	SetObservedNetworkFeeVoter(ctx cosmos.Context, networkFeeVoter ObservedNetworkFeeVoter)
	GetObservedNetworkFeeVoterIterator(ctx cosmos.Context) cosmos.Iterator
	GetObservedNetworkFeeVoter(ctx cosmos.Context, height int64, chain common.Chain, rate int64) (ObservedNetworkFeeVoter, error)
}

type KeeperChainContract interface {
	SetChainContract(ctx cosmos.Context, cc ChainContract)
	GetChainContract(ctx cosmos.Context, chain common.Chain) (ChainContract, error)
	GetChainContracts(ctx cosmos.Context, chains common.Chains) []ChainContract
	GetChainContractIterator(ctx cosmos.Context) cosmos.Iterator
}

type KeeperSolvencyVoter interface {
	SetSolvencyVoter(_ cosmos.Context, _ SolvencyVoter)
	GetSolvencyVoter(_ cosmos.Context, _ common.TxID, _ common.Chain) (SolvencyVoter, error)
}

// NewKeeper creates new instances of the thorchain Keeper
type KeeperTHORName interface {
	THORNameExists(ctx cosmos.Context, _ string) bool
	GetTHORName(ctx cosmos.Context, _ string) (THORName, error)
	SetTHORName(ctx cosmos.Context, name THORName)
	GetTHORNameIterator(ctx cosmos.Context) cosmos.Iterator
	DeleteTHORName(ctx cosmos.Context, _ string) error
}

type KeeperHalt interface {
	IsTradingHalt(ctx cosmos.Context, msg cosmos.Msg) bool
	IsGlobalTradingHalted(ctx cosmos.Context) bool
	IsChainTradingHalted(ctx cosmos.Context, chain common.Chain) bool
	IsChainHalted(ctx cosmos.Context, chain common.Chain) bool
	IsLPPaused(ctx cosmos.Context, chain common.Chain) bool
}

type KeeperAnchors interface {
	GetAnchors(ctx cosmos.Context, asset common.Asset) []common.Asset
	AnchorMedian(ctx cosmos.Context, assets []common.Asset) cosmos.Uint
	DollarsPerRune(ctx cosmos.Context) cosmos.Uint
	DollarInRune(ctx cosmos.Context) cosmos.Uint // TODO: remove me on hard fork
}

// NewKVStore creates new instances of the thorchain Keeper
func NewKeeper(cdc codec.BinaryCodec, coinKeeper bankkeeper.Keeper, accountKeeper authkeeper.AccountKeeper, storeKey cosmos.StoreKey) Keeper {
	version := semver.MustParse("0.0.0")
	return kv1.NewKVStore(cdc, coinKeeper, accountKeeper, storeKey, version)
}
