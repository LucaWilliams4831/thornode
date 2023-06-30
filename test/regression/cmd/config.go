package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	tmhttp "github.com/tendermint/tendermint/rpc/client/http"
	"gitlab.com/thorchain/thornode/app"
	"gitlab.com/thorchain/thornode/app/params"
	"gitlab.com/thorchain/thornode/cmd"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

////////////////////////////////////////////////////////////////////////////////////////
// Cosmos
////////////////////////////////////////////////////////////////////////////////////////

var encodingConfig params.EncodingConfig

func init() {
	// initialize the bech32 prefix for testnet/mocknet
	config := cosmos.GetConfig()
	config.SetBech32PrefixForAccount("tthor", "tthorpub")
	config.SetBech32PrefixForValidator("tthorv", "tthorvpub")
	config.SetBech32PrefixForConsensusNode("tthorc", "tthorcpub")
	config.Seal()

	// initialize the codec
	encodingConfig = app.MakeEncodingConfig()
}

func clientContextAndFactory(routine int) (client.Context, tx.Factory) {
	// create new rpc client
	node := fmt.Sprintf("http://localhost:%d", 26657+routine)
	rpcClient, err := tmhttp.New(node, "/websocket")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create tendermint client")
	}

	// create cosmos-sdk client context
	clientCtx := client.Context{
		Client:            rpcClient,
		ChainID:           "thorchain",
		JSONCodec:         encodingConfig.Marshaler,
		Codec:             encodingConfig.Marshaler,
		InterfaceRegistry: encodingConfig.InterfaceRegistry,
		Keyring:           keyRing,
		BroadcastMode:     flags.BroadcastSync,
		SkipConfirm:       true,
		TxConfig:          encodingConfig.TxConfig,
		AccountRetriever:  authtypes.AccountRetriever{},
		NodeURI:           node,
		LegacyAmino:       encodingConfig.Amino,
	}

	// create tx factory
	txFactory := tx.Factory{}
	txFactory = txFactory.WithKeybase(clientCtx.Keyring)
	txFactory = txFactory.WithTxConfig(clientCtx.TxConfig)
	txFactory = txFactory.WithAccountRetriever(clientCtx.AccountRetriever)
	txFactory = txFactory.WithChainID(clientCtx.ChainID)
	txFactory = txFactory.WithGas(1e8)
	txFactory = txFactory.WithSignMode(signing.SignMode_SIGN_MODE_DIRECT)

	return clientCtx, txFactory
}

////////////////////////////////////////////////////////////////////////////////////////
// Logging
////////////////////////////////////////////////////////////////////////////////////////

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Logger = log.With().Caller().Logger()

	// set to info level if DEBUG is not set (debug is the default level)
	if os.Getenv("DEBUG") == "" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

////////////////////////////////////////////////////////////////////////////////////////
// Colors
////////////////////////////////////////////////////////////////////////////////////////

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorPurple = "\033[35m"

	// save for later
	// ColorYellow = "\033[33m"
	// ColorBlue   = "\033[34m"
	// ColorCyan   = "\033[36m"
	// ColorGray   = "\033[37m"
	// ColorWhite  = "\033[97m"
)

////////////////////////////////////////////////////////////////////////////////////////
// HTTP
////////////////////////////////////////////////////////////////////////////////////////

var httpClient = &http.Client{
	Transport: &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second * getTimeFactor(),
		}).Dial,
	},
	Timeout: 5 * time.Second * getTimeFactor(),
}

////////////////////////////////////////////////////////////////////////////////////////
// Thorchain Module Addresses
////////////////////////////////////////////////////////////////////////////////////////

// TODO: determine how to return these programmatically without keeper
const (
	ModuleAddrThorchain    = "tthor1v8ppstuf6e3x0r4glqc68d5jqcs2tf38ulmsrp"
	ModuleAddrAsgard       = "tthor1g98cy3n9mmjrpn0sxmn63lztelera37nrytwp2"
	ModuleAddrBond         = "tthor17gw75axcnr8747pkanye45pnrwk7p9c3uhzgff"
	ModuleAddrTransfer     = "tthor1yl6hdjhmkf37639730gffanpzndzdpmhv07zme"
	ModuleAddrReserve      = "tthor1dheycdevq39qlkxs2a6wuuzyn4aqxhve3hhmlw"
	ModuleAddrFeeCollector = "tthor17xpfvakm2amg962yls6f84z3kell8c5ljftt88"
	ModuleAddrLending      = "tthor1x0kgm82cnj0vtmzdvz4avk3e7sj427t0al8wky"
)

////////////////////////////////////////////////////////////////////////////////////////
// Keys
////////////////////////////////////////////////////////////////////////////////////////

var (
	keyRing         = keyring.NewInMemory()
	addressToName   = map[string]string{} // thor...->dog, 0x...->dog
	templateAddress = map[string]string{} // addr_thor_dog->thor..., addr_eth_dog->0x...
	templatePubKey  = map[string]string{} // pubkey_dog->thorpub...

	birdMnemonic   = strings.Repeat("bird ", 23) + "asthma"
	catMnemonic    = strings.Repeat("cat ", 23) + "crawl"
	deerMnemonic   = strings.Repeat("deer ", 23) + "diesel"
	dogMnemonic    = strings.Repeat("dog ", 23) + "fossil"
	duckMnemonic   = strings.Repeat("duck ", 23) + "face"
	fishMnemonic   = strings.Repeat("fish ", 23) + "fade"
	foxMnemonic    = strings.Repeat("fox ", 23) + "filter"
	frogMnemonic   = strings.Repeat("frog ", 23) + "flat"
	goatMnemonic   = strings.Repeat("goat ", 23) + "install"
	hawkMnemonic   = strings.Repeat("hawk ", 23) + "juice"
	lionMnemonic   = strings.Repeat("lion ", 23) + "misery"
	mouseMnemonic  = strings.Repeat("mouse ", 23) + "option"
	muleMnemonic   = strings.Repeat("mule ", 23) + "major"
	pigMnemonic    = strings.Repeat("pig ", 23) + "quick"
	rabbitMnemonic = strings.Repeat("rabbit ", 23) + "rent"
	wolfMnemonic   = strings.Repeat("wolf ", 23) + "victory"

	// mnemonics contains the set of all mnemonics for accounts used in tests
	mnemonics = [...]string{
		dogMnemonic,
		catMnemonic,
		foxMnemonic,
		pigMnemonic,
		birdMnemonic,
		deerMnemonic,
		duckMnemonic,
		fishMnemonic,
		frogMnemonic,
		goatMnemonic,
		hawkMnemonic,
		lionMnemonic,
		mouseMnemonic,
		muleMnemonic,
		rabbitMnemonic,
		wolfMnemonic,
	}
)

func init() {
	// register functions for all mnemonic-chain addresses
	for _, m := range mnemonics {
		name := strings.Split(m, " ")[0]

		// create pubkey for mnemonic
		derivedPriv, err := hd.Secp256k1.Derive()(m, "", cmd.THORChainHDPath)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to derive private key")
		}
		privKey := hd.Secp256k1.Generate()(derivedPriv)
		s, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, privKey.PubKey())
		if err != nil {
			log.Fatal().Err(err).Msg("failed to bech32ify pubkey")
		}
		pk := common.PubKey(s)

		// add key to keyring
		_, err = keyRing.NewAccount(name, m, "", cmd.THORChainHDPath, hd.Secp256k1)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to add account to keyring")
		}

		for _, chain := range common.AllChains {

			// register template address for all chains
			addr, err := pk.GetAddress(chain)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to get address")
			}
			lowerChain := strings.ToLower(chain.String())
			templateAddress[fmt.Sprintf("addr_%s_%s", lowerChain, name)] = addr.String()

			// register address to name
			addressToName[addr.String()] = name

			// register pubkey for thorchain
			if chain == common.THORChain {
				templatePubKey[fmt.Sprintf("pubkey_%s", name)] = pk.String()
			}
		}
	}
}
