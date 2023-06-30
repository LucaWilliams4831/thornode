//go:build regtest
// +build regtest

package app

import (
	"net/http"
	"os"

	"github.com/rs/zerolog/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

var (
	begin = make(chan struct{})
	end   = make(chan struct{})
)

func init() {
	// start an http server to unblock a block creation when a request is received
	newBlock := func(w http.ResponseWriter, r *http.Request) {
		begin <- struct{}{}
		<-end
	}
	http.HandleFunc("/newBlock", newBlock)
	portString := os.Getenv("CREATE_BLOCK_PORT")
	go func() {
		err := http.ListenAndServe(":"+portString, nil)
		if err != nil {
			log.Fatal().Err(err).Msg("fail to start http server")
		}
	}()
}

func (app *THORChainApp) BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	<-begin
	return app.mm.BeginBlock(ctx, req)
}

// EndBlocker application updates every end block
func (app *THORChainApp) EndBlocker(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock {
	defer func() { end <- struct{}{} }()
	return app.mm.EndBlock(ctx, req)
}
