// Please put all the test related function to here
package types

import (
	"math/rand"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/tendermint/tendermint/crypto"

	"gitlab.com/thorchain/thornode/cmd"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// GetRandomValidatorNode creates a random validator node account, used for testing
func GetRandomValidatorNode(status NodeStatus) NodeAccount {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s) // #nosec G404 this is a method only used for test purpose
	accts := simtypes.RandomAccounts(r, 1)

	k, _ := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeConsPub, accts[0].PubKey)
	pubKeys := common.PubKeySet{
		Secp256k1: GetRandomPubKey(),
		Ed25519:   GetRandomPubKey(),
	}
	addr, _ := pubKeys.Secp256k1.GetThorAddress()
	bondAddr := common.Address(addr.String())
	na := NewNodeAccount(addr, status, pubKeys, k, cosmos.NewUint(100*common.One), bondAddr, 1)
	na.Version = constants.SWVersion.String()
	if na.Status == NodeStatus_Active {
		na.ActiveBlockHeight = 10
		na.Bond = cosmos.NewUint(1000 * common.One)
	}
	na.IPAddress = "192.168.0.1"
	na.Type = NodeType_TypeValidator

	return na
}

// GetRandomVaultNode creates a random vault node account, used for testing
func GetRandomVaultNode(status NodeStatus) NodeAccount {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s) // #nosec G404 this is a method only used for test purpose
	accts := simtypes.RandomAccounts(r, 1)

	k, _ := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeConsPub, accts[0].PubKey)
	pubKeys := common.PubKeySet{
		Secp256k1: GetRandomPubKey(),
		Ed25519:   GetRandomPubKey(),
	}
	addr, _ := pubKeys.Secp256k1.GetThorAddress()
	bondAddr := common.Address(addr.String())
	na := NewNodeAccount(addr, status, pubKeys, k, cosmos.NewUint(100*common.One), bondAddr, 1)
	na.Version = constants.SWVersion.String()
	if na.Status == NodeStatus_Active {
		na.ActiveBlockHeight = 10
		na.Bond = cosmos.NewUint(1000 * common.One)
	}
	na.IPAddress = "192.168.0.1"
	na.Type = NodeType_TypeVault

	return na
}

func GetRandomObservedTx() ObservedTx {
	return NewObservedTx(GetRandomTx(), 33, GetRandomPubKey(), 33)
}

func GetRandomTxOutItem() TxOutItem {
	return TxOutItem{
		Chain:       common.TERRAChain,
		ToAddress:   GetRandomTERRAAddress(),
		VaultPubKey: GetRandomPubKey(),
		Coin:        common.NewCoin(common.LUNAAsset, cosmos.NewUint(100000)),
		Memo:        "OUT:xyz",
		MaxGas:      common.Gas{common.NewCoin(common.LUNAAsset, cosmos.NewUint(100))},
		InHash:      GetRandomTxHash(),
	}
}

func GetRandomObservedTxVoter() ObservedTxVoter {
	observedTx := GetRandomObservedTx()
	return ObservedTxVoter{
		TxID:    GetRandomTxHash(),
		Tx:      observedTx,
		Height:  10,
		Txs:     ObservedTxs{observedTx},
		Actions: []TxOutItem{GetRandomTxOutItem()},
	}
}

// GetRandomTx
func GetRandomTx() common.Tx {
	return common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{common.NewCoin(common.BNBAsset, cosmos.OneUint())},
		common.Gas{
			{Asset: common.BNBAsset, Amount: cosmos.NewUint(37500)},
		},
		"",
	)
}

// GetRandomBech32Addr is an account address used for test
func GetRandomBech32Addr() cosmos.AccAddress {
	name := common.RandStringBytesMask(10)
	return cosmos.AccAddress(crypto.AddressHash([]byte(name)))
}

func GetRandomBech32ConsensusPubKey() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404 this is a method only used for test purpose
	accts := simtypes.RandomAccounts(r, 1)
	result, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeConsPub, accts[0].PubKey)
	if err != nil {
		panic(err)
	}
	return result
}

// GetRandomRUNEAddress will just create a random rune address used for test purpose
func GetRandomRUNEAddress() common.Address {
	return GetRandomTHORAddress()
}

// GetRandomTHORAddress will just create a random thor address used for test purpose
func GetRandomTHORAddress() common.Address {
	name := common.RandStringBytesMask(10)
	str, _ := common.ConvertAndEncode(cmd.Bech32PrefixAccAddr, crypto.AddressHash([]byte(name)))
	thor, _ := common.NewAddress(str)
	return thor
}

// GetRandomBNBAddress will just create a random bnb address used for test purpose
func GetRandomBNBAddress() common.Address {
	name := common.RandStringBytesMask(10)
	str, _ := common.ConvertAndEncode("tbnb", crypto.AddressHash([]byte(name)))
	bnb, _ := common.NewAddress(str)
	return bnb
}

// GetRandomTERRAAddress will just create a random terra address used for test purpose
func GetRandomTERRAAddress() common.Address {
	name := common.RandStringBytesMask(10)
	str, _ := common.ConvertAndEncode("terra", crypto.AddressHash([]byte(name)))
	terra, _ := common.NewAddress(str)
	return terra
}

// GetRandomETHAddress get a random ETH address for test purpose
func GetRandomETHAddress() common.Address {
	pKey := GetRandomPubKey()
	addr, _ := pKey.GetAddress(common.ETHChain)
	return addr
}

// GetRandomGAIAAddress will just create a random terra address used for test purpose
func GetRandomGAIAAddress() common.Address {
	name := common.RandStringBytesMask(10)
	str, _ := common.ConvertAndEncode("cosmos", crypto.AddressHash([]byte(name)))
	gaia, _ := common.NewAddress(str)
	return gaia
}

func GetRandomBTCAddress() common.Address {
	pubKey := GetRandomPubKey()
	addr, _ := pubKey.GetAddress(common.BTCChain)
	return addr
}

func GetRandomLTCAddress() common.Address {
	pubKey := GetRandomPubKey()
	addr, _ := pubKey.GetAddress(common.LTCChain)
	return addr
}

func GetRandomDOGEAddress() common.Address {
	pubKey := GetRandomPubKey()
	addr, _ := pubKey.GetAddress(common.DOGEChain)
	return addr
}

func GetRandomBCHAddress() common.Address {
	pubKey := GetRandomPubKey()
	addr, _ := pubKey.GetAddress(common.BCHChain)
	return addr
}

// GetRandomTxHash create a random txHash used for test purpose
func GetRandomTxHash() common.TxID {
	txHash, _ := common.NewTxID(common.RandStringBytesMask(64))
	return txHash
}

// GetRandomPubKeySet return a random common.PubKeySet for test purpose
func GetRandomPubKeySet() common.PubKeySet {
	return common.NewPubKeySet(GetRandomPubKey(), GetRandomPubKey())
}

func GetRandomVault() Vault {
	return NewVault(32, VaultStatus_ActiveVault, VaultType_AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
}

func GetRandomYggVault() Vault {
	return NewVault(32, VaultStatus_ActiveVault, VaultType_YggdrasilVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
}

func GetRandomPubKey() common.PubKey {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404
	accts := simtypes.RandomAccounts(r, 1)
	bech32PubKey, _ := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, accts[0].PubKey)
	pk, _ := common.NewPubKey(bech32PubKey)
	return pk
}

// SetupConfigForTest used for test purpose
func SetupConfigForTest() {
	config := cosmos.GetConfig()
	config.SetBech32PrefixForAccount(cmd.Bech32PrefixAccAddr, cmd.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(cmd.Bech32PrefixValAddr, cmd.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(cmd.Bech32PrefixConsAddr, cmd.Bech32PrefixConsPub)
	config.SetCoinType(cmd.THORChainCoinType)
	config.SetPurpose(cmd.THORChainCoinPurpose)
	types.SetCoinDenomRegex(func() string {
		return cmd.DenomRegex
	})
}

// create a codec used only for testing
func MakeTestCodec() *codec.LegacyAmino {
	cdc := codec.NewLegacyAmino()
	banktypes.RegisterLegacyAminoCodec(cdc)
	authtypes.RegisterLegacyAminoCodec(cdc)
	RegisterCodec(cdc)
	cosmos.RegisterCodec(cdc)
	// codec.RegisterCrypto(cdc)
	return cdc
}

// GetCurrentVersion - intended for unit tests, fetches the current version of
// THORNode via `version` file
// #nosec G304 this is a method only used for test purpose
func GetCurrentVersion() semver.Version {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Join(path.Dir(filename), "../../..")
	dat, err := os.ReadFile(path.Join(dir, "version"))
	if err != nil {
		panic(err)
	}
	v, err := semver.Make(strings.TrimSpace(string(dat)))
	if err != nil {
		panic(err)
	}
	return v
}
