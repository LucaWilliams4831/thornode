package gaia

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	ctypes "github.com/cosmos/cosmos-sdk/types"
	btypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client/http"

	"github.com/rs/zerolog/log"
	"gitlab.com/thorchain/thornode/bifrost/metrics"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/gaia/wasm"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/config"

	"gitlab.com/thorchain/thornode/cmd"
	. "gopkg.in/check.v1"
)

// -------------------------------------------------------------------------------------
// Mock FeeTx
// -------------------------------------------------------------------------------------

type MockFeeTx struct {
	fee ctypes.Coins
	gas uint64
}

func (m *MockFeeTx) GetMsgs() []ctypes.Msg {
	return nil
}

func (m *MockFeeTx) ValidateBasic() error {
	return nil
}

func (m *MockFeeTx) GetGas() uint64 {
	return m.gas
}

func (m *MockFeeTx) GetFee() ctypes.Coins {
	return m.fee
}

func (m *MockFeeTx) FeePayer() ctypes.AccAddress {
	return nil
}

func (m *MockFeeTx) FeeGranter() ctypes.AccAddress {
	return nil
}

// -------------------------------------------------------------------------------------
// Tests
// -------------------------------------------------------------------------------------

type BlockScannerTestSuite struct {
	m      *metrics.Metrics
	bridge thorclient.ThorchainBridge
	keys   *thorclient.Keys
}

var _ = Suite(&BlockScannerTestSuite{})

func (s *BlockScannerTestSuite) SetUpSuite(c *C) {
	s.m = GetMetricForTest(c)
	c.Assert(s.m, NotNil)
	cfg := config.BifrostClientConfiguration{
		ChainID:         "thorchain",
		ChainHost:       "localhost",
		SignerName:      "bob",
		SignerPasswd:    "password",
		ChainHomeFolder: "",
	}

	kb := cKeys.NewInMemory()
	_, _, err := kb.NewMnemonic(cfg.SignerName, cKeys.English, cmd.THORChainHDPath, cfg.SignerPasswd, hd.Secp256k1)
	c.Assert(err, IsNil)
	thorKeys := thorclient.NewKeysWithKeybase(kb, cfg.SignerName, cfg.SignerPasswd)
	c.Assert(err, IsNil)
	s.bridge, err = thorclient.NewThorchainBridge(cfg, s.m, thorKeys)
	c.Assert(err, IsNil)
	s.keys = thorKeys
}

func (s *BlockScannerTestSuite) TestCalculateAverageGasFees(c *C) {
	cfg := config.BifrostBlockScannerConfiguration{ChainID: common.GAIAChain, GasPriceResolution: 100_000}
	blockScanner := CosmosBlockScanner{cfg: cfg}

	atomToThorchain := int64(100)

	blockScanner.updateGasCache(&MockFeeTx{
		gas: GasLimit / 2,
		fee: ctypes.Coins{ctypes.NewCoin("uatom", ctypes.NewInt(10000))},
	})
	c.Check(len(blockScanner.feeCache), Equals, 1)
	c.Check(blockScanner.averageFee().String(), Equals, fmt.Sprintf("%d", uint64(20000*atomToThorchain)))

	blockScanner.updateGasCache(&MockFeeTx{
		gas: GasLimit / 2,
		fee: ctypes.Coins{ctypes.NewCoin("uatom", ctypes.NewInt(10000))},
	})
	c.Check(len(blockScanner.feeCache), Equals, 2)
	c.Check(blockScanner.averageFee().String(), Equals, fmt.Sprintf("%d", uint64(20000*atomToThorchain)))

	// two blocks at half fee should average to 75% of last
	blockScanner.updateGasCache(&MockFeeTx{
		gas: GasLimit,
		fee: ctypes.Coins{ctypes.NewCoin("uatom", ctypes.NewInt(10000))},
	})
	blockScanner.updateGasCache(&MockFeeTx{
		gas: GasLimit,
		fee: ctypes.Coins{ctypes.NewCoin("uatom", ctypes.NewInt(10000))},
	})
	c.Check(len(blockScanner.feeCache), Equals, 4)
	c.Check(blockScanner.averageFee().String(), Equals, fmt.Sprintf("%d", uint64(15000*atomToThorchain)))

	// skip transactions with multiple coins
	blockScanner.updateGasCache(&MockFeeTx{
		gas: GasLimit,
		fee: ctypes.Coins{
			ctypes.NewCoin("uatom", ctypes.NewInt(10000)),
			ctypes.NewCoin("uusd", ctypes.NewInt(10000)),
		},
	})
	c.Check(len(blockScanner.feeCache), Equals, 4)
	c.Check(blockScanner.averageFee().String(), Equals, fmt.Sprintf("%d", uint64(15000*atomToThorchain)))

	// skip transactions with fees not in uatom
	blockScanner.updateGasCache(&MockFeeTx{
		gas: GasLimit,
		fee: ctypes.Coins{
			ctypes.NewCoin("uusd", ctypes.NewInt(10000)),
		},
	})
	c.Check(len(blockScanner.feeCache), Equals, 4)
	c.Check(blockScanner.averageFee().String(), Equals, fmt.Sprintf("%d", uint64(15000*atomToThorchain)))

	// skip transactions with zero fee
	blockScanner.updateGasCache(&MockFeeTx{
		gas: GasLimit,
		fee: ctypes.Coins{
			ctypes.NewCoin("uusd", ctypes.NewInt(0)),
		},
	})
	c.Check(len(blockScanner.feeCache), Equals, 4)
	c.Check(blockScanner.averageFee().String(), Equals, fmt.Sprintf("%d", uint64(15000*atomToThorchain)))

	// ensure we only cache the transaction limit number of blocks
	for i := 0; i < GasCacheTransactions; i++ {
		blockScanner.updateGasCache(&MockFeeTx{
			gas: GasLimit,
			fee: ctypes.Coins{
				ctypes.NewCoin("uatom", ctypes.NewInt(10000)),
			},
		})
	}
	c.Check(len(blockScanner.feeCache), Equals, GasCacheTransactions)
	c.Check(blockScanner.averageFee().String(), Equals, fmt.Sprintf("%d", uint64(10000*atomToThorchain)))
}

func (s *BlockScannerTestSuite) TestGetBlock(c *C) {
	cfg := config.BifrostBlockScannerConfiguration{ChainID: common.GAIAChain}
	mockRPC := NewMockTmServiceClient()

	blockScanner := CosmosBlockScanner{
		cfg:       cfg,
		tmService: mockRPC,
	}

	block, err := blockScanner.GetBlock(1)

	c.Assert(err, IsNil)
	c.Assert(len(block.Data.Txs), Equals, 1)
	c.Assert(block.Header.Height, Equals, int64(6509672))
}

func (s *BlockScannerTestSuite) TestProcessTxs(c *C) {
	cfg := config.BifrostBlockScannerConfiguration{ChainID: common.GAIAChain}
	mockTmServiceClient := NewMockTmServiceClient()

	registry := s.bridge.GetContext().InterfaceRegistry
	registry.RegisterImplementations((*ctypes.Msg)(nil), &wasm.MsgExecuteContract{})
	btypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonFile, err := os.Open("./test-data/tx_results_by_height.json")
		if err != nil {
			c.Fatal("unable to load tx_results_by_height.json")
		}
		defer jsonFile.Close()

		res, _ := io.ReadAll(jsonFile)
		if _, err := w.Write(res); err != nil {
			c.Fatal("unable to write /block_result", err)
		}
	})
	server := httptest.NewServer(h)
	defer server.Close()

	rpcClient, err := rpcclient.NewWithClient(server.URL, "/websocket", server.Client())
	if err != nil {
		c.Fatal("fail to create tendermint rpcclient")
	}

	blockScanner := CosmosBlockScanner{
		cfg:       cfg,
		tmService: mockTmServiceClient,
		txService: rpcClient,
		cdc:       cdc,
		logger:    log.Logger.With().Str("module", "blockscanner").Str("chain", common.GAIAChain.String()).Logger(),
	}

	block, err := blockScanner.GetBlock(1)
	c.Assert(err, IsNil)

	txInItems, err := blockScanner.processTxs(1, block.Data.Txs)
	c.Assert(err, IsNil)

	// proccessTxs should filter out everything besides the valid MsgSend
	c.Assert(len(txInItems), Equals, 1)
}
