package rest

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types/rest"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/thornode/x/thorchain/query"
)

// Ping - endpoint to check that the API is up and available
func pingHandler(cliCtx client.Context, storeName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"ping":"pong"}`)
	}
}

// Generic wrapper to generate GET handler
func getHandlerWrapper(q query.Query, storeName string, cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		heightStr, ok := r.URL.Query()["height"]
		if ok && len(heightStr) > 0 {
			height, err := strconv.ParseInt(heightStr[0], 10, 64)
			if err != nil {
				rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			cliCtx = cliCtx.WithHeight(height)
		} else {
			cliCtx = cliCtx.WithHeight(0)
		}
		param := mux.Vars(r)[restURLParam]
		text, err := r.URL.MarshalBinary()
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		res, _, err := cliCtx.QueryWithData(q.Path(storeName, param, mux.Vars(r)[restURLParam2]), text)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")

		// get the block height and time of the latest block on the node
		latest, err := cliCtx.Client.Block(r.Context(), nil)
		if err != nil {
			log.Debug().Err(err).Msg("fail to get latest block")
		} else {
			w.Header().Set("X-Thorchain-Height", fmt.Sprintf("%d", latest.Block.Height))
			w.Header().Set("X-Thorchain-Time", latest.Block.Time.Format(time.RFC3339))
		}

		_, _ = w.Write(res)
	}
}
