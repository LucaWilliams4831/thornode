//go:build regtest
// +build regtest

package thorchain

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	abci "github.com/tendermint/tendermint/abci/types"

	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/config"
	q "gitlab.com/thorchain/thornode/x/thorchain/query"
)

func init() {
	initManager = func(mgr *Mgrs, ctx cosmos.Context) {
		_ = mgr.BeginBlock(ctx)
	}

	optionalQuery = func(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
		switch path[0] {
		case q.QueryExport.Key:
			return queryExport(ctx, path[1:], req, mgr)
		case q.QueryBlockEvents.Key:
			return queryBlockEvents(ctx, path[1:], req, mgr)
		default:
			return nil, cosmos.ErrUnknownRequest(
				fmt.Sprintf("unknown thorchain query endpoint: %s", path[0]),
			)
		}
	}
}

func queryExport(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	return jsonify(ctx, ExportGenesis(ctx, mgr.Keeper()))
}

func queryBlockEvents(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	// get tendermint port from config
	portSplit := strings.Split(config.GetThornode().Tendermint.RPC.ListenAddress, ":")
	port := portSplit[len(portSplit)-1]

	// get block results
	res, err := http.Get(fmt.Sprintf("http://localhost:%s/block_results?height=%d", port, ctx.BlockHeight()))
	if err != nil {
		return nil, err
	}

	// response type
	type tendermintResponse struct {
		Result struct {
			BeginBlockEvents []abci.Event `json:"begin_block_events"`
			EndBlockEvents   []abci.Event `json:"end_block_events"`
			TxsResults       []struct {
				Events []abci.Event `json:"events"`
			} `json:"txs_results"`
		} `json:"result"`
	}

	// unmarshal block results
	blockResults := tendermintResponse{}
	if err := json.NewDecoder(res.Body).Decode(&blockResults); err != nil {
		return nil, err
	}

	// response type
	type response struct {
		Tx    []map[string]string `json:"tx"`
		Begin []map[string]string `json:"begin"`
		End   []map[string]string `json:"end"`
	}

	// convert events to maps on response
	r := response{}
	for _, event := range blockResults.Result.BeginBlockEvents {
		m := make(map[string]string)
		m["type"] = event.Type
		for _, attr := range event.Attributes {
			m[string(attr.Key)] = string(attr.Value)
		}
		r.Begin = append(r.Begin, m)
	}
	for _, event := range blockResults.Result.EndBlockEvents {
		m := make(map[string]string)
		m["type"] = event.Type
		for _, attr := range event.Attributes {
			m[string(attr.Key)] = string(attr.Value)
		}
		r.End = append(r.End, m)
	}
	for _, tx := range blockResults.Result.TxsResults {
		for _, event := range tx.Events {
			m := make(map[string]string)
			m["type"] = event.Type
			for _, attr := range event.Attributes {
				m[string(attr.Key)] = string(attr.Value)
			}
			r.Tx = append(r.Tx, m)
		}
	}

	return jsonify(ctx, r)
}
