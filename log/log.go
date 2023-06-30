package log

import (
	"strings"

	"github.com/rs/zerolog"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"gitlab.com/thorchain/thornode/config"
)

var _ tmlog.Logger = (*TendermintLogWrapper)(nil)

// TendermintLogWrapper provides a wrapper around a zerolog.Logger instance. It implements
// Tendermint's Logger interface.
type TendermintLogWrapper struct {
	zerolog.Logger
}

// Info implements Tendermint's Logger interface and logs with level INFO. A set
// of key/value tuples may be provided to add context to the log. The number of
// tuples must be even and the key of the tuple must be a string.
func (z TendermintLogWrapper) Info(msg string, keyVals ...interface{}) {
	if z.filter(msg) {
		return
	}
	z.Logger.Info().Fields(getLogFields(keyVals...)).Msg(msg)
}

// Error implements Tendermint's Logger interface and logs with level ERR. A set
// of key/value tuples may be provided to add context to the log. The number of
// tuples must be even and the key of the tuple must be a string.
func (z TendermintLogWrapper) Error(msg string, keyVals ...interface{}) {
	if z.filter(msg) {
		return
	}
	z.Logger.Error().Fields(getLogFields(keyVals...)).Msg(msg)
}

// Debug implements Tendermint's Logger interface and logs with level DEBUG. A set
// of key/value tuples may be provided to add context to the log. The number of
// tuples must be even and the key of the tuple must be a string.
func (z TendermintLogWrapper) Debug(msg string, keyVals ...interface{}) {
	if z.filter(msg) {
		return
	}
	z.Logger.Debug().Fields(getLogFields(keyVals...)).Msg(msg)
}

// With returns a new wrapped logger with additional context provided by a set
// of key/value tuples. The number of tuples must be even and the key of the
// tuple must be a string.
func (z TendermintLogWrapper) With(keyVals ...interface{}) tmlog.Logger {
	// skip filtering modules if unexpected keyVals or debug level
	if len(keyVals)%2 != 0 || z.Logger.GetLevel() <= zerolog.DebugLevel {
		return TendermintLogWrapper{
			Logger: z.Logger.With().Fields(getLogFields(keyVals...)).Logger(),
		}
	}

	for i := 0; i < len(keyVals); i += 2 {
		name, ok := keyVals[i].(string)
		if !ok {
			z.Logger.Error().Interface("key", keyVals[i]).Msg("non-string logging key provided")
		}
		if name != "module" {
			continue
		}
		value, ok := keyVals[i+1].(string)
		if !ok {
			continue
		}
		for _, item := range config.GetThornode().LogFilter.Modules {
			if strings.EqualFold(item, value) {
				return TendermintLogWrapper{
					Logger: z.Logger.Level(zerolog.WarnLevel).With().Fields(getLogFields(keyVals...)).Logger(),
				}
			}
		}
	}

	return TendermintLogWrapper{
		Logger: z.Logger.With().Fields(getLogFields(keyVals...)).Logger(),
	}
}

func getLogFields(keyVals ...interface{}) map[string]interface{} {
	if len(keyVals)%2 != 0 {
		return nil
	}

	fields := make(map[string]interface{})
	for i := 0; i < len(keyVals); i += 2 {
		val, ok := keyVals[i].(string)
		if ok {
			fields[val] = keyVals[i+1]
		}
	}

	return fields
}

func (z TendermintLogWrapper) filter(msg string) bool {
	if z.Logger.GetLevel() > zerolog.DebugLevel {
		for _, filter := range config.GetThornode().LogFilter.Messages {
			if filter == msg {
				return true
			}
		}
	}
	return false
}
