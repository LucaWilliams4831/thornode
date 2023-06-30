package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"text/template"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

////////////////////////////////////////////////////////////////////////////////////////
// Run
////////////////////////////////////////////////////////////////////////////////////////

func run(out io.Writer, path string, routine int) error {
	localLog := consoleLogger(out)

	home := "/" + strconv.Itoa(routine)
	localLog.Info().Msgf("Running regression test: %s", path)

	// clear data directory
	localLog.Debug().Msg("Clearing data directory")
	thornodePath := filepath.Join(home, ".thornode")
	cmdOut, err := exec.Command("rm", "-rf", thornodePath).CombinedOutput()
	if err != nil {
		fmt.Println(string(cmdOut))
		log.Fatal().Err(err).Msg("failed to clear data directory")
	}

	// use same environment for all commands
	env := []string{
		"HOME=" + home,
		"THOR_TENDERMINT_INSTRUMENTATION_PROMETHEUS=false",
		// block time should be short, but all consecutive checks must complete within timeout
		fmt.Sprintf("THOR_TENDERMINT_CONSENSUS_TIMEOUT_COMMIT=%s", time.Second*getTimeFactor()),
		// all ports will be offset by the routine number
		fmt.Sprintf("THOR_COSMOS_API_ADDRESS=tcp://0.0.0.0:%d", 1317+routine),
		fmt.Sprintf("THOR_TENDERMINT_RPC_LISTEN_ADDRESS=tcp://0.0.0.0:%d", 26657+routine),
		fmt.Sprintf("THOR_TENDERMINT_P2P_LISTEN_ADDRESS=tcp://0.0.0.0:%d", 27000+routine),
		"CREATE_BLOCK_PORT=" + strconv.Itoa(8080+routine),
		"GOCOVERDIR=/mnt/coverage",
	}

	// if DEBUG is set also output thornode debug logs
	if os.Getenv("DEBUG") != "" {
		env = append(env, "THOR_TENDERMINT_LOG_LEVEL=debug")
	}

	// init chain with dog mnemonic
	localLog.Debug().Msg("Initializing chain")
	cmd := exec.Command("thornode", "init", "local", "--chain-id", "thorchain", "--recover")
	cmd.Stdin = bytes.NewBufferString(dogMnemonic + "\n")
	cmd.Env = env
	cmdOut, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(cmdOut))
		log.Fatal().Err(err).Msg("failed to initialize chain")
	}

	// init chain
	localLog.Debug().Msg("Initializing chain")
	cmd = exec.Command("thornode", "init", "local", "--chain-id", "thorchain", "-o")
	cmd.Env = env
	cmdOut, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(cmdOut))
		log.Fatal().Err(err).Msg("failed to initialize chain")
	}

	// create routine local state (used later by custom template functions in operations)
	nativeTxIDsMu.Lock()
	nativeTxIDs[routine] = []string{}
	nativeTxIDsMu.Unlock()
	tmpls := template.Must(templates.Clone())

	// ensure no naming collisions
	if tmpls.Lookup(filepath.Base(path)) != nil {
		log.Fatal().Msgf("test name collision: %s", filepath.Base(path))
	}

	// read the file
	f, err := os.Open(path)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open test file")
	}
	fileBytes, err := io.ReadAll(f)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read test file")
	}
	f.Close()

	// track line numbers
	opLines := []int{0}
	scanner := bufio.NewScanner(bytes.NewBuffer(fileBytes))
	for i := 0; scanner.Scan(); i++ {
		line := scanner.Text()
		if line == "---" {
			opLines = append(opLines, i+2)
		}
	}

	// parse the template
	tmpl, err := tmpls.Parse(string(fileBytes))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse template")
	}

	// render the template
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to render template")
	}

	// all operations we will execute
	ops := []Operation{}

	// track whether we've seen non-state operations
	seenNonState := false

	dec := yaml.NewDecoder(buf)
	for {
		// decode into temporary type
		op := map[string]any{}
		err = dec.Decode(&op)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal().Err(err).Msg("failed to decode operation")
		}

		// warn empty operations
		if len(op) == 0 {
			localLog.Warn().Msg("empty operation, line numbers may be wrong")
			continue
		}

		// state operations must be first
		if op["type"] == "state" && seenNonState {
			log.Fatal().Msg("state operations must be first")
		}
		if op["type"] != "state" {
			seenNonState = true
		}

		ops = append(ops, NewOperation(op))
	}

	// warn if no operations found
	if len(ops) == 0 {
		err = errors.New("no operations found")
		localLog.Err(err).Msg("")
		return err
	}

	// execute all state operations
	stateOpCount := 0
	for i, op := range ops {
		if _, ok := op.(*OpState); ok {
			localLog.Info().Int("line", opLines[i]).Msgf(">>> [%d] %s", i+1, op.OpType())
			err = op.Execute(out, routine, cmd.Process, nil)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to execute state operation")
			}
			stateOpCount++
		}
	}
	ops = ops[stateOpCount:]
	opLines = opLines[stateOpCount:]

	// validate genesis
	localLog.Debug().Msg("Validating genesis")
	cmd = exec.Command("thornode", "validate-genesis")
	cmd.Env = env
	cmdOut, err = cmd.CombinedOutput()
	if err != nil {
		// dump the genesis
		fmt.Println(ColorPurple + "Genesis:" + ColorReset)
		f, err := os.OpenFile(filepath.Join(home, ".thornode/config/genesis.json"), os.O_RDWR, 0o644)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to open genesis file")
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
		f.Close()

		// dump error and exit
		fmt.Println(string(cmdOut))
		log.Fatal().Err(err).Msg("genesis validation failed")
	}

	// render config
	localLog.Debug().Msg("Rendering config")
	cmd = exec.Command("thornode", "render-config")
	cmd.Env = env
	err = cmd.Run()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to render config")
	}

	// overwrite private validator key
	localLog.Debug().Msg("Overwriting private validator key")
	keyPath := filepath.Join(home, ".thornode/config/priv_validator_key.json")
	cmd = exec.Command("cp", "/mnt/priv_validator_key.json", keyPath)
	err = cmd.Run()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to overwrite private validator key")
	}

	logLevel := "info"
	switch os.Getenv("DEBUG") {
	case "trace", "debug", "info", "warn", "error", "fatal", "panic":
		logLevel = os.Getenv("DEBUG")
	}

	// setup process io
	thornode := exec.Command("/regtest/cover-thornode", "--log_level", logLevel, "start")
	thornode.Env = env

	stderr, err := thornode.StderrPipe()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup thornode stderr")
	}
	stderrScanner := bufio.NewScanner(stderr)
	stderrLines := make(chan string, 100)
	go func() {
		for stderrScanner.Scan() {
			stderrLines <- stderrScanner.Text()
		}
	}()
	if os.Getenv("DEBUG") != "" {
		thornode.Stdout = os.Stdout
		thornode.Stderr = os.Stderr
	}

	// start thornode process
	localLog.Debug().Msg("Starting thornode")
	err = thornode.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start thornode")
	}

	// wait for thornode to listen on block creation port
	time.Sleep(time.Second)
	for i := 0; ; i++ {
		if i%100 == 0 {
			localLog.Debug().Msg("Waiting for thornode to listen")
		}
		time.Sleep(100 * time.Millisecond)
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", 8080+routine))
		if err == nil {
			conn.Close()
			break
		}
	}

	// run the operations
	var returnErr error
	localLog.Info().Msgf("Executing %d operations", len(ops))
	for i, op := range ops {
		localLog.Info().Int("line", opLines[i]).Msgf(">>> [%d] %s", stateOpCount+i+1, op.OpType())
		returnErr = op.Execute(out, routine, thornode.Process, stderrLines)
		if returnErr != nil {
			localLog.Error().Err(returnErr).
				Int("line", opLines[i]).
				Int("op", stateOpCount+i+1).
				Str("type", op.OpType()).
				Str("path", path).
				Msg("operation failed")
			dumpLogs(out, stderrLines)
			break
		}
	}

	// log success
	if returnErr == nil {
		localLog.Info().Msg("All operations succeeded")
	}

	// stop thornode process
	localLog.Debug().Msg("Stopping thornode")
	err = thornode.Process.Signal(syscall.SIGUSR1)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to stop thornode")
	}

	// wait for process to exit
	_, err = thornode.Process.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to wait for thornode")
	}

	// if failed and debug enabled restart to allow inspection
	if returnErr != nil && os.Getenv("DEBUG") != "" {

		// remove validator key (otherwise thornode will hang in begin block)
		localLog.Debug().Msg("Removing validator key")
		cmd = exec.Command("rm", keyPath)
		cmdOut, err = cmd.CombinedOutput()
		if err != nil {
			fmt.Println(string(cmdOut))
			log.Fatal().Err(err).Msg("failed to remove validator key")
		}

		// restart thornode
		localLog.Debug().Msg("Restarting thornode")
		thornode = exec.Command("thornode", "start")
		thornode.Env = env
		thornode.Stdout = os.Stdout
		thornode.Stderr = os.Stderr
		err = thornode.Start()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to restart thornode")
		}

		// wait for thornode
		localLog.Debug().Msg("Waiting for thornode")
		_, err = thornode.Process.Wait()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to wait for thornode")
		}
	}

	return returnErr
}
