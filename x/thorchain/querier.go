package thorchain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"

	"gitlab.com/thorchain/tss/go-tss/conversion"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	openapi "gitlab.com/thorchain/thornode/openapi/gen"
	q "gitlab.com/thorchain/thornode/x/thorchain/query"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

var (
	initManager   = func(mgr *Mgrs, ctx cosmos.Context) {}
	optionalQuery = func(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
		return nil, cosmos.ErrUnknownRequest(
			fmt.Sprintf("unknown thorchain query endpoint: %s", path[0]),
		)
	}
)

// NewQuerier is the module level router for state queries
func NewQuerier(mgr *Mgrs, kbs cosmos.KeybaseStore) cosmos.Querier {
	return func(ctx cosmos.Context, path []string, req abci.RequestQuery) (res []byte, err error) {
		initManager(mgr, ctx) // NOOP except regtest

		defer telemetry.MeasureSince(time.Now(), path[0])
		switch path[0] {
		case q.QueryPool.Key:
			return queryPool(ctx, path[1:], req, mgr)
		case q.QueryPools.Key:
			return queryPools(ctx, req, mgr)
		case q.QuerySavers.Key:
			return queryLiquidityProviders(ctx, path[1:], req, mgr, true)
		case q.QuerySaver.Key:
			return queryLiquidityProvider(ctx, path[1:], req, mgr, true)
		case q.QueryBorrowers.Key:
			return queryBorrowers(ctx, path[1:], req, mgr)
		case q.QueryBorrower.Key:
			return queryBorrower(ctx, path[1:], req, mgr)
		case q.QueryLiquidityProviders.Key:
			return queryLiquidityProviders(ctx, path[1:], req, mgr, false)
		case q.QueryLiquidityProvider.Key:
			return queryLiquidityProvider(ctx, path[1:], req, mgr, false)
		case q.QueryTxStages.Key:
			return queryTxStages(ctx, path[1:], req, mgr)
		case q.QueryTxStatus.Key:
			return queryTxStatus(ctx, path[1:], req, mgr)
		case q.QueryTxVoter.Key:
			return queryTxVoters(ctx, path[1:], req, mgr)
		case q.QueryTxVoterOld.Key:
			return queryTxVoters(ctx, path[1:], req, mgr)
		case q.QueryTx.Key:
			return queryTx(ctx, path[1:], req, mgr)
		case q.QueryKeysignArray.Key:
			return queryKeysign(ctx, kbs, path[1:], req, mgr)
		case q.QueryKeysignArrayPubkey.Key:
			return queryKeysign(ctx, kbs, path[1:], req, mgr)
		case q.QueryKeygensPubkey.Key:
			return queryKeygen(ctx, kbs, path[1:], req, mgr)
		case q.QueryQueue.Key:
			return queryQueue(ctx, path[1:], req, mgr)
		case q.QueryHeights.Key:
			return queryLastBlockHeights(ctx, path[1:], req, mgr)
		case q.QueryChainHeights.Key:
			return queryLastBlockHeights(ctx, path[1:], req, mgr)
		case q.QueryNode.Key:
			return queryNode(ctx, path[1:], req, mgr)
		case q.QueryNodes.Key:
			return queryNodes(ctx, path[1:], req, mgr)
		case q.QueryInboundAddresses.Key:
			return queryInboundAddresses(ctx, path[1:], req, mgr)
		case q.QueryNetwork.Key:
			return queryNetwork(ctx, mgr)
		case q.QueryPOL.Key:
			return queryPOL(ctx, mgr)
		case q.QueryBalanceModule.Key:
			return queryBalanceModule(ctx, path[1:], mgr)
		case q.QueryVaultsAsgard.Key:
			return queryAsgardVaults(ctx, mgr)
		case q.QueryVaultsYggdrasil.Key:
			return queryYggdrasilVaults(ctx, mgr)
		case q.QueryVault.Key:
			return queryVault(ctx, path[1:], mgr)
		case q.QueryVaultPubkeys.Key:
			return queryVaultsPubkeys(ctx, mgr)
		case q.QueryConstantValues.Key:
			return queryConstantValues(ctx, path[1:], req, mgr)
		case q.QueryVersion.Key:
			return queryVersion(ctx, path[1:], req, mgr)
		case q.QueryMimirValues.Key:
			return queryMimirValues(ctx, path[1:], req, mgr)
		case q.QueryMimirWithKey.Key:
			return queryMimirWithKey(ctx, path[1:], req, mgr)
		case q.QueryMimirAdminValues.Key:
			return queryMimirAdminValues(ctx, path[1:], req, mgr)
		case q.QueryMimirNodesAllValues.Key:
			return queryMimirNodesAllValues(ctx, path[1:], req, mgr)
		case q.QueryMimirNodesValues.Key:
			return queryMimirNodesValues(ctx, path[1:], req, mgr)
		case q.QueryMimirNodeValues.Key:
			return queryMimirNodeValues(ctx, path[1:], req, mgr)
		case q.QueryBan.Key:
			return queryBan(ctx, path[1:], req, mgr)
		case q.QueryRagnarok.Key:
			return queryRagnarok(ctx, mgr)
		case q.QueryPendingOutbound.Key:
			return queryPendingOutbound(ctx, mgr)
		case q.QueryScheduledOutbound.Key:
			return queryScheduledOutbound(ctx, mgr)
		case q.QuerySwapQueue.Key:
			return querySwapQueue(ctx, mgr)
		case q.QueryTssKeygenMetrics.Key:
			return queryTssKeygenMetric(ctx, path[1:], req, mgr)
		case q.QueryTssMetrics.Key:
			return queryTssMetric(ctx, path[1:], req, mgr)
		case q.QueryTHORName.Key:
			return queryTHORName(ctx, path[1:], req, mgr)
		case q.QueryQuoteSwap.Key:
			return queryQuoteSwap(ctx, path[1:], req, mgr)
		case q.QueryQuoteSaverDeposit.Key:
			return queryQuoteSaverDeposit(ctx, path[1:], req, mgr)
		case q.QueryQuoteSaverWithdraw.Key:
			return queryQuoteSaverWithdraw(ctx, path[1:], req, mgr)
		case q.QueryQuoteLoanOpen.Key:
			return queryQuoteLoanOpen(ctx, path[1:], req, mgr)
		case q.QueryQuoteLoanClose.Key:
			return queryQuoteLoanClose(ctx, path[1:], req, mgr)
		case q.QueryInvariants.Key:
			return queryInvariants(ctx, mgr)
		case q.QueryInvariant.Key:
			return queryInvariant(ctx, path[1:], mgr)
		default:
			return optionalQuery(ctx, path, req, mgr)
		}
	}
}

func checkPending(ctx cosmos.Context, mgr *Mgrs, voter ObservedTxVoter) (bool, bool) {
	isSwap := false
	isPending := false

	memo, err := ParseMemo(mgr.GetVersion(), voter.Tx.Tx.Memo)
	if err != nil {
		// If unable to parse, assume not a (valid) swap or limit order memo.
		return isSwap, isPending
	}

	memoType := memo.GetType()
	// If the memo asset is a synth, as with Savers add liquidity or withdraw, a swap is assumed to be involved.
	if memoType == TxSwap || memoType == TxLimitOrder || memo.GetAsset().IsVaultAsset() {
		isSwap = true
		// Only check the KVStore when the inbound transaction has already been finalised
		// and when there haven't been any Actions planned.
		// This will also check the KVStore when an inbound transaction has no output,
		// such as the output being not enough to cover a fee.
		if voter.FinalisedHeight != 0 && len(voter.Actions) == 0 {
			// Use of Swap Queue or Order Book depends on Mimir key EnableOrderBooks rather than memo type, so check both.
			isPending = mgr.Keeper().HasSwapQueueItem(ctx, voter.TxID, 0) || mgr.Keeper().HasOrderBookItem(ctx, voter.TxID)
		}
	}

	return isSwap, isPending
}

func getPeerIDFromPubKey(pubkey common.PubKey) string {
	peerID, err := conversion.GetPeerIDFromPubKey(pubkey.String())
	if err != nil {
		// Don't break the entire endpoint if something goes wrong with the Peer ID derivation.
		return err.Error()
	}

	return peerID.String()
}

func jsonify(ctx cosmos.Context, r any) ([]byte, error) {
	res, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		ctx.Logger().Error("fail to marshal response to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}
	return res, nil
}

func queryRagnarok(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	ragnarokInProgress := mgr.Keeper().RagnarokInProgress(ctx)
	return jsonify(ctx, ragnarokInProgress)
}

func queryBalanceModule(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	moduleName := path[0]
	if len(moduleName) == 0 {
		moduleName = AsgardName
	}

	modAddr := mgr.Keeper().GetModuleAccAddress(moduleName)
	bal := mgr.Keeper().GetBalance(ctx, modAddr)
	balance := struct {
		Name    string            `json:"name"`
		Address cosmos.AccAddress `json:"address"`
		Coins   sdk.Coins         `json:"coins"`
	}{
		Name:    moduleName,
		Address: modAddr,
		Coins:   bal,
	}
	return jsonify(ctx, balance)
}

func queryTHORName(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	name, err := mgr.Keeper().GetTHORName(ctx, path[0])
	if err != nil {
		return nil, ErrInternal(err, "fail to fetch THORName")
	}

	// convert to openapi types
	aliases := []openapi.ThornameAlias{}
	for _, alias := range name.Aliases {
		aliases = append(aliases, openapi.ThornameAlias{
			Chain:   wrapString(alias.Chain.String()),
			Address: wrapString(alias.Address.String()),
		})
	}
	resp := openapi.Thorname{
		Name:              wrapString(name.Name),
		ExpireBlockHeight: wrapInt64(name.ExpireBlockHeight),
		Owner:             wrapString(name.Owner.String()),
		PreferredAsset:    name.PreferredAsset.String(),
		Aliases:           aliases,
	}

	return jsonify(ctx, resp)
}

func queryVault(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	if len(path) < 1 {
		return nil, errors.New("not enough parameters")
	}
	pubkey, err := common.NewPubKey(path[0])
	if err != nil {
		return nil, fmt.Errorf("%s is invalid pubkey", path[0])
	}
	v, err := mgr.Keeper().GetVault(ctx, pubkey)
	if err != nil {
		return nil, fmt.Errorf("fail to get vault with pubkey(%s),err:%w", pubkey, err)
	}
	if v.IsEmpty() {
		return nil, errors.New("vault not found")
	}

	resp := types.QueryVaultResp{
		BlockHeight:           v.BlockHeight,
		PubKey:                v.PubKey,
		Coins:                 v.Coins,
		Type:                  v.Type,
		Status:                v.Status,
		StatusSince:           v.StatusSince,
		Membership:            v.Membership,
		Chains:                v.Chains,
		InboundTxCount:        v.InboundTxCount,
		OutboundTxCount:       v.OutboundTxCount,
		PendingTxBlockHeights: v.PendingTxBlockHeights,
		Routers:               v.Routers,
		Frozen:                v.Frozen,
		Addresses:             getVaultChainAddress(ctx, v),
	}
	return jsonify(ctx, resp)
}

func queryAsgardVaults(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	vaults, err := mgr.Keeper().GetAsgardVaults(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get asgard vaults: %w", err)
	}

	var vaultsWithFunds []types.QueryVaultResp
	for _, vault := range vaults {
		if vault.Status == InactiveVault {
			continue
		}
		if !vault.IsAsgard() {
			continue
		}
		// Being in a RetiringVault blocks a node from unbonding, so display them even if having no funds.
		if vault.HasFunds() || vault.Status == ActiveVault || vault.Status == RetiringVault {
			vaultsWithFunds = append(vaultsWithFunds, types.QueryVaultResp{
				BlockHeight:           vault.BlockHeight,
				PubKey:                vault.PubKey,
				Coins:                 vault.Coins,
				Type:                  vault.Type,
				Status:                vault.Status,
				StatusSince:           vault.StatusSince,
				Membership:            vault.Membership,
				Chains:                vault.Chains,
				InboundTxCount:        vault.InboundTxCount,
				OutboundTxCount:       vault.OutboundTxCount,
				PendingTxBlockHeights: vault.PendingTxBlockHeights,
				Routers:               vault.Routers,
				Frozen:                vault.Frozen,
				Addresses:             getVaultChainAddress(ctx, vault),
			})
		}
	}

	return jsonify(ctx, vaultsWithFunds)
}

func getVaultChainAddress(ctx cosmos.Context, vault Vault) []QueryChainAddress {
	var result []QueryChainAddress
	allChains := append(vault.GetChains(), common.THORChain)
	for _, c := range allChains.Distinct() {
		addr, err := vault.PubKey.GetAddress(c)
		if err != nil {
			ctx.Logger().Error("fail to get address for %s:%w", c.String(), err)
			continue
		}
		result = append(result,
			QueryChainAddress{
				Chain:   c,
				Address: addr,
			})
	}
	return result
}

func queryYggdrasilVaults(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	vaults := make(Vaults, 0)
	iter := mgr.Keeper().GetVaultIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var vault Vault
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal yggdrasil", "error", err)
			return nil, fmt.Errorf("fail to unmarshal yggdrasil: %w", err)
		}
		if vault.IsYggdrasil() && vault.HasFunds() {
			vaults = append(vaults, vault)
		}
	}

	respVaults := make([]QueryYggdrasilVaults, len(vaults))
	for i, vault := range vaults {
		totalValue := cosmos.ZeroUint()

		// find the bond of this node account
		na, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, vault.PubKey)
		if err != nil {
			ctx.Logger().Error("fail to get node account by pubkey", "error", err)
			continue
		}

		// calculate the total value of this yggdrasil vault
		for _, coin := range vault.Coins {
			if coin.Asset.IsRune() {
				totalValue = totalValue.Add(coin.Amount)
			} else {
				pool, err := mgr.Keeper().GetPool(ctx, coin.Asset)
				if err != nil {
					ctx.Logger().Error("fail to get pool", "error", err)
					continue
				}
				totalValue = totalValue.Add(pool.AssetValueInRune(coin.Amount))
			}
		}

		respVaults[i] = QueryYggdrasilVaults{
			BlockHeight:           vault.BlockHeight,
			PubKey:                vault.PubKey,
			Coins:                 vault.Coins,
			Type:                  vault.Type,
			StatusSince:           vault.StatusSince,
			Membership:            vault.Membership,
			Chains:                vault.Chains,
			InboundTxCount:        vault.InboundTxCount,
			OutboundTxCount:       vault.OutboundTxCount,
			PendingTxBlockHeights: vault.PendingTxBlockHeights,
			Routers:               vault.Routers,
			Status:                na.Status,
			Bond:                  na.Bond,
			TotalValue:            totalValue,
			Addresses:             getVaultChainAddress(ctx, vault),
		}
	}

	return jsonify(ctx, respVaults)
}

func queryVaultsPubkeys(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	var resp QueryVaultsPubKeys
	resp.Asgard = make([]QueryVaultPubKeyContract, 0)
	resp.Yggdrasil = make([]QueryVaultPubKeyContract, 0)
	resp.Inactive = make([]QueryVaultPubKeyContract, 0)
	iter := mgr.Keeper().GetVaultIterator(ctx)

	active, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, err
	}
	// usually ChurnInterval is 43200, sometimes when the network has issues , we 10x this value ,
	// so if the inactive vault is older than 500_000 blocks , should not be monitored
	cutOffAge := ctx.BlockHeight() - 500000
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var vault Vault
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
			return nil, fmt.Errorf("fail to unmarshal vault: %w", err)
		}
		if vault.IsYggdrasil() {
			na, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, vault.PubKey)
			if err != nil {
				ctx.Logger().Error("fail to unmarshal vault", "error", err)
				return nil, fmt.Errorf("fail to unmarshal vault: %w", err)
			}
			if !na.Bond.IsZero() {
				resp.Yggdrasil = append(resp.Yggdrasil, QueryVaultPubKeyContract{
					PubKey:  vault.PubKey,
					Routers: vault.Routers,
				})
			}
		} else if vault.IsAsgard() {
			switch vault.Status {
			case ActiveVault, RetiringVault:
				resp.Asgard = append(resp.Asgard, QueryVaultPubKeyContract{
					PubKey:  vault.PubKey,
					Routers: vault.Routers,
				})
			case InactiveVault:
				if vault.BlockHeight < cutOffAge {
					continue
				}
				activeMembers, err := vault.GetMembers(active.GetNodeAddresses())
				if err != nil {
					ctx.Logger().Error("fail to get active members of vault", "error", err)
					continue
				}
				allMembers := vault.Membership
				if HasSuperMajority(len(activeMembers), len(allMembers)) {
					resp.Inactive = append(resp.Inactive, QueryVaultPubKeyContract{
						PubKey:  vault.PubKey,
						Routers: vault.Routers,
					})
				}
			}
		}
	}
	return jsonify(ctx, resp)
}

func queryNetwork(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	data, err := mgr.Keeper().GetNetwork(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get network", "error", err)
		return nil, fmt.Errorf("fail to get network: %w", err)
	}

	vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		return nil, fmt.Errorf("fail to get retiring vaults: %w", err)
	}
	vaultsMigrating := (len(vaults) != 0)

	targetOutboundFeeSurplus := mgr.Keeper().GetConfigInt64(ctx, constants.TargetOutboundFeeSurplusRune)
	maxMultiplierBasisPoints := mgr.Keeper().GetConfigInt64(ctx, constants.MaxOutboundFeeMultiplierBasisPoints)
	minMultiplierBasisPoints := mgr.Keeper().GetConfigInt64(ctx, constants.MinOutboundFeeMultiplierBasisPoints)
	outboundFeeMultiplier := mgr.gasMgr.CalcOutboundFeeMultiplier(ctx, cosmos.NewUint(uint64(targetOutboundFeeSurplus)), cosmos.NewUint(data.OutboundGasSpentRune), cosmos.NewUint(data.OutboundGasWithheldRune), cosmos.NewUint(uint64(maxMultiplierBasisPoints)), cosmos.NewUint(uint64(minMultiplierBasisPoints)))

	result := openapi.NetworkResponse{
		// Due to using openapi. this will be displayed in alphabetical order,
		// so its schema (and order here) should also be in alphabetical order.
		BondRewardRune:        data.BondRewardRune.String(),
		BurnedBep2Rune:        data.BurnedBep2Rune.String(),
		BurnedErc20Rune:       data.BurnedErc20Rune.String(),
		TotalBondUnits:        data.TotalBondUnits.String(),
		TotalReserve:          mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName).String(),
		VaultsMigrating:       vaultsMigrating,
		GasSpentRune:          cosmos.NewUint(data.OutboundGasSpentRune).String(),
		GasWithheldRune:       cosmos.NewUint(data.OutboundGasWithheldRune).String(),
		OutboundFeeMultiplier: wrapString(outboundFeeMultiplier.String()),

		// TODO: These are temporarily hardcoded to the dollar value fees, for sanity check
		// before enabling. Switch to the keeper fee methods after USD fees are live.
		NativeTxFeeRune:    mgr.Keeper().DollarConfigInRune(ctx, constants.NativeTransactionFeeUSD).String(),
		OutboundTxFeeRune:  mgr.Keeper().DollarConfigInRune(ctx, constants.OutboundTransactionFeeUSD).String(),
		TnsRegisterFeeRune: mgr.Keeper().DollarConfigInRune(ctx, constants.TNSRegisterFeeUSD).String(),
		TnsFeePerBlockRune: mgr.Keeper().DollarConfigInRune(ctx, constants.TNSFeePerBlockUSD).String(),
	}

	return jsonify(ctx, result)
}

func queryPOL(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	data, err := mgr.Keeper().GetPOL(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get POL", "error", err)
		return nil, fmt.Errorf("fail to get POL: %w", err)
	}
	polValue, err := polPoolValue(ctx, mgr)
	if err != nil {
		ctx.Logger().Error("fail to fetch POL value", "error", err)
		return nil, fmt.Errorf("fail to fetch POL value: %w", err)
	}
	pnl := data.PnL(polValue)
	result := openapi.POLResponse{
		RuneDeposited:  data.RuneDeposited.String(),
		RuneWithdrawn:  data.RuneWithdrawn.String(),
		Value:          polValue.String(),
		Pnl:            pnl.String(),
		CurrentDeposit: data.CurrentDeposit().String(),
	}

	return jsonify(ctx, result)
}

func queryInboundAddresses(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	active, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active vaults", "error", err)
		return nil, fmt.Errorf("fail to get active vaults: %w", err)
	}

	var resp []openapi.InboundAddress
	constAccessor := mgr.GetConstants()
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	if mgr.Keeper() == nil {
		ctx.Logger().Error("keeper is nil, can't fulfill query")
		return nil, errors.New("keeper is nil, can't fulfill query")
	}
	// select vault that is most secure
	vault := mgr.Keeper().GetMostSecure(ctx, active, signingTransactionPeriod)

	chains := vault.GetChains()

	if len(chains) == 0 {
		chains = common.Chains{common.RuneAsset().Chain}
	}

	isGlobalTradingPaused := mgr.Keeper().IsGlobalTradingHalted(ctx)

	for _, chain := range chains {
		// tx send to thorchain doesn't need an address , thus here skip it
		if chain == common.THORChain {
			continue
		}

		isChainTradingPaused := mgr.Keeper().IsChainTradingHalted(ctx, chain)
		isChainLpPaused := mgr.Keeper().IsLPPaused(ctx, chain)

		vaultAddress, err := vault.PubKey.GetAddress(chain)
		if err != nil {
			ctx.Logger().Error("fail to get address for chain", "error", err)
			return nil, fmt.Errorf("fail to get address for chain: %w", err)
		}
		cc := vault.GetContract(chain)
		gasRate := mgr.GasMgr().GetGasRate(ctx, chain)
		networkFeeInfo, err := mgr.GasMgr().GetNetworkFee(ctx, chain)
		if err != nil {
			ctx.Logger().Error("fail to get network fee info", "error", err)
			return nil, fmt.Errorf("fail to get network fee info: %w", err)
		}

		// because THORNode is using 1e8, while GWei in ETH is in 1e9, thus the minimum THORNode can represent is 10Gwei
		// here convert the gas rate to Gwei , so api user don't need to convert it , make it easier for people to understand
		if chain.IsEVM() {
			gasRate = gasRate.MulUint64(10)
		}

		// Retrieve the value the network charges for outbound fees for this chain
		// Note: If calculating the outbound fee of a non-gas asset outbound, the returned value must be converted to RUNE,
		// then to the non-gas asset using the pool depths. That will be the value deducted from the non-gas asset outbound amount
		outboundFee := mgr.GasMgr().GetFee(ctx, chain, chain.GetGasAsset())

		addr := openapi.InboundAddress{
			Chain:                wrapString(chain.String()),
			PubKey:               wrapString(vault.PubKey.String()),
			Address:              wrapString(vaultAddress.String()),
			Router:               wrapString(cc.Router.String()),
			Halted:               isGlobalTradingPaused || isChainTradingPaused,
			GlobalTradingPaused:  &isGlobalTradingPaused,
			ChainTradingPaused:   &isChainTradingPaused,
			ChainLpActionsPaused: &isChainLpPaused,
			GasRate:              wrapString(gasRate.String()),
			GasRateUnits:         wrapString(chain.GetGasUnits()),
			OutboundTxSize:       wrapString(cosmos.NewUint(networkFeeInfo.TransactionSize).String()),
			OutboundFee:          wrapString(outboundFee.String()),
			DustThreshold:        wrapString(chain.DustThreshold().String()),
		}

		resp = append(resp, addr)
	}

	return jsonify(ctx, resp)
}

// queryNode return the Node information related to the request node address
// /thorchain/node/{nodeaddress}
func queryNode(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("node address not provided")
	}
	nodeAddress := path[0]
	addr, err := cosmos.AccAddressFromBech32(nodeAddress)
	if err != nil {
		return nil, cosmos.ErrUnknownRequest("invalid account address")
	}

	nodeAcc, err := mgr.Keeper().GetNodeAccount(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fail to get node accounts: %w", err)
	}

	slashPts, err := mgr.Keeper().GetNodeAccountSlashPoints(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fail to get node slash points: %w", err)
	}
	jail, err := mgr.Keeper().GetNodeAccountJail(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return nil, fmt.Errorf("fail to get node jail: %w", err)
	}

	bp, err := mgr.Keeper().GetBondProviders(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return nil, fmt.Errorf("fail to get bond providers: %w", err)
	}
	bp.Adjust(mgr.GetVersion(), nodeAcc.Bond)

	active, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get all active node account: %w", err)
	}

	result := NewQueryNodeAccount(nodeAcc)
	result.PeerID = getPeerIDFromPubKey(nodeAcc.PubKeySet.Secp256k1)
	result.SlashPoints = slashPts

	result.Jail = Jail{
		// Since redundant, leave out the node address
		ReleaseHeight: jail.ReleaseHeight,
		Reason:        jail.Reason,
	}

	result.BondProviders = BondProviders{
		// Since redundant, leave out the node address
		NodeOperatorFee: bp.NodeOperatorFee,
		Providers:       bp.Providers,
	}

	// CurrentAward is an estimation of reward for node in active status
	// Node in other status should not have current reward
	if nodeAcc.Status == NodeActive && !nodeAcc.Bond.IsZero() {
		network, err := mgr.Keeper().GetNetwork(ctx)
		if err != nil {
			return nil, fmt.Errorf("fail to get network: %w", err)
		}
		vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			return nil, fmt.Errorf("fail to get active vaults: %w", err)
		}
		if len(vaults) == 0 {
			return nil, fmt.Errorf("no active vaults")
		}

		bondHardCap := getHardBondCap(active)
		totalEffectiveBond, err := getTotalEffectiveBond(ctx, mgr, bondHardCap)
		if err != nil {
			return nil, fmt.Errorf("fail to get total effective bond: %w", err)
		}

		// Note that unlike actual BondRewardRune distribution in manager_validator_current.go ,
		// this estimate treats lastChurnHeight as the block_height of the first (oldest) Asgard vault,
		// rather than the active_block_height of the youngest active node.
		// As an example, note from the below URLs that these are 5293728 and 5293733 respectively in block 5336942.
		// https://thornode.ninerealms.com/thorchain/vaults/asgard?height=5336942
		// https://thornode.ninerealms.com/thorchain/nodes?height=5336942
		// (Nodes .cxmy and .uy3a .)
		lastChurnHeight := vaults[0].BlockHeight

		reward, err := getNodeCurrentRewards(ctx, mgr, nodeAcc, lastChurnHeight, network.BondRewardRune, totalEffectiveBond, bondHardCap)
		if err != nil {
			return nil, fmt.Errorf("fail to get current node rewards: %w", err)
		}

		result.CurrentAward = reward
	}

	chainHeights, err := mgr.Keeper().GetLastObserveHeight(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fail to get last observe chain height: %w", err)
	}

	// analyze-ignore(map-iteration)
	for c, h := range chainHeights {
		result.ObserveChains = append(result.ObserveChains, types.QueryChainHeight{
			Chain:  c,
			Height: h,
		})
	}

	preflightCheckResult, err := getNodePreflightResult(ctx, mgr, nodeAcc)
	if err != nil {
		ctx.Logger().Error("fail to get node preflight result", "error", err)
	} else {
		result.PreflightStatus = preflightCheckResult
	}
	return jsonify(ctx, result)
}

func getNodePreflightResult(ctx cosmos.Context, mgr *Mgrs, nodeAcc NodeAccount) (QueryNodeAccountPreflightCheck, error) {
	constAccessor := mgr.GetConstants()
	preflightResult := QueryNodeAccountPreflightCheck{}
	status, err := mgr.ValidatorMgr().NodeAccountPreflightCheck(ctx, nodeAcc, constAccessor)
	preflightResult.Status = status
	if err != nil {
		preflightResult.Description = err.Error()
		preflightResult.Code = 1
	} else {
		preflightResult.Description = "OK"
		preflightResult.Code = 0
	}
	return preflightResult, nil
}

// Estimates current rewards for the NodeAccount taking into account bond-weighted rewards and slash points
func getNodeCurrentRewards(ctx cosmos.Context, mgr *Mgrs, nodeAcc NodeAccount, lastChurnHeight int64, totalBondReward, totalEffectiveBond, bondHardCap cosmos.Uint) (cosmos.Uint, error) {
	slashPts, err := mgr.Keeper().GetNodeAccountSlashPoints(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get node slash points: %w", err)
	}

	// Find number of blocks since the last churn (the last bond reward payout)
	totalActiveBlocks := ctx.BlockHeight() - lastChurnHeight

	// find number of blocks they were well behaved (ie active - slash points)
	earnedBlocks := totalActiveBlocks - slashPts
	if earnedBlocks < 0 {
		earnedBlocks = 0
	}

	naEffectiveBond := nodeAcc.Bond
	if naEffectiveBond.GT(bondHardCap) {
		naEffectiveBond = bondHardCap
	}

	// reward = totalBondReward * (naEffectiveBond / totalEffectiveBond) * (unslashed blocks since last churn / blocks since last churn)
	reward := common.GetUncappedShare(naEffectiveBond, totalEffectiveBond, totalBondReward)
	reward = common.GetUncappedShare(cosmos.NewUint(uint64(earnedBlocks)), cosmos.NewUint(uint64(totalActiveBlocks)), reward)
	return reward, nil
}

// Calculates total "effective bond" - the total bond when taking into account the
// Bond-weighted hard-cap
func getTotalEffectiveBond(ctx cosmos.Context, mgr *Mgrs, bondHardCap cosmos.Uint) (cosmos.Uint, error) {
	activeNodes, err := mgr.Keeper().ListValidatorsByStatus(ctx, NodeActive)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get active nodes: %w", err)
	}

	totalEffectiveBond := cosmos.ZeroUint()
	for _, item := range activeNodes {
		b := item.Bond
		if item.Bond.GT(bondHardCap) {
			b = bondHardCap
		}

		totalEffectiveBond = totalEffectiveBond.Add(b)
	}

	return totalEffectiveBond, nil
}

// queryNodes return all the nodes that has bond
// /thorchain/nodes
func queryNodes(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	nodeAccounts, err := mgr.Keeper().ListValidatorsWithBond(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get node accounts: %w", err)
	}

	active, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get all active node account: %w", err)
	}

	network, err := mgr.Keeper().GetNetwork(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get network: %w", err)
	}

	vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return nil, fmt.Errorf("fail to get active vaults: %w", err)
	}
	if len(vaults) == 0 {
		return nil, fmt.Errorf("no active vaults")
	}

	bondHardCap := getHardBondCap(active)
	totalEffectiveBond, err := getTotalEffectiveBond(ctx, mgr, bondHardCap)
	if err != nil {
		return nil, fmt.Errorf("fail to get total effective bond: %w", err)
	}

	lastChurnHeight := vaults[0].BlockHeight
	version := mgr.GetVersion()
	result := make([]QueryNodeAccount, len(nodeAccounts))
	for i, na := range nodeAccounts {
		if na.RequestedToLeave && na.Bond.LTE(cosmos.NewUint(common.One)) {
			// ignore the node , it left and also has very little bond
			continue
		}

		slashPts, err := mgr.Keeper().GetNodeAccountSlashPoints(ctx, na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get node slash points: %w", err)
		}

		result[i] = NewQueryNodeAccount(na)
		result[i].PeerID = getPeerIDFromPubKey(na.PubKeySet.Secp256k1)
		result[i].SlashPoints = slashPts
		if na.Status == NodeActive {
			reward, err := getNodeCurrentRewards(ctx, mgr, na, lastChurnHeight, network.BondRewardRune, totalEffectiveBond, bondHardCap)
			if err != nil {
				return nil, fmt.Errorf("fail to get current node rewards: %w", err)
			}

			result[i].CurrentAward = reward
		}

		jail, err := mgr.Keeper().GetNodeAccountJail(ctx, na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get node jail: %w", err)
		}
		result[i].Jail = Jail{
			// Since redundant, leave out the node address
			ReleaseHeight: jail.ReleaseHeight,
			Reason:        jail.Reason,
		}
		chainHeights, err := mgr.Keeper().GetLastObserveHeight(ctx, na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get last observe chain height: %w", err)
		}

		// analyze-ignore(map-iteration)
		for c, h := range chainHeights {
			result[i].ObserveChains = append(result[i].ObserveChains, types.QueryChainHeight{
				Chain:  c,
				Height: h,
			})
		}

		preflightCheckResult, err := getNodePreflightResult(ctx, mgr, na)
		if err != nil {
			ctx.Logger().Error("fail to get node preflight result", "error", err)
		} else {
			result[i].PreflightStatus = preflightCheckResult
		}

		bp, err := mgr.Keeper().GetBondProviders(ctx, result[i].NodeAddress)
		if err != nil {
			ctx.Logger().Error("fail to get bond providers", "error", err)
		}
		bp.Adjust(version, na.Bond)
		result[i].BondProviders = BondProviders{
			// Since redundant, leave out the node address
			NodeOperatorFee: bp.NodeOperatorFee,
			Providers:       bp.Providers,
		}
	}

	return jsonify(ctx, result)
}

// queryBorrowers
func queryBorrowers(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("asset not provided")
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to get parse asset", "error", err)
		return nil, fmt.Errorf("fail to parse asset: %w", err)
	}

	var loans Loans
	iterator := mgr.Keeper().GetLoanIterator(ctx, asset)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var loan Loan
		mgr.Keeper().Cdc().MustUnmarshal(iterator.Value(), &loan)
		if loan.CollateralUp.Equal(loan.CollateralDown) && loan.DebtUp.Equal(loan.DebtDown) {
			continue
		}
		loans = append(loans, loan)
	}
	var res []byte
	res, err = json.MarshalIndent(loans, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal borrowers to json", "error", err)
		return nil, fmt.Errorf("fail to marshal borrowers to json: %w", err)
	}
	return res, nil
}

// queryBorrower
func queryBorrower(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) < 2 {
		return nil, errors.New("asset/loan not provided")
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to get parse asset", "error", err)
		return nil, fmt.Errorf("fail to parse asset: %w", err)
	}

	addr, err := common.NewAddress(path[1])
	if err != nil {
		ctx.Logger().Error("fail to get parse address", "error", err)
		return nil, fmt.Errorf("fail to parse address: %w", err)
	}

	loan, err := mgr.Keeper().GetLoan(ctx, asset, addr)
	if err != nil {
		ctx.Logger().Error("fail to get borrower", "error", err)
		return nil, fmt.Errorf("fail to borrower: %w", err)
	}

	var res []byte
	res, err = json.MarshalIndent(loan, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal borrower to json", "error", err)
		return nil, fmt.Errorf("fail to marshal borrower to json: %w", err)
	}
	return res, nil
}

// queryLiquidityProviders
// isSavers is true if request is for the savers of a Savers Pool, if false the request is for an L1 pool
func queryLiquidityProviders(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs, isSavers bool) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("asset not provided")
	}
	if isSavers {
		path[0] = strings.Replace(path[0], ".", "/", 1)
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to get parse asset", "error", err)
		return nil, fmt.Errorf("fail to parse asset: %w", err)
	}
	if asset.IsDerivedAsset() {
		return nil, fmt.Errorf("must not be a derived asset")
	}
	if isSavers && !asset.IsVaultAsset() {
		return nil, fmt.Errorf("invalid request: requested pool is not a SaversPool")
	} else if !isSavers && asset.IsVaultAsset() {
		return nil, fmt.Errorf("invalid request: requested pool is a SaversPool")
	}

	poolAsset := asset
	if isSavers {
		poolAsset = asset.GetSyntheticAsset()
	}

	pool, err := mgr.Keeper().GetPool(ctx, poolAsset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return nil, fmt.Errorf("fail to get pool: %w", err)
	}

	var lps LiquidityProviders
	var savers []QuerySaver
	iterator := mgr.Keeper().GetLiquidityProviderIterator(ctx, asset)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var lp LiquidityProvider
		mgr.Keeper().Cdc().MustUnmarshal(iterator.Value(), &lp)
		if !isSavers {
			lps = append(lps, lp)
		} else {
			savers = append(savers, NewQuerySaver(lp, pool))
		}
	}
	if !isSavers {
		return jsonify(ctx, lps)
	} else {
		return jsonify(ctx, savers)
	}
}

// queryLiquidityProvider
// isSavers is true if request is for the savers of a Savers Pool, if false the request is for an L1 pool
func queryLiquidityProvider(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs, isSavers bool) ([]byte, error) {
	if len(path) < 2 {
		return nil, errors.New("asset/lp not provided")
	}
	if isSavers {
		path[0] = strings.Replace(path[0], ".", "/", 1)
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to get parse asset", "error", err)
		return nil, fmt.Errorf("fail to parse asset: %w", err)
	}

	if asset.IsDerivedAsset() {
		return nil, fmt.Errorf("must not be a derived asset")
	}

	if isSavers && !asset.IsVaultAsset() {
		return nil, fmt.Errorf("invalid request: requested pool is not a SaversPool")
	} else if !isSavers && asset.IsVaultAsset() {
		return nil, fmt.Errorf("invalid request: requested pool is a SaversPool")
	}

	addr, err := common.NewAddress(path[1])
	if err != nil {
		ctx.Logger().Error("fail to get parse address", "error", err)
		return nil, fmt.Errorf("fail to parse address: %w", err)
	}
	lp, err := mgr.Keeper().GetLiquidityProvider(ctx, asset, addr)
	if err != nil {
		ctx.Logger().Error("fail to get liquidity provider", "error", err)
		return nil, fmt.Errorf("fail to liquidity provider: %w", err)
	}

	poolAsset := asset
	if isSavers {
		poolAsset = asset.GetSyntheticAsset()
	}

	pool, err := mgr.Keeper().GetPool(ctx, poolAsset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return nil, fmt.Errorf("fail to get pool: %w", err)
	}

	if !isSavers {
		synthSupply := mgr.Keeper().GetTotalSupply(ctx, poolAsset.GetSyntheticAsset())
		liqp := NewQueryLiquidityProvider(lp, pool, synthSupply, mgr.GetVersion())
		return jsonify(ctx, liqp)
	} else {
		saver := NewQuerySaver(lp, pool)
		return jsonify(ctx, saver)
	}
}

func queryPool(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("asset not provided")
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse asset", "error", err)
		return nil, fmt.Errorf("could not parse asset: %w", err)
	}

	pool, err := mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return nil, fmt.Errorf("could not get pool: %w", err)
	}
	if pool.IsEmpty() {
		return nil, fmt.Errorf("pool: %s doesn't exist", path[0])
	}

	// Get Savers Vault for this L1 pool if it's a gas asset
	saversAsset := pool.Asset.GetSyntheticAsset()
	saversPool, err := mgr.Keeper().GetPool(ctx, saversAsset)
	if err != nil {
		return nil, fmt.Errorf("fail to unmarshal savers vault: %w", err)
	}

	saversDepth := saversPool.BalanceAsset
	saversUnits := saversPool.LPUnits

	synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	pool.CalcUnits(mgr.GetVersion(), synthSupply)

	synthMintPausedErr := isSynthMintPaused(ctx, mgr, saversAsset, cosmos.ZeroUint())
	synthSupplyRemaining, _ := getSynthSupplyRemainingV102(ctx, mgr, saversAsset)

	totalCollateral, err := mgr.Keeper().GetTotalCollateral(ctx, pool.Asset)
	if err != nil {
		return nil, fmt.Errorf("fail to fetch total loan collateral: %w", err)
	}

	p := NewQueryPool(pool)
	p.SynthSupply = synthSupply.String()
	p.SaversDepth = saversDepth.String()
	p.SaversUnits = saversUnits.String()
	p.SynthMintPaused = (synthMintPausedErr != nil)
	p.SynthSupplyRemaining = synthSupplyRemaining.String()
	p.LoanCollateral = totalCollateral.String()

	return jsonify(ctx, p)
}

func queryPools(ctx cosmos.Context, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	pools := make([]QueryPool, 0)
	iterator := mgr.Keeper().GetPoolIterator(ctx)
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &pool); err != nil {
			return nil, fmt.Errorf("fail to unmarshal pool: %w", err)
		}
		// ignore pool if no liquidity provider units
		if pool.LPUnits.IsZero() {
			continue
		}

		// Ignore synth asset pool (savers). Info will be on the L1 pool
		if pool.Asset.IsVaultAsset() {
			continue
		}

		// Get Savers Vault
		saversAsset := pool.Asset.GetSyntheticAsset()
		saversPool, err := mgr.Keeper().GetPool(ctx, saversAsset)
		if err != nil {
			return nil, fmt.Errorf("fail to unmarshal savers vault: %w", err)
		}

		saversDepth := saversPool.BalanceAsset
		saversUnits := saversPool.LPUnits

		synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
		pool.CalcUnits(mgr.GetVersion(), synthSupply)

		synthMintPausedErr := isSynthMintPaused(ctx, mgr, pool.Asset, cosmos.ZeroUint())
		synthSupplyRemaining, _ := getSynthSupplyRemainingV102(ctx, mgr, pool.Asset)

		totalCollateral, err := mgr.Keeper().GetTotalCollateral(ctx, pool.Asset)
		if err != nil {
			return nil, fmt.Errorf("fail to fetch total loan collateral: %w", err)
		}

		p := NewQueryPool(pool)
		p.SynthSupply = synthSupply.String()
		p.SaversDepth = saversDepth.String()
		p.SaversUnits = saversUnits.String()
		p.SynthMintPaused = (synthMintPausedErr != nil)
		p.SynthSupplyRemaining = synthSupplyRemaining.String()
		p.LoanCollateral = totalCollateral.String()

		pools = append(pools, p)
	}
	return jsonify(ctx, pools)
}

func extractVoter(ctx cosmos.Context, path []string, mgr *Mgrs) (common.TxID, ObservedTxVoter, error) {
	if len(path) == 0 {
		return "", ObservedTxVoter{}, errors.New("tx id not provided")
	}
	hash, err := common.NewTxID(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse tx id", "error", err)
		return "", ObservedTxVoter{}, fmt.Errorf("fail to parse tx id: %w", err)
	}
	voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fail to get observed tx voter", "error", err)
		return "", ObservedTxVoter{}, fmt.Errorf("fail to get observed tx voter: %w", err)
	}
	return hash, voter, nil
}

func queryTxVoters(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	hash, voter, err := extractVoter(ctx, path, mgr)
	if err != nil {
		return nil, err
	}
	// when tx in voter doesn't exist , double check tx out voter
	if len(voter.Txs) == 0 {
		voter, err = mgr.Keeper().GetObservedTxOutVoter(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("fail to get observed tx out voter: %w", err)
		}
		if len(voter.Txs) == 0 {
			return nil, fmt.Errorf("tx: %s doesn't exist", hash)
		}
	}

	result := NewQueryTxSigners(voter)

	return jsonify(ctx, result)
}

func queryTxStages(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// First, get the ObservedTxVoter of interest.
	_, voter, err := extractVoter(ctx, path, mgr)
	if err != nil {
		return nil, err
	}
	// when no TxIn voter don't check TxOut voter, as TxOut THORChain observation or not matters little to the user once signed and broadcast
	// Rather than a "tx: %s doesn't exist" result, allow a response to an existing-but-unobserved hash with Observation.Started 'false'.

	isSwap, isPending := checkPending(ctx, mgr, voter)

	result := NewQueryTxStages(ctx, voter, isSwap, isPending)

	return jsonify(ctx, result)
}

func queryTxStatus(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// First, get the ObservedTxVoter of interest.
	_, voter, err := extractVoter(ctx, path, mgr)
	if err != nil {
		return nil, err
	}
	// when no TxIn voter don't check TxOut voter, as TxOut THORChain observation or not matters little to the user once signed and broadcast
	// Rather than a "tx: %s doesn't exist" result, allow a response to an existing-but-unobserved hash with Stages.Observation.Started 'false'.

	isSwap, isPending := checkPending(ctx, mgr, voter)

	result := NewQueryTxStatus(ctx, voter, isSwap, isPending)

	return jsonify(ctx, result)
}

func queryTx(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	hash, voter, err := extractVoter(ctx, path, mgr)
	if err != nil {
		return nil, err
	}
	if len(voter.Txs) == 0 {
		voter, err = mgr.Keeper().GetObservedTxOutVoter(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("fail to get observed tx out voter: %w", err)
		}
		if len(voter.Txs) == 0 {
			return nil, fmt.Errorf("tx: %s doesn't exist", hash)
		}
	}

	nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get node accounts: %w", err)
	}
	keysignMetric, err := mgr.Keeper().GetTssKeysignMetric(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fail to get keysign metrics", "error", err)
	}
	result := struct {
		QueryObservedTx `json:"observed_tx"`
		FinalisedHeight int64                  `json:"finalised_height,omitempty"`
		OutboundHeight  int64                  `json:"outbound_height,omitempty"`
		KeysignMetrics  types.TssKeysignMetric `json:"keysign_metric"`
	}{
		FinalisedHeight: voter.FinalisedHeight,
		OutboundHeight:  voter.OutboundHeight,
		KeysignMetrics:  *keysignMetric,
	}
	result.QueryObservedTx = NewQueryObservedTx(voter.GetTx(nodeAccounts))
	return jsonify(ctx, result)
}

func extractBlockHeight(ctx cosmos.Context, path []string) (int64, error) {
	if len(path) == 0 {
		return -1, errors.New("block height not provided")
	}
	height, err := strconv.ParseInt(path[0], 0, 64)
	if err != nil {
		ctx.Logger().Error("fail to parse block height", "error", err)
		return -1, fmt.Errorf("fail to parse block height: %w", err)
	}
	if height > ctx.BlockHeight() {
		return -1, fmt.Errorf("block height not available yet")
	}
	return height, nil
}

func queryKeygen(ctx cosmos.Context, kbs cosmos.KeybaseStore, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	height, err := extractBlockHeight(ctx, path)
	if err != nil {
		return nil, err
	}

	keygenBlock, err := mgr.Keeper().GetKeygenBlock(ctx, height)
	if err != nil {
		ctx.Logger().Error("fail to get keygen block", "error", err)
		return nil, fmt.Errorf("fail to get keygen block: %w", err)
	}

	if len(path) > 1 {
		pk, err := common.NewPubKey(path[1])
		if err != nil {
			ctx.Logger().Error("fail to parse pubkey", "error", err)
			return nil, fmt.Errorf("fail to parse pubkey: %w", err)
		}
		// only return those keygen contains the request pub key
		newKeygenBlock := NewKeygenBlock(keygenBlock.Height)
		for _, keygen := range keygenBlock.Keygens {
			if keygen.GetMembers().Contains(pk) {
				newKeygenBlock.Keygens = append(newKeygenBlock.Keygens, keygen)
			}
		}
		keygenBlock = newKeygenBlock
	}

	buf, err := json.Marshal(keygenBlock)
	if err != nil {
		ctx.Logger().Error("fail to marshal keygen block to json", "error", err)
		return nil, fmt.Errorf("fail to marshal keygen block to json: %w", err)
	}
	sig, _, err := kbs.Keybase.Sign("thorchain", buf)
	if err != nil {
		ctx.Logger().Error("fail to sign keygen", "error", err)
		return nil, fmt.Errorf("fail to sign keygen: %w", err)
	}

	query := QueryKeygenBlock{
		KeygenBlock: keygenBlock,
		Signature:   base64.StdEncoding.EncodeToString(sig),
	}

	return jsonify(ctx, query)
}

func queryKeysign(ctx cosmos.Context, kbs cosmos.KeybaseStore, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	height, err := extractBlockHeight(ctx, path)
	if err != nil {
		return nil, err
	}

	pk := common.EmptyPubKey
	if len(path) > 1 {
		pk, err = common.NewPubKey(path[1])
		if err != nil {
			ctx.Logger().Error("fail to parse pubkey", "error", err)
			return nil, fmt.Errorf("fail to parse pubkey: %w", err)
		}
	}

	txs, err := mgr.Keeper().GetTxOut(ctx, height)
	if err != nil {
		ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
		return nil, fmt.Errorf("fail to get tx out array from key value store: %w", err)
	}

	if !pk.IsEmpty() {
		newTxs := &TxOut{
			Height: txs.Height,
		}
		for _, tx := range txs.TxArray {
			if pk.Equals(tx.VaultPubKey) {
				newTxs.TxArray = append(newTxs.TxArray, tx)
			}
		}
		txs = newTxs
	}

	buf, err := json.Marshal(txs)
	if err != nil {
		ctx.Logger().Error("fail to marshal keysign block to json", "error", err)
		return nil, fmt.Errorf("fail to marshal keysign block to json: %w", err)
	}
	sig, _, err := kbs.Keybase.Sign("thorchain", buf)
	if err != nil {
		ctx.Logger().Error("fail to sign keysign", "error", err)
		return nil, fmt.Errorf("fail to sign keysign: %w", err)
	}
	query := QueryKeysign{
		Keysign:   *txs,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}

	return jsonify(ctx, query)
}

// queryOutQueue - iterates over txout, counting how many transactions are waiting to be sent
func queryQueue(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	constAccessor := mgr.GetConstants()
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := ctx.BlockHeight() - signingTransactionPeriod
	query := QueryQueue{
		ScheduledOutboundValue: cosmos.ZeroUint(),
	}

	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			continue
		}
		query.Swap++
	}

	iter2 := mgr.Keeper().GetOrderBookItemIterator(ctx)
	defer iter2.Close()
	for ; iter2.Valid(); iter2.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			ctx.Logger().Error("failed to load MsgSwap", "error", err)
			continue
		}
		query.Swap++
	}

	for height := startHeight; height <= ctx.BlockHeight(); height++ {
		txs, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			return nil, fmt.Errorf("fail to get tx out array from key value store: %w", err)
		}
		for _, tx := range txs.TxArray {
			if tx.OutHash.IsEmpty() {
				memo, _ := ParseMemoWithTHORNames(ctx, mgr.Keeper(), tx.Memo)
				if memo.IsInternal() {
					query.Internal++
				} else if memo.IsOutbound() {
					query.Outbound++
				}
			}
		}
	}

	// sum outbound value
	maxTxOutOffset, err := mgr.Keeper().GetMimir(ctx, constants.MaxTxOutOffset.String())
	if maxTxOutOffset < 0 || err != nil {
		maxTxOutOffset = constAccessor.GetInt64Value(constants.MaxTxOutOffset)
	}
	txOutDelayMax, err := mgr.Keeper().GetMimir(ctx, constants.TxOutDelayMax.String())
	if txOutDelayMax <= 0 || err != nil {
		txOutDelayMax = constAccessor.GetInt64Value(constants.TxOutDelayMax)
	}

	for height := ctx.BlockHeight() + 1; height <= ctx.BlockHeight()+txOutDelayMax; height++ {
		value, err := mgr.Keeper().GetTxOutValue(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			continue
		}
		if height > ctx.BlockHeight()+maxTxOutOffset && value.IsZero() {
			// we've hit our max offset, and an empty block, we can assume the
			// rest will be empty as well
			break
		}
		query.ScheduledOutboundValue = query.ScheduledOutboundValue.Add(value)
	}

	return jsonify(ctx, query)
}

func queryLastBlockHeights(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	var chains common.Chains
	if len(path) > 0 && len(path[0]) > 0 {
		var err error
		chain, err := common.NewChain(path[0])
		if err != nil {
			ctx.Logger().Error("fail to parse chain", "error", err, "chain", path[0])
			return nil, fmt.Errorf("fail to retrieve chain: %w", err)
		}
		chains = append(chains, chain)
	} else {
		asgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			return nil, fmt.Errorf("fail to get active asgard: %w", err)
		}
		for _, vault := range asgards {
			chains = vault.GetChains().Distinct()
			break
		}
	}
	var result []QueryResLastBlockHeights
	for _, c := range chains {
		if c == common.THORChain {
			continue
		}
		chainHeight, err := mgr.Keeper().GetLastChainHeight(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("fail to get last chain height: %w", err)
		}

		signed, err := mgr.Keeper().GetLastSignedHeight(ctx)
		if err != nil {
			return nil, fmt.Errorf("fail to get last sign height: %w", err)
		}
		result = append(result, QueryResLastBlockHeights{
			Chain:            c,
			LastChainHeight:  chainHeight,
			LastSignedHeight: signed,
			Thorchain:        ctx.BlockHeight(),
		})
	}

	return jsonify(ctx, result)
}

func queryConstantValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	constAccessor := mgr.GetConstants()
	return jsonify(ctx, constAccessor)
}

func queryVersion(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	v, hasV := mgr.Keeper().GetVersionWithCtx(ctx)
	if !hasV {
		// re-compute version if not stored
		v = mgr.Keeper().GetLowestActiveVersion(ctx)
	}

	minJoinLast, minJoinLastChangedHeight := mgr.Keeper().GetMinJoinLast(ctx)

	ver := QueryVersion{
		Current:         v,
		Next:            minJoinLast,
		NextSinceHeight: minJoinLastChangedHeight, // omitted if 0
		Querier:         constants.SWVersion,
	}
	return jsonify(ctx, ver)
}

func queryMimirWithKey(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 && len(path[0]) == 0 {
		return nil, fmt.Errorf("no mimir key")
	}
	v, err := mgr.Keeper().GetMimir(ctx, path[0])
	if err != nil {
		return nil, fmt.Errorf("fail to get mimir with key:%s, err : %w", path[0], err)
	}
	return jsonify(ctx, v)
}

func queryMimirValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	values := make(map[string]int64)

	// collect keys
	iter := mgr.Keeper().GetMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		k := strings.TrimPrefix(string(iter.Key()), "mimir//")
		values[k] = 0
	}
	iterNode := mgr.Keeper().GetNodeMimirIterator(ctx)
	defer iterNode.Close()
	for ; iterNode.Valid(); iterNode.Next() {
		k := strings.TrimPrefix(string(iterNode.Key()), "nodemimir//")
		values[k] = 0
	}

	// analyze-ignore(map-iteration)
	for k := range values {
		v, err := mgr.Keeper().GetMimir(ctx, k)
		if err != nil {
			return nil, fmt.Errorf("fail to get mimir, err: %w", err)
		}
		// v from GetMimir is of type int64.
		if v == -1 {
			// This key has node votes but no node consensus or Admin-set value,
			// so do not display its unset status in the Mimir endpoint.
			delete(values, k)
			continue
		}
		values[k] = v
	}

	return jsonify(ctx, values)
}

func queryMimirAdminValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	values := make(map[string]int64)
	iter := mgr.Keeper().GetMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		value := types.ProtoInt64{}
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &value); err != nil {
			ctx.Logger().Error("fail to unmarshal mimir value", "error", err)
			return nil, fmt.Errorf("fail to unmarshal mimir value: %w", err)
		}
		k := strings.TrimPrefix(string(iter.Key()), "mimir//")
		values[k] = value.GetValue()
	}
	return jsonify(ctx, values)
}

func queryMimirNodesAllValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	mimirs := NodeMimirs{}
	iter := mgr.Keeper().GetNodeMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		m := NodeMimirs{}
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &m); err != nil {
			ctx.Logger().Error("fail to unmarshal node mimir value", "error", err)
			return nil, fmt.Errorf("fail to unmarshal node mimir value: %w", err)
		}
		mimirs.Mimirs = append(mimirs.Mimirs, m.Mimirs...)
	}

	return jsonify(ctx, mimirs)
}

func queryMimirNodesValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	activeNodes, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("fail to fetch active node accounts", "error", err)
		return nil, fmt.Errorf("fail to fetch active node accounts: %w", err)
	}
	active := activeNodes.GetNodeAddresses()

	values := make(map[string]int64)
	iter := mgr.Keeper().GetNodeMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		mimirs := NodeMimirs{}
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &mimirs); err != nil {
			ctx.Logger().Error("fail to unmarshal node mimir value", "error", err)
			return nil, fmt.Errorf("fail to unmarshal node mimir value: %w", err)
		}
		k := strings.TrimPrefix(string(iter.Key()), "nodemimir//")
		if v, ok := mimirs.HasSuperMajority(k, active); ok {
			values[k] = v
		}
	}

	return jsonify(ctx, values)
}

func queryMimirNodeValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	acc, err := cosmos.AccAddressFromBech32(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse thor address", "error", err)
		return nil, fmt.Errorf("fail to parse thor address: %w", err)
	}

	values := make(map[string]int64)
	iter := mgr.Keeper().GetNodeMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		mimirs := NodeMimirs{}
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &mimirs); err != nil {
			ctx.Logger().Error("fail to unmarshal node mimir value", "error", err)
			return nil, fmt.Errorf("fail to unmarshal node mimir value: %w", err)
		}

		k := strings.TrimPrefix(string(iter.Key()), "nodemimir//")
		if v, ok := mimirs.Get(k, acc); ok {
			values[k] = v
		}
	}

	return jsonify(ctx, values)
}

func queryBan(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("node address not available")
	}
	addr, err := cosmos.AccAddressFromBech32(path[0])
	if err != nil {
		ctx.Logger().Error("invalid node address", "error", err)
		return nil, fmt.Errorf("invalid node address: %w", err)
	}

	ban, err := mgr.Keeper().GetBanVoter(ctx, addr)
	if err != nil {
		ctx.Logger().Error("fail to get ban voter", "error", err)
		return nil, fmt.Errorf("fail to get ban voter: %w", err)
	}

	return jsonify(ctx, ban)
}

func queryScheduledOutbound(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	result := make([]QueryTxOutItem, 0)
	constAccessor := mgr.GetConstants()
	maxTxOutOffset, err := mgr.Keeper().GetMimir(ctx, constants.MaxTxOutOffset.String())
	if maxTxOutOffset < 0 || err != nil {
		maxTxOutOffset = constAccessor.GetInt64Value(constants.MaxTxOutOffset)
	}
	for height := ctx.BlockHeight() + 1; height <= ctx.BlockHeight()+17280; height++ {
		txOut, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			continue
		}
		if height > ctx.BlockHeight()+maxTxOutOffset && len(txOut.TxArray) == 0 {
			// we've hit our max offset, and an empty block, we can assume the
			// rest will be empty as well
			break
		}
		for _, toi := range txOut.TxArray {
			result = append(result, NewQueryTxOutItem(toi, height))
		}
	}

	return jsonify(ctx, result)
}

func queryPendingOutbound(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	constAccessor := mgr.GetConstants()
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := ctx.BlockHeight() - signingTransactionPeriod
	var result []TxOutItem
	for height := startHeight; height <= ctx.BlockHeight(); height++ {
		txs, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			return nil, fmt.Errorf("fail to get tx out array from key value store: %w", err)
		}
		for _, tx := range txs.TxArray {
			if tx.OutHash.IsEmpty() {
				result = append(result, tx)
			}
		}
	}

	return jsonify(ctx, result)
}

func querySwapQueue(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	var result []MsgSwap

	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			continue
		}
		result = append(result, msg)
	}

	return jsonify(ctx, result)
}

func queryTssKeygenMetric(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	var pubKeys common.PubKeys
	if len(path) > 0 {
		pkey, err := common.NewPubKey(path[0])
		if err != nil {
			return nil, fmt.Errorf("fail to parse pubkey(%s) err:%w", path[0], err)
		}
		pubKeys = append(pubKeys, pkey)
	}
	var result []*types.TssKeygenMetric
	for _, pkey := range pubKeys {
		m, err := mgr.Keeper().GetTssKeygenMetric(ctx, pkey)
		if err != nil {
			return nil, fmt.Errorf("fail to get tss keygen metric for pubkey(%s):%w", pkey, err)
		}
		result = append(result, m)
	}
	return jsonify(ctx, result)
}

func queryTssMetric(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	var pubKeys common.PubKeys
	// get all active asgard
	vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return nil, fmt.Errorf("fail to get active asgards:%w", err)
	}
	for _, v := range vaults {
		pubKeys = append(pubKeys, v.PubKey)
	}
	var keygenMetrics []*types.TssKeygenMetric
	for _, pkey := range pubKeys {
		m, err := mgr.Keeper().GetTssKeygenMetric(ctx, pkey)
		if err != nil {
			return nil, fmt.Errorf("fail to get tss keygen metric for pubkey(%s):%w", pkey, err)
		}
		if len(m.NodeTssTimes) == 0 {
			continue
		}
		keygenMetrics = append(keygenMetrics, m)
	}
	keysignMetric, err := mgr.Keeper().GetLatestTssKeysignMetric(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get keysign metric:%w", err)
	}
	m := struct {
		KeygenMetrics []*types.TssKeygenMetric `json:"keygen"`
		KeysignMetric *types.TssKeysignMetric  `json:"keysign"`
	}{
		KeygenMetrics: keygenMetrics,
		KeysignMetric: keysignMetric,
	}
	return json.MarshalIndent(m, "", "	")
}

func queryInvariants(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	result := openapi.InvariantsResponse{}
	for _, invarRoute := range mgr.Keeper().InvariantRoutes() {
		result.Invariants = append(result.Invariants, invarRoute.Route)
	}
	return jsonify(ctx, result)
}

func queryInvariant(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	if len(path) < 1 {
		return nil, fmt.Errorf("invalid path: %v", path)
	}
	for _, invarRoute := range mgr.Keeper().InvariantRoutes() {
		if strings.EqualFold(invarRoute.Route, path[0]) {
			msg, broken := invarRoute.Invar(ctx)
			result := openapi.InvariantResponse{
				Invariant: invarRoute.Route,
				Broken:    broken,
				Msg:       msg,
			}
			return jsonify(ctx, result)
		}
	}
	return nil, fmt.Errorf("invariant not registered: %s", path[0])
}

// -------------------------------------------------------------------------------------
// Generic Helpers
// -------------------------------------------------------------------------------------

func wrapString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func wrapInt64(d int64) *int64 {
	if d == 0 {
		return nil
	}
	return &d
}

// TODO: Migrate callers to use simulate instead.
func simulateInternal(ctx cosmos.Context, mgr *Mgrs, msg sdk.Msg) (sdk.Events, error) {
	// validate
	err := msg.ValidateBasic()
	if err != nil {
		return nil, fmt.Errorf("failed validate: %w", err)
	}

	// intercept events and avoid modifying state
	cms := ctx.MultiStore().CacheMultiStore() // never call cms.Write()
	em := cosmos.NewEventManager()
	ctx = ctx.WithMultiStore(cms).WithEventManager(em)

	// disable logging
	ctx = ctx.WithLogger(nullLogger)

	// simulate the message handler
	_, err = NewInternalHandler(mgr)(ctx, msg)
	return em.Events(), err
}

func eventMap(e sdk.Event) map[string]string {
	m := map[string]string{}
	m["type"] = e.Type
	for _, a := range e.Attributes {
		m[string(a.Key)] = string(a.Value)
	}
	return m
}

func simulate(ctx cosmos.Context, mgr *Mgrs, msg sdk.Msg) (sdk.Events, error) {
	// use the first active node account as the signer
	nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, fmt.Errorf("no active node accounts: %w", err)
	}

	// set the signer
	switch m := msg.(type) {
	case *MsgLoanOpen:
		m.Signer = nodeAccounts[0].NodeAddress
	case *MsgLoanRepayment:
		m.Signer = nodeAccounts[0].NodeAddress
	}

	// set random txid
	txid := common.TxID(common.RandStringBytesMask(64))
	ctx = ctx.WithValue(constants.CtxLoanTxID, txid)

	// validate
	err = msg.ValidateBasic()
	if err != nil {
		return nil, fmt.Errorf("failed to validate message: %w", err)
	}

	// intercept events and avoid modifying state
	cms := ctx.MultiStore().CacheMultiStore() // never call cms.Write()
	em := cosmos.NewEventManager()
	ctx = ctx.WithMultiStore(cms).WithEventManager(em)

	// disable logging
	// ctx = ctx.WithLogger(nullLogger)

	// reset the swap queue
	iter := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {

		// unmarshal message
		var qm MsgSwap
		err := mgr.cdc.Unmarshal(iter.Value(), &qm)
		if err != nil {
			continue
		}

		// split txid and index
		ss := strings.Split(string(iter.Key()), "-")
		i, err := strconv.Atoi(ss[len(ss)-1])
		if err != nil {
			continue
		}

		// remove from queue
		mgr.Keeper().RemoveSwapQueueItem(ctx, common.TxID(ss[0]), i)
	}

	// simulate the loan open
	_, err = NewInternalHandler(mgr)(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to simulate loan: %w", err)
	}

	// simulate end block
	err = mgr.swapQ.EndBlock(ctx, mgr)
	if err != nil {
		return nil, fmt.Errorf("failed to simulate end block: %w", err)
	}

	return em.Events(), nil
}
