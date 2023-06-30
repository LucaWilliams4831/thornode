package app

import (
	tmlog "github.com/tendermint/tendermint/libs/log"
	"gitlab.com/thorchain/thornode/log"
)

var _ tmlog.Logger = (*log.TendermintLogWrapper)(nil)
