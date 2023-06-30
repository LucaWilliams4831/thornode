package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

var (
	amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewAminoCodec(amino)
)

func init() {
	RegisterCodec(amino)
}

// RegisterCodec register the msg types for amino
func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgSwap{}, "thorchain/Swap", nil)
	cdc.RegisterConcrete(&MsgTssPool{}, "thorchain/TssPool", nil)
	cdc.RegisterConcrete(&MsgTssKeysignFail{}, "thorchain/TssKeysignFail", nil)
	cdc.RegisterConcrete(&MsgAddLiquidity{}, "thorchain/AddLiquidity", nil)
	cdc.RegisterConcrete(&MsgWithdrawLiquidity{}, "thorchain/WidthdrawLiquidity", nil)
	cdc.RegisterConcrete(&MsgObservedTxIn{}, "thorchain/ObservedTxIn", nil)
	cdc.RegisterConcrete(&MsgObservedTxOut{}, "thorchain/ObservedTxOut", nil)
	cdc.RegisterConcrete(&MsgDonate{}, "thorchain/MsgDonate", nil)
	cdc.RegisterConcrete(&MsgBond{}, "thorchain/MsgBond", nil)
	cdc.RegisterConcrete(&MsgUnBond{}, "thorchain/MsgUnBond", nil)
	cdc.RegisterConcrete(&MsgLeave{}, "thorchain/MsgLeave", nil)
	cdc.RegisterConcrete(&MsgNoOp{}, "thorchain/MsgNoOp", nil)
	cdc.RegisterConcrete(&MsgOutboundTx{}, "thorchain/MsgOutboundTx", nil)
	cdc.RegisterConcrete(&MsgSetVersion{}, "thorchain/MsgSetVersion", nil)
	cdc.RegisterConcrete(&MsgSetNodeKeys{}, "thorchain/MsgSetNodeKeys", nil)
	cdc.RegisterConcrete(&MsgSetIPAddress{}, "thorchain/MsgSetIPAddress", nil)
	cdc.RegisterConcrete(&MsgYggdrasil{}, "thorchain/MsgYggdrasil", nil)
	cdc.RegisterConcrete(&MsgReserveContributor{}, "thorchain/MsgReserveContributor", nil)
	cdc.RegisterConcrete(&MsgErrataTx{}, "thorchain/MsgErrataTx", nil)
	cdc.RegisterConcrete(&MsgBan{}, "thorchain/MsgBan", nil)
	cdc.RegisterConcrete(&MsgSwitch{}, "thorchain/MsgSwitch", nil)
	cdc.RegisterConcrete(&MsgMimir{}, "thorchain/MsgMimir", nil)
	cdc.RegisterConcrete(&MsgDeposit{}, "thorchain/MsgDeposit", nil)
	cdc.RegisterConcrete(&MsgNetworkFee{}, "thorchain/MsgNetworkFee", nil)
	cdc.RegisterConcrete(&MsgMigrate{}, "thorchain/MsgMigrate", nil)
	cdc.RegisterConcrete(&MsgRagnarok{}, "thorchain/MsgRagnarok", nil)
	cdc.RegisterConcrete(&MsgRefundTx{}, "thorchain/MsgRefundTx", nil)
	cdc.RegisterConcrete(&MsgSend{}, "thorchain/MsgSend", nil)
	cdc.RegisterConcrete(&MsgNodePauseChain{}, "thorchain/MsgNodePauseChain", nil)
	cdc.RegisterConcrete(&MsgSolvency{}, "thorchain/MsgSolvency", nil)
	cdc.RegisterConcrete(&MsgManageTHORName{}, "thorchain/MsgManageTHORName", nil)
}

// RegisterInterfaces register the types
func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSwap{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgTssPool{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgTssKeysignFail{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgAddLiquidity{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgWithdrawLiquidity{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgObservedTxIn{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgObservedTxOut{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgDonate{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgBond{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgUnBond{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgLeave{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgNoOp{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgOutboundTx{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSetVersion{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSetNodeKeys{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSetIPAddress{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgYggdrasil{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgReserveContributor{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgErrataTx{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgBan{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSwitch{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgMimir{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgDeposit{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgNetworkFee{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgMigrate{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgRagnarok{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgRefundTx{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSend{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgNodePauseChain{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgManageTHORName{})
	registry.RegisterImplementations((*cosmos.Msg)(nil), &MsgSolvency{})
}
