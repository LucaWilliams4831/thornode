package keeper

import (
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

const (
	ModuleName  = types.ModuleName
	ReserveName = types.ReserveName
	AsgardName  = types.AsgardName
	BondName    = types.BondName
	RouterKey   = types.RouterKey
	StoreKey    = types.StoreKey

	ActiveVault = types.VaultStatus_ActiveVault

	// Node status
	NodeActive = types.NodeStatus_Active
)

var (
	NewPool                = types.NewPool
	NewJail                = types.NewJail
	ModuleCdc              = types.ModuleCdc
	RegisterCodec          = types.RegisterCodec
	GetRandomVault         = types.GetRandomVault
	GetRandomValidatorNode = types.GetRandomValidatorNode
	GetRandomBNBAddress    = types.GetRandomBNBAddress
	GetRandomTxHash        = types.GetRandomTxHash
	GetRandomBech32Addr    = types.GetRandomBech32Addr
	GetRandomPubKey        = types.GetRandomPubKey
)

type (
	MsgSwap = types.MsgSwap

	PoolStatus               = types.PoolStatus
	Pool                     = types.Pool
	Pools                    = types.Pools
	LiquidityProvider        = types.LiquidityProvider
	Loan                     = types.Loan
	ObservedTxVoter          = types.ObservedTxVoter
	BanVoter                 = types.BanVoter
	ErrataTxVoter            = types.ErrataTxVoter
	TssVoter                 = types.TssVoter
	TssKeysignFailVoter      = types.TssKeysignFailVoter
	TssKeygenMetric          = types.TssKeygenMetric
	TssKeysignMetric         = types.TssKeysignMetric
	TxOutItem                = types.TxOutItem
	TxOut                    = types.TxOut
	KeygenBlock              = types.KeygenBlock
	ReserveContributors      = types.ReserveContributors
	Vault                    = types.Vault
	Vaults                   = types.Vaults
	Jail                     = types.Jail
	BondProvider             = types.BondProvider
	BondProviders            = types.BondProviders
	NodeAccount              = types.NodeAccount
	NodeAccounts             = types.NodeAccounts
	NodeMimirs               = types.NodeMimirs
	NodeStatus               = types.NodeStatus
	Network                  = types.Network
	ProtocolOwnedLiquidity   = types.ProtocolOwnedLiquidity
	VaultStatus              = types.VaultStatus
	NetworkFee               = types.NetworkFee
	ObservedNetworkFeeVoter  = types.ObservedNetworkFeeVoter
	RagnarokWithdrawPosition = types.RagnarokWithdrawPosition
	ChainContract            = types.ChainContract
	SolvencyVoter            = types.SolvencyVoter
	THORName                 = types.THORName
)
