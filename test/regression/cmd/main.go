package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"
)

////////////////////////////////////////////////////////////////////////////////////////
// Main
////////////////////////////////////////////////////////////////////////////////////////

func main() {
	// parse the regex in the RUN environment variable to determine which tests to run
	runRegex := regexp.MustCompile(".*")
	if len(os.Getenv("RUN")) > 0 {
		runRegex = regexp.MustCompile(os.Getenv("RUN"))
	}

	// find all regression tests in path
	files := []string{}
	err := filepath.Walk("suites", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// skip files that are not yaml
		if filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" {
			return nil
		}

		if runRegex.MatchString(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to find regression tests")
	}

	// sort the files
	sort.Strings(files)

	// keep track of the results
	mu := sync.Mutex{}
	succeeded := []string{}
	failed := []string{}

	// get parallelism from environment variable if DEBUG is not set
	parallelism := 1
	sem := make(chan struct{}, 1)
	wg := sync.WaitGroup{}
	if len(os.Getenv("PARALLELISM")) > 0 && len(os.Getenv("DEBUG")) == 0 {
		parallelism, err = strconv.Atoi(os.Getenv("PARALLELISM"))
		if err != nil {
			log.Fatal().Err(err).Msg("failed to parse PARALLELISM")
		}
		sem = make(chan struct{}, parallelism)
	}
	if parallelism > 1 {
		log.Info().Int("parallelism", parallelism).Msg("running tests in parallel")
	}

	// run tests
	for i, file := range files {
		sem <- struct{}{}
		wg.Add(1)

		go func(routine int, file string) {
			// create home directory
			home := "/" + strconv.Itoa(routine)
			_ = os.MkdirAll(home, 0o755)

			// create a buffer to capture the logs
			var out io.Writer = os.Stderr
			buf := new(bytes.Buffer)
			if parallelism > 1 {
				out = buf
			}

			// release semaphore and wait group
			defer func() {
				<-sem
				wg.Done()

				// write buffer to outputs
				mu.Lock()
				if parallelism > 1 {
					fmt.Println(buf.String())
				}
				mu.Unlock()
			}()

			// run test
			if parallelism == 1 {
				fmt.Println()
			}
			err = run(out, file, routine)
			if err != nil {
				mu.Lock()
				failed = append(failed, file)
				mu.Unlock()
				return
			}

			// check export state
			err = export(out, file, routine)
			if err != nil {
				mu.Lock()
				failed = append(failed, file)
				mu.Unlock()
				return
			}

			// success
			mu.Lock()
			succeeded = append(succeeded, file)
			mu.Unlock()
		}(i, file)
	}

	// wait for all tests to finish
	wg.Wait()

	// print the results
	fmt.Println()
	fmt.Printf("%sSucceeded:%s %d\n", ColorGreen, ColorReset, len(succeeded))
	for _, file := range succeeded {
		fmt.Printf("- %s\n", file)
	}
	fmt.Printf("%sFailed:%s %d\n", ColorRed, ColorReset, len(failed))
	for _, file := range failed {
		fmt.Printf("- %s\n", file)
	}
	fmt.Println()

	// exit with error code if any tests failed
	if len(failed) > 0 {
		os.Exit(1)
	}
}
