package config

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	maddr "github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	tmhttp "github.com/tendermint/tendermint/rpc/client/http"

	"gitlab.com/thorchain/thornode/common"
)

// -------------------------------------------------------------------------------------
// Config
// -------------------------------------------------------------------------------------

const (
	MaxSeedRetries   = 3
	SeedRetryBackoff = time.Second * 5
)

var (
	//go:embed default.yaml
	defaultConfig []byte

	//go:embed *.tmpl
	templates embed.FS

	// config is the global configuration, it should never be returned by reference.
	config Config

	httpClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}
)

type Config struct {
	Thornode Thornode `mapstructure:"thor"`
	Bifrost  Bifrost  `mapstructure:"bifrost"`
}

// GetThornode returns the global thornode configuration.
func GetThornode() Thornode {
	return config.Thornode
}

// GetBifrost returns the global thornode configuration.
func GetBifrost() Bifrost {
	return config.Bifrost
}

// -------------------------------------------------------------------------------------
// Init
// -------------------------------------------------------------------------------------

// Init should be called at the beginning of execution to load base configuration and
// generate dependent configuration files. The defaults for the config package will be
// loaded from values defined in defaults.yaml in this package, then overridden the
// corresponding environment variables.
func Init() {
	assert := func(err error) {
		if err != nil {
			log.Fatal().Err(err).Msg("failed to bind env")
		}
	}

	// TODO: The following can be cleaned once all deployments are updated to use
	// explicit keys for the new configuration package. In the meantime we will preserve
	// mappings from historical environment for backwards compatibility.
	assert(viper.BindEnv("bifrost.thorchain.signer_name", "SIGNER_NAME"))
	assert(viper.BindEnv(
		"bifrost.chains.btc.block_scanner.block_height_discover_back_off",
		"BLOCK_SCANNER_BACKOFF",
	))
	assert(viper.BindEnv(
		"bifrost.chains.doge.block_scanner.block_height_discover_back_off",
		"BLOCK_SCANNER_BACKOFF",
	))
	assert(viper.BindEnv(
		"bifrost.chains.ltc.block_scanner.block_height_discover_back_off",
		"BLOCK_SCANNER_BACKOFF",
	))
	assert(viper.BindEnv(
		"bifrost.chains.bch.block_scanner.block_height_discover_back_off",
		"BLOCK_SCANNER_BACKOFF",
	))
	assert(viper.BindEnv(
		"bifrost.chains.eth.block_scanner.block_height_discover_back_off",
		"BLOCK_SCANNER_BACKOFF",
	))
	assert(viper.BindEnv(
		"bifrost.signer.block_scanner.block_height_discover_back_off",
		"THOR_BLOCK_TIME",
	))
	assert(viper.BindEnv(
		"thor.tendermint.consensus.timeout_commit",
		"THOR_BLOCK_TIME",
	))
	assert(viper.BindEnv("bifrost.tss.bootstrap_peers", "PEER"))
	assert(viper.BindEnv("bifrost.tss.external_ip", "EXTERNAL_IP"))
	assert(viper.BindEnv("bifrost.thorchain.chain_id", "CHAIN_ID"))
	assert(viper.BindEnv("bifrost.thorchain.chain_host", "CHAIN_API"))
	assert(viper.BindEnv(
		"bifrost.thorchain.chain_rpc",
		"CHAIN_RPC",
	))
	assert(viper.BindEnv(
		"bifrost.signer.block_scanner.rpc_host",
		"CHAIN_RPC",
	))
	assert(viper.BindEnv(
		"bifrost.chains.bnb.rpc_host",
		"BINANCE_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.bnb.block_scanner.rpc_host",
		"BINANCE_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.bnb.block_scanner.start_block_height",
		"BINANCE_START_BLOCK_HEIGHT",
	))
	assert(viper.BindEnv(
		"bifrost.chains.BTC.rpc_host",
		"BTC_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.BTC.block_scanner.rpc_host",
		"BTC_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.BTC.block_scanner.start_block_height",
		"BTC_START_BLOCK_HEIGHT",
	))
	assert(viper.BindEnv(
		"bifrost.chains.ETH.rpc_host",
		"ETH_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.ETH.block_scanner.rpc_host",
		"ETH_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.ETH.block_scanner.start_block_height",
		"ETH_START_BLOCK_HEIGHT",
	))
	assert(viper.BindEnv(
		"bifrost.chains.AVAX.rpc_host",
		"AVAX_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.AVAX.block_scanner.rpc_host",
		"AVAX_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.AVAX.block_scanner.start_block_height",
		"AVAX_START_BLOCK_HEIGHT",
	))
	assert(viper.BindEnv(
		"bifrost.chains.DOGE.rpc_host",
		"DOGE_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.DOGE.block_scanner.rpc_host",
		"DOGE_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.DOGE.block_scanner.start_block_height",
		"DOGE_START_BLOCK_HEIGHT",
	))
	assert(viper.BindEnv(
		"bifrost.chains.GAIA.rpc_host",
		"GAIA_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.GAIA.block_scanner.rpc_host",
		"GAIA_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.GAIA.block_scanner.start_block_height",
		"GAIA_START_BLOCK_HEIGHT",
	))
	assert(viper.BindEnv(
		"bifrost.chains.LTC.rpc_host",
		"LTC_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.LTC.block_scanner.rpc_host",
		"LTC_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.LTC.block_scanner.start_block_height",
		"LTC_START_BLOCK_HEIGHT",
	))
	assert(viper.BindEnv(
		"bifrost.chains.BCH.rpc_host",
		"BCH_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.BCH.block_scanner.rpc_host",
		"BCH_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.BCH.block_scanner.start_block_height",
		"BCH_START_BLOCK_HEIGHT",
	))
	assert(viper.BindEnv(
		"bifrost.chains.GAIA.cosmos_grpc_host",
		"GAIA_GRPC_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.GAIA.block_scanner.cosmos_grpc_host",
		"GAIA_GRPC_HOST",
	))
	assert(viper.BindEnv(
		"bifrost.chains.GAIA.cosmos_grpc_tls",
		"GAIA_GRPC_TLS",
	))
	assert(viper.BindEnv(
		"bifrost.chains.GAIA.block_scanner.cosmos_grpc_tls",
		"GAIA_GRPC_TLS",
	))
	assert(viper.BindEnv("bifrost.chains.GAIA.disabled", "GAIA_DISABLED"))
	assert(viper.BindEnv("bifrost.chains.DOGE.disabled", "DOGE_DISABLED"))
	assert(viper.BindEnv("bifrost.chains.LTC.disabled", "LTC_DISABLED"))
	assert(viper.BindEnv("bifrost.chains.AVAX.disabled", "AVAX_DISABLED"))
	assert(viper.BindEnv("bifrost.chains.AVAX.block_scanner.gas_cache_size", "AVAX_GAS_CACHE_SIZE"))

	// always override from environment
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	viper.AllowEmptyEnv(true)

	// load defaults
	viper.SetConfigType("yaml")
	if err := viper.ReadConfig(bytes.NewBuffer(defaultConfig)); err != nil {
		log.Fatal().Err(err).Msg("failed to read default config")
	}

	if err := viper.Unmarshal(&config); err != nil {
		log.Fatal().Err(err).Msg("failed to unmarshal config")
	}
}

func InitBifrost() {
	chains := map[common.Chain]BifrostChainConfiguration{}
	for chainId, chain := range config.Bifrost.Chains {
		// validate chain configurations
		if err := chain.ChainID.Validate(); err != nil {
			log.Fatal().Err(err).
				Stringer("chain", chainId).
				Stringer("chain_id", chain.ChainID).
				Msg("chain failed validation")
		}
		if err := chain.BlockScanner.ChainID.Validate(); err != nil {
			log.Fatal().Err(err).
				Stringer("chain", chainId).
				Stringer("chain_id", chain.BlockScanner.ChainID).
				Msg("chain failed validation")
		}
		// set shared backoff override
		chain.BackOff = config.Bifrost.BackOff
		chains[chain.ChainID] = chain
	}
	config.Bifrost.Chains = chains

	// create observer db paths
	for _, chain := range config.Bifrost.Chains {
		err := os.MkdirAll(chain.BlockScanner.DBPath, os.ModePerm)
		if err != nil {
			log.Fatal().Err(err).Str("path", chain.BlockScanner.DBPath).
				Msg("failed to create observer db directory")
		}
	}

	// create signer db path
	err := os.MkdirAll(config.Bifrost.Signer.SignerDbPath, os.ModePerm)
	if err != nil {
		log.Fatal().Err(err).Str("path", config.Bifrost.Signer.SignerDbPath).
			Msg("failed to create signer db directory")
	}

	// set signer password explicitly from environment variable
	config.Bifrost.Thorchain.SignerPasswd = os.Getenv("SIGNER_PASSWD")

	// set bootstrap peers from seeds endpoint if unset
	if len(config.Bifrost.TSS.BootstrapPeers) == 0 {
		config.Bifrost.TSS.BootstrapPeers = resolveAddrs(getSeedAddrs())
	}
}

func InitThornode(ctx context.Context) {
	// Environment variables prefixed with `THORNODE` will be read by viper in cosmos-sdk
	// initialization and overwrite configuration we apply in this package.
	for _, env := range os.Environ() {
		envKey := strings.Split(env, "=")[0]
		if strings.HasPrefix(envKey, "THORNODE_") {
			log.Warn().Msgf("environment variable %s could overwrite config", env)
		}
	}

	// if auto statesync enable, find latest snapshot height and hash that should exist
	if config.Thornode.AutoStateSync.Enabled {
		thornodeAutoStateSync(ctx)
	}

	// dynamically set seeds
	seedAddrs, tmSeeds := thornodeSeeds()
	config.Thornode.Tendermint.P2P.Seeds = strings.Join(tmSeeds, ",")

	// dynamically set rpc listen address
	if config.Thornode.Tendermint.RPC.ListenAddress == "" {
		config.Thornode.Tendermint.RPC.ListenAddress = fmt.Sprintf("tcp://0.0.0.0:%d", rpcPort)
	}
	if config.Thornode.Tendermint.P2P.ListenAddress == "" {
		config.Thornode.Tendermint.P2P.ListenAddress = fmt.Sprintf("tcp://0.0.0.0:%d", p2pPort)
	}

	// set the Tendermint external address
	if os.Getenv("EXTERNAL_IP") != "" {
		config.Thornode.Tendermint.P2P.ExternalAddress = fmt.Sprintf("%s:%d", os.Getenv("EXTERNAL_IP"), p2pPort)
	}

	// set paths
	home := os.ExpandEnv("$HOME/.thornode")
	tendermintPath := filepath.Join(home, "config", "config.toml")
	cosmosPath := filepath.Join(home, "config", "app.toml")

	// template tendermint config into place
	t := template.Must(template.ParseFS(templates, "*.tmpl"))
	tendermintFile, err := os.OpenFile(tendermintPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open config.toml")
	}
	err = t.ExecuteTemplate(tendermintFile, "config.toml.tmpl", config.Thornode.Tendermint)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to render config.toml")
	}

	// template cosmos config into place
	cosmosFile, err := os.OpenFile(cosmosPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open app.toml")
	}
	err = t.ExecuteTemplate(cosmosFile, "app.toml.tmpl", config.Thornode.Cosmos)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to render app.toml")
	}

	// fetch genesis
	if len(seedAddrs) > 0 {
		thornodeFetchGenesis(seedAddrs)
	} else {
		log.Warn().Msg("no seeds, skipping genesis fetch")
	}
}

// -------------------------------------------------------------------------------------
// Thornode
// -------------------------------------------------------------------------------------

type Thornode struct {
	// NodeRelayURL is the URL of the node relay service.
	NodeRelayURL string `mapstructure:"node_relay_url"`

	// LogFilter will drop logs matching the modules and messages when not in debug level.
	LogFilter struct {
		// Modules is a list of modules to filter.
		Modules []string `mapstructure:"modules"`

		// Messages is a list of messages to filter.
		Messages []string `mapstructure:"messages"`
	} `mapstructure:"log_filter"`

	AutoStateSync struct {
		Enabled bool `mapstructure:"enabled"`

		// BlockBuffer is the number of blocks in the past we will automatically reference
		// for the trust state from one of the configured RPC endpoints.
		BlockBuffer int64 `mapstructure:"block_buffer"`

		// Peers will be used to template the persistent peers in the Tendermint P2P config
		// on the first launch. These peers are static and typically provided by benevolent
		// community members, since the statesync snapshot creation is very expensive and
		// cannot be enabled on nodes unless they are willing to fall behind for a few hours
		// while the snapshots create. Once the initial snapshot is recovered, subsequent
		// restarts will unset the fixed persistent peers to free up peer slots on nodes
		// that are known statesync providers.
		Peers []string `mapstructure:"peers"`
	} `mapstructure:"auto_state_sync"`

	API struct {
		LimitCount    float64       `mapstructure:"limit_count"`
		LimitDuration time.Duration `mapstructure:"limit_duration"`
	} `mapstructure:"api"`

	// Cosmos contains values used in templating the Cosmos app.toml.
	Cosmos struct {
		Pruning         string `mapstructure:"pruning"`
		HaltHeight      int64  `mapstructure:"halt_height"`
		MinRetainBlocks int64  `mapstructure:"min_retain_blocks"`

		Telemetry struct {
			Enabled                 bool  `mapstructure:"enabled"`
			PrometheusRetentionTime int64 `mapstructure:"prometheus_retention_time"`
		} `mapstructure:"telemetry"`

		API struct {
			Enable            bool   `mapstructure:"enable"`
			EnabledUnsafeCORS bool   `mapstructure:"enabled_unsafe_cors"`
			Address           string `mapstructure:"address"`
		} `mapstructure:"api"`

		StateSync struct {
			SnapshotInterval   int64 `mapstructure:"snapshot_interval"`
			SnapshotKeepRecent int64 `mapstructure:"snapshot_keep_recent"`
		} `mapstructure:"state_sync"`
	} `mapstructure:"cosmos"`

	// Tendermint contains values used in templating the Tendermint config.toml.
	Tendermint struct {
		Consensus struct {
			TimeoutProposeDelta   time.Duration `mapstructure:"timeout_propose_delta"`
			TimeoutPrevoteDelta   time.Duration `mapstructure:"timeout_prevote_delta"`
			TimeoutPrecommitDelta time.Duration `mapstructure:"timeout_precommit_delta"`
			TimeoutCommit         time.Duration `mapstructure:"timeout_commit"`
		} `mapstructure:"consensus"`

		Log struct {
			Level  string `mapstructure:"level"`
			Format string `mapstructure:"format"`
		} `mapstructure:"log"`

		RPC struct {
			ListenAddress     string `mapstructure:"listen_address"`
			CORSAllowedOrigin string `mapstructure:"cors_allowed_origin"`
		} `mapstructure:"rpc"`

		P2P struct {
			ExternalAddress     string `mapstructure:"external_address"`
			ListenAddress       string `mapstructure:"listen_address"`
			PersistentPeers     string `mapstructure:"persistent_peers"`
			AddrBookStrict      bool   `mapstructure:"addr_book_strict"`
			MaxNumInboundPeers  int64  `mapstructure:"max_num_inbound_peers"`
			MaxNumOutboundPeers int64  `mapstructure:"max_num_outbound_peers"`
			AllowDuplicateIP    bool   `mapstructure:"allow_duplicate_ip"`
			Seeds               string `mapstructure:"seeds"`
		} `mapstructure:"p2p"`

		StateSync struct {
			Enable      bool   `mapstructure:"enable"`
			RPCServers  string `mapstructure:"rpc_servers"`
			TrustHeight int64  `mapstructure:"trust_height"`
			TrustHash   string `mapstructure:"trust_hash"`
			TrustPeriod string `mapstructure:"trust_period"`
		} `mapstructure:"state_sync"`

		Instrumentation struct {
			Prometheus bool `mapstructure:"prometheus"`
		} `mapstructure:"instrumentation"`
	} `mapstructure:"tendermint"`
}

// -------------------------------------------------------------------------------------
// Bifrost
// -------------------------------------------------------------------------------------

type Bifrost struct {
	Signer    BifrostSignerConfiguration                 `mapstructure:"signer"`
	Thorchain BifrostClientConfiguration                 `mapstructure:"thorchain"`
	Metrics   BifrostMetricsConfiguration                `mapstructure:"metrics"`
	Chains    map[common.Chain]BifrostChainConfiguration `mapstructure:"chains"`
	TSS       BifrostTSSConfiguration                    `mapstructure:"tss"`
	BackOff   BifrostBackOff                             `mapstructure:"back_off"`

	ObserverLevelDB LevelDBOptions `mapstructure:"observer_leveldb"`
}

// LevelDBOptions are a superset of the options passed to the LevelDB constructor.
type LevelDBOptions struct {
	// FilterBitsPerKey is the number of bits per key for the bloom filter.
	FilterBitsPerKey int `mapstructure:"filter_bits_per_key"`

	// CompactionTableSizeMultiplier is the multiplier for compaction (table size is 2Mb).
	CompactionTableSizeMultiplier float64 `mapstructure:"compaction_table_size_multiplier"`

	// WriteBuffer is the size of the write buffer in bytes. LevelDB default is 4Mb.
	WriteBuffer int `mapstructure:"write_buffer"`

	// BlockCacheCapacity is the size of the block cache in bytes. LevelDB default is 8Mb.
	BlockCacheCapacity int `mapstructure:"block_cache_capacity"`

	// CompactOnInit will trigger a full compaction at init.
	CompactOnInit bool `mapstructure:"compact_on_init"`
}

// Options returns the corresponding LevelDB options for the constructor.
func (b LevelDBOptions) Options() *opt.Options {
	o := &opt.Options{}

	// only override zero values if set
	if b.CompactionTableSizeMultiplier > 0 {
		o.CompactionTableSizeMultiplier = b.CompactionTableSizeMultiplier
	}
	if b.WriteBuffer > 0 {
		o.WriteBuffer = b.WriteBuffer
	}
	if b.BlockCacheCapacity > 0 {
		o.BlockCacheCapacity = b.BlockCacheCapacity
	}
	if b.FilterBitsPerKey > 0 {
		o.Filter = filter.NewBloomFilter(b.FilterBitsPerKey)
	}

	return o
}

type BifrostSignerConfiguration struct {
	BackupKeyshares bool                             `mapstructure:"backup_keyshares"`
	SignerDbPath    string                           `mapstructure:"signer_db_path"`
	BlockScanner    BifrostBlockScannerConfiguration `mapstructure:"block_scanner"`
	RetryInterval   time.Duration                    `mapstructure:"retry_interval"`
	// RescheduleBufferBlocks is the number of blocks before reschedule we will stop
	// attempting to sign and broadcast (for outbounds not in round 7 retry).
	RescheduleBufferBlocks int64          `mapstructure:"reschedule_buffer_blocks"`
	LevelDB                LevelDBOptions `mapstructure:"leveldb"`

	// AutoObserve will automatically submit the observation for outbound transactions once
	// they are signed - regardless of broadcast success.
	AutoObserve bool `mapstructure:"auto_observe"`
}

type BifrostBackOff struct {
	InitialInterval     time.Duration `mapstructure:"initial_interval"`
	RandomizationFactor float64       `mapstructure:"randomization_factor"`
	Multiplier          float64       `mapstructure:"multiplier"`
	MaxInterval         time.Duration `mapstructure:"max_interval"`
	MaxElapsedTime      time.Duration `mapstructure:"max_elapsed_time"`
}

type BifrostChainConfiguration struct {
	ChainID             common.Chain                     `mapstructure:"chain_id"`
	ChainHost           string                           `mapstructure:"chain_host"`
	ChainNetwork        string                           `mapstructure:"chain_network"`
	UserName            string                           `mapstructure:"username"`
	Password            string                           `mapstructure:"password"`
	RPCHost             string                           `mapstructure:"rpc_host"`
	CosmosGRPCHost      string                           `mapstructure:"cosmos_grpc_host"`
	CosmosGRPCTLS       bool                             `mapstructure:"cosmos_grpc_tls"`
	HTTPostMode         bool                             `mapstructure:"http_post_mode"` // Bitcoin core only supports HTTP POST mode
	DisableTLS          bool                             `mapstructure:"disable_tls"`    // Bitcoin core does not provide TLS by default
	BlockScanner        BifrostBlockScannerConfiguration `mapstructure:"block_scanner"`
	BackOff             BifrostBackOff                   `mapstructure:"back_off"`
	OptToRetire         bool                             `mapstructure:"opt_to_retire"` // don't emit support for this chain during keygen process
	ParallelMempoolScan int                              `mapstructure:"parallel_mempool_scan"`
	Disabled            bool                             `mapstructure:"disabled"`
	SolvencyBlocks      int64                            `mapstructure:"solvency_blocks"`

	// MemPoolTxIDCacheSize is the number of transaction ids to cache in memory. This
	// prevents read on LevelDB which may hit disk for every transaction in the mempool in
	// a loop - which causes concern in recent times with growing Bitcoin mempool.
	MempoolTxIDCacheSize int `mapstructure:"mempool_tx_id_cache_size"`

	// ScannerLevelDB is the LevelDB configuration for the block scanner.
	ScannerLevelDB LevelDBOptions `mapstructure:"scanner_leveldb"`
}

func (b *BifrostChainConfiguration) Validate() {
	if b.RPCHost == "" {
		log.Fatal().Str("chain", b.ChainID.String()).Msg("rpc host is required")
	}
}

type BifrostBlockScannerConfiguration struct {
	RPCHost                    string        `mapstructure:"rpc_host"`
	StartBlockHeight           int64         `mapstructure:"start_block_height"`
	BlockScanProcessors        int           `mapstructure:"block_scan_processors"`
	HTTPRequestTimeout         time.Duration `mapstructure:"http_request_timeout"`
	HTTPRequestReadTimeout     time.Duration `mapstructure:"http_request_read_timeout"`
	HTTPRequestWriteTimeout    time.Duration `mapstructure:"http_request_write_timeout"`
	MaxHTTPRequestRetry        int           `mapstructure:"max_http_request_retry"`
	BlockHeightDiscoverBackoff time.Duration `mapstructure:"block_height_discover_back_off"`
	BlockRetryInterval         time.Duration `mapstructure:"block_retry_interval"`
	EnforceBlockHeight         bool          `mapstructure:"enforce_block_height"`
	DBPath                     string        `mapstructure:"db_path"`
	ChainID                    common.Chain  `mapstructure:"chain_id"`

	// The following configuration values apply only to a subset of chains.

	// CosmosGRPCHost is the <host>:<port> of the gRPC endpoint of the Cosmos SDK chain.
	CosmosGRPCHost string `mapstructure:"cosmos_grpc_host"`

	// CosmosGRPCTLS is a boolean value indicating whether the gRPC host is using TLS.
	CosmosGRPCTLS bool `mapstructure:"cosmos_grpc_tls"`

	// GasCacheBlocks is the number of blocks worth of gas price data cached to determine
	// the gas price reported to Thorchain.
	GasCacheBlocks int `mapstructure:"gas_cache_blocks"`

	// Concurrency is the number of goroutines used for RPC requests on data within a
	// block - e.g. transactions, receipts, logs, etc. Blocks are processed sequentially.
	Concurrency int64 `mapstructure:"concurrency"`

	// GasPriceResolution is the resolution of price per gas unit in the base asset of the
	// chain (wei, tavax, uatom, satoshi, etc) and is transitively the floor price.
	GasPriceResolution int64 `mapstructure:"gas_price_resolution"`

	// ObservationFlexibilityBlocks is the number of blocks behind the current tip we will
	// submit network fee and solvency observations.
	ObservationFlexibilityBlocks int64 `mapstructure:"observation_flexibility_blocks"`

	// MaxGasFee is the maximum gas fee (in base asset) that will be used for a transaction.
	MaxGasFee int64 `mapstructure:"max_gas_fee"`

	// MaxContractGas is the maximum gas allowed for a contract transaction.
	MaxContractGas int64 `mapstructure:"max_contract_gas"`

	// WhitelistTokens is the set of whitelisted token addresses. Inbounds for all other
	// tokens are ignored.
	WhitelistTokens []string `mapstructure:"whitelist_tokens"`
}

func (b *BifrostBlockScannerConfiguration) Validate() {
	if b.RPCHost == "" {
		log.Fatal().Str("chain", b.ChainID.String()).Msg("rpc host is required")
	}
}

type BifrostClientConfiguration struct {
	ChainID         common.Chain `mapstructure:"chain_id" `
	ChainHost       string       `mapstructure:"chain_host"`
	ChainRPC        string       `mapstructure:"chain_rpc"`
	ChainHomeFolder string       `mapstructure:"chain_home_folder"`
	SignerName      string       `mapstructure:"signer_name"`
	SignerPasswd    string
	BackOff         BifrostBackOff `mapstructure:"back_off"`
}

type BifrostMetricsConfiguration struct {
	Enabled      bool           `mapstructure:"enabled"`
	PprofEnabled bool           `mapstructure:"pprof_enabled"`
	ListenPort   int            `mapstructure:"listen_port"`
	ReadTimeout  time.Duration  `mapstructure:"read_timeout"`
	WriteTimeout time.Duration  `mapstructure:"write_timeout"`
	Chains       []common.Chain `mapstructure:"chains"`
}

type BifrostTSSConfiguration struct {
	BootstrapPeers               []string `mapstructure:"bootstrap_peers"`
	Rendezvous                   string   `mapstructure:"rendezvous"`
	P2PPort                      int      `mapstructure:"p2p_port"`
	InfoAddress                  string   `mapstructure:"info_address"`
	ExternalIP                   string   `mapstructure:"external_ip"`
	MaxKeyshareRecoverScanBlocks int64    `mapstructure:"max_keyshare_recover_scan_blocks"`
}

// GetBootstrapPeers return the internal bootstrap peers in a slice of maddr.Multiaddr
func (c BifrostTSSConfiguration) GetBootstrapPeers() ([]maddr.Multiaddr, error) {
	var addrs []maddr.Multiaddr

	for _, ip := range resolveAddrs(c.BootstrapPeers) {
		if len(ip) == 0 {
			continue
		}

		// fetch the p2pid
		res, err := httpClient.Get(fmt.Sprintf("http://%s:6040/p2pid", ip))
		if err != nil {
			log.Error().Err(err).Msg("failed to get p2p id")
			continue
		}

		// skip peers with a bad response status
		if res.StatusCode != http.StatusOK {
			log.Warn().Msgf("failed to get p2p id, status code: %d", res.StatusCode)
			continue
		}

		// read the response
		body, err := io.ReadAll(res.Body)
		if err != nil {
			log.Error().Err(err).Msg("failed to read p2p id response")
			continue
		}
		res.Body.Close()

		// format the multiaddr
		peerMultiAddr := fmt.Sprintf("/ip4/%s/tcp/5040/ipfs/%s", ip, string(body))

		addr, err := maddr.NewMultiaddr(peerMultiAddr)
		if err != nil {
			log.Error().Err(err).Str("addr", peerMultiAddr).Msg("failed to parse multiaddr")
			continue
		}
		addrs = append(addrs, addr)
	}

	if len(addrs) == 0 {
		log.Error().Msg("no bootstrap peers found")
		assertBifrostHasSeeds()
	} else {
		log.Info().Interface("peers", addrs).Msg("bootstrap peers")
	}
	return addrs, nil
}

// -------------------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------------------

func resolveAddrs(addrs []string) []string {
	resolvedAddrs := []string{}
	for _, addr := range addrs {
		if net.ParseIP(addr) == nil {
			ips, err := net.LookupHost(addr)
			if err != nil {
				log.Warn().Err(err).Msg("failed to resolve address")
			} else {
				resolvedAddrs = append(resolvedAddrs, ips[0]) // just take the first
			}
		} else {
			resolvedAddrs = append(resolvedAddrs, addr)
		}
	}

	return resolvedAddrs
}

func thornodeSeeds() (seedAddrs, tmSeeds []string) {
	// use environment variable if set
	seeds := os.Getenv("SEEDS")
	if seeds != "" {
		seedAddrs = strings.Split(seeds, ",")
	} else {
		log.Info().Msg("seeds not provided, initializing automatically...")
		seedAddrs = getSeedAddrs()
	}

	// resolve any hostnames
	seedAddrs = resolveAddrs(seedAddrs)

	// skip further steps if there were no seeds to check
	if len(seedAddrs) == 0 {
		log.Warn().Msg("no seeds found")
		return
	}

	// initialize seed with their node id if the network matches
	wg := sync.WaitGroup{}
	mu := sync.Mutex{}

	for try := 0; try < MaxSeedRetries; try++ {
		for _, seed := range seedAddrs {
			wg.Add(1)
			go func(seedIP string) {
				defer wg.Done()

				// get node status
				res, err := httpClient.Get(fmt.Sprintf("http://%s:%d/status", seedIP, rpcPort))
				if err != nil {
					log.Error().Err(err).Msg("failed to get node status")
					return
				}

				// decode status response
				type status struct {
					Result struct {
						NodeInfo struct {
							ID      string `json:"id"`
							Network string `json:"network"`
						} `json:"node_info"`
					} `json:"result"`
				}
				var s status
				dec := json.NewDecoder(res.Body)
				err = dec.Decode(&s)
				if err != nil {
					log.Error().Err(err).Msg("failed to decode node status")
					return
				}

				// skip if the node is not on the same network
				if s.Result.NodeInfo.Network != os.Getenv("CHAIN_ID") {
					log.Error().
						Str("network", s.Result.NodeInfo.Network).
						Str("expected", os.Getenv("CHAIN_ID")).
						Msg("node is not on the same network")
					return
				}

				// update seeds
				mu.Lock()
				tmSeeds = append(tmSeeds, fmt.Sprintf("%s@%s:%d", s.Result.NodeInfo.ID, seedIP, p2pPort))
				mu.Unlock()
			}(seed)
		}
		wg.Wait()

		// retry a few times if we have no seeds
		if len(tmSeeds) > 0 {
			break
		}
		log.Info().Msg("retrying to fetch seeds...")
		time.Sleep(SeedRetryBackoff)
	}

	log.Info().Msgf("found %d p2p seeds", len(tmSeeds))
	return
}

func thornodeAutoStateSync(ctx context.Context) {
	// if we already have a state assume we have a snapshot and skip
	dataDir := os.ExpandEnv("$HOME/.thornode/data/state.db")
	if _, err := os.Stat(dataDir); err == nil {
		log.Info().Msg("data directory detected, skipping auto statesync configuration")
		return
	}

	for _, host := range strings.Split(config.Thornode.Tendermint.StateSync.RPCServers, ",") {
		log.Info().Msgf("auto statesync enabled, determining trust height via %s", host)

		client, err := tmhttp.New(host, "")
		if err != nil {
			log.Err(err).Str("host", host).Msg("failed to create tendermint client")
			continue
		}

		// get the height of the expected snapshot
		status, err := client.Status(ctx)
		if err != nil {
			log.Err(err).Str("host", host).Msg("failed to get status")
			continue
		}
		height := status.SyncInfo.LatestBlockHeight - config.Thornode.AutoStateSync.BlockBuffer

		// get the hash of the trust block
		block, err := client.Block(ctx, &height)
		if err != nil {
			log.Err(err).Str("host", host).Int64("height", height).Msg("failed to get block")
			continue
		}
		hash := block.BlockID.Hash.String()

		// set the trusted hash and height in tendermint
		log.Info().Int64("height", height).Str("hash", hash).Msg("setting automatic statesync trust")
		config.Thornode.Tendermint.StateSync.Enable = true
		config.Thornode.Tendermint.StateSync.TrustHeight = height
		config.Thornode.Tendermint.StateSync.TrustHash = hash

		// set the persistent peers in tendermint to the known auto statesync peers
		config.Thornode.Tendermint.P2P.PersistentPeers = strings.Join(config.Thornode.AutoStateSync.Peers, ",")

		// success
		return
	}

	log.Fatal().Msg("failed to determine statesync trust height from any rpc host")
}

func thornodeFetchGenesis(seeds []string) {
	home := os.ExpandEnv("$HOME/.thornode")
	genesisPath := filepath.Join(home, "config", "genesis.json")

	// check to see if we already have a genesis file
	if fi, err := os.Stat(genesisPath); !os.IsNotExist(err) || (fi != nil && fi.Size() == 0) {
		log.Info().Msg("genesis file already exists, skipping fetch")
		return
	}

	// iterate peers until we succeed in fetching genesis
	for try := 0; try < MaxSeedRetries; try++ {
		for _, seed := range seeds {
			// use the default http client to avoid the short timeout
			res, err := http.Get(fmt.Sprintf("http://%s:%d/genesis", seed, rpcPort))
			if err != nil || res.StatusCode != http.StatusOK {
				log.Error().Err(err).Str("seed", seed).Msg("failed to fetch genesis")
				continue
			}

			// decode genesis response
			type genesisResponse struct {
				Result struct {
					Genesis interface{} `json:"genesis"`
				} `json:"result"`
			}
			var g genesisResponse
			dec := json.NewDecoder(res.Body)
			err = dec.Decode(&g)
			if err != nil {
				log.Error().Err(err).Str("seed", seed).Msg("failed to decode genesis")
				continue
			}

			// open genesis file
			err = os.MkdirAll(filepath.Dir(genesisPath), 0o755)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create genesis directory")
			}
			f, err := os.OpenFile(genesisPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create genesis file")
			}

			// encode back to file
			enc := json.NewEncoder(f)
			err = enc.Encode(g.Result.Genesis)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to write genesis file")
			}

			// success
			f.Close()
			log.Info().Msg("genesis file fetched")
			return
		}

		time.Sleep(SeedRetryBackoff)
		log.Info().Msg("retrying to fetch genesis...")
	}
}
