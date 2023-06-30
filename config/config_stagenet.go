//go:build stagenet
// +build stagenet

package config

import (
	"os"

	"github.com/rs/zerolog/log"
)

const (
	rpcPort = 27147
	p2pPort = 27146
)

func getSeedAddrs() (addrs []string) {
	return []string{"stagenet-seed.ninerealms.com"}
}

func assertBifrostHasSeeds() {
	// fail if seed file is missing or empty since bifrost will hang
	seedPath := os.ExpandEnv("$HOME/.thornode/address_book.seed")
	fi, err := os.Stat(seedPath)
	if os.IsNotExist(err) {
		log.Fatal().Msg("no seed file found")
	}
	if err != nil {
		log.Fatal().Err(err).Msg("failed to stat seed file")
	}
	if fi.Size() == 0 {
		log.Fatal().Msg("seed file is empty")
	}
}
