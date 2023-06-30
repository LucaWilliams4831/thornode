//go:build testnet || mocknet
// +build testnet mocknet

package config

import (
	"os"

	"github.com/rs/zerolog/log"
)

const (
	rpcPort = 26657
	p2pPort = 26656
)

func getSeedAddrs() []string {
	return []string{}
}

func assertBifrostHasSeeds() {
	// fail if seed file is missing or empty since bifrost will hang
	seedPath := os.ExpandEnv("$HOME/.thornode/address_book.seed")
	fi, err := os.Stat(seedPath)
	if os.IsNotExist(err) {
		log.Warn().Msg("no seed file found")
	}
	if err != nil {
		log.Warn().Err(err).Msg("failed to stat seed file")
		return
	}
	if fi.Size() == 0 {
		log.Warn().Msg("seed file is empty")
	}
}
