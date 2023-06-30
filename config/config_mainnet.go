//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package config

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
)

const (
	rpcPort = 27147
	p2pPort = 27146
)

func getSeedAddrs() (addrs []string) {
	// fetch seeds
	res, err := http.Get("https://api.ninerealms.com/thorchain/seeds")
	if err != nil {
		log.Error().Err(err).Msg("failed to get seeds")
		return
	}

	// unmarshal seeds response
	var seedsResponse []string
	dec := json.NewDecoder(res.Body)
	err = dec.Decode(&seedsResponse)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal seeds response")
	}

	return seedsResponse
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
