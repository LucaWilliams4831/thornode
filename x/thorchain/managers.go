package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	kv1 "gitlab.com/thorchain/thornode/x/thorchain/keeper/v1"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

const (
	genesisBlockHeight = 1
)

// ErrNotEnoughToPayFee will happen when the emitted asset is not enough to pay for fee
var ErrNotEnoughToPayFee = errors.New("not enough asset to pay for fees")

// Manager is an interface to define all the required methods
type Manager interface {
	GetConstants() constants.ConstantValues
	GetVersion() semver.Version
	Keeper() keeper.Keeper
	GasMgr() GasManager
	EventMgr() EventManager
	TxOutStore() TxOutStore
	NetworkMgr() NetworkManager
	ValidatorMgr() ValidatorManager
	ObMgr() ObserverManager
	PoolMgr() PoolManager
	SwapQ() SwapQueue
	OrderBookMgr() OrderBook
	Slasher() Slasher
	YggManager() YggManager
}

// GasManager define all the methods required to manage gas
type GasManager interface {
	BeginBlock(mgr Manager)
	EndBlock(ctx cosmos.Context, keeper keeper.Keeper, eventManager EventManager)
	AddGasAsset(gas common.Gas, increaseTxCount bool)
	ProcessGas(ctx cosmos.Context, keeper keeper.Keeper)
	GetGas() common.Gas
	GetFee(ctx cosmos.Context, chain common.Chain, asset common.Asset) cosmos.Uint
	GetMaxGas(ctx cosmos.Context, chain common.Chain) (common.Coin, error)
	GetGasRate(ctx cosmos.Context, chain common.Chain) cosmos.Uint
	GetNetworkFee(ctx cosmos.Context, chain common.Chain) (types.NetworkFee, error)
	SubGas(gas common.Gas)
	CalcOutboundFeeMultiplier(ctx cosmos.Context, targetSurplusRune, gasSpentRune, gasWithheldRune, maxMultiplier, minMultiplier cosmos.Uint) cosmos.Uint
}

// EventManager define methods need to be support to manage events
type EventManager interface {
	EmitEvent(ctx cosmos.Context, evt EmitEventItem) error
	EmitGasEvent(ctx cosmos.Context, gasEvent *EventGas) error
	EmitSwapEvent(ctx cosmos.Context, swap *EventSwap) error
	EmitFeeEvent(ctx cosmos.Context, feeEvent *EventFee) error
}

// TxOutStore define the method required for TxOutStore
type TxOutStore interface {
	EndBlock(ctx cosmos.Context, mgr Manager) error
	GetBlockOut(ctx cosmos.Context) (*TxOut, error)
	ClearOutboundItems(ctx cosmos.Context)
	GetOutboundItems(ctx cosmos.Context) ([]TxOutItem, error)
	TryAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem, minOut cosmos.Uint) (bool, error)
	UnSafeAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem) error
	GetOutboundItemByToAddress(cosmos.Context, common.Address) []TxOutItem
	CalcTxOutHeight(cosmos.Context, semver.Version, TxOutItem) (int64, error)
}

// ObserverManager define the method to manage observes
type ObserverManager interface {
	BeginBlock()
	EndBlock(ctx cosmos.Context, keeper keeper.Keeper)
	AppendObserver(chain common.Chain, addrs []cosmos.AccAddress)
	List() []cosmos.AccAddress
}

// ValidatorManager define the method to manage validators
type ValidatorManager interface {
	BeginBlock(ctx cosmos.Context, mgr Manager, existingValidators []string) error
	EndBlock(ctx cosmos.Context, mgr Manager) []abci.ValidatorUpdate
	RequestYggReturn(ctx cosmos.Context, node NodeAccount, mgr Manager) error
	processRagnarok(ctx cosmos.Context, mgr Manager) error
	NodeAccountPreflightCheck(ctx cosmos.Context, na NodeAccount, constAccessor constants.ConstantValues) (NodeStatus, error)
}

// NetworkManager interface define the contract of network Manager
type NetworkManager interface {
	TriggerKeygen(ctx cosmos.Context, nas NodeAccounts) error
	RotateVault(ctx cosmos.Context, vault Vault) error
	BeginBlock(ctx cosmos.Context, mgr Manager) error
	EndBlock(ctx cosmos.Context, mgr Manager) error
	UpdateNetwork(ctx cosmos.Context, constAccessor constants.ConstantValues, gasManager GasManager, eventMgr EventManager) error
	RecallChainFunds(ctx cosmos.Context, chain common.Chain, mgr Manager, excludeNode common.PubKeys) error
	SpawnDerivedAsset(ctx cosmos.Context, asset common.Asset, mgr Manager)
}

// PoolManager interface define the contract of PoolManager
type PoolManager interface {
	EndBlock(ctx cosmos.Context, mgr Manager) error
}

// SwapQueue interface define the contract of Swap Queue
type SwapQueue interface {
	EndBlock(ctx cosmos.Context, mgr Manager) error
}

// OrderBook interface define the contract of Order Book
type OrderBook interface {
	EndBlock(ctx cosmos.Context, mgr Manager) error
}

// Slasher define all the method to perform slash
type Slasher interface {
	BeginBlock(ctx cosmos.Context, req abci.RequestBeginBlock, constAccessor constants.ConstantValues)
	HandleDoubleSign(ctx cosmos.Context, addr crypto.Address, infractionHeight int64, constAccessor constants.ConstantValues) error
	LackObserving(ctx cosmos.Context, constAccessor constants.ConstantValues) error
	LackSigning(ctx cosmos.Context, mgr Manager) error
	SlashVault(ctx cosmos.Context, vaultPK common.PubKey, coins common.Coins, mgr Manager) error
	IncSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress)
	DecSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress)
}

// YggManager define method to fund yggdrasil
type YggManager interface {
	Fund(ctx cosmos.Context, mgr Manager) error
}

// Mgrs is an implementation of Manager interface
type Mgrs struct {
	currentVersion semver.Version
	constAccessor  constants.ConstantValues
	gasMgr         GasManager
	eventMgr       EventManager
	txOutStore     TxOutStore
	networkMgr     NetworkManager
	validatorMgr   ValidatorManager
	obMgr          ObserverManager
	poolMgr        PoolManager
	swapQ          SwapQueue
	orderBook      OrderBook
	slasher        Slasher
	yggManager     YggManager

	K             keeper.Keeper
	cdc           codec.BinaryCodec
	coinKeeper    bankkeeper.Keeper
	accountKeeper authkeeper.AccountKeeper
	storeKey      cosmos.StoreKey
}

// NewManagers  create a new Manager
func NewManagers(keeper keeper.Keeper, cdc codec.BinaryCodec, coinKeeper bankkeeper.Keeper, accountKeeper authkeeper.AccountKeeper, storeKey cosmos.StoreKey) *Mgrs {
	return &Mgrs{
		K:             keeper,
		cdc:           cdc,
		coinKeeper:    coinKeeper,
		accountKeeper: accountKeeper,
		storeKey:      storeKey,
	}
}

func (mgr *Mgrs) GetVersion() semver.Version {
	return mgr.currentVersion
}

func (mgr *Mgrs) GetConstants() constants.ConstantValues {
	return mgr.constAccessor
}

// BeginBlock detect whether there are new version available, if it is available then create a new version of Mgr
func (mgr *Mgrs) BeginBlock(ctx cosmos.Context) error {
	v := mgr.K.GetLowestActiveVersion(ctx)
	if v.Equals(mgr.GetVersion()) {
		return nil
	}
	// version is different , thus all the manager need to re-create
	mgr.currentVersion = v
	mgr.constAccessor = constants.GetConstantValues(v)
	var err error

	mgr.K, err = GetKeeper(v, mgr.cdc, mgr.coinKeeper, mgr.accountKeeper, mgr.storeKey)
	if err != nil {
		return fmt.Errorf("fail to create keeper: %w", err)
	}

	shouldEmitVersion := false
	if v.GTE(semver.MustParse("1.96.0")) { // TODO remove version checks after fork
		storedVer, hasStoredVer := mgr.Keeper().GetVersionWithCtx(ctx)
		if !hasStoredVer || v.GT(storedVer) {
			// store the version for contextual lookups if it has been upgraded
			mgr.Keeper().SetVersionWithCtx(ctx, v)
		}

		if v.GTE(semver.MustParse("1.107.0")) { // TODO remove version checks after fork, here doing 'shouldEmitVersion :='
			// Managers might be new from a fresh node,
			// but the keeper should be consistent.
			shouldEmitVersion = (hasStoredVer && v.GT(storedVer))
		}
	}

	mgr.gasMgr, err = GetGasManager(v, mgr.K)
	if err != nil {
		return fmt.Errorf("fail to create gas manager: %w", err)
	}

	mgr.eventMgr, err = GetEventManager(v)
	if err != nil {
		return fmt.Errorf("fail to get event manager: %w", err)
	}
	// Having created the event manager, emit a version event
	// as long as there was indeed a previously-recorded version changed from.
	// (As indicated by the keeper, rather than by managers created for the first time.)
	if shouldEmitVersion {
		evt := NewEventVersion(v)
		if err := mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			ctx.Logger().Error("fail to emit version event", "error", err)
		}
	}

	mgr.txOutStore, err = GetTxOutStore(v, mgr.K, mgr.eventMgr, mgr.gasMgr)
	if err != nil {
		return fmt.Errorf("fail to get tx out store: %w", err)
	}

	mgr.networkMgr, err = GetNetworkManager(v, mgr.K, mgr.txOutStore, mgr.eventMgr)
	if err != nil {
		return fmt.Errorf("fail to get vault manager: %w", err)
	}

	mgr.poolMgr, err = GetPoolManager(v)
	if err != nil {
		return fmt.Errorf("fail to get pool manager: %w", err)
	}

	mgr.validatorMgr, err = GetValidatorManager(v, mgr.K, mgr.networkMgr, mgr.txOutStore, mgr.eventMgr)
	if err != nil {
		return fmt.Errorf("fail to get validator manager: %w", err)
	}

	mgr.obMgr, err = GetObserverManager(v)
	if err != nil {
		return fmt.Errorf("fail to get observer manager: %w", err)
	}

	mgr.swapQ, err = GetSwapQueue(v, mgr.K)
	if err != nil {
		return fmt.Errorf("fail to create swap queue: %w", err)
	}

	mgr.orderBook, err = GetOrderBook(v, mgr.K)
	if err != nil {
		return fmt.Errorf("fail to create order book: %w", err)
	}

	mgr.slasher, err = GetSlasher(v, mgr.K, mgr.eventMgr)
	if err != nil {
		return fmt.Errorf("fail to create swap queue: %w", err)
	}

	mgr.yggManager, err = GetYggManager(v, mgr.K)
	if err != nil {
		return fmt.Errorf("fail to create swap queue: %w", err)
	}
	return nil
}

// Keeper return Keeper
func (mgr *Mgrs) Keeper() keeper.Keeper { return mgr.K }

// GasMgr return GasManager
func (mgr *Mgrs) GasMgr() GasManager { return mgr.gasMgr }

// EventMgr return EventMgr
func (mgr *Mgrs) EventMgr() EventManager { return mgr.eventMgr }

// TxOutStore return an TxOutStore
func (mgr *Mgrs) TxOutStore() TxOutStore { return mgr.txOutStore }

// VaultMgr return a valid NetworkManager
func (mgr *Mgrs) NetworkMgr() NetworkManager { return mgr.networkMgr }

// PoolMgr return a valid PoolManager
func (mgr *Mgrs) PoolMgr() PoolManager { return mgr.poolMgr }

// ValidatorMgr return an implementation of ValidatorManager
func (mgr *Mgrs) ValidatorMgr() ValidatorManager { return mgr.validatorMgr }

// ObMgr return an implementation of ObserverManager
func (mgr *Mgrs) ObMgr() ObserverManager { return mgr.obMgr }

// SwapQ return an implementation of SwapQueue
func (mgr *Mgrs) SwapQ() SwapQueue { return mgr.swapQ }

// OrderBookMgr
func (mgr *Mgrs) OrderBookMgr() OrderBook { return mgr.orderBook }

// Slasher return an implementation of Slasher
func (mgr *Mgrs) Slasher() Slasher { return mgr.slasher }

// YggManager return an implementation of YggManager
func (mgr *Mgrs) YggManager() YggManager { return mgr.yggManager }

// GetKeeper return Keeper
func GetKeeper(version semver.Version, cdc codec.BinaryCodec, coinKeeper bankkeeper.Keeper, accountKeeper authkeeper.AccountKeeper, storeKey cosmos.StoreKey) (keeper.Keeper, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return kv1.NewKVStore(cdc, coinKeeper, accountKeeper, storeKey, version), nil
	}
	return nil, errInvalidVersion
}

// GetGasManager return GasManager
func GetGasManager(version semver.Version, keeper keeper.Keeper) (GasManager, error) {
	constAccessor := constants.GetConstantValues(version)
	switch {
	case version.GTE(semver.MustParse("1.113.0")):
		return newGasMgrV113(constAccessor, keeper), nil
	case version.GTE(semver.MustParse("1.112.0")):
		return newGasMgrV112(constAccessor, keeper), nil
	case version.GTE(semver.MustParse("1.109.0")):
		return newGasMgrV109(constAccessor, keeper), nil
	case version.GTE(semver.MustParse("1.108.0")):
		return newGasMgrV108(constAccessor, keeper), nil
	case version.GTE(semver.MustParse("1.102.0")):
		return newGasMgrV102(constAccessor, keeper), nil
	case version.GTE(semver.MustParse("1.99.0")):
		return newGasMgrV99(constAccessor, keeper), nil
	case version.GTE(semver.MustParse("1.94.0")):
		return newGasMgrV94(constAccessor, keeper), nil
	case version.GTE(semver.MustParse("1.89.0")):
		return newGasMgrV89(constAccessor, keeper), nil
	case version.GTE(semver.MustParse("0.81.0")):
		return newGasMgrV81(constAccessor, keeper), nil
	}
	return nil, errInvalidVersion
}

// GetEventManager will return an implementation of EventManager
func GetEventManager(version semver.Version) (EventManager, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return newEventMgrV1(), nil
	}
	return nil, errInvalidVersion
}

// GetTxOutStore will return an implementation of the txout store that
func GetTxOutStore(version semver.Version, keeper keeper.Keeper, eventMgr EventManager, gasManager GasManager) (TxOutStore, error) {
	constAccessor := constants.GetConstantValues(version)
	switch {
	case version.GTE(semver.MustParse("1.113.0")):
		return newTxOutStorageV113(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.112.0")):
		return newTxOutStorageV112(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.109.0")):
		return newTxOutStorageV109(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.108.0")):
		return newTxOutStorageV108(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.107.0")):
		return newTxOutStorageV107(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.102.0")):
		return newTxOutStorageV102(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.98.0")):
		return newTxOutStorageV98(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.97.0")):
		return newTxOutStorageV97(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.95.0")):
		return newTxOutStorageV95(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.94.0")):
		return newTxOutStorageV94(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.93.0")):
		return newTxOutStorageV93(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.88.0")):
		return newTxOutStorageV88(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.85.0")):
		return newTxOutStorageV85(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.84.0")):
		return newTxOutStorageV84(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("1.83.0")):
		return newTxOutStorageV83(keeper, constAccessor, eventMgr, gasManager), nil
	case version.GTE(semver.MustParse("0.78.0")):
		return newTxOutStorageV78(keeper, constAccessor, eventMgr, gasManager), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetNetworkManager  retrieve a NetworkManager that is compatible with the given version
func GetNetworkManager(version semver.Version, keeper keeper.Keeper, txOutStore TxOutStore, eventMgr EventManager) (NetworkManager, error) {
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		return newNetworkMgrV112(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.111.0")):
		return newNetworkMgrV111(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.109.0")):
		return newNetworkMgrV109(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.107.0")):
		return newNetworkMgrV107(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.106.0")):
		return newNetworkMgrV106(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.102.0")):
		return newNetworkMgrV102(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.99.0")):
		return newNetworkMgrV99(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.98.0")):
		return newNetworkMgrV98(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.96.0")):
		return newNetworkMgrV96(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.95.0")):
		return newNetworkMgrV95(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.94.0")):
		return newNetworkMgrV94(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.93.0")):
		return newNetworkMgrV93(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.92.0")):
		return newNetworkMgrV92(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.91.0")):
		return newNetworkMgrV91(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.90.0")):
		return newNetworkMgrV90(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.89.0")):
		return newNetworkMgrV89(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.87.0")):
		return newNetworkMgrV87(keeper, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("0.76.0")):
		return newNetworkMgrV76(keeper, txOutStore, eventMgr), nil
	}
	return nil, errInvalidVersion
}

// GetValidatorManager create a new instance of Validator Manager
func GetValidatorManager(version semver.Version, keeper keeper.Keeper, networkMgr NetworkManager, txOutStore TxOutStore, eventMgr EventManager) (ValidatorManager, error) {
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		return newValidatorMgrV112(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.110.0")):
		return newValidatorMgrV110(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.109.0")):
		return newValidatorMgrV109(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.106.0")):
		return newValidatorMgrV106(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.103.0")):
		return newValidatorMgrV103(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.95.0")):
		return newValidatorMgrV95(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.92.0")):
		return newValidatorMgrV92(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.87.0")):
		return newValidatorMgrV87(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("1.84.0")):
		return newValidatorMgrV84(keeper, networkMgr, txOutStore, eventMgr), nil
	case version.GTE(semver.MustParse("0.80.0")):
		return newValidatorMgrV80(keeper, networkMgr, txOutStore, eventMgr), nil

	}
	return nil, errInvalidVersion
}

// GetObserverManager return an instance that implements ObserverManager interface
// when there is no version can match the given semver , it will return nil
func GetObserverManager(version semver.Version) (ObserverManager, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return newObserverMgrV1(), nil
	}
	return nil, errInvalidVersion
}

// GetPoolManager return an implementation of PoolManager
func GetPoolManager(version semver.Version) (PoolManager, error) {
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		return newPoolMgrV112(), nil
	case version.GTE(semver.MustParse("1.108.0")):
		return newPoolMgrV108(), nil
	case version.GTE(semver.MustParse("1.98.0")):
		return newPoolMgrV98(), nil
	case version.GTE(semver.MustParse("1.95.0")):
		return newPoolMgrV95(), nil
	case version.GTE(semver.MustParse("0.73.0")):
		return newPoolMgrV73(), nil
	}
	return nil, errInvalidVersion
}

// GetSwapQueue retrieve a SwapQueue that is compatible with the given version
func GetSwapQueue(version semver.Version, keeper keeper.Keeper) (SwapQueue, error) {
	switch {
	case version.GTE(semver.MustParse("1.104.0")):
		return newSwapQueueV104(keeper), nil
	case version.GTE(semver.MustParse("1.103.0")):
		return newSwapQueueV103(keeper), nil
	case version.GTE(semver.MustParse("1.95.0")):
		return newSwapQv95(keeper), nil
	case version.GTE(semver.MustParse("1.94.0")):
		return newSwapQv94(keeper), nil
	case version.GTE(semver.MustParse("0.58.0")):
		return newSwapQv58(keeper), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetOrderBook retrieve a OrderBook that is compatible with the given version
func GetOrderBook(version semver.Version, keeper keeper.Keeper) (OrderBook, error) {
	switch {
	case version.GTE(semver.MustParse("1.104.0")):
		return newOrderBookV104(keeper), nil
	case version.GTE(semver.MustParse("1.103.0")):
		return newOrderBookV103(keeper), nil
	case version.GTE(semver.MustParse("0.1.0")):
		return newOrderBookV1(keeper), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetSlasher return an implementation of Slasher
func GetSlasher(version semver.Version, keeper keeper.Keeper, eventMgr EventManager) (Slasher, error) {
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		return newSlasherV112(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.109.0")):
		return newSlasherV109(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.108.0")):
		return newSlasherV108(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.92.0")):
		return newSlasherV92(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.89.0")):
		return newSlasherV89(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.88.0")):
		return newSlasherV88(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.87.0")):
		return newSlasherV87(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("1.86.0")):
		return newSlasherV86(keeper, eventMgr), nil
	case version.GTE(semver.MustParse("0.75.0")):
		return newSlasherV75(keeper, eventMgr), nil
	default:
		return nil, errInvalidVersion
	}
}

// GetYggManager return an implementation of YggManager
func GetYggManager(version semver.Version, keeper keeper.Keeper) (YggManager, error) {
	switch {
	case version.GTE(semver.MustParse("1.112.0")):
		return newYggMgrV112(keeper), nil
	case version.GTE(semver.MustParse("0.79.0")):
		return newYggMgrV79(keeper), nil
	}
	return nil, errInvalidVersion
}
