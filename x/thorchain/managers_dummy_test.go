package thorchain

import (
	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type DummyMgr struct {
	K             keeper.Keeper
	constAccessor constants.ConstantValues
	gasMgr        GasManager
	eventMgr      EventManager
	txOutStore    TxOutStore
	networkMgr    NetworkManager
	validatorMgr  ValidatorManager
	obMgr         ObserverManager
	poolMgr       PoolManager
	swapQ         SwapQueue
	orderBook     OrderBook
	slasher       Slasher
	yggManager    YggManager
}

func NewDummyMgrWithKeeper(k keeper.Keeper) *DummyMgr {
	return &DummyMgr{
		K:             k,
		constAccessor: constants.GetConstantValues(GetCurrentVersion()),
		gasMgr:        NewDummyGasManager(),
		eventMgr:      NewDummyEventMgr(),
		txOutStore:    NewTxStoreDummy(),
		networkMgr:    NewNetworkMgrDummy(),
		validatorMgr:  NewValidatorDummyMgr(),
		obMgr:         NewDummyObserverManager(),
		poolMgr:       NewDummyPoolManager(),
		slasher:       NewDummySlasher(),
		yggManager:    NewDummyYggManger(),
		// TODO add dummy swap queue
		// TODO add dummy order book
	}
}

func NewDummyMgr() *DummyMgr {
	return &DummyMgr{
		K:             keeper.KVStoreDummy{},
		constAccessor: constants.GetConstantValues(GetCurrentVersion()),
		gasMgr:        NewDummyGasManager(),
		eventMgr:      NewDummyEventMgr(),
		txOutStore:    NewTxStoreDummy(),
		networkMgr:    NewNetworkMgrDummy(),
		validatorMgr:  NewValidatorDummyMgr(),
		obMgr:         NewDummyObserverManager(),
		poolMgr:       NewDummyPoolManager(),
		slasher:       NewDummySlasher(),
		yggManager:    NewDummyYggManger(),
		// TODO add dummy swap queue
		// TODO add dummy order book
	}
}

func (m DummyMgr) GetVersion() semver.Version             { return GetCurrentVersion() }
func (m DummyMgr) GetConstants() constants.ConstantValues { return m.constAccessor }
func (m DummyMgr) Keeper() keeper.Keeper                  { return m.K }
func (m DummyMgr) GasMgr() GasManager                     { return m.gasMgr }
func (m DummyMgr) EventMgr() EventManager                 { return m.eventMgr }
func (m DummyMgr) TxOutStore() TxOutStore                 { return m.txOutStore }
func (m DummyMgr) NetworkMgr() NetworkManager             { return m.networkMgr }
func (m DummyMgr) ValidatorMgr() ValidatorManager         { return m.validatorMgr }
func (m DummyMgr) ObMgr() ObserverManager                 { return m.obMgr }
func (m DummyMgr) PoolMgr() PoolManager                   { return m.poolMgr }
func (m DummyMgr) SwapQ() SwapQueue                       { return m.swapQ }
func (m DummyMgr) Slasher() Slasher                       { return m.slasher }
func (m DummyMgr) YggManager() YggManager                 { return m.yggManager }
func (m DummyMgr) OrderBookMgr() OrderBook                { return m.orderBook }
