package main

import (
	"io"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

// trunk-ignore-all(golangci-lint/forcetypeassert)

// deepMerge merges two maps recursively - if there is an array in the map it will deep
// merge elements that match on keys in overrideKeys.
func deepMerge(a, b map[string]any, overrideKeys ...string) map[string]any {
	result := make(map[string]any)
	for k, v := range a {
		result[k] = v
	}

	for k := range b {
		switch v := b[k].(type) {
		case []any:
			if av, ok := result[k]; ok {
				if av, ok := av.([]any); ok {

					// deep merge if key in overrideKeys matches in the source and destination
					for _, ov := range overrideKeys {
						for i := range av {
							for j := range v {
								if av[i].(map[string]any)[ov] == v[j].(map[string]any)[ov] {
									av[i] = deepMerge(av[i].(map[string]any), v[j].(map[string]any), overrideKeys...)

									// remove value from v
									v = append(v[:j], v[j+1:]...)
									break
								}
							}
						}
					}

					result[k] = append(av, v...)
					continue
				}
			}
		case map[string]any:
			if av, ok := result[k]; ok {
				if av, ok := av.(map[string]any); ok {
					result[k] = deepMerge(av, v, overrideKeys...)
					continue
				}
			}
		}
		result[k] = b[k]
	}
	return result
}

func dumpLogs(out io.Writer, logs chan string) {
	for {
		select {
		case line := <-logs:
			_, _ = out.Write([]byte(line + "\n"))
			continue
		default:
		}
		break
	}
}

func drainLogs(logs chan string) {
	// if DEBUG is set skip draining logs
	if os.Getenv("DEBUG") != "" {
		return
	}

	for {
		select {
		case <-logs:
			continue
		case <-time.After(100 * time.Millisecond):
		}
		break
	}
}

func processRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(os.Signal(nil))
	return err == nil
}

func getTimeFactor() time.Duration {
	tf, err := strconv.ParseInt(os.Getenv("TIME_FACTOR"), 10, 64)
	if err != nil {
		return time.Duration(1)
	}
	return time.Duration(tf)
}

func consoleLogger(w io.Writer) zerolog.Logger {
	return zerolog.New(zerolog.ConsoleWriter{Out: w}).With().Timestamp().Caller().Logger()
}
