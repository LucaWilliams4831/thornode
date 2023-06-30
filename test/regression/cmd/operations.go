package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"text/template"
	"time"

	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
	"gopkg.in/yaml.v3"
)

////////////////////////////////////////////////////////////////////////////////////////
// Operation
////////////////////////////////////////////////////////////////////////////////////////

type Operation interface {
	Execute(out io.Writer, routine int, thornode *os.Process, logs chan string) error
	OpType() string
}

type OpBase struct {
	Type string `json:"type"`
}

func (op *OpBase) OpType() string {
	return op.Type
}

func NewOperation(opMap map[string]any) Operation {
	// ensure type is provided
	t, ok := opMap["type"].(string)
	if !ok {
		log.Fatal().Interface("type", opMap["type"]).Msg("operation type is not a string")
	}

	// create the operation for the type
	var op Operation
	switch t {
	case "state":
		op = &OpState{}
	case "check":
		op = &OpCheck{}
	case "create-blocks":
		op = &OpCreateBlocks{}
	case "tx-ban":
		op = &OpTxBan{}
	case "tx-deposit":
		op = &OpTxDeposit{}
	case "tx-errata-tx":
		op = &OpTxErrataTx{}
	case "tx-mimir":
		op = &OpTxMimir{}
	case "tx-observed-in":
		op = &OpTxObservedIn{}
	case "tx-observed-out":
		op = &OpTxObservedOut{}
	case "tx-network-fee":
		op = &OpTxNetworkFee{}
	case "tx-node-pause-chain":
		op = &OpTxNodePauseChain{}
	case "tx-send":
		op = &OpTxSend{}
	case "tx-set-ip-address":
		op = &OpTxSetIPAddress{}
	case "tx-set-node-keys":
		op = &OpTxSetNodeKeys{}
	case "tx-solvency":
		op = &OpTxSolvency{}
	case "tx-tss-keysign":
		op = &OpTxTssKeysign{}
	case "tx-tss-pool":
		op = &OpTxTssPool{}
	case "tx-version":
		op = &OpTxVersion{}
	default:
		log.Fatal().Str("type", t).Msg("unknown operation type")
	}

	// create decoder supporting embedded structs and weakly typed input
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		ErrorUnused:      true,
		Squash:           true,
		Result:           op,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create decoder")
	}

	switch op.(type) {
	// internal types have MarshalJSON methods necessary to decode
	case *OpTxBan,
		*OpTxErrataTx,
		*OpTxObservedIn,
		*OpTxObservedOut,
		*OpTxDeposit,
		*OpTxMimir,
		*OpTxNetworkFee,
		*OpTxNodePauseChain,
		*OpTxSolvency,
		*OpTxSend,
		*OpTxSetIPAddress,
		*OpTxSetNodeKeys,
		*OpTxVersion,
		*OpTxTssKeysign,
		*OpTxTssPool:
		// encode as json
		buf := bytes.NewBuffer(nil)
		enc := json.NewEncoder(buf)
		err = enc.Encode(opMap)
		if err != nil {
			log.Fatal().Interface("op", opMap).Err(err).Msg("failed to encode operation")
		}

		// unmarshal json to op
		err = json.NewDecoder(buf).Decode(op)

	default:
		err = dec.Decode(opMap)
	}
	if err != nil {
		log.Fatal().Interface("op", opMap).Err(err).Msg("failed to decode operation")
	}

	// require check description and default status check to 200 if endpoint is set
	if oc, ok := op.(*OpCheck); ok && oc.Endpoint != "" {
		if oc.Status == 0 {
			oc.Status = 200
		}
	}

	return op
}

////////////////////////////////////////////////////////////////////////////////////////
// OpState
////////////////////////////////////////////////////////////////////////////////////////

type OpState struct {
	OpBase  `yaml:",inline"`
	Genesis map[string]any `json:"genesis"`
}

func (op *OpState) Execute(_ io.Writer, routine int, _ *os.Process, _ chan string) error {
	// extract HOME from command environment
	home := fmt.Sprintf("/%d", routine)

	// load genesis file
	f, err := os.OpenFile(filepath.Join(home, ".thornode/config/genesis.json"), os.O_RDWR, 0o644)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open genesis file")
	}

	// unmarshal genesis into map
	var genesisMap map[string]any
	err = json.NewDecoder(f).Decode(&genesisMap)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to decode genesis file")
	}

	// merge updates into genesis
	genesis := deepMerge(genesisMap, op.Genesis, "address")

	// reset file
	err = f.Truncate(0)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to truncate genesis file")
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to seek genesis file")
	}

	// marshal genesis into file
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	err = enc.Encode(genesis)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to encode genesis file")
	}

	return f.Close()
}

////////////////////////////////////////////////////////////////////////////////////////
// OpCheck
////////////////////////////////////////////////////////////////////////////////////////

type OpCheck struct {
	OpBase      `yaml:",inline"`
	Description string            `json:"description"`
	Endpoint    string            `json:"endpoint"`
	Params      map[string]string `json:"params"`
	Status      int               `json:"status"`
	Asserts     []string          `json:"asserts"`
}

func (op *OpCheck) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	localLog := consoleLogger(out)

	// abort if no endpoint is set (empty check op is allowed for breakpoint convenience)
	if op.Endpoint == "" {
		return fmt.Errorf("check")
	}

	// build request
	req, err := http.NewRequest("GET", op.Endpoint, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build request")
	}

	// parse the endpoint and add routine to the port number
	port, err := strconv.Atoi(req.URL.Port())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse port")
	}
	if req.URL.Hostname() != "localhost" { // host must be localhost
		log.Fatal().Str("host", req.URL.Hostname()).Msg("endpoint host must be localhost")
	}
	req.URL.Host = fmt.Sprintf("localhost:%d", port+routine)

	// add params
	q := req.URL.Query()
	for k, v := range op.Params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	// send request
	resp, err := httpClient.Do(req)
	if err != nil {
		localLog.Err(err).Msg("failed to send request")
		return err
	}

	// read response
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		localLog.Err(err).Msg("failed to read response")
		return err
	}

	// ensure status code matches
	if resp.StatusCode != op.Status {
		// dump pretty output for debugging
		_, _ = out.Write([]byte(ColorPurple + "\nOperation:" + ColorReset + "\n"))
		_ = yaml.NewEncoder(out).Encode(op)
		_, _ = out.Write([]byte(ColorPurple + "\nEndpoint Response:" + ColorReset + "\n"))
		_, _ = out.Write([]byte(string(buf) + "\n"))

		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// ensure response is not empty
	if len(buf) == 0 {
		if os.Getenv("DEBUG") == "" {
			fmt.Println(ColorPurple + "\nLogs:" + ColorReset)
			dumpLogs(out, logs)
		}

		fmt.Println(ColorPurple + "\nOperation:" + ColorReset)
		_ = yaml.NewEncoder(os.Stdout).Encode(op)
		fmt.Println()
		return fmt.Errorf("empty response")
	}

	// pipe response to jq for assertions
	for _, a := range op.Asserts {
		// render the assert expression (used for native_txid)
		funcMap := template.FuncMap{
			"native_txid": func(i int) string {
				// allow reverse indexing
				nativeTxIDsMu.Lock()
				defer nativeTxIDsMu.Unlock()
				if i < 0 {
					i += len(nativeTxIDs[routine]) + 1
				}
				return nativeTxIDs[routine][i-1]
			},
		}
		tmpl := template.Must(template.Must(templates.Clone()).Funcs(funcMap).Parse(a))
		expr := bytes.NewBuffer(nil)
		err = tmpl.Execute(expr, nil)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to render assert expression")
		}
		a = expr.String()

		cmd := exec.Command("jq", "-e", a)
		cmd.Stdin = bytes.NewReader(buf)
		cmdOut, err := cmd.CombinedOutput()
		if err != nil {
			if cmd.ProcessState.ExitCode() == 1 && os.Getenv("DEBUG") == "" {
				// dump process logs if the assert expression failed
				_, _ = out.Write([]byte(ColorPurple + "\nLogs:" + ColorReset + "\n"))
				dumpLogs(out, logs)
			}

			// dump pretty output for debugging
			_, _ = out.Write([]byte(ColorPurple + "\nOperation:" + ColorReset + "\n"))
			_ = yaml.NewEncoder(out).Encode(op)
			_, _ = out.Write([]byte(ColorPurple + "\nFailed Assert: " + ColorReset + expr.String() + "\n"))
			_, _ = out.Write([]byte(ColorPurple + "\nEndpoint Response:" + ColorReset + "\n"))
			_, _ = out.Write([]byte(string(buf) + "\n"))

			// log fatal on syntax errors and skip logs
			if cmd.ProcessState.ExitCode() != 1 {
				drainLogs(logs)
				_, _ = out.Write([]byte(ColorRed + string(cmdOut) + ColorReset + "\n"))
			}

			return err
		}
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////////////
// OpCreateBlocks
////////////////////////////////////////////////////////////////////////////////////////

type OpCreateBlocks struct {
	OpBase `yaml:",inline"`
	Count  int  `json:"count"`
	Exit   *int `json:"exit"`
}

func (op *OpCreateBlocks) Execute(out io.Writer, routine int, p *os.Process, logs chan string) error {
	localLog := consoleLogger(out)

	// clear existing log output
	drainLogs(logs)

	for i := 0; i < op.Count; i++ {
		// http request to localhost to unblock block creation
		_, err := httpClient.Get(fmt.Sprintf("http://localhost:%d/newBlock", 8080+routine))
		if err != nil {
			// if exit code is not set this was unexpected
			if op.Exit == nil {
				localLog.Err(err).Msg("failed to create block")
				return err
			}

			// if exit code is set, this was expected
			if processRunning(p.Pid) {
				localLog.Err(err).Msg("block did not exit as expected")
				return err
			}

			// if process is not running, check exit code
			ps, err := p.Wait()
			if err != nil {
				localLog.Err(err).Msg("failed to wait for process")
				return err
			}
			if ps.ExitCode() != *op.Exit {
				localLog.Error().Int("exit", ps.ExitCode()).Int("expect", *op.Exit).Msg("bad exit code")
				return err
			}

			// exit code is correct, return nil
			return nil
		}
	}

	// if exit code is set, this was unexpected
	if op.Exit != nil {
		localLog.Error().Int("expect", *op.Exit).Msg("expected exit code")
		return errors.New("expected exit code")
	}

	// avoid minor raciness after end block
	time.Sleep(200 * time.Millisecond * getTimeFactor())

	return nil
}

////////////////////////////////////////////////////////////////////////////////////////
// Transaction Operations
////////////////////////////////////////////////////////////////////////////////////////

// ------------------------------ OpTxBan ------------------------------

type OpTxBan struct {
	OpBase      `yaml:",inline"`
	NodeAddress sdk.AccAddress `json:"node_address"`
	Signer      sdk.AccAddress `json:"signer"`
	Sequence    *int64         `json:"sequence"`
}

func (op *OpTxBan) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	msg := types.NewMsgBan(op.NodeAddress, op.Signer)
	return sendMsg(out, routine, msg, op.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxErrataTx ------------------------------

type OpTxErrataTx struct {
	OpBase   `yaml:",inline"`
	TxID     common.TxID    `json:"tx_id"`
	Chain    common.Chain   `json:"chain"`
	Signer   sdk.AccAddress `json:"signer"`
	Sequence *int64         `json:"sequence"`
}

func (op *OpTxErrataTx) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	msg := types.NewMsgErrataTx(op.TxID, op.Chain, op.Signer)
	return sendMsg(out, routine, msg, op.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxNetworkFee ------------------------------

type OpTxNetworkFee struct {
	OpBase          `yaml:",inline"`
	BlockHeight     int64          `json:"block_height"`
	Chain           common.Chain   `json:"chain"`
	TransactionSize uint64         `json:"transaction_size"`
	TransactionRate uint64         `json:"transaction_rate"`
	Signer          sdk.AccAddress `json:"signer"`
	Sequence        *int64         `json:"sequence"`
}

func (op *OpTxNetworkFee) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	msg := types.NewMsgNetworkFee(op.BlockHeight, op.Chain, op.TransactionSize, op.TransactionRate, op.Signer)
	return sendMsg(out, routine, msg, op.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxNodePauseChain ------------------------------

type OpTxNodePauseChain struct {
	OpBase   `yaml:",inline"`
	Value    int64          `json:"value"`
	Signer   sdk.AccAddress `json:"signer"`
	Sequence *int64         `json:"sequence"`
}

func (op *OpTxNodePauseChain) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	msg := types.NewMsgNodePauseChain(op.Value, op.Signer)
	return sendMsg(out, routine, msg, op.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxObservedIn ------------------------------

type OpTxObservedIn struct {
	OpBase   `yaml:",inline"`
	Txs      []types.ObservedTx `json:"txs"`
	Signer   sdk.AccAddress     `json:"signer"`
	Sequence *int64             `json:"sequence"`
}

func (op *OpTxObservedIn) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	msg := types.NewMsgObservedTxIn(op.Txs, op.Signer)
	return sendMsg(out, routine, msg, op.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxObservedOut ------------------------------

type OpTxObservedOut struct {
	OpBase   `yaml:",inline"`
	Txs      []types.ObservedTx `json:"txs"`
	Signer   sdk.AccAddress     `json:"signer"`
	Sequence *int64             `json:"sequence"`
}

func (op *OpTxObservedOut) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	// render the memos (used for native_txid)
	funcMap := template.FuncMap{
		"native_txid": func(i int) string {
			// allow reverse indexing
			nativeTxIDsMu.Lock()
			defer nativeTxIDsMu.Unlock()
			if i < 0 {
				i += len(nativeTxIDs[routine]) + 1
			}
			return nativeTxIDs[routine][i-1]
		},
	}
	for i := range op.Txs {
		tx := &op.Txs[i]
		tmpl := template.Must(template.Must(templates.Clone()).Funcs(funcMap).Parse(tx.Tx.Memo))
		memo := bytes.NewBuffer(nil)
		err := tmpl.Execute(memo, nil)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to render memo")
		}
		tx.Tx.Memo = memo.String()
	}

	msg := types.NewMsgObservedTxOut(op.Txs, op.Signer)
	return sendMsg(out, routine, msg, op.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxSetIPAddress ------------------------------

type OpTxSetIPAddress struct {
	OpBase    `yaml:",inline"`
	IPAddress string         `json:"ip_address"`
	Signer    sdk.AccAddress `json:"signer"`
	Sequence  *int64         `json:"sequence"`
}

func (op *OpTxSetIPAddress) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	msg := types.NewMsgSetIPAddress(op.IPAddress, op.Signer)
	return sendMsg(out, routine, msg, op.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxSolvency ------------------------------

type OpTxSolvency struct {
	OpBase   `yaml:",inline"`
	Chain    common.Chain   `json:"chain"`
	PubKey   common.PubKey  `json:"pub_key"`
	Coins    common.Coins   `json:"coins"`
	Height   int64          `json:"height"`
	Signer   sdk.AccAddress `json:"signer"`
	Sequence *int64         `json:"sequence"`
}

func (op *OpTxSolvency) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	msg, err := types.NewMsgSolvency(op.Chain, op.PubKey, op.Coins, op.Height, op.Signer)
	if err != nil {
		return err
	}
	return sendMsg(out, routine, msg, op.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxSetNodeKeys ------------------------------

type OpTxSetNodeKeys struct {
	OpBase              `yaml:",inline"`
	PubKeySet           common.PubKeySet `json:"pub_key_set"`
	ValidatorConsPubKey string           `json:"validator_cons_pub_key"`
	Signer              sdk.AccAddress   `json:"signer"`
	Sequence            *int64           `json:"sequence"`
}

func (op *OpTxSetNodeKeys) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	msg := types.NewMsgSetNodeKeys(op.PubKeySet, op.ValidatorConsPubKey, op.Signer)
	return sendMsg(out, routine, msg, op.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxTssKeysign ------------------------------

type OpTxTssKeysign struct {
	OpBase                  `yaml:",inline"`
	types.MsgTssKeysignFail `yaml:",inline"`
	// Signer                  sdk.AccAddress `json:"signer"`
	Sequence *int64 `json:"sequence"`
}

func (op *OpTxTssKeysign) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	return sendMsg(out, routine, &op.MsgTssKeysignFail, op.MsgTssKeysignFail.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxTssPool ------------------------------

type OpTxTssPool struct {
	OpBase          `yaml:",inline"`
	PubKeys         []string         `json:"pub_keys"`
	PoolPubKey      common.PubKey    `json:"pool_pub_key"`
	KeysharesBackup []byte           `json:"keyshares_backup"`
	KeygenType      types.KeygenType `json:"keygen_type"`
	Height          int64            `json:"height"`
	Blame           types.Blame      `json:"blame"`
	Chains          []string         `json:"chains"`
	Signer          sdk.AccAddress   `json:"signer"`
	KeygenTime      int64            `json:"keygen_time"`
	Sequence        *int64           `json:"sequence"`
}

func (op *OpTxTssPool) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	msg, err := types.NewMsgTssPool(op.PubKeys, op.PoolPubKey, op.KeysharesBackup, op.KeygenType, op.Height, op.Blame, op.Chains, op.Signer, op.KeygenTime)
	if err != nil {
		return err
	}
	return sendMsg(out, routine, msg, op.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxDeposit ------------------------------

type OpTxDeposit struct {
	OpBase           `yaml:",inline"`
	types.MsgDeposit `yaml:",inline"`
	Sequence         *int64 `json:"sequence"`
}

func (op *OpTxDeposit) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	return sendMsg(out, routine, &op.MsgDeposit, op.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxMimir ------------------------------

type OpTxMimir struct {
	OpBase         `yaml:",inline"`
	types.MsgMimir `yaml:",inline"`
	Sequence       *int64 `json:"sequence"`
}

func (op *OpTxMimir) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	return sendMsg(out, routine, &op.MsgMimir, op.Signer, op.Sequence, op, logs)
}

// ------------------------------ OpTxSend ------------------------------

type OpTxSend struct {
	OpBase        `yaml:",inline"`
	types.MsgSend `yaml:",inline"`
	Sequence      *int64 `json:"sequence"`
}

func (op *OpTxSend) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	return sendMsg(out, routine, &op.MsgSend, op.FromAddress, op.Sequence, op, logs)
}

// ------------------------------ OpTxVersion ------------------------------

type OpTxVersion struct {
	OpBase              `yaml:",inline"`
	types.MsgSetVersion `yaml:",inline"`
	Sequence            *int64 `json:"sequence"`
}

func (op *OpTxVersion) Execute(out io.Writer, routine int, _ *os.Process, logs chan string) error {
	return sendMsg(out, routine, &op.MsgSetVersion, op.Signer, op.Sequence, op, logs)
}

////////////////////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////////////////////

func sendMsg(out io.Writer, routine int, msg sdk.Msg, signer sdk.AccAddress, seq *int64, op any, logs chan string) error {
	log := log.Output(zerolog.ConsoleWriter{Out: out})

	// check that message is valid
	err := msg.ValidateBasic()
	if err != nil {
		enc := json.NewEncoder(out) // json instead of yaml to encode amount
		enc.SetIndent("", "  ")
		_ = enc.Encode(op)
		log.Fatal().Err(err).Msg("failed to validate basic")
	}

	clientCtx, txFactory := clientContextAndFactory(routine)

	// custom client context
	buf := bytes.NewBuffer(nil)
	ctx := clientCtx.WithFromAddress(signer)
	ctx = ctx.WithFromName(addressToName[signer.String()])
	ctx = ctx.WithOutput(buf)

	// override the sequence if provided
	txf := txFactory
	if seq != nil {
		txf = txFactory.WithSequence(uint64(*seq))
	}

	// send message
	err = tx.GenerateOrBroadcastTxWithFactory(ctx, txf, msg)
	if err != nil {
		_, _ = out.Write([]byte(ColorPurple + "\nOperation:" + ColorReset))
		enc := json.NewEncoder(out) // json instead of yaml to encode amount
		enc.SetIndent("", "  ")
		_ = enc.Encode(op)
		_, _ = out.Write([]byte(ColorPurple + "\nTx Output:" + ColorReset))
		drainLogs(logs)
		return err
	}

	// extract txhash from output json
	var txRes sdk.TxResponse
	err = encodingConfig.Marshaler.UnmarshalJSON(buf.Bytes(), &txRes)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to unmarshal tx response")
	}

	// fail if tx did not send, otherwise add to out native tx ids
	if txRes.Code != 0 {
		log.Debug().Uint32("code", txRes.Code).Str("log", txRes.RawLog).Msg("tx send failed")
	} else {
		nativeTxIDsMu.Lock()
		nativeTxIDs[routine] = append(nativeTxIDs[routine], txRes.TxHash)
		nativeTxIDsMu.Unlock()
	}

	return err
}
