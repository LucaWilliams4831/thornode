package thorchain

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	abci "github.com/tendermint/tendermint/abci/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

// NewGenesisState create a new instance of GenesisState
func NewGenesisState() GenesisState {
	return GenesisState{}
}

// ValidateGenesis validate genesis is valid or not
func ValidateGenesis(data GenesisState) error {
	for _, record := range data.Pools {
		if err := record.Valid(); err != nil {
			return err
		}
	}

	for _, voter := range data.ObservedTxInVoters {
		if err := voter.Valid(); err != nil {
			return err
		}
	}

	for _, voter := range data.ObservedTxOutVoters {
		if err := voter.Valid(); err != nil {
			return err
		}
	}

	for _, out := range data.TxOuts {
		if err := out.Valid(); err != nil {
			return err
		}
	}

	for _, ta := range data.NodeAccounts {
		if err := ta.Valid(); err != nil {
			return err
		}
	}

	for _, vault := range data.Vaults {
		if err := vault.Valid(); err != nil {
			return err
		}
	}

	if data.LastSignedHeight < 0 {
		return errors.New("last signed height cannot be negative")
	}
	for _, c := range data.LastChainHeights {
		if c.Height < 0 {
			return fmt.Errorf("invalid chain(%s) height", c.Chain)
		}
	}

	for _, item := range data.MsgSwaps {
		if err := item.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid swap msg: %w", err)
		}
	}
	for _, nf := range data.NetworkFees {
		if err := nf.Valid(); err != nil {
			return fmt.Errorf("invalid network fee: %w", err)
		}
	}

	for _, cc := range data.ChainContracts {
		if cc.IsEmpty() {
			return fmt.Errorf("chain contract cannot be empty")
		}
	}

	for _, n := range data.THORNames {
		if len(n.Name) > 30 {
			return errors.New("THORName cannot exceed 30 characters")
		}
		if !IsValidTHORNameV1(n.Name) {
			return errors.New("invalid THORName")
		}
	}

	for _, loan := range data.Loans {
		if err := loan.Valid(); err != nil {
			return fmt.Errorf("invalid loan: %w", err)
		}
	}

	return nil
}

// DefaultGenesisState the default values THORNode put in the Genesis
func DefaultGenesisState() GenesisState {
	return GenesisState{
		Pools:               make([]Pool, 0),
		NodeAccounts:        NodeAccounts{},
		BondProviders:       make([]BondProviders, 0),
		TxOuts:              make([]TxOut, 0),
		LiquidityProviders:  make(LiquidityProviders, 0),
		Vaults:              make(Vaults, 0),
		ObservedTxInVoters:  make(ObservedTxVoters, 0),
		ObservedTxOutVoters: make(ObservedTxVoters, 0),
		LastSignedHeight:    0,
		LastChainHeights:    make([]LastChainHeight, 0),
		Network:             NewNetwork(),
		POL:                 NewProtocolOwnedLiquidity(),
		MsgSwaps:            make([]MsgSwap, 0),
		NetworkFees:         make([]NetworkFee, 0),
		ChainContracts:      make([]ChainContract, 0),
		THORNames:           make([]THORName, 0),
		StoreVersion:        38, // refer to func `GetStoreVersion` , let's keep it consistent
		Loans:               make([]Loan, 0),
	}
}

// InitGenesis read the data in GenesisState and apply it to data store
func InitGenesis(ctx cosmos.Context, keeper keeper.Keeper, data GenesisState) []abci.ValidatorUpdate {
	for _, record := range data.Pools {
		if err := keeper.SetPool(ctx, record); err != nil {
			panic(err)
		}
	}

	for _, lp := range data.LiquidityProviders {
		keeper.SetLiquidityProvider(ctx, lp)
	}

	validators := make([]abci.ValidatorUpdate, 0, len(data.NodeAccounts))
	for _, nodeAccount := range data.NodeAccounts {
		if nodeAccount.Status == NodeActive {
			// Only Active node will become validator
			pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeConsPub, nodeAccount.ValidatorConsPubKey)
			if err != nil {
				ctx.Logger().Error("fail to parse consensus public key", "key", nodeAccount.ValidatorConsPubKey, "error", err)
				panic(err)
			}
			validators = append(validators, abci.Ed25519ValidatorUpdate(pk.Bytes(), 100))
		}

		if err := keeper.SetNodeAccount(ctx, nodeAccount); err != nil {
			// we should panic
			panic(err)
		}
	}

	for _, vault := range data.Vaults {
		if err := keeper.SetVault(ctx, vault); err != nil {
			panic(err)
		}
	}

	for _, bp := range data.BondProviders {
		if err := keeper.SetBondProviders(ctx, bp); err != nil {
			panic(err)
		}
	}

	for _, voter := range data.ObservedTxInVoters {
		keeper.SetObservedTxInVoter(ctx, voter)
	}

	for _, voter := range data.ObservedTxOutVoters {
		keeper.SetObservedTxOutVoter(ctx, voter)
	}

	for idx := range data.TxOuts {
		if err := keeper.SetTxOut(ctx, &data.TxOuts[idx]); err != nil {
			ctx.Logger().Error("fail to save tx out during genesis", "error", err)
			panic(err)
		}
	}

	if data.LastSignedHeight > 0 {
		if err := keeper.SetLastSignedHeight(ctx, data.LastSignedHeight); err != nil {
			panic(err)
		}
	}

	for _, c := range data.LastChainHeights {
		chain, err := common.NewChain(c.Chain)
		if err != nil {
			panic(err)
		}
		if err := keeper.SetLastChainHeight(ctx, chain, c.Height); err != nil {
			panic(err)
		}
	}
	if err := keeper.SetNetwork(ctx, data.Network); err != nil {
		panic(err)
	}

	// FORK TODO: uncomment this on next fork
	// if err := keeper.SetPOL(ctx, data.POL); err != nil {
	// panic(err)
	// }

	for _, item := range data.MsgSwaps {
		if err := keeper.SetOrderBookItem(ctx, item); err != nil {
			panic(err)
		}
	}
	for _, nf := range data.NetworkFees {
		if err := keeper.SaveNetworkFee(ctx, nf.Chain, nf); err != nil {
			panic(err)
		}
	}

	for _, cc := range data.ChainContracts {
		keeper.SetChainContract(ctx, cc)
	}

	for _, n := range data.THORNames {
		keeper.SetTHORName(ctx, n)
	}

	for _, loan := range data.Loans {
		keeper.SetLoan(ctx, loan)
	}

	// Mint coins into the reserve
	if data.Reserve > 0 {
		coin := common.NewCoin(common.RuneNative, cosmos.NewUint(data.Reserve))
		if err := keeper.MintToModule(ctx, ModuleName, coin); err != nil {
			panic(err)
		}
		if err := keeper.SendFromModuleToModule(ctx, ModuleName, ReserveName, common.NewCoins(coin)); err != nil {
			panic(err)
		}
	}

	for _, admin := range ADMINS {
		addr, err := cosmos.AccAddressFromBech32(admin)
		if err != nil {
			panic(err)
		}
		mimir, _ := common.NewAsset("THOR.MIMIR")
		coin := common.NewCoin(mimir, cosmos.NewUint(1000*common.One))
		// mint some gas asset
		err = keeper.MintToModule(ctx, ModuleName, coin)
		if err != nil {
			panic(err)
		}
		if err := keeper.SendFromModuleToAccount(ctx, ModuleName, addr, common.NewCoins(coin)); err != nil {
			panic(err)
		}
	}
	for _, item := range data.Mimirs {
		if len(item.Key) == 0 {
			continue
		}
		keeper.SetMimir(ctx, item.Key, item.Value)
	}
	keeper.SetStoreVersion(ctx, data.StoreVersion)
	reserveAddr, _ := keeper.GetModuleAddress(ReserveName)
	ctx.Logger().Info("Reserve Module", "address", reserveAddr.String())
	bondAddr, _ := keeper.GetModuleAddress(BondName)
	ctx.Logger().Info("Bond    Module", "address", bondAddr.String())
	asgardAddr, _ := keeper.GetModuleAddress(AsgardName)
	ctx.Logger().Info("Asgard  Module", "address", asgardAddr.String())

	return validators
}

func getLiquidityProviders(ctx cosmos.Context, k keeper.Keeper, asset common.Asset) LiquidityProviders {
	liquidityProviders := make(LiquidityProviders, 0)
	iterator := k.GetLiquidityProviderIterator(ctx, asset)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var lp LiquidityProvider
		k.Cdc().MustUnmarshal(iterator.Value(), &lp)
		if lp.Units.IsZero() && lp.PendingRune.IsZero() && lp.PendingAsset.IsZero() {
			continue
		}
		liquidityProviders = append(liquidityProviders, lp)
	}
	return liquidityProviders
}

func getValidPools(ctx cosmos.Context, k keeper.Keeper) Pools {
	var pools Pools
	iterator := k.GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		k.Cdc().MustUnmarshal(iterator.Value(), &pool)
		if pool.IsEmpty() {
			continue
		}
		if pool.Status == PoolSuspended {
			continue
		}
		pools = append(pools, pool)
	}
	return pools
}

// ExportGenesis export the data in Genesis
func ExportGenesis(ctx cosmos.Context, k keeper.Keeper) GenesisState {
	var iterator cosmos.Iterator
	pools := getValidPools(ctx, k)
	var liquidityProviders LiquidityProviders
	for _, pool := range pools {
		liquidityProviders = append(liquidityProviders, getLiquidityProviders(ctx, k, pool.Asset)...)
	}

	var nodeAccounts NodeAccounts
	iterator = k.GetNodeAccountIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var na NodeAccount
		k.Cdc().MustUnmarshal(iterator.Value(), &na)
		if na.IsEmpty() {
			continue
		}
		if na.Bond.IsZero() {
			continue
		}
		nodeAccounts = append(nodeAccounts, na)
	}

	bps := make([]BondProviders, 0)
	for _, na := range nodeAccounts {
		bp, err := k.GetBondProviders(ctx, na.NodeAddress)
		if err != nil {
			panic(err)
		}
		bps = append(bps, bp)
	}

	var observedTxInVoters ObservedTxVoters
	var outs []TxOut
	startBlockHeight := ctx.BlockHeight()
	endBlockHeight := ctx.BlockHeight() + 17200

	for height := startBlockHeight; height < endBlockHeight; height++ {
		txOut, err := k.GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out", "error", err, "height", height)
			continue
		}
		if txOut.IsEmpty() {
			continue
		}
		includeTxOut := false
		for _, item := range txOut.TxArray {
			if item.OutHash.IsEmpty() {
				includeTxOut = true
			}
			if item.InHash.IsEmpty() || item.InHash.Equals(common.BlankTxID) {
				continue
			}
			txInVoter, err := k.GetObservedTxInVoter(ctx, item.InHash)
			if err != nil {
				ctx.Logger().Error("fail to get observed tx in", "error", err, "hash", item.InHash.String())
				continue
			}
			observedTxInVoters = append(observedTxInVoters, txInVoter)
		}
		if includeTxOut {
			outs = append(outs, *txOut)
		}
	}

	lastSignedHeight, err := k.GetLastSignedHeight(ctx)
	if err != nil {
		panic(err)
	}

	chainHeights, err := k.GetLastChainHeights(ctx)
	if err != nil {
		panic(err)
	}
	lastChainHeights := make([]LastChainHeight, 0)
	// analyze-ignore(map-iteration)
	for k, v := range chainHeights {
		lastChainHeights = append(lastChainHeights, LastChainHeight{
			Chain:  k.String(),
			Height: v,
		})
	}
	// Let's sort it , so it is deterministic
	sort.Slice(lastChainHeights, func(i, j int) bool {
		return lastChainHeights[i].Chain < lastChainHeights[j].Chain
	})
	network, err := k.GetNetwork(ctx)
	if err != nil {
		panic(err)
	}

	pol, err := k.GetPOL(ctx)
	if err != nil {
		panic(err)
	}

	vaults := make(Vaults, 0)
	iterVault := k.GetVaultIterator(ctx)
	defer iterVault.Close()
	for ; iterVault.Valid(); iterVault.Next() {
		var vault Vault
		k.Cdc().MustUnmarshal(iterVault.Value(), &vault)
		if vault.Status == types.VaultStatus_InactiveVault || vault.Status == types.VaultStatus_InitVault {
			continue
		}
		vaults = append(vaults, vault)
	}

	swapMsgs := make([]MsgSwap, 0)
	iterMsgSwap := k.GetOrderBookItemIterator(ctx)
	defer iterMsgSwap.Close()
	for ; iterMsgSwap.Valid(); iterMsgSwap.Next() {
		var m MsgSwap
		k.Cdc().MustUnmarshal(iterMsgSwap.Value(), &m)
		swapMsgs = append(swapMsgs, m)
	}

	networkFees := make([]NetworkFee, 0)
	iterNetworkFee := k.GetNetworkFeeIterator(ctx)
	defer iterNetworkFee.Close()
	for ; iterNetworkFee.Valid(); iterNetworkFee.Next() {
		var nf NetworkFee
		k.Cdc().MustUnmarshal(iterNetworkFee.Value(), &nf)
		networkFees = append(networkFees, nf)
	}

	chainContracts := make([]ChainContract, 0)
	iter := k.GetChainContractIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var cc ChainContract
		k.Cdc().MustUnmarshal(iter.Value(), &cc)
		chainContracts = append(chainContracts, cc)
	}

	names := make([]THORName, 0)
	iterNames := k.GetTHORNameIterator(ctx)
	defer iterNames.Close()
	for ; iterNames.Valid(); iterNames.Next() {
		var n THORName
		k.Cdc().MustUnmarshal(iterNames.Value(), &n)
		names = append(names, n)
	}
	mimirs := make([]Mimir, 0)
	mimirIter := k.GetMimirIterator(ctx)
	defer mimirIter.Close()
	for ; mimirIter.Valid(); mimirIter.Next() {
		value := types.ProtoInt64{}
		if err := k.Cdc().Unmarshal(mimirIter.Value(), &value); err != nil {
			ctx.Logger().Error("fail to unmarshal mimir value", "error", err)
			continue
		}
		mimirs = append(mimirs, Mimir{
			Key:   strings.ReplaceAll(string(mimirIter.Key()), "mimir//", ""),
			Value: value.GetValue(),
		})
	}
	storeVersion := k.GetStoreVersion(ctx)

	// collect all assets
	seenAssets := make(map[common.Asset]bool)
	assets := make([]common.Asset, 0)
	for _, vault := range vaults {
		for _, coin := range vault.Coins {
			if !seenAssets[coin.Asset] {
				seenAssets[coin.Asset] = true
				assets = append(assets, coin.Asset)
			}
		}
	}

	// export loans from all assets
	loans := make([]Loan, 0)
	for _, asset := range assets {
		loanIter := k.GetLoanIterator(ctx, asset)
		defer loanIter.Close()
		for ; loanIter.Valid(); loanIter.Next() {
			var loan Loan
			k.Cdc().MustUnmarshal(loanIter.Value(), &loan)
			loans = append(loans, loan)
		}
	}

	return GenesisState{
		Pools:              pools,
		LiquidityProviders: liquidityProviders,
		ObservedTxInVoters: observedTxInVoters,
		TxOuts:             outs,
		NodeAccounts:       nodeAccounts,
		BondProviders:      bps,
		Vaults:             vaults,
		LastSignedHeight:   lastSignedHeight,
		LastChainHeights:   lastChainHeights,
		Network:            network,
		POL:                pol,
		MsgSwaps:           swapMsgs,
		NetworkFees:        networkFees,
		ChainContracts:     chainContracts,
		THORNames:          names,
		Loans:              loans,
		Mimirs:             mimirs,
		StoreVersion:       storeVersion,
	}
}
