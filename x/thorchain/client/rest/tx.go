package rest

import (
	"net/http"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/types/rest"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type deposit struct {
	BaseReq rest.BaseReq `json:"base_req"`
	Coins   common.Coins `json:"coins"`
	Memo    string       `json:"memo"`
}

func hasNilAmountInBaseRequestValid(req rest.BaseReq) bool {
	if len(req.Fees) > 0 {
		for _, c := range req.Fees {
			if c.Amount.IsNil() {
				return true
			}
		}
	}
	if len(req.GasPrices) > 0 {
		for _, c := range req.GasPrices {
			if c.Amount.IsNil() {
				return true
			}
		}
	}
	return false
}

func newDepositHandler(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req deposit

		if !rest.ReadRESTReq(w, r, cliCtx.LegacyAmino, &req) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}

		baseReq := req.BaseReq.Sanitize()
		if hasNilAmountInBaseRequestValid(baseReq) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "invalid coin amount")
			return
		}
		if !baseReq.ValidateBasic(w) {
			return
		}

		baseReq.Gas = "100000000"
		addr, err := cosmos.AccAddressFromBech32(req.BaseReq.From)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		msg := types.NewMsgDeposit(req.Coins, req.Memo, addr)
		err = msg.ValidateBasic()
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		tx.WriteGeneratedTxResponse(cliCtx, w, baseReq, msg)
	}
}

type newErrataTx struct {
	BaseReq rest.BaseReq `json:"base_req"`
	TxID    common.TxID  `json:"txid"`
	Chain   common.Chain `json:"chain"`
}

func newErrataTxHandler(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req newErrataTx

		if !rest.ReadRESTReq(w, r, cliCtx.LegacyAmino, &req) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}

		baseReq := req.BaseReq.Sanitize()
		if hasNilAmountInBaseRequestValid(baseReq) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "invalid coin amount")
			return
		}
		if !baseReq.ValidateBasic(w) {
			return
		}
		baseReq.Gas = "auto"
		addr, err := cosmos.AccAddressFromBech32(req.BaseReq.From)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		msg := types.NewMsgErrataTx(req.TxID, req.Chain, addr)
		err = msg.ValidateBasic()
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		tx.WriteGeneratedTxResponse(cliCtx, w, baseReq, msg)
	}
}

type newTssPool struct {
	BaseReq         rest.BaseReq     `json:"base_req"`
	InputPubKeys    []string         `json:"input_pubkeys"`
	KeygenType      types.KeygenType `json:"keygen_type"`
	Height          int64            `json:"height"`
	Blame           types.Blame      `json:"blame"`
	PoolPubKey      common.PubKey    `json:"pool_pub_key"`
	Chains          []string         `json:"chains"`
	KeygenTime      int64            `json:"keygen_time"`
	KeysharesBackup []byte           `json:"keyshares_backup"`
}

func newTssPoolHandler(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req newTssPool

		if !rest.ReadRESTReq(w, r, cliCtx.LegacyAmino, &req) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}

		baseReq := req.BaseReq.Sanitize()
		if hasNilAmountInBaseRequestValid(baseReq) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "invalid coin amount")
			return
		}
		if !baseReq.ValidateBasic(w) {
			return
		}
		baseReq.Gas = "auto"
		addr, err := cosmos.AccAddressFromBech32(req.BaseReq.From)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		msg, err := types.NewMsgTssPool(req.InputPubKeys, req.PoolPubKey, req.KeysharesBackup, req.KeygenType, req.Height, req.Blame, req.Chains, addr, req.KeygenTime)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		err = msg.ValidateBasic()
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		tx.WriteGeneratedTxResponse(cliCtx, w, baseReq, msg)
	}
}

type txHashReq struct {
	BaseReq rest.BaseReq      `json:"base_req"`
	Txs     types.ObservedTxs `json:"txs"`
}

func postTxsHandler(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req txHashReq

		if !rest.ReadRESTReq(w, r, cliCtx.LegacyAmino, &req) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}

		baseReq := req.BaseReq.Sanitize()
		if hasNilAmountInBaseRequestValid(baseReq) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "invalid coin amount")
			return
		}
		if !baseReq.ValidateBasic(w) {
			return
		}
		baseReq.Gas = "auto"
		addr, err := cosmos.AccAddressFromBech32(req.BaseReq.From)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		var inbound types.ObservedTxs
		var outbound types.ObservedTxs

		for _, tx := range req.Txs {
			chain := common.EmptyChain
			if len(tx.Tx.Coins) > 0 {
				chain = tx.Tx.Coins[0].Asset.Chain
			}

			obAddr, err := tx.ObservedPubKey.GetAddress(chain)
			if err != nil {
				rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			if tx.Tx.ToAddress.Equals(obAddr) { // nolint
				inbound = append(inbound, tx)
			} else if tx.Tx.FromAddress.Equals(obAddr) {
				outbound = append(outbound, tx)
			} else {
				rest.WriteErrorResponse(w, http.StatusBadRequest, "Unable to determine the direction of observation")
				return
			}
		}

		msgs := make([]cosmos.Msg, 0)

		if len(inbound) > 0 {
			msg := types.NewMsgObservedTxIn(inbound, addr)
			err = msg.ValidateBasic()
			if err != nil {
				rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			msgs = append(msgs, msg)
		}

		if len(outbound) > 0 {
			msg := types.NewMsgObservedTxOut(outbound, addr)
			err = msg.ValidateBasic()
			if err != nil {
				rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			msgs = append(msgs, msg)
		}

		tx.WriteGeneratedTxResponse(cliCtx, w, baseReq, msgs...)
	}
}
