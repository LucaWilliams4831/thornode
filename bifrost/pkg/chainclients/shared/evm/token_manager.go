package evm

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ecommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/syndtr/goleveldb/leveldb"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/evm/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/common/tokenlist"
)

const (
	decimalMethod = "decimals"
	symbolMethod  = "symbol"
)

// TokenManager manages a LevelDB of ERC20 token meta data and interfaces with token smart contracts
type TokenManager struct {
	tokenDb         *LevelDBTokenMeta
	defaultDecimals uint64
	nativeAsset     common.Asset
	requestTimeout  time.Duration
	tokenWhitelist  []tokenlist.ERC20Token
	erc20ABI        *abi.ABI
	vaultABI        *abi.ABI
	client          *ethclient.Client
	logger          zerolog.Logger
}

// NewTokenManager returns an instance of TokenManager
func NewTokenManager(db *leveldb.DB,
	prefixTokenMeta string,
	nativeAsset common.Asset,
	defaultDecimals uint64,
	requestTimeout time.Duration,
	tokenWhitelist []tokenlist.ERC20Token,
	ethClient *ethclient.Client,
	routerContractABI,
	erc20ContractABI string,
) (*TokenManager, error) {
	tokenDb, err := NewLevelDBTokenMeta(db, prefixTokenMeta)
	if err != nil {
		return nil, fmt.Errorf("fail to create tokenDb: %w", err)
	}

	vaultABI, erc20ABI, err := GetContractABI(routerContractABI, erc20ContractABI)
	if err != nil {
		return nil, fmt.Errorf("fail to create contract abi: %w", err)
	}

	if ethClient == nil {
		return nil, errors.New("ETH client is nil")
	}

	return &TokenManager{
		tokenDb:         tokenDb,
		defaultDecimals: defaultDecimals,
		nativeAsset:     nativeAsset,
		erc20ABI:        erc20ABI,
		vaultABI:        vaultABI,
		tokenWhitelist:  tokenWhitelist,
		client:          ethClient,
		requestTimeout:  requestTimeout,
		logger:          log.Logger.With().Str("module", "token_manager").Str("chain", nativeAsset.Chain.String()).Logger(),
	}, nil
}

func (h *TokenManager) getContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), h.requestTimeout)
}

// GetTokens return all the token meta data
func (h *TokenManager) GetTokens() ([]*types.TokenMeta, error) {
	return h.tokenDb.GetTokens()
}

func (h *TokenManager) GetTokenMeta(token string) (types.TokenMeta, error) {
	tokenMeta, err := h.tokenDb.GetTokenMeta(token)
	if err != nil {
		return types.TokenMeta{}, fmt.Errorf("fail to get token meta: %w", err)
	}
	if tokenMeta.IsEmpty() {
		isWhiteListToken := false
		for _, item := range h.tokenWhitelist {
			if strings.EqualFold(item.Address, token) {
				isWhiteListToken = true
				break
			}
		}
		if !isWhiteListToken {
			h.logger.Info().Str("token", token).Msg("TM: token not whitelisted")
			return types.TokenMeta{}, fmt.Errorf("token: %s is not whitelisted", token)
		}
		symbol, err := h.getSymbol(token)
		if err != nil {
			h.logger.Info().Str("token", token).Msg("fail to get symbol")
			return types.TokenMeta{}, fmt.Errorf("fail to get symbol: %w", err)
		}
		decimals, err := h.getDecimals(token)
		if err != nil {
			h.logger.Err(err).Uint64("default decimals", h.defaultDecimals).Msg("failed to get decimals from smart contract, returning default")
		}
		tokenMeta = types.NewTokenMeta(symbol, token, decimals)
		if err = h.tokenDb.SaveTokenMeta(symbol, token, decimals); err != nil {
			h.logger.Info().Str("token", token).Msg("fail to save token meta")
			return types.TokenMeta{}, fmt.Errorf("fail to save token meta: %w", err)
		}
	}
	return tokenMeta, nil
}

func (h *TokenManager) SaveTokenMeta(symbol, address string, decimals uint64) error {
	return h.tokenDb.SaveTokenMeta(symbol, address, decimals)
}

// IsNative returns true if the token address equals the native token address
func IsNative(token string) bool {
	return strings.EqualFold(token, NativeTokenAddr)
}

func (h *TokenManager) GetAssetFromTokenAddress(token string) (common.Asset, error) {
	if IsNative(token) {
		return h.nativeAsset, nil
	}
	tokenMeta, err := h.GetTokenMeta(token)
	if err != nil {
		return common.EmptyAsset, fmt.Errorf("fail to get token meta: %w", err)
	}
	if tokenMeta.IsEmpty() {
		return common.EmptyAsset, fmt.Errorf("token metadata is empty")
	}
	return common.NewAsset(fmt.Sprintf("%s.%s-%s", h.nativeAsset.Chain, tokenMeta.Symbol, strings.ToUpper(tokenMeta.Address)))
}

// convertAmount will convert the amount to 1e8 , the decimals used by THORChain
func (h *TokenManager) ConvertAmount(token string, amt *big.Int) cosmos.Uint {
	if IsNative(token) {
		return cosmos.NewUintFromBigInt(amt).QuoUint64(common.One * 100)
	}
	decimals := h.defaultDecimals
	tokenMeta, err := h.GetTokenMeta(token)
	if err != nil {
		h.logger.Err(err).Str("address", token).Msg("failed to get token meta for token address")
	}
	if !tokenMeta.IsEmpty() {
		decimals = tokenMeta.Decimal
	}
	if decimals != h.defaultDecimals {
		var value big.Int
		amt = amt.Mul(amt, value.Exp(big.NewInt(10), big.NewInt(int64(h.defaultDecimals)), nil))
		amt = amt.Div(amt, value.Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	}
	return cosmos.NewUintFromBigInt(amt).QuoUint64(common.One * 100)
}

// ConvertThorchainAmountToWei converts amt in 1e8 decimals to wei (1e18 decimals)
func (h *TokenManager) ConvertThorchainAmountToWei(amt *big.Int) *big.Int {
	return big.NewInt(0).Mul(amt, big.NewInt(common.One*100))
}

// ConvertSigningAmount converts a value of a token to wei (1e18 decimals)
func (h *TokenManager) ConvertSigningAmount(amt *big.Int, token string) *big.Int {
	// convert 1e8 to 1e18
	amt = h.ConvertThorchainAmountToWei(amt)
	if IsNative(token) {
		return amt
	}
	tm, err := h.GetTokenMeta(token)
	if err != nil {
		h.logger.Err(err).Str("token", token).Msg("failed to get token meta for token")
		return amt
	}

	if tm.Decimal == h.defaultDecimals {
		// when the smart contract is using 1e18 as decimals , that means is based on WEI
		// thus the input amt is correct amount to send out
		return amt
	}
	var value big.Int
	amt = amt.Mul(amt, value.Exp(big.NewInt(10), big.NewInt(int64(tm.Decimal)), nil))
	amt = amt.Div(amt, value.Exp(big.NewInt(10), big.NewInt(int64(h.defaultDecimals)), nil))
	return amt
}

// return value 0 means use the default value which is common.THORChainDecimals, use 1e8 as precision
func (h *TokenManager) GetTokenDecimalsForTHORChain(token string) int64 {
	if IsNative(token) {
		return 0
	}
	tokenMeta, err := h.GetTokenMeta(token)
	if err != nil {
		h.logger.Err(err).Str("token", token).Msg("failed to get token meta for token address")
	}
	if tokenMeta.IsEmpty() {
		return 0
	}
	// when the token's precision is more than THORChain , that's fine , just use THORChainDecimals
	if tokenMeta.Decimal >= common.THORChainDecimals {
		return 0
	}
	return int64(tokenMeta.Decimal)
}

// getDecimals calls the token's contract and retrieves its decimals
func (h *TokenManager) getDecimals(token string) (uint64, error) {
	if IsNative(token) {
		return h.defaultDecimals, nil
	}
	to := ecommon.HexToAddress(token)
	input, err := h.erc20ABI.Pack(decimalMethod)
	if err != nil {
		return h.defaultDecimals, fmt.Errorf("fail to pack decimal method: %w", err)
	}
	ctx, cancel := h.getContext()
	defer cancel()
	res, err := h.client.CallContract(ctx, ethereum.CallMsg{
		To:   &to,
		Data: input,
	}, nil)
	if err != nil {
		return h.defaultDecimals, fmt.Errorf("fail to call smart contract get decimals: %w", err)
	}
	output, err := h.erc20ABI.Unpack(decimalMethod, res)
	if err != nil {
		return h.defaultDecimals, fmt.Errorf("fail to unpack decimal method call result: %w", err)
	}
	switch output[0].(type) {
	case uint8:
		decimals, ok := abi.ConvertType(output[0], new(uint8)).(*uint8)
		if !ok {
			return h.defaultDecimals, fmt.Errorf("dev error: fail to cast uint8")
		}
		return uint64(*decimals), nil
	case *big.Int:
		decimals, ok := abi.ConvertType(output[0], new(*big.Int)).(*big.Int)
		if !ok {
			return h.defaultDecimals, fmt.Errorf("dev error: fail to cast big.Int")
		}
		return decimals.Uint64(), nil
	}
	return h.defaultDecimals, fmt.Errorf("%s is %T fail to parse it", output[0], output[0])
}

// replace the . in symbol to *, and replace the - in symbol to #
// because . and - had been reserved to use in THORChain symbol
var symbolReplacer = strings.NewReplacer(".", "*", "-", "#", `\u0000`, "", "\u0000", "")

func sanitiseSymbol(symbol string) string {
	return symbolReplacer.Replace(symbol)
}

// getSymbol calls the token's contract and retrieves its symbol
func (h *TokenManager) getSymbol(token string) (string, error) {
	if IsNative(token) {
		return h.nativeAsset.Symbol.String(), nil
	}
	to := ecommon.HexToAddress(token)
	input, err := h.erc20ABI.Pack(symbolMethod)
	if err != nil {
		return "", nil
	}
	ctx, cancel := h.getContext()
	defer cancel()
	res, err := h.client.CallContract(ctx, ethereum.CallMsg{
		To:   &to,
		Data: input,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("fail to call to smart contract and get symbol: %w", err)
	}
	var symbol string
	output, err := h.erc20ABI.Unpack(symbolMethod, res)
	if err != nil {
		symbol = string(res)
		h.logger.Err(err).Str("token", token).Str("symbol", symbol).Msg("fail to unpack symbol method call, token address")
		return sanitiseSymbol(symbol), nil
	}
	// nolint
	symbol = *abi.ConvertType(output[0], new(string)).(*string)
	return sanitiseSymbol(symbol), nil
}

// GetBalance call smart contract to find out the balance of the given address and token
func (h *TokenManager) GetBalance(addr, token string, height *big.Int, vaultAddr string) (*big.Int, error) {
	ctx, cancel := h.getContext()
	defer cancel()
	if IsNative(token) {
		return h.client.BalanceAt(ctx, ecommon.HexToAddress(addr), height)
	}
	input, err := h.vaultABI.Pack("vaultAllowance", ecommon.HexToAddress(addr), ecommon.HexToAddress(token))
	if err != nil {
		return nil, fmt.Errorf("fail to create vaultAllowance data to call smart contract")
	}
	h.logger.Debug().Str("vault addr", vaultAddr).Msg("query vault for balance")
	toAddr := ecommon.HexToAddress(vaultAddr)
	res, err := h.client.CallContract(ctx, ethereum.CallMsg{
		From: ecommon.HexToAddress(addr),
		To:   &toAddr,
		Data: input,
	}, height)
	if err != nil {
		return nil, err
	}
	output, err := h.vaultABI.Unpack("vaultAllowance", res)
	if err != nil {
		return nil, err
	}
	value, ok := abi.ConvertType(output[0], new(*big.Int)).(**big.Int)
	if !ok {
		return *value, fmt.Errorf("dev error: unable to get big.Int")
	}
	return *value, nil
}
