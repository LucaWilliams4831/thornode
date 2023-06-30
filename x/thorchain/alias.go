package thorchain

import (
	"github.com/cosmos/cosmos-sdk/codec"

	"gitlab.com/thorchain/thornode/x/thorchain/aggregators"
	mem "gitlab.com/thorchain/thornode/x/thorchain/memo"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

const (
	ModuleName       = types.ModuleName
	ReserveName      = types.ReserveName
	AsgardName       = types.AsgardName
	BondName         = types.BondName
	LendingName      = types.LendingName
	RouterKey        = types.RouterKey
	StoreKey         = types.StoreKey
	DefaultCodespace = types.DefaultCodespace

	// pool status
	PoolAvailable = types.PoolStatus_Available
	PoolStaged    = types.PoolStatus_Staged
	PoolSuspended = types.PoolStatus_Suspended

	// Admin config keys
	MaxWithdrawBasisPoints = types.MaxWithdrawBasisPoints

	// Vaults
	AsgardVault    = types.VaultType_AsgardVault
	YggdrasilVault = types.VaultType_YggdrasilVault
	UnknownVault   = types.VaultType_UnknownVault
	ActiveVault    = types.VaultStatus_ActiveVault
	InactiveVault  = types.VaultStatus_InactiveVault
	RetiringVault  = types.VaultStatus_RetiringVault
	InitVault      = types.VaultStatus_InitVault

	// Node status
	NodeActive      = types.NodeStatus_Active
	NodeWhiteListed = types.NodeStatus_Whitelisted
	NodeDisabled    = types.NodeStatus_Disabled
	NodeReady       = types.NodeStatus_Ready
	NodeStandby     = types.NodeStatus_Standby
	NodeUnknown     = types.NodeStatus_Unknown

	// Node type
	NodeTypeUnknown   = types.NodeType_TypeUnknown
	NodeTypeValidator = types.NodeType_TypeValidator
	NodeTypeVault     = types.NodeType_TypeVault

	// Bond type
	BondPaid     = types.BondType_bond_paid
	BondReturned = types.BondType_bond_returned
	BondCost     = types.BondType_bond_cost
	BondReward   = types.BondType_bond_reward
	AsgardKeygen = types.KeygenType_AsgardKeygen

	// Bond type
	AddPendingLiquidity      = types.PendingLiquidityType_add
	WithdrawPendingLiquidity = types.PendingLiquidityType_withdraw

	// Order Type
	MarketOrder = types.OrderType_market
	LimitOrder  = types.OrderType_limit

	// Mint/Burn type
	MintSupplyType = types.MintBurnSupplyType_mint
	BurnSupplyType = types.MintBurnSupplyType_burn

	// Memos
	TxSwap            = mem.TxSwap
	TxLimitOrder      = mem.TxLimitOrder
	TxAdd             = mem.TxAdd
	TxBond            = mem.TxBond
	TxYggdrasilFund   = mem.TxYggdrasilFund
	TxYggdrasilReturn = mem.TxYggdrasilReturn
	TxMigrate         = mem.TxMigrate
	TxRagnarok        = mem.TxRagnarok
	TxReserve         = mem.TxReserve
	TxOutbound        = mem.TxOutbound
	TxRefund          = mem.TxRefund
	TxUnBond          = mem.TxUnbond
	TxLeave           = mem.TxLeave
	TxWithdraw        = mem.TxWithdraw
	TxTHORName        = mem.TxTHORName
	TxLoanOpen        = mem.TxLoanOpen
	TxLoanRepayment   = mem.TxLoanRepayment
)

var (
	NewPool                        = types.NewPool
	NewNetwork                     = types.NewNetwork
	NewProtocolOwnedLiquidity      = types.NewProtocolOwnedLiquidity
	NewObservedTx                  = types.NewObservedTx
	NewTssVoter                    = types.NewTssVoter
	NewBanVoter                    = types.NewBanVoter
	NewErrataTxVoter               = types.NewErrataTxVoter
	NewObservedTxVoter             = types.NewObservedTxVoter
	NewMsgLoanOpen                 = types.NewMsgLoanOpen
	NewMsgLoanRepayment            = types.NewMsgLoanRepayment
	NewMsgMimir                    = types.NewMsgMimir
	NewMsgNodePauseChain           = types.NewMsgNodePauseChain
	NewMsgDeposit                  = types.NewMsgDeposit
	NewMsgTssPool                  = types.NewMsgTssPool
	NewMsgTssKeysignFail           = types.NewMsgTssKeysignFail
	NewMsgObservedTxIn             = types.NewMsgObservedTxIn
	NewMsgObservedTxOut            = types.NewMsgObservedTxOut
	NewMsgNoOp                     = types.NewMsgNoOp
	NewMsgConsolidate              = types.NewMsgConsolidate
	NewMsgDonate                   = types.NewMsgDonate
	NewMsgAddLiquidity             = types.NewMsgAddLiquidity
	NewMsgWithdrawLiquidity        = types.NewMsgWithdrawLiquidity
	NewMsgSwap                     = types.NewMsgSwap
	NewKeygen                      = types.NewKeygen
	NewKeygenBlock                 = types.NewKeygenBlock
	NewMsgSetNodeKeys              = types.NewMsgSetNodeKeys
	NewMsgManageTHORName           = types.NewMsgManageTHORName
	NewTxOut                       = types.NewTxOut
	NewEventRewards                = types.NewEventRewards
	NewEventPool                   = types.NewEventPool
	NewEventDonate                 = types.NewEventDonate
	NewEventSwap                   = types.NewEventSwap
	NewEventLimitOrder             = types.NewEventLimitOrder
	NewEventAddLiquidity           = types.NewEventAddLiquidity
	NewEventWithdraw               = types.NewEventWithdraw
	NewEventRefund                 = types.NewEventRefund
	NewEventBond                   = types.NewEventBond
	NewEventSwitch                 = types.NewEventSwitch
	NewEventSwitchV87              = types.NewEventSwitchV87
	NewEventGas                    = types.NewEventGas
	NewEventScheduledOutbound      = types.NewEventScheduledOutbound
	NewEventSecurity               = types.NewEventSecurity
	NewEventSlash                  = types.NewEventSlash
	NewEventSlashPoint             = types.NewEventSlashPoint
	NewEventReserve                = types.NewEventReserve
	NewEventErrata                 = types.NewEventErrata
	NewEventFee                    = types.NewEventFee
	NewEventOutbound               = types.NewEventOutbound
	NewEventSetMimir               = types.NewEventSetMimir
	NewEventSetNodeMimir           = types.NewEventSetNodeMimir
	NewEventTssKeygenMetric        = types.NewEventTssKeygenMetric
	NewEventTssKeysignMetric       = types.NewEventTssKeysignMetric
	NewEventPoolBalanceChanged     = types.NewEventPoolBalanceChanged
	NewEventPendingLiquidity       = types.NewEventPendingLiquidity
	NewEventTHORName               = types.NewEventTHORName
	NewEventMintBurn               = types.NewEventMintBurn
	NewEventVersion                = types.NewEventVersion
	NewEventLoanOpen               = types.NewEventLoanOpen
	NewEventLoanRepayment          = types.NewEventLoanRepayment
	NewPoolMod                     = types.NewPoolMod
	NewMsgRefundTx                 = types.NewMsgRefundTx
	NewMsgOutboundTx               = types.NewMsgOutboundTx
	NewMsgMigrate                  = types.NewMsgMigrate
	NewMsgRagnarok                 = types.NewMsgRagnarok
	NewQueryLiquidityProvider      = types.NewQueryLiquidityProvider
	NewQueryNodeAccount            = types.NewQueryNodeAccount
	NewQueryObservedTx             = types.NewQueryObservedTx
	NewQueryPool                   = types.NewQueryPool
	NewQuerySaver                  = types.NewQuerySaver
	NewQueryTxOutItem              = types.NewQueryTxOutItem
	NewQueryTxSigners              = types.NewQueryTxSigners
	NewQueryTxStages               = types.NewQueryTxStages
	NewQueryTxStatus               = types.NewQueryTxStatus
	ModuleCdc                      = types.ModuleCdc
	RegisterCodec                  = types.RegisterCodec
	RegisterInterfaces             = types.RegisterInterfaces
	NewBondProviders               = types.NewBondProviders
	NewBondProvider                = types.NewBondProvider
	NewNodeAccount                 = types.NewNodeAccount
	NewVault                       = types.NewVault
	NewReserveContributor          = types.NewReserveContributor
	NewMsgYggdrasil                = types.NewMsgYggdrasil
	NewMsgReserveContributor       = types.NewMsgReserveContributor
	NewMsgBond                     = types.NewMsgBond
	NewMsgUnBond                   = types.NewMsgUnBond
	NewMsgErrataTx                 = types.NewMsgErrataTx
	NewMsgBan                      = types.NewMsgBan
	NewMsgSwitch                   = types.NewMsgSwitch
	NewMsgLeave                    = types.NewMsgLeave
	NewMsgSetVersion               = types.NewMsgSetVersion
	NewMsgSetIPAddress             = types.NewMsgSetIPAddress
	NewMsgNetworkFee               = types.NewMsgNetworkFee
	NewNetworkFee                  = types.NewNetworkFee
	NewTHORName                    = types.NewTHORName
	NewLoan                        = types.NewLoan
	GetPoolStatus                  = types.GetPoolStatus
	GetRandomVault                 = types.GetRandomVault
	GetRandomYggVault              = types.GetRandomYggVault
	GetRandomTx                    = types.GetRandomTx
	GetRandomObservedTx            = types.GetRandomObservedTx
	GetRandomTxOutItem             = types.GetRandomTxOutItem
	GetRandomObservedTxVoter       = types.GetRandomObservedTxVoter
	GetRandomValidatorNode         = types.GetRandomValidatorNode
	GetRandomVaultNode             = types.GetRandomVaultNode
	GetRandomTHORAddress           = types.GetRandomTHORAddress
	GetRandomRUNEAddress           = types.GetRandomRUNEAddress
	GetRandomBNBAddress            = types.GetRandomBNBAddress
	GetRandomTERRAAddress          = types.GetRandomTERRAAddress
	GetRandomGAIAAddress           = types.GetRandomGAIAAddress
	GetRandomBTCAddress            = types.GetRandomBTCAddress
	GetRandomLTCAddress            = types.GetRandomLTCAddress
	GetRandomTxHash                = types.GetRandomTxHash
	GetRandomBech32Addr            = types.GetRandomBech32Addr
	GetRandomBech32ConsensusPubKey = types.GetRandomBech32ConsensusPubKey
	GetRandomPubKey                = types.GetRandomPubKey
	GetRandomPubKeySet             = types.GetRandomPubKeySet
	GetCurrentVersion              = types.GetCurrentVersion
	SetupConfigForTest             = types.SetupConfigForTest
	HasSimpleMajority              = types.HasSimpleMajority
	HasSuperMajority               = types.HasSuperMajority
	HasMinority                    = types.HasMinority
	DefaultGenesis                 = types.DefaultGenesis
	NewSolvencyVoter               = types.NewSolvencyVoter
	NewMsgSolvency                 = types.NewMsgSolvency

	// Memo
	ParseMemo              = mem.ParseMemo
	ParseMemoWithTHORNames = mem.ParseMemoWithTHORNames
	FetchAddress           = mem.FetchAddress
	NewRefundMemo          = mem.NewRefundMemo
	NewOutboundMemo        = mem.NewOutboundMemo
	NewRagnarokMemo        = mem.NewRagnarokMemo
	NewYggdrasilReturn     = mem.NewYggdrasilReturn
	NewYggdrasilFund       = mem.NewYggdrasilFund
	NewMigrateMemo         = mem.NewMigrateMemo

	FetchDexAggregator = aggregators.FetchDexAggregator
)

type (
	MsgSend                        = types.MsgSend
	MsgDeposit                     = types.MsgDeposit
	MsgSwitch                      = types.MsgSwitch
	MsgBond                        = types.MsgBond
	MsgUnBond                      = types.MsgUnBond
	MsgNoOp                        = types.MsgNoOp
	MsgConsolidate                 = types.MsgConsolidate
	MsgDonate                      = types.MsgDonate
	MsgWithdrawLiquidity           = types.MsgWithdrawLiquidity
	MsgAddLiquidity                = types.MsgAddLiquidity
	MsgOutboundTx                  = types.MsgOutboundTx
	MsgMimir                       = types.MsgMimir
	MsgNodePauseChain              = types.MsgNodePauseChain
	MsgMigrate                     = types.MsgMigrate
	MsgRagnarok                    = types.MsgRagnarok
	MsgRefundTx                    = types.MsgRefundTx
	MsgErrataTx                    = types.MsgErrataTx
	MsgBan                         = types.MsgBan
	MsgSwap                        = types.MsgSwap
	MsgSetVersion                  = types.MsgSetVersion
	MsgSetIPAddress                = types.MsgSetIPAddress
	MsgSetNodeKeys                 = types.MsgSetNodeKeys
	MsgLeave                       = types.MsgLeave
	MsgReserveContributor          = types.MsgReserveContributor
	MsgYggdrasil                   = types.MsgYggdrasil
	MsgObservedTxIn                = types.MsgObservedTxIn
	MsgObservedTxOut               = types.MsgObservedTxOut
	MsgTssPool                     = types.MsgTssPool
	MsgTssKeysignFail              = types.MsgTssKeysignFail
	MsgNetworkFee                  = types.MsgNetworkFee
	MsgManageTHORName              = types.MsgManageTHORName
	MsgSolvency                    = types.MsgSolvency
	MsgLoanOpen                    = types.MsgLoanOpen
	MsgLoanRepayment               = types.MsgLoanRepayment
	QueryVersion                   = types.QueryVersion
	QueryQueue                     = types.QueryQueue
	QueryNodeAccountPreflightCheck = types.QueryNodeAccountPreflightCheck
	QueryKeygenBlock               = types.QueryKeygenBlock
	QueryResLastBlockHeights       = types.QueryResLastBlockHeights
	QueryKeysign                   = types.QueryKeysign
	QueryYggdrasilVaults           = types.QueryYggdrasilVaults
	QueryVaultsPubKeys             = types.QueryVaultsPubKeys
	QueryVaultPubKeyContract       = types.QueryVaultPubKeyContract
	QueryNodeAccount               = types.QueryNodeAccount
	QueryObservedTx                = types.QueryObservedTx
	QueryPool                      = types.QueryPool
	QueryTxOutItem                 = types.QueryTxOutItem
	QueryTxSigners                 = types.QueryTxSigners
	QueryTxStages                  = types.QueryTxStages
	QueryTxStatus                  = types.QueryTxStatus
	QuerySaver                     = types.QuerySaver
	QueryChainAddress              = types.QueryChainAddress
	PoolStatus                     = types.PoolStatus
	Pool                           = types.Pool
	Pools                          = types.Pools
	LiquidityProvider              = types.LiquidityProvider
	LiquidityProviders             = types.LiquidityProviders
	Loan                           = types.Loan
	Loans                          = types.Loans
	ObservedTxs                    = types.ObservedTxs
	ObservedTx                     = types.ObservedTx
	ObservedTxVoter                = types.ObservedTxVoter
	ObservedTxVoters               = types.ObservedTxVoters
	BanVoter                       = types.BanVoter
	ErrataTxVoter                  = types.ErrataTxVoter
	TssVoter                       = types.TssVoter
	TssKeysignFailVoter            = types.TssKeysignFailVoter
	TxOutItem                      = types.TxOutItem
	TxOut                          = types.TxOut
	Keygen                         = types.Keygen
	KeygenBlock                    = types.KeygenBlock
	EventSwap                      = types.EventSwap
	EventAddLiquidity              = types.EventAddLiquidity
	EventWithdraw                  = types.EventWithdraw
	EventDonate                    = types.EventDonate
	EventRewards                   = types.EventRewards
	EventErrata                    = types.EventErrata
	EventReserve                   = types.EventReserve
	EventLoanOpen                  = types.EventLoanOpen
	EventLoanRepayment             = types.EventLoanRepayment
	PoolAmt                        = types.PoolAmt
	PoolMod                        = types.PoolMod
	PoolMods                       = types.PoolMods
	ReserveContributor             = types.ReserveContributor
	ReserveContributors            = types.ReserveContributors
	Vault                          = types.Vault
	Vaults                         = types.Vaults
	NodeAccount                    = types.NodeAccount
	NodeAccounts                   = types.NodeAccounts
	NodeStatus                     = types.NodeStatus
	BondProviders                  = types.BondProviders
	BondProvider                   = types.BondProvider
	Network                        = types.Network
	ProtocolOwnedLiquidity         = types.ProtocolOwnedLiquidity
	VaultStatus                    = types.VaultStatus
	GasPool                        = types.GasPool
	EventGas                       = types.EventGas
	EventPool                      = types.EventPool
	EventRefund                    = types.EventRefund
	EventBond                      = types.EventBond
	EventFee                       = types.EventFee
	EventSlash                     = types.EventSlash
	EventOutbound                  = types.EventOutbound
	NetworkFee                     = types.NetworkFee
	ObservedNetworkFeeVoter        = types.ObservedNetworkFeeVoter
	Jail                           = types.Jail
	RagnarokWithdrawPosition       = types.RagnarokWithdrawPosition
	ChainContract                  = types.ChainContract
	Blame                          = types.Blame
	Node                           = types.Node
	THORName                       = types.THORName
	THORNameAlias                  = types.THORNameAlias
	NodeMimir                      = types.NodeMimir
	NodeMimirs                     = types.NodeMimirs

	// Memo
	SwapMemo              = mem.SwapMemo
	AddLiquidityMemo      = mem.AddLiquidityMemo
	WithdrawLiquidityMemo = mem.WithdrawLiquidityMemo
	DonateMemo            = mem.DonateMemo
	RefundMemo            = mem.RefundMemo
	MigrateMemo           = mem.MigrateMemo
	RagnarokMemo          = mem.RagnarokMemo
	BondMemo              = mem.BondMemo
	UnbondMemo            = mem.UnbondMemo
	OutboundMemo          = mem.OutboundMemo
	LeaveMemo             = mem.LeaveMemo
	YggdrasilFundMemo     = mem.YggdrasilFundMemo
	YggdrasilReturnMemo   = mem.YggdrasilReturnMemo
	ReserveMemo           = mem.ReserveMemo
	SwitchMemo            = mem.SwitchMemo
	NoOpMemo              = mem.NoOpMemo
	ConsolidateMemo       = mem.ConsolidateMemo
	ManageTHORNameMemo    = mem.ManageTHORNameMemo

	// Proto
	ProtoStrings = types.ProtoStrings

	LoanOpenMemo      = mem.LoanOpenMemo
	LoanRepaymentMemo = mem.LoanRepaymentMemo
)

var _ codec.ProtoMarshaler = &types.LiquidityProvider{}
