package keeperv1

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

// NOTE: Always end a dbPrefix with a slash ("/"). This is to ensure that there
// are no prefixes that contain another prefix. In the scenario where this is
// true, an iterator for a specific type, will get more than intended, and may
// include a different type. The slash is used to protect us from this
// scenario.
// Also, use underscores between words and use lowercase characters only

const (
	prefixStoreVersion            types.DbPrefix = "_ver/"
	prefixObservedTxIn            types.DbPrefix = "observed_tx_in/"
	prefixObservedTxOut           types.DbPrefix = "observed_tx_out/"
	prefixObservedLink            types.DbPrefix = "ob_link/"
	prefixPool                    types.DbPrefix = "pool/"
	prefixPoolLUVI                types.DbPrefix = "luvi/"
	prefixTxOut                   types.DbPrefix = "txout/"
	prefixTotalLiquidityFee       types.DbPrefix = "total_liquidity_fee/"
	prefixPoolLiquidityFee        types.DbPrefix = "pool_liquidity_fee/"
	prefixPoolSwapSlip            types.DbPrefix = "pool_swap_slip/"
	prefixLiquidityProvider       types.DbPrefix = "lp/"
	prefixLastChainHeight         types.DbPrefix = "last_chain_height/"
	prefixLastSignedHeight        types.DbPrefix = "last_signed_height/"
	prefixLastObserveHeight       types.DbPrefix = "last_observe_height/"
	prefixNodeAccount             types.DbPrefix = "node_account/"
	prefixBondProviders           types.DbPrefix = "bond_providers/"
	prefixVault                   types.DbPrefix = "vault/"
	prefixVaultAsgardIndex        types.DbPrefix = "vault_asgard_index/"
	prefixNetwork                 types.DbPrefix = "network/"
	prefixPOL                     types.DbPrefix = "pol/"
	prefixLoan                    types.DbPrefix = "loan/"
	prefixLoanTotalCollateral     types.DbPrefix = "loan_col_total/"
	prefixObservingAddresses      types.DbPrefix = "observing_addresses/"
	prefixTss                     types.DbPrefix = "tss/"
	prefixTssKeysignFailure       types.DbPrefix = "tssKeysignFailure/"
	prefixKeygen                  types.DbPrefix = "keygen/"
	prefixRagnarokHeight          types.DbPrefix = "ragnarokHeight/"
	prefixRagnarokNth             types.DbPrefix = "ragnarokNth/"
	prefixRagnarokPending         types.DbPrefix = "ragnarokPending/"
	prefixRagnarokPosition        types.DbPrefix = "ragnarokPosition/"
	prefixRagnarokPoolHeight      types.DbPrefix = "ragnarokPool/"
	prefixErrataTx                types.DbPrefix = "errata/"
	prefixBanVoter                types.DbPrefix = "ban/"
	prefixNodeSlashPoints         types.DbPrefix = "slash/"
	prefixNodeJail                types.DbPrefix = "jail/"
	prefixSwapQueueItem           types.DbPrefix = "swapitem/"
	prefixOrderBookItem           types.DbPrefix = "o/"
	prefixOrderBookLimitIndex     types.DbPrefix = "olim/"
	prefixOrderBookMarketIndex    types.DbPrefix = "omark/"
	prefixOrderBookProcessor      types.DbPrefix = "oproc/"
	prefixMimir                   types.DbPrefix = "mimir/"
	prefixMinJoinLast             types.DbPrefix = "minjoinlast/"
	prefixNodeMimir               types.DbPrefix = "nodemimir/"
	prefixNodePauseChain          types.DbPrefix = "node_pause_chain/"
	prefixNetworkFee              types.DbPrefix = "network_fee/"
	prefixNetworkFeeVoter         types.DbPrefix = "network_fee_voter/"
	prefixTssKeygenMetric         types.DbPrefix = "tss_keygen_metric/"
	prefixTssKeysignMetric        types.DbPrefix = "tss_keysign_metric/"
	prefixTssKeysignMetricLatest  types.DbPrefix = "latest_tss_keysign_metric/"
	prefixChainContract           types.DbPrefix = "chain_contract/"
	prefixSolvencyVoter           types.DbPrefix = "solvency_voter/"
	prefixTHORName                types.DbPrefix = "thorname/"
	prefixRollingPoolLiquidityFee types.DbPrefix = "rolling_pool_liquidity_fee/"
	prefixVersion                 types.DbPrefix = "version/"
)

func dbError(ctx cosmos.Context, wrapper string, err error) error {
	err = fmt.Errorf("KVStore Error: %s: %w", wrapper, err)
	ctx.Logger().Error("keeper error", "error", err)
	return err
}

// KVStore Keeper maintains the link to data storage and exposes getter/setter methods for the various parts of the state machine
type KVStore struct {
	cdc           codec.BinaryCodec
	coinKeeper    bankkeeper.Keeper
	accountKeeper authkeeper.AccountKeeper
	storeKey      cosmos.StoreKey // Unexposed key to access store from cosmos.Context
	version       semver.Version
	constAccessor constants.ConstantValues
}

// NewKVStore creates new instances of the thorchain Keeper
func NewKVStore(cdc codec.BinaryCodec, coinKeeper bankkeeper.Keeper, accountKeeper authkeeper.AccountKeeper, storeKey cosmos.StoreKey, version semver.Version) KVStore {
	return KVStore{
		coinKeeper:    coinKeeper,
		accountKeeper: accountKeeper,
		storeKey:      storeKey,
		cdc:           cdc,
		version:       version,
		constAccessor: constants.GetConstantValues(version),
	}
}

// Cdc return the amino codec
func (k KVStore) Cdc() codec.BinaryCodec {
	return k.cdc
}

// GetVersion return the current version
func (k KVStore) GetVersion() semver.Version {
	return k.version
}

func (k *KVStore) SetVersion(ver semver.Version) {
	k.version = ver
}

// GetKey return a key that can be used to store into key value store
func (k KVStore) GetKey(ctx cosmos.Context, prefix types.DbPrefix, key string) string {
	return fmt.Sprintf("%s/%s", prefix, strings.ToUpper(key))
}

// SetStoreVersion save the store version
func (k KVStore) SetStoreVersion(ctx cosmos.Context, value int64) {
	key := k.GetKey(ctx, prefixStoreVersion, "")
	store := ctx.KVStore(k.storeKey)
	ver := ProtoInt64{Value: value}
	store.Set([]byte(key), k.cdc.MustMarshal(&ver))
}

// GetStoreVersion get the current key value store version
func (k KVStore) GetStoreVersion(ctx cosmos.Context) int64 {
	key := k.GetKey(ctx, prefixStoreVersion, "")
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		// thornode start at version 0.38.0, thus when there is no store version , it return 38
		return 38
	}
	var ver ProtoInt64
	buf := store.Get([]byte(key))
	k.cdc.MustUnmarshal(buf, &ver)
	return ver.Value
}

// getIterator - get an iterator for given prefix
func (k KVStore) getIterator(ctx cosmos.Context, prefix types.DbPrefix) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefix))
}

// del - delete data from the kvstore
func (k KVStore) del(ctx cosmos.Context, key string) {
	store := ctx.KVStore(k.storeKey)
	if store.Has([]byte(key)) {
		store.Delete([]byte(key))
	}
}

// has - kvstore has key
func (k KVStore) has(ctx cosmos.Context, key string) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has([]byte(key))
}

func (k KVStore) setInt64(ctx cosmos.Context, key string, record int64) {
	store := ctx.KVStore(k.storeKey)
	value := ProtoInt64{Value: record}
	buf := k.cdc.MustMarshal(&value)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getInt64(ctx cosmos.Context, key string, record *int64) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	value := ProtoInt64{}
	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	*record = value.GetValue()
	return true, nil
}

func (k KVStore) setUint64(ctx cosmos.Context, key string, record uint64) {
	store := ctx.KVStore(k.storeKey)
	value := ProtoUint64{Value: record}
	buf := k.cdc.MustMarshal(&value)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getUint64(ctx cosmos.Context, key string, record *uint64) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	value := ProtoUint64{Value: *record}
	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	*record = value.GetValue()
	return true, nil
}

func (k KVStore) setAccAddresses(ctx cosmos.Context, key string, record []cosmos.AccAddress) {
	store := ctx.KVStore(k.storeKey)
	value := ProtoAccAddresses{Value: record}
	buf := k.cdc.MustMarshal(&value)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getAccAddresses(ctx cosmos.Context, key string, record *[]cosmos.AccAddress) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	var value ProtoAccAddresses
	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	*record = value.Value
	return true, nil
}

func (k KVStore) setStrings(ctx cosmos.Context, key string, record []string) {
	store := ctx.KVStore(k.storeKey)
	value := ProtoStrings{Value: record}
	buf := k.cdc.MustMarshal(&value)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getStrings(ctx cosmos.Context, key string, record *[]string) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	var value ProtoStrings
	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	*record = value.Value
	return true, nil
}

func (k KVStore) setUint(ctx cosmos.Context, key string, record cosmos.Uint) {
	store := ctx.KVStore(k.storeKey)
	value := ProtoUint{Value: record}
	buf := k.cdc.MustMarshal(&value)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getUint(ctx cosmos.Context, key string, record *cosmos.Uint) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	var value ProtoUint
	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return false, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	*record = value.Value
	return true, nil
}

func (k KVStore) setBools(ctx cosmos.Context, key string, record []bool) {
	store := ctx.KVStore(k.storeKey)
	value := ProtoBools{Value: record}
	buf := k.cdc.MustMarshal(&value)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getBools(ctx cosmos.Context, key string, record *[]bool) (bool, error) {
	store := ctx.KVStore(k.storeKey)

	var value ProtoBools
	bz := store.Get([]byte(key))
	if bz == nil {
		return false, nil
	}
	if err := k.cdc.Unmarshal(bz, &value); err != nil {
		return false, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	if record == nil {
		return false, nil
	}
	*record = value.Value
	return true, nil
}

// GetRuneBalanceOfModule get the RUNE balance
func (k KVStore) GetRuneBalanceOfModule(ctx cosmos.Context, moduleName string) cosmos.Uint {
	return k.GetBalanceOfModule(ctx, moduleName, common.RuneNative.Native())
}

func (k KVStore) GetBalanceOfModule(ctx cosmos.Context, moduleName, denom string) cosmos.Uint {
	addr := k.accountKeeper.GetModuleAddress(moduleName)
	coin := k.coinKeeper.GetBalance(ctx, addr, denom)
	return cosmos.NewUintFromBigInt(coin.Amount.BigInt())
}

// SendFromModuleToModule transfer asset from one module to another
func (k KVStore) SendFromModuleToModule(ctx cosmos.Context, from, to string, coins common.Coins) error {
	cosmosCoins := make(cosmos.Coins, len(coins))
	for i, c := range coins {
		cosmosCoins[i] = cosmos.NewCoin(c.Asset.Native(), cosmos.NewIntFromBigInt(c.Amount.BigInt()))
	}
	return k.coinKeeper.SendCoinsFromModuleToModule(ctx, from, to, cosmosCoins)
}

func (k KVStore) SendCoins(ctx cosmos.Context, from, to cosmos.AccAddress, coins cosmos.Coins) error {
	return k.coinKeeper.SendCoins(ctx, from, to, coins)
}

func (k KVStore) AddCoins(ctx cosmos.Context, addr cosmos.AccAddress, coins cosmos.Coins) error {
	return k.coinKeeper.SendCoinsFromModuleToAccount(ctx, ModuleName, addr, coins)
}

// SendFromAccountToModule transfer fund from one account to a module
func (k KVStore) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coins common.Coins) error {
	cosmosCoins := make(cosmos.Coins, len(coins))
	for i, c := range coins {
		cosmosCoins[i] = cosmos.NewCoin(c.Asset.Native(), cosmos.NewIntFromBigInt(c.Amount.BigInt()))
	}
	return k.coinKeeper.SendCoinsFromAccountToModule(ctx, from, to, cosmosCoins)
}

// SendFromModuleToAccount transfer fund from module to an account
func (k KVStore) SendFromModuleToAccount(ctx cosmos.Context, from string, to cosmos.AccAddress, coins common.Coins) error {
	cosmosCoins := make(cosmos.Coins, len(coins))
	for i, c := range coins {
		cosmosCoins[i] = cosmos.NewCoin(c.Asset.Native(), cosmos.NewIntFromBigInt(c.Amount.BigInt()))
	}
	return k.coinKeeper.SendCoinsFromModuleToAccount(ctx, from, to, cosmosCoins)
}

func (k KVStore) BurnFromModule(ctx cosmos.Context, module string, coin common.Coin) error {
	coinToBurn, err := coin.Native()
	if err != nil {
		return fmt.Errorf("fail to parse coins: %w", err)
	}
	coinsToBurn := cosmos.Coins{coinToBurn}
	err = k.coinKeeper.BurnCoins(ctx, module, coinsToBurn)
	if err != nil {
		return fmt.Errorf("fail to burn assets: %w", err)
	}

	return nil
}

func (k KVStore) MintToModule(ctx cosmos.Context, module string, coin common.Coin) error {
	version := k.GetVersion()
	if version.GTE(semver.MustParse("1.111.0")) {
		// circuit breaker
		// mint new rune coins until we hit the cap (500m). Once we do, borrow
		// from the reserve instead of minting new tokens
		maxAmt, _ := k.GetMimir(ctx, constants.MaxRuneSupply.String())
		if coin.Asset.IsNativeRune() && maxAmt > 0 {
			currentSupply := k.GetTotalSupply(ctx, common.RuneAsset())  // current circulating supply of rune
			maxSupply := cosmos.NewUint(uint64(maxAmt))                 // max supply of rune (ie 500m)
			availableSupply := common.SafeSub(maxSupply, currentSupply) // available supply to be mint
			// if available supply is less than the coin.Amount, we need to
			// borrow from the reserve
			if availableSupply.LT(coin.Amount) {
				// Never mint an amount that would exceed MaxRuneSupply.
				borrowReserveAmt := common.SafeSub(coin.Amount, availableSupply) // to borrow from reserve
				coin.Amount = common.SafeSub(coin.Amount, borrowReserveAmt)      // to mint later in this func

				reserveCoin := common.NewCoin(common.RuneAsset(), borrowReserveAmt)
				if err := k.SendFromModuleToModule(ctx, ReserveName, module, common.NewCoins(reserveCoin)); err != nil {
					// If unable to move the needed surplus coin from the Reserve, error out without any minting.
					ctx.Logger().Error("fail to move coins during circuit breaker", "error", err)
					return err
				}
			}
		}
		if coin.Amount.IsZero() {
			// Don't proceed if the remaining amount to mint is zero.
			return nil
		}
	}

	coinToMint, err := coin.Native()
	if err != nil {
		return fmt.Errorf("fail to parse coins: %w", err)
	}
	coinsToMint := cosmos.Coins{coinToMint}
	err = k.coinKeeper.MintCoins(ctx, module, coinsToMint)
	if err != nil {
		return fmt.Errorf("fail to mint assets: %w", err)
	}

	if version.GTE(semver.MustParse("1.95.0")) {
		// check if we've exceeded max rune supply cap. If we have, there could
		// be an issue (infinite mint bug/exploit), or maybe runway rune
		// hyperinflation. In any case, pause everything and allow the
		// community time to find a solution to the issue.
		coin := k.coinKeeper.GetSupply(ctx, common.RuneAsset().Native())
		maxAmt, _ := k.GetMimir(ctx, "MaxRuneSupply")
		if maxAmt > 0 && coin.Amount.GT(cosmos.NewInt(maxAmt)) {
			k.SetMimir(ctx, "HaltTrading", 1)
			k.SetMimir(ctx, "HaltChainGlobal", 1)
			k.SetMimir(ctx, "PauseLP", 1)
			k.SetMimir(ctx, "HaltTHORChain", 1)
		}
	}

	return nil
}

func (k KVStore) MintAndSendToAccount(ctx cosmos.Context, to cosmos.AccAddress, coin common.Coin) error {
	// Mint coins into the reserve
	if err := k.MintToModule(ctx, ModuleName, coin); err != nil {
		return err
	}
	return k.SendFromModuleToAccount(ctx, ModuleName, to, common.NewCoins(coin))
}

func (k KVStore) GetModuleAddress(module string) (common.Address, error) {
	return common.NewAddress(k.accountKeeper.GetModuleAddress(module).String())
}

func (k KVStore) GetModuleAccAddress(module string) cosmos.AccAddress {
	return k.accountKeeper.GetModuleAddress(module)
}

func (k KVStore) GetBalance(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Coins {
	return k.coinKeeper.GetAllBalances(ctx, addr)
}

func (k KVStore) HasCoins(ctx cosmos.Context, addr cosmos.AccAddress, coins cosmos.Coins) bool {
	balance := k.coinKeeper.GetAllBalances(ctx, addr)
	return balance.IsAllGTE(coins)
}

func (k KVStore) GetAccount(ctx cosmos.Context, addr cosmos.AccAddress) cosmos.Account {
	return k.accountKeeper.GetAccount(ctx, addr)
}

func (k KVStore) GetNativeTxFee(ctx cosmos.Context) cosmos.Uint {
	if k.usdFeesEnabled(ctx) {
		return k.DollarConfigInRune(ctx, constants.NativeTransactionFeeUSD)
	}
	fee := k.GetConfigInt64(ctx, constants.NativeTransactionFee)
	return cosmos.NewUint(uint64(fee))
}

func (k KVStore) GetOutboundTxFee(ctx cosmos.Context) cosmos.Uint {
	if k.usdFeesEnabled(ctx) {
		return k.DollarConfigInRune(ctx, constants.OutboundTransactionFeeUSD)
	}
	fee := k.GetConfigInt64(ctx, constants.OutboundTransactionFee)
	return cosmos.NewUint(uint64(fee))
}

func (k KVStore) GetTHORNameRegisterFee(ctx cosmos.Context) cosmos.Uint {
	if k.usdFeesEnabled(ctx) {
		return k.DollarConfigInRune(ctx, constants.TNSRegisterFeeUSD)
	}
	fee := k.GetConstants().GetInt64Value(constants.TNSRegisterFee)
	return cosmos.NewUint(uint64(fee))
}

func (k KVStore) GetTHORNamePerBlockFee(ctx cosmos.Context) cosmos.Uint {
	if k.usdFeesEnabled(ctx) {
		return k.DollarConfigInRune(ctx, constants.TNSFeePerBlockUSD)
	}
	fee := k.GetConstants().GetInt64Value(constants.TNSFeePerBlock)
	return cosmos.NewUint(uint64(fee))
}

// DollarConfigInRune returns the dollar denominated config value in RUNE. If the RUNE
// price feed returns zero, the USD value will be returned.
func (k KVStore) DollarConfigInRune(ctx cosmos.Context, value constants.ConstantName) cosmos.Uint {
	usd := cosmos.SafeUintFromInt64(k.GetConfigInt64(ctx, value))
	runeUSDPrice := k.DollarsPerRune(ctx)
	if !runeUSDPrice.IsZero() {
		return usd.MulUint64(common.One).Quo(runeUSDPrice)
	}
	return usd
}

func (k KVStore) usdFeesEnabled(ctx cosmos.Context) bool {
	// TODO on hard fork remove version check
	version, hasVersion := k.GetVersionWithCtx(ctx)
	if !hasVersion || version.LT(semver.MustParse("1.114.0")) {
		return false
	}
	usdFees, _ := k.GetMimir(ctx, constants.EnableUSDFees.String())
	return usdFees > 0
}
