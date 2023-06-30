package thorchain

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	sdkRest "github.com/cosmos/cosmos-sdk/x/auth/client/rest"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"
	abci "github.com/tendermint/tendermint/abci/types"

	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"

	"gitlab.com/thorchain/thornode/x/thorchain/client/cli"
	"gitlab.com/thorchain/thornode/x/thorchain/client/rest"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// type check to ensure the interface is properly implemented
var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
)

// AppModuleBasic app module Basics object
type AppModuleBasic struct{}

// Name return the module's name
func (AppModuleBasic) Name() string {
	return ModuleName
}

// RegisterLegacyAminoCodec registers the module's types for the given codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	RegisterCodec(cdc)
}

// RegisterInterfaces registers the module's interface types
func (a AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	RegisterInterfaces(reg)
}

// DefaultGenesis returns default genesis state as raw bytes for the module.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(DefaultGenesis())
}

// ValidateGenesis check of the Genesis
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var data GenesisState
	if err := cdc.UnmarshalJSON(bz, &data); err != nil {
		return err
	}
	// Once json successfully marshalled, passes along to genesis.go
	return ValidateGenesis(data)
}

// RegisterRESTRoutes register rest routes
func (AppModuleBasic) RegisterRESTRoutes(ctx client.Context, rtr *mux.Router) {
	rest.RegisterRoutes(ctx, rtr, StoreKey)
	sdkRest.RegisterTxRoutes(ctx, rtr)
	sdkRest.RegisterRoutes(ctx, rtr, StoreKey)
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the mint module.
// thornode current doesn't have grpc enpoint yet
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	// types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx))
}

// GetQueryCmd get the root query command of this module
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

// GetTxCmd get the root tx command of this module
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.GetTxCmd()
}

// ____________________________________________________________________________

// AppModule implements an application module for the thorchain module.
type AppModule struct {
	AppModuleBasic
	mgr              *Mgrs
	keybaseStore     cosmos.KeybaseStore
	telemetryEnabled bool
}

// NewAppModule creates a new AppModule Object
func NewAppModule(k keeper.Keeper, cdc codec.BinaryCodec, coinKeeper bankkeeper.Keeper, accountKeeper authkeeper.AccountKeeper, storeKey cosmos.StoreKey, telemetryEnabled bool) AppModule {
	kb, err := cosmos.GetKeybase(os.Getenv(cosmos.EnvChainHome))
	if err != nil {
		panic(err)
	}
	return AppModule{
		AppModuleBasic:   AppModuleBasic{},
		mgr:              NewManagers(k, cdc, coinKeeper, accountKeeper, storeKey),
		keybaseStore:     kb,
		telemetryEnabled: telemetryEnabled,
	}
}

func (AppModule) Name() string {
	return ModuleName
}

func (AppModule) ConsensusVersion() uint64 {
	return 1
}

func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {}

func (am AppModule) Route() cosmos.Route {
	return cosmos.NewRoute(RouterKey, NewExternalHandler(am.mgr))
}

func (am AppModule) NewHandler() sdk.Handler {
	return NewExternalHandler(am.mgr)
}

func (am AppModule) QuerierRoute() string {
	return ModuleName
}

// LegacyQuerierHandler returns the capability module's Querier.
func (am AppModule) LegacyQuerierHandler(legacyQuerierCdc *codec.LegacyAmino) sdk.Querier {
	return NewQuerier(am.mgr, am.keybaseStore)
}

// RegisterServices registers module services.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	// types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	// types.RegisterQueryServer(cfg.QueryServer(), am.keeper)
}

func (am AppModule) NewQuerierHandler() sdk.Querier {
	return func(ctx cosmos.Context, path []string, req abci.RequestQuery) ([]byte, error) {
		return nil, nil
	}
}

// BeginBlock called when a block get proposed
func (am AppModule) BeginBlock(ctx sdk.Context, req abci.RequestBeginBlock) {
	info := req.GetLastCommitInfo()
	var existingValidators []string
	for _, v := range info.GetVotes() {
		addr := sdk.ValAddress(v.Validator.GetAddress())
		existingValidators = append(existingValidators, addr.String())
	}

	ctx.Logger().Debug("Begin Block", "height", req.Header.Height)
	version := am.mgr.GetVersion()
	if version.LT(semver.MustParse("1.96.0")) { // TODO: leave only the else path after hard fork
		if version.LT(semver.MustParse("1.90.0")) {
			_ = am.mgr.Keeper().GetLowestActiveVersion(ctx)
		}

		localVer := semver.MustParse(constants.SWVersion.String())
		if version.Major > localVer.Major || version.Minor > localVer.Minor {
			panic(fmt.Sprintf("Unsupported Version: update your binary (your version: %s, network consensus version: %s)", constants.SWVersion.String(), version.String()))
		}

		// Does a kvstore migration
		smgr := newStoreMgr(am.mgr)
		if err := smgr.Iterator(ctx); err != nil {
			os.Exit(10) // halt the chain if unsuccessful
		}

		am.mgr.Keeper().ClearObservingAddresses(ctx)
		if err := am.mgr.BeginBlock(ctx); err != nil {
			ctx.Logger().Error("fail to get managers", "error", err)
		}
	} else {
		// Check/Update the network version before checking the local version or checking whether to do a new-version store migration
		if err := am.mgr.BeginBlock(ctx); err != nil {
			ctx.Logger().Error("fail to get managers", "error", err)
		}

		localVer := semver.MustParse(constants.SWVersion.String())
		if version.Major > localVer.Major || version.Minor > localVer.Minor {
			panic(fmt.Sprintf("Unsupported Version: update your binary (your version: %s, network consensus version: %s)", constants.SWVersion.String(), version.String()))
		}

		// Does a kvstore migration
		smgr := newStoreMgr(am.mgr)
		if err := smgr.Iterator(ctx); err != nil {
			os.Exit(10) // halt the chain if unsuccessful
		}

		am.mgr.Keeper().ClearObservingAddresses(ctx)
	}
	am.mgr.GasMgr().BeginBlock(am.mgr)
	if err := am.mgr.NetworkMgr().BeginBlock(ctx, am.mgr); err != nil {
		ctx.Logger().Error("fail to begin network manager", "error", err)
	}
	am.mgr.Slasher().BeginBlock(ctx, req, am.mgr.GetConstants())
	if err := am.mgr.ValidatorMgr().BeginBlock(ctx, am.mgr, existingValidators); err != nil {
		ctx.Logger().Error("Fail to begin block on validator", "error", err)
	}
}

// EndBlock called when a block get committed
func (am AppModule) EndBlock(ctx sdk.Context, req abci.RequestEndBlock) []abci.ValidatorUpdate {
	ctx.Logger().Debug("End Block", "height", req.Height)
	if am.mgr.GetVersion().LT(semver.MustParse("1.90.0")) {
		_ = am.mgr.Keeper().GetLowestActiveVersion(ctx) // TODO: remove me on hard fork
	}
	if err := am.mgr.SwapQ().EndBlock(ctx, am.mgr); err != nil {
		ctx.Logger().Error("fail to process swap queue", "error", err)
	}

	if am.mgr.GetVersion().GTE(semver.MustParse("1.98.0")) {
		if err := am.mgr.OrderBookMgr().EndBlock(ctx, am.mgr); err != nil {
			ctx.Logger().Error("fail to process order books", "error", err)
		}
	}

	// slash node accounts for not observing any accepted inbound tx
	if err := am.mgr.Slasher().LackObserving(ctx, am.mgr.GetConstants()); err != nil {
		ctx.Logger().Error("Unable to slash for lack of observing:", "error", err)
	}
	if err := am.mgr.Slasher().LackSigning(ctx, am.mgr); err != nil {
		ctx.Logger().Error("Unable to slash for lack of signing:", "error", err)
	}

	if err := am.mgr.PoolMgr().EndBlock(ctx, am.mgr); err != nil {
		ctx.Logger().Error("fail to process pools", "error", err)
	}

	am.mgr.ObMgr().EndBlock(ctx, am.mgr.Keeper())

	// update network data to account for block rewards and reward units
	if err := am.mgr.NetworkMgr().UpdateNetwork(ctx, am.mgr.GetConstants(), am.mgr.GasMgr(), am.mgr.EventMgr()); err != nil {
		ctx.Logger().Error("fail to update network data", "error", err)
	}

	if err := am.mgr.NetworkMgr().EndBlock(ctx, am.mgr); err != nil {
		ctx.Logger().Error("fail to end block for vault manager", "error", err)
	}

	validators := am.mgr.ValidatorMgr().EndBlock(ctx, am.mgr)

	// Fill up Yggdrasil vaults
	// We do this AFTER validatorMgr.EndBlock, because we don't want to send
	// funds to a yggdrasil vault that is being churned out this block.
	if err := am.mgr.YggManager().Fund(ctx, am.mgr); err != nil {
		ctx.Logger().Error("unable to fund yggdrasil", "error", err)
	}

	if err := am.mgr.TxOutStore().EndBlock(ctx, am.mgr); err != nil {
		ctx.Logger().Error("fail to process txout endblock", "error", err)
	}

	am.mgr.GasMgr().EndBlock(ctx, am.mgr.Keeper(), am.mgr.EventMgr())

	// telemetry
	if am.telemetryEnabled {
		if err := emitEndBlockTelemetry(ctx, am.mgr); err != nil {
			ctx.Logger().Error("unable to emit end block telemetry", "error", err)
		}
	}

	return validators
}

// InitGenesis initialise genesis
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) []abci.ValidatorUpdate {
	var genState GenesisState
	ModuleCdc.MustUnmarshalJSON(data, &genState)
	return InitGenesis(ctx, am.mgr.Keeper(), genState)
}

// ExportGenesis export genesis
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := ExportGenesis(ctx, am.mgr.Keeper())
	return ModuleCdc.MustMarshalJSON(&gs)
}
