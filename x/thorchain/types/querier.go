package types

import (
	"errors"
	"fmt"
	"strings"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	gitlab_com_thorchain_thornode_common "gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// QueryResLastBlockHeights used to return the block height query
type QueryResLastBlockHeights struct {
	Chain            common.Chain `json:"chain"`
	LastChainHeight  int64        `json:"last_observed_in"`
	LastSignedHeight int64        `json:"last_signed_out"`
	Thorchain        int64        `json:"thorchain"`
}

// String implement fmt.Stringer return a string representation of QueryResLastBlockHeights
func (h QueryResLastBlockHeights) String() string {
	return fmt.Sprintf("Chain: %d, Signed: %d, THORChain: %d", h.LastChainHeight, h.LastSignedHeight, h.Thorchain)
}

// QueryQueue a struct store the total outstanding out items
type QueryQueue struct {
	Swap                   int64       `json:"swap"`
	Outbound               int64       `json:"outbound"`
	Internal               int64       `json:"internal"`
	ScheduledOutboundValue cosmos.Uint `json:"scheduled_outbound_value"`
}

// String implement fmt.Stringer
func (h QueryQueue) String() string {
	return fmt.Sprintf("Swap: %d, Outboud: %d", h.Swap, h.Outbound)
}

// QueryNodeAccountPreflightCheck is structure to hold all the information need to return to client
// include current node status , and whether it might get churned in next
type QueryNodeAccountPreflightCheck struct {
	Status      NodeStatus `json:"status"`
	Description string     `json:"reason"`
	Code        int        `json:"code"`
}

// String implement fmt.Stringer
func (n QueryNodeAccountPreflightCheck) String() string {
	sb := strings.Builder{}
	sb.WriteString("Result Status:" + n.Status.String() + "\n")
	sb.WriteString("Description:" + n.Description + "\n")
	return sb.String()
}

// QueryKeygenBlock query keygen, displays signed keygen requests
type QueryKeygenBlock struct {
	KeygenBlock KeygenBlock `json:"keygen_block"`
	Signature   string      `json:"signature"`
}

// String implement fmt.Stringer
func (n QueryKeygenBlock) String() string {
	return n.KeygenBlock.String()
}

// QueryKeysign query keysign result
type QueryKeysign struct {
	Keysign   TxOut  `json:"keysign"`
	Signature string `json:"signature"`
}

// QueryYggdrasilVaults query yggdrasil vault result
type QueryYggdrasilVaults struct {
	BlockHeight           int64                                       `json:"block_height,omitempty"`
	PubKey                gitlab_com_thorchain_thornode_common.PubKey `json:"pub_key,omitempty"`
	Coins                 gitlab_com_thorchain_thornode_common.Coins  `json:"coins"`
	Type                  VaultType                                   `json:"type,omitempty"`
	StatusSince           int64                                       `json:"status_since,omitempty"`
	Membership            []string                                    `json:"membership,omitempty"`
	Chains                []string                                    `json:"chains,omitempty"`
	InboundTxCount        int64                                       `json:"inbound_tx_count,omitempty"`
	OutboundTxCount       int64                                       `json:"outbound_tx_count,omitempty"`
	PendingTxBlockHeights []int64                                     `json:"pending_tx_block_heights,omitempty"`
	Routers               []ChainContract                             `json:"routers"`
	Status                NodeStatus                                  `json:"status"`
	Bond                  cosmos.Uint                                 `json:"bond"`
	TotalValue            cosmos.Uint                                 `json:"total_value"`
	Addresses             []QueryChainAddress                         `json:"addresses"`
}

// QueryVaultResp used represent the informat return to client for query asgard
type QueryVaultResp struct {
	BlockHeight           int64                                       `json:"block_height,omitempty"`
	PubKey                gitlab_com_thorchain_thornode_common.PubKey `json:"pub_key,omitempty"`
	Coins                 gitlab_com_thorchain_thornode_common.Coins  `json:"coins"`
	Type                  VaultType                                   `json:"type,omitempty"`
	Status                VaultStatus                                 `json:"status,omitempty"`
	StatusSince           int64                                       `json:"status_since,omitempty"`
	Membership            []string                                    `json:"membership,omitempty"`
	Chains                []string                                    `json:"chains,omitempty"`
	InboundTxCount        int64                                       `json:"inbound_tx_count,omitempty"`
	OutboundTxCount       int64                                       `json:"outbound_tx_count,omitempty"`
	PendingTxBlockHeights []int64                                     `json:"pending_tx_block_heights,omitempty"`
	Routers               []ChainContract                             `json:"routers"`
	Addresses             []QueryChainAddress                         `json:"addresses"`
	Frozen                []string                                    `json:"frozen,omitempty"`
}

type QueryVersion struct {
	Current         semver.Version `json:"current"`
	Next            semver.Version `json:"next"`
	NextSinceHeight int64          `json:"next_since_height,omitempty"` // Don't display this for heights when not yet stored (0).
	Querier         semver.Version `json:"querier"`
}

type QueryChainAddress struct {
	Chain   common.Chain   `json:"chain"`
	Address common.Address `json:"address"`
}

// QueryChainHeight chain height
type QueryChainHeight struct {
	Chain  common.Chain `json:"chain"`
	Height int64        `json:"height"`
}

// QueryLiquidityProvider holds all the information related to a liquidity provider
type QueryLiquidityProvider struct {
	Asset              common.Asset   `json:"asset"`
	RuneAddress        common.Address `json:"rune_address,omitempty"`
	AssetAddress       common.Address `json:"asset_address,omitempty"`
	LastAddHeight      int64          `json:"last_add_height,omitempty"`
	LastWithdrawHeight int64          `json:"last_withdraw_height,omitempty"`
	Units              cosmos.Uint    `json:"units"`
	PendingRune        cosmos.Uint    `json:"pending_rune"`
	PendingAsset       cosmos.Uint    `json:"pending_asset"`
	PendingTxId        string         `json:"pending_tx_id,omitempty"`
	RuneDepositValue   cosmos.Uint    `json:"rune_deposit_value"`
	AssetDepositValue  cosmos.Uint    `json:"asset_deposit_value"`
	RuneRedeemValue    cosmos.Uint    `json:"rune_redeem_value"`
	AssetRedeemValue   cosmos.Uint    `json:"asset_redeem_value"`
	LuviDepositValue   cosmos.Uint    `json:"luvi_deposit_value"`
	LuviRedeemValue    cosmos.Uint    `json:"luvi_redeem_value"`
	LuviGrowthPct      cosmos.Dec     `json:"luvi_growth_pct"`
}

// NewQueryLiquidityProvider creates a new QueryLiquidityProvider based on the given liquidity provider and pool
func NewQueryLiquidityProvider(lp LiquidityProvider, pool Pool, synthSupply cosmos.Uint, version semver.Version) QueryLiquidityProvider {
	_, runeRedeemValue := lp.GetRuneRedeemValue(version, pool, synthSupply)
	_, assetRedeemValue := lp.GetAssetRedeemValue(version, pool, synthSupply)
	_, luviDepositValue := lp.GetLuviDepositValue(pool)
	_, luviRedeemValue := lp.GetLuviRedeemValue(runeRedeemValue, assetRedeemValue)

	lgp := cosmos.NewDec(0)
	if !luviDepositValue.IsZero() {
		ldv := cosmos.NewDec(luviDepositValue.BigInt().Int64())
		lrv := cosmos.NewDec(luviRedeemValue.BigInt().Int64())
		lgp = lrv.Sub(ldv)
		lgp = lgp.Quo(ldv)
	}

	return QueryLiquidityProvider{
		Asset:              lp.Asset.GetLayer1Asset(),
		AssetAddress:       lp.AssetAddress,
		RuneAddress:        lp.RuneAddress,
		LastAddHeight:      lp.LastAddHeight,
		LastWithdrawHeight: lp.LastWithdrawHeight,
		PendingAsset:       lp.PendingAsset,
		PendingRune:        lp.PendingRune,
		PendingTxId:        lp.PendingTxID.String(),
		Units:              lp.Units,
		AssetDepositValue:  lp.AssetDepositValue,
		RuneDepositValue:   lp.RuneDepositValue,
		RuneRedeemValue:    runeRedeemValue,
		AssetRedeemValue:   assetRedeemValue,
		LuviRedeemValue:    luviRedeemValue,
		LuviDepositValue:   luviDepositValue,
		LuviGrowthPct:      lgp,
	}
}

// QueryNodeAccount hold all the information related to node account
type QueryNodeAccount struct {
	NodeAddress         cosmos.AccAddress              `json:"node_address"`
	Status              NodeStatus                     `json:"status"`
	PubKeySet           common.PubKeySet               `json:"pub_key_set"`
	ValidatorConsPubKey string                         `json:"validator_cons_pub_key"`
	PeerID              string                         `json:"peer_id"`
	ActiveBlockHeight   int64                          `json:"active_block_height"`
	StatusSince         int64                          `json:"status_since"`
	NodeOperatorAddress common.Address                 `json:"node_operator_address"`
	TotalBond           cosmos.Uint                    `json:"total_bond"`
	BondProviders       BondProviders                  `json:"bond_providers"`
	SignerMembership    common.PubKeys                 `json:"signer_membership"`
	RequestedToLeave    bool                           `json:"requested_to_leave"`
	ForcedToLeave       bool                           `json:"forced_to_leave"`
	LeaveScore          uint64                         `json:"leave_height"`
	IPAddress           string                         `json:"ip_address"`
	Version             semver.Version                 `json:"version"`
	SlashPoints         int64                          `json:"slash_points"`
	Jail                Jail                           `json:"jail"`
	CurrentAward        cosmos.Uint                    `json:"current_award"`
	ObserveChains       []QueryChainHeight             `json:"observe_chains"`
	PreflightStatus     QueryNodeAccountPreflightCheck `json:"preflight_status"`
}

// NewQueryNodeAccount create a new QueryNodeAccount based on the given node account parameter
func NewQueryNodeAccount(na NodeAccount) QueryNodeAccount {
	return QueryNodeAccount{
		NodeAddress:         na.NodeAddress,
		Status:              na.Status,
		PubKeySet:           na.PubKeySet,
		ValidatorConsPubKey: na.ValidatorConsPubKey,
		ActiveBlockHeight:   na.ActiveBlockHeight,
		StatusSince:         na.StatusSince,
		NodeOperatorAddress: na.BondAddress,
		TotalBond:           na.Bond,
		SignerMembership:    na.GetSignerMembership(),
		RequestedToLeave:    na.RequestedToLeave,
		ForcedToLeave:       na.ForcedToLeave,
		LeaveScore:          na.LeaveScore,
		IPAddress:           na.IPAddress,
		Version:             na.GetVersion(),
	}
}

// QueryObservedTx holds all the information related to the ObservedTx
type QueryObservedTx struct {
	Tx                              common.Tx     `json:"tx"`
	Status                          Status        `json:"status,omitempty"`
	OutHashes                       []string      `json:"out_hashes,omitempty"`
	BlockHeight                     int64         `json:"block_height,omitempty"`
	ExternalObservedHeight          int64         `json:"external_observed_height,omitempty"`
	Signers                         []string      `json:"signers,omitempty"`
	ObservedPubKey                  common.PubKey `json:"observed_pub_key,omitempty"`
	KeysignMs                       int64         `json:"keysign_ms,omitempty"`
	FinaliseHeight                  int64         `json:"finalise_height,omitempty"`
	ExternalConfirmationDelayHeight int64         `json:"external_confirmation_delay_height,omitempty"`
	Aggregator                      string        `json:"aggregator,omitempty"`
	AggregatorTarget                string        `json:"aggregator_target,omitempty"`
	AggregatorTargetLimit           *cosmos.Uint  `json:"aggregator_target_limit,omitempty"`
}

// NewQueryObservedTx create a new QueryObservedTx based on the given ObservedTx parameters
func NewQueryObservedTx(obTx ObservedTx) QueryObservedTx {
	return QueryObservedTx{
		Tx:                              obTx.Tx,
		Status:                          obTx.Status,
		OutHashes:                       obTx.OutHashes,
		BlockHeight:                     obTx.BlockHeight,
		ExternalObservedHeight:          obTx.BlockHeight,
		Signers:                         obTx.Signers,
		ObservedPubKey:                  obTx.ObservedPubKey,
		KeysignMs:                       obTx.KeysignMs,
		FinaliseHeight:                  obTx.FinaliseHeight,
		ExternalConfirmationDelayHeight: obTx.FinaliseHeight,
		Aggregator:                      obTx.Aggregator,
		AggregatorTarget:                obTx.AggregatorTarget,
		AggregatorTargetLimit:           obTx.AggregatorTargetLimit,
	}
}

// QueryPool holds all the information related to a pool
type QueryPool struct {
	Asset                string `json:"asset"`
	Status               string `json:"status"`
	Decimals             int64  `json:"decimals,omitempty"`
	PendingInboundAsset  string `json:"pending_inbound_asset"`
	PendingInboundRune   string `json:"pending_inbound_rune"`
	BalanceAsset         string `json:"balance_asset"`
	BalanceRune          string `json:"balance_rune"`
	PoolUnits            string `json:"pool_units"`
	LPUnits              string `json:"LP_units"`
	SynthUnits           string `json:"synth_units"`
	SynthSupply          string `json:"synth_supply"`
	SaversDepth          string `json:"savers_depth"`
	SaversUnits          string `json:"savers_units"`
	SynthMintPaused      bool   `json:"synth_mint_paused"`
	SynthSupplyRemaining string `json:"synth_supply_remaining"`
	LoanCollateral       string `json:"loan_collateral"`
}

// NewQueryPool creates a new QueryPool based on the given pool parameters
func NewQueryPool(pool Pool) QueryPool {
	return QueryPool{
		Asset:               pool.Asset.String(),
		Status:              pool.Status.String(),
		Decimals:            pool.Decimals,
		PendingInboundAsset: pool.PendingInboundAsset.String(),
		PendingInboundRune:  pool.PendingInboundRune.String(),
		BalanceAsset:        pool.BalanceAsset.String(),
		BalanceRune:         pool.BalanceRune.String(),
		PoolUnits:           pool.GetPoolUnits().String(),
		LPUnits:             pool.LPUnits.String(),
		SynthUnits:          pool.SynthUnits.String(),
	}
}

type QueryTxOutItem struct {
	Chain       common.Chain   `json:"chain"`
	ToAddress   common.Address `json:"to_address"`
	VaultPubKey common.PubKey  `json:"vault_pub_key,omitempty"`
	Coin        common.Coin    `json:"coin"`
	Memo        string         `json:"memo,omitempty"`
	MaxGas      common.Gas     `json:"max_gas"`
	GasRate     int64          `json:"gas_rate,omitempty"`
	InHash      common.TxID    `json:"in_hash,omitempty"`
	OutHash     common.TxID    `json:"out_hash,omitempty"`
	Height      int64          `json:"height"`
}

// NewQueryTxOutItem create a new QueryTxOutItem based on the given txout item parameter
func NewQueryTxOutItem(toi TxOutItem, height int64) QueryTxOutItem {
	return QueryTxOutItem{
		Chain:       toi.Chain,
		ToAddress:   toi.ToAddress,
		VaultPubKey: toi.VaultPubKey,
		Coin:        toi.Coin,
		Memo:        toi.Memo,
		MaxGas:      toi.MaxGas,
		GasRate:     toi.GasRate,
		InHash:      toi.InHash,
		OutHash:     toi.OutHash,
		Height:      height,
	}
}

type InboundObservedStage struct {
	Started   *bool `json:"started,omitempty"`
	Completed bool  `json:"completed"`
}

// Querier context contains the query's provided height, but not the full block context,
// so do not use BlockTime to provide a timestamp estimate.
type InboundConfirmationCountedStage struct {
	CountingStartHeight             int64        `json:"counting_start_height,omitempty"`
	Chain                           common.Chain `json:"chain,omitempty"`
	ExternalObservedHeight          int64        `json:"external_observed_height,omitempty"`
	ExternalConfirmationDelayHeight int64        `json:"external_confirmation_delay_height,omitempty"`
	RemainingConfirmationSeconds    *int64       `json:"remaining_confirmation_seconds,omitempty"`
	Completed                       bool         `json:"completed"`
}

type DoneStage struct {
	Completed bool `json:"completed"`
}

type OutboundDelayStage struct {
	RemainingDelayBlocks  *int64 `json:"remaining_delay_blocks,omitempty"`
	RemainingDelaySeconds *int64 `json:"remaining_delay_seconds,omitempty"`
	Completed             bool   `json:"completed"`
}

type OutboundSignedStage struct {
	ScheduledOutboundHeight *int64 `json:"scheduled_outbound_height,omitempty"`
	BlocksSinceScheduled    *int64 `json:"blocks_since_scheduled,omitempty"`
	Completed               bool   `json:"completed"`
}

type QueryTxStages struct {
	// Pointers so that the omitempty can recognise 'nil'.
	// Structs used for all stages for easier user looping through 'Completed' fields.
	InboundObserved            InboundObservedStage             `json:"inbound_observed"`
	InboundConfirmationCounted *InboundConfirmationCountedStage `json:"inbound_confirmation_counted,omitempty"`
	InboundFinalised           DoneStage                        `json:"inbound_finalised"`
	SwapFinalised              *DoneStage                       `json:"swap_finalised,omitempty"`
	OutboundDelay              *OutboundDelayStage              `json:"outbound_delay,omitempty"`
	OutboundSigned             *OutboundSignedStage             `json:"outbound_signed,omitempty"`
}

func NewQueryTxStages(ctx cosmos.Context, voter ObservedTxVoter, isSwap, isPending bool) QueryTxStages {
	var result QueryTxStages

	// Set the Completed state first.
	result.InboundObserved.Completed = !voter.Tx.IsEmpty()
	// Only fill in other fields if not Completed.
	if !result.InboundObserved.Completed {
		var obStart bool
		result.InboundObserved.Started = &obStart
		if len(voter.Txs) == 0 {
			obStart = false
			// Since observation not started, end directly.
			return result
		}
		obStart = true
	}

	// Current block height is relevant in the confirmation counting and outbound stages.
	currentHeight := ctx.BlockHeight()

	// Only fill in InboundConfirmationCounted when confirmation counting took place.
	if voter.Height != 0 {
		var confCount InboundConfirmationCountedStage

		// Set the Completed state first.
		confCount.Completed = !(voter.Tx.FinaliseHeight > voter.Tx.BlockHeight)

		// Only fill in other fields if not Completed.
		if !confCount.Completed {
			confCount.CountingStartHeight = voter.Height
			confCount.Chain = voter.Tx.Tx.Chain
			confCount.ExternalObservedHeight = voter.Tx.BlockHeight
			confCount.ExternalConfirmationDelayHeight = voter.Tx.FinaliseHeight

			estConfMs := voter.Tx.Tx.Chain.ApproximateBlockMilliseconds() * (confCount.ExternalConfirmationDelayHeight - confCount.ExternalObservedHeight)
			if currentHeight > confCount.CountingStartHeight {
				estConfMs -= (currentHeight - confCount.CountingStartHeight) * common.THORChain.ApproximateBlockMilliseconds()
			}
			estConfSec := estConfMs / 1000
			// Floor at 0.
			if estConfSec < 0 {
				estConfSec = 0
			}
			confCount.RemainingConfirmationSeconds = &estConfSec
		}

		result.InboundConfirmationCounted = &confCount
	}

	// InboundFinalised is always displayed, default Completed state false.
	result.InboundFinalised.Completed = (voter.FinalisedHeight != 0)

	// Whether there's an external outbound or not, show the SwapFinalised stage from the start.
	if isSwap {
		var swapFinalisedState DoneStage

		swapFinalisedState.Completed = false
		if !isPending && result.InboundFinalised.Completed {
			// Record as completed only when not pending after the inbound has already been finalised.
			swapFinalisedState.Completed = true
		}

		result.SwapFinalised = &swapFinalisedState
	}

	// Only fill ExternalOutboundDelay and ExternalOutboundKeysign for inbound transactions with an external outbound;
	// namely, transactions with an outbound_height .
	if voter.OutboundHeight == 0 {
		return result
	}

	// Only display the OutboundDelay stage when there's a delay.
	if voter.OutboundHeight > voter.FinalisedHeight {
		var outDelay OutboundDelayStage

		// Set the Completed state first.
		outDelay.Completed = (currentHeight >= voter.OutboundHeight)

		// Only fill in other fields if not Completed.
		if !outDelay.Completed {
			remainBlocks := voter.OutboundHeight - currentHeight
			outDelay.RemainingDelayBlocks = &remainBlocks

			remainSec := remainBlocks * common.THORChain.ApproximateBlockMilliseconds() / 1000
			outDelay.RemainingDelaySeconds = &remainSec
		}

		result.OutboundDelay = &outDelay
	}

	var outSigned OutboundSignedStage

	// Set the Completed state first.
	outSigned.Completed = (voter.Tx.Status != Status_incomplete)

	// Only fill in other fields if not Completed.
	if !outSigned.Completed {
		scheduledHeight := voter.OutboundHeight
		outSigned.ScheduledOutboundHeight = &scheduledHeight

		// Only fill in BlocksSinceScheduled if the outbound delay is complete.
		if currentHeight >= scheduledHeight {
			sinceScheduled := currentHeight - scheduledHeight
			outSigned.BlocksSinceScheduled = &sinceScheduled
		}
	}

	result.OutboundSigned = &outSigned

	return result
}

type QueryPlannedOutTx struct {
	Chain     common.Chain   `json:"chain"`
	ToAddress common.Address `json:"to_address"`
	Coin      common.Coin    `json:"coin"`
	Refund    bool           `json:"refund"`
}

func NewQueryPlannedOutTxs(outTxs ...TxOutItem) []QueryPlannedOutTx {
	var result []QueryPlannedOutTx
	var isRefund bool
	for _, outTx := range outTxs {
		// Assume that any outbound memo not matching the expected pattern isn't a refund memo.
		isRefund = strings.HasPrefix(outTx.Memo, "REFUND")

		result = append(result, QueryPlannedOutTx{outTx.Chain, outTx.ToAddress, outTx.Coin, isRefund})
	}

	return result
}

type QueryTxStatus struct {
	// A Tx pointer so that the omitempty can recognise 'nil'.
	Tx            *common.Tx          `json:"tx,omitempty"`
	PlannedOutTxs []QueryPlannedOutTx `json:"planned_out_txs,omitempty"`
	OutTxs        []common.Tx         `json:"out_txs,omitempty"`
	Stages        QueryTxStages       `json:"stages"`
}

func NewQueryTxStatus(ctx cosmos.Context, voter ObservedTxVoter, isSwap, isPending bool) QueryTxStatus {
	var result QueryTxStatus

	// If there's a consensus Tx, display that.
	// If not, but there's at least one observation, display the first observation's Tx.
	// If there are no observations yet, don't display a Tx (only showing the 'Observation' stage with 'Started' false).
	if !voter.Tx.Tx.IsEmpty() {
		result.Tx = &voter.Tx.Tx
	} else if len(voter.Txs) > 0 {
		result.Tx = &voter.Txs[0].Tx
	}

	// If there are no voter Actions yet, result PlannedOutTxs will stay empty and not be displayed.
	result.PlannedOutTxs = NewQueryPlannedOutTxs(voter.Actions...)

	// If there are no voter OutTxs yet, result OutTxs will stay empty and not be displayed.
	result.OutTxs = voter.OutTxs

	result.Stages = NewQueryTxStages(ctx, voter, isSwap, isPending)

	return result
}

// Valid checks whether the QueryObservedTx represents valid information
func (m *QueryObservedTx) Valid() error {
	if err := m.Tx.Valid(); err != nil {
		return err
	}
	// Memo should not be empty, but it can't be checked here, because a
	// message failed validation will be rejected by THORNode.
	// Thus THORNode can't refund customer accordingly , which will result fund lost
	if m.ExternalObservedHeight <= 0 {
		return errors.New("external observed height can't be zero")
	}
	if m.ObservedPubKey.IsEmpty() {
		return errors.New("observed pool pubkey is empty")
	}
	if m.ExternalConfirmationDelayHeight <= 0 {
		return errors.New("external confirmation delay height can't be zero")
	}
	return nil
}

type QueryTxSigners struct {
	TxID            common.TxID       `json:"tx_id,omitempty"`
	Tx              QueryObservedTx   `json:"tx"`
	Height          int64             `json:"height,omitempty"`
	Txs             []QueryObservedTx `json:"txs"`
	Actions         []TxOutItem       `json:"actions"`
	OutTxs          []common.Tx       `json:"out_txs"`
	FinalisedHeight int64             `json:"finalised_height,omitempty"`
	UpdatedVault    bool              `json:"updated_vault,omitempty"`
	Reverted        bool              `json:"reverted,omitempty"`
	OutboundHeight  int64             `json:"outbound_height,omitempty"`
}

// NewQueryTxSigners creates a new QueryTxSigners based on the given ObservedTxVoter parameters
func NewQueryTxSigners(voter ObservedTxVoter) QueryTxSigners {
	result := QueryTxSigners{
		TxID:            voter.TxID,
		Height:          voter.Height,
		Actions:         voter.Actions,
		OutTxs:          voter.OutTxs,
		FinalisedHeight: voter.FinalisedHeight,
		UpdatedVault:    voter.UpdatedVault,
		Reverted:        voter.Reverted,
		OutboundHeight:  voter.OutboundHeight,
	}

	result.Tx = NewQueryObservedTx(voter.Tx)

	for _, obTx := range voter.Txs {
		result.Txs = append(result.Txs, NewQueryObservedTx(obTx))
	}

	return result
}

// Valid checks whether the QueryTxSigners represents valid information
func (m *QueryTxSigners) Valid() error {
	if m.TxID.IsEmpty() {
		return errors.New("cannot have an empty tx id")
	}

	// check all other normal tx
	for _, in := range m.Txs {
		if err := in.Valid(); err != nil {
			return err
		}
	}

	return nil
}

// QuerySaver holds all the information related to a saver
type QuerySaver struct {
	Asset              common.Asset   `json:"asset"`
	AssetAddress       common.Address `json:"asset_address"`
	LastAddHeight      int64          `json:"last_add_height,omitempty"`
	LastWithdrawHeight int64          `json:"last_withdraw_height,omitempty"`
	Units              cosmos.Uint    `json:"units"`
	AssetDepositValue  cosmos.Uint    `json:"asset_deposit_value"`
	AssetRedeemValue   cosmos.Uint    `json:"asset_redeem_value"`
	GrowthPct          cosmos.Dec     `json:"growth_pct"`
}

// NewQuerySaver creates a new QuerySaver based on the given liquidity provider parameters and pool
func NewQuerySaver(lp LiquidityProvider, pool Pool) QuerySaver {
	assetRedeemableValue := lp.GetSaversAssetRedeemValue(pool)

	gp := cosmos.NewDec(0)
	if !lp.AssetDepositValue.IsZero() {
		adv := cosmos.NewDec(lp.AssetDepositValue.BigInt().Int64())
		arv := cosmos.NewDec(assetRedeemableValue.BigInt().Int64())
		gp = arv.Sub(adv)
		gp = gp.Quo(adv)
	}

	return QuerySaver{
		Asset:              lp.Asset.GetLayer1Asset(),
		AssetAddress:       lp.AssetAddress,
		LastAddHeight:      lp.LastAddHeight,
		LastWithdrawHeight: lp.LastWithdrawHeight,
		Units:              lp.Units,
		AssetDepositValue:  lp.AssetDepositValue,
		AssetRedeemValue:   lp.GetSaversAssetRedeemValue(pool),
		GrowthPct:          gp,
	}
}

// QueryVaultPubKeyContract is a type to combine PubKey and it's related contract
type QueryVaultPubKeyContract struct {
	PubKey  common.PubKey   `json:"pub_key"`
	Routers []ChainContract `json:"routers"`
}

// QueryVaultsPubKeys represent the result for query vaults pubkeys
type QueryVaultsPubKeys struct {
	Asgard    []QueryVaultPubKeyContract `json:"asgard"`
	Yggdrasil []QueryVaultPubKeyContract `json:"yggdrasil"`
	Inactive  []QueryVaultPubKeyContract `json:"inactive"`
}
