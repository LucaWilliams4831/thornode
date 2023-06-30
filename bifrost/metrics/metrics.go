package metrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/thornode/config"
)

// MetricName
type MetricName string

const (
	TotalBlockScanned       MetricName = `total_block_scanned`
	CurrentPosition         MetricName = `current_position`
	TotalRetryBlocks        MetricName = `total_retry_blocks`
	CommonBlockScannerError MetricName = `block_scanner_error`

	ThorchainBlockScannerError MetricName = `thorchain_block_scan_error`
	BlockDiscoveryDuration     MetricName = `block_discovery_duration`

	ThorchainClientError    MetricName = `thorchain_client_error`
	TxToThorchain           MetricName = `tx_to_thorchain`
	TxToThorchainSigned     MetricName = `tx_to_thorchain_signed`
	SignToThorchainDuration MetricName = `sign_to_thorchain_duration`
	SendToThorchainDuration MetricName = `send_to_thorchain_duration`

	ObserverError MetricName = `observer_error`
	SignerError   MetricName = `signer_error`

	PubKeyManagerError MetricName = `pubkey_manager_error`
)

// Metrics used to provide promethus metrics
type Metrics struct {
	logger zerolog.Logger
	cfg    config.BifrostMetricsConfiguration
	s      *http.Server
}

var (
	counters = map[MetricName]prometheus.Counter{
		TotalBlockScanned: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "block_scanner",
			Subsystem: "common_block_scanner",
			Name:      "total_block_scanned_total",
			Help:      "Total number of block scanned",
		}),
		CurrentPosition: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "block_scanner",
			Subsystem: "common_block_scanner",
			Name:      "current_position_total",
			Help:      "current block scan position",
		}),
		TotalRetryBlocks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "block_scanner",
			Subsystem: "common_block_scanner",
			Name:      "total_retry_blocks_total",
			Help:      "total blocks retried ",
		}),
		TxToThorchain: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "observer",
			Subsystem: "thorchain_client",
			Name:      "tx_to_thorchain_total",
			Help:      "number of tx observer post to thorchain successfully",
		}),
		TxToThorchainSigned: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "observer",
			Subsystem: "thorchain_client",
			Name:      "tx_to_thorchain_signed_total",
			Help:      "number of tx observer signed successfully",
		}),
	}
	counterVecs = map[MetricName]*prometheus.CounterVec{
		CommonBlockScannerError: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "block_scanner",
			Subsystem: "common_block_scanner",
			Name:      "errors_total",
			Help:      "errors in common block scanner",
		}, []string{
			"error_name", "additional",
		}),

		ThorchainBlockScannerError: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "block_scanner",
			Subsystem: "thorchain_block_scanner",
			Name:      "errors_total",
			Help:      "errors in thorchain block scanner",
		}, []string{
			"error_name", "additional",
		}),

		ThorchainClientError: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "thorchain",
			Subsystem: "thorchain_client",
			Name:      "errors_total",
			Help:      "errors in thorchain client",
		}, []string{
			"error_name", "additional",
		}),

		ObserverError: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "observer",
			Subsystem: "observer",
			Name:      "errors_total",
			Help:      "errors in observer",
		}, []string{
			"error_name", "additional",
		}),
		SignerError: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "signer",
			Subsystem: "signer",
			Name:      "errors_total",
			Help:      "errors in signer",
		}, []string{
			"error_name", "additional",
		}),
		PubKeyManagerError: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pubkey_manager",
			Subsystem: "pubkey_manager",
			Name:      "errors_total",
			Help:      "errors in pubkey manager",
		}, []string{
			"error_name", "additional",
		}),
	}

	histograms = map[MetricName]prometheus.Histogram{
		BlockDiscoveryDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "block_scanner",
			Subsystem: "common_block_scanner",
			Name:      "block_discovery",
			Help:      "how long it takes to discovery a block height",
		}),
		SignToThorchainDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "observer",
			Subsystem: "thorchain",
			Name:      "sign_to_thorchain_duration",
			Help:      "how long it takes to sign a tx to thorchain",
		}),
		SendToThorchainDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "observer",
			Subsystem: "thorchain",
			Name:      "send_to_thorchain_duration",
			Help:      "how long it takes to sign and broadcast to binance",
		}),
	}

	gauges = map[MetricName]prometheus.Gauge{}
)

// NewMetrics create a new instance of Metrics
func NewMetrics(cfg config.BifrostMetricsConfiguration) (*Metrics, error) {
	// Add chain metrics
	for _, chain := range cfg.Chains {
		AddChainMetrics(chain, counters, counterVecs, gauges, histograms)
	}
	// Register metrics
	for _, item := range counterVecs {
		prometheus.MustRegister(item)
	}
	for _, item := range counters {
		prometheus.MustRegister(item)
	}
	for _, item := range histograms {
		prometheus.MustRegister(item)
	}
	// create a new mux server
	server := http.NewServeMux()
	// register a new handler for the /metrics endpoint
	server.Handle("/metrics",
		promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer,
			promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
				Timeout: cfg.WriteTimeout,
			}),
		),
	)

	// register pprof handlers if enabled
	if cfg.PprofEnabled {
		server.HandleFunc("/debug/pprof/", pprof.Index)
		server.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		server.HandleFunc("/debug/pprof/profile", pprof.Profile)
		server.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		server.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	// start an http server using the mux server
	s := &http.Server{
		Addr:        fmt.Sprintf(":%d", cfg.ListenPort),
		Handler:     server,
		ReadTimeout: cfg.ReadTimeout,
	}
	return &Metrics{
		logger: log.With().Str("module", "metrics").Logger(),
		cfg:    cfg,
		s:      s,
	}, nil
}

// GetCounter return a counter by name, if it doesn't exist, then it return nil
func (m *Metrics) GetCounter(name MetricName) prometheus.Counter {
	if counter, ok := counters[name]; ok {
		return counter
	}
	return nil
}

// GetHistograms return a histogram by name
func (m *Metrics) GetHistograms(name MetricName) prometheus.Histogram {
	if h, ok := histograms[name]; ok {
		return h
	}
	return nil
}

// GetGauges return a gauge by name
func (m *Metrics) GetGauge(name MetricName) prometheus.Gauge {
	if g, ok := gauges[name]; ok {
		return g
	}
	return nil
}

func (m *Metrics) GetCounterVec(name MetricName) *prometheus.CounterVec {
	if c, ok := counterVecs[name]; ok {
		return c
	}
	return nil
}

// Start
func (m *Metrics) Start() error {
	if !m.cfg.Enabled {
		return nil
	}
	go func() {
		m.logger.Info().Int("port", m.cfg.ListenPort).Msg("start metric server")
		if err := m.s.ListenAndServe(); err != nil {
			m.logger.Error().Err(err).Msg("fail to stop metric server")
		}
	}()
	return nil
}

// Stop
func (m *Metrics) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	return m.s.Shutdown(ctx)
}
