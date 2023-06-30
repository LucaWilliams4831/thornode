package rest

import (
	"fmt"
	"io"
	"net/http"

	"github.com/cosmos/cosmos-sdk/client"
	clientrest "github.com/cosmos/cosmos-sdk/client/rest"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/rest"
	"github.com/cosmos/cosmos-sdk/x/auth/legacy/legacytx"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
)

// BroadcastReq defines a tx broadcasting request.
type BroadcastReq struct {
	Tx   StdTx  `json:"tx" yaml:"tx"`
	Mode string `json:"mode" yaml:"mode"`
}

var _ codectypes.UnpackInterfacesMessage = BroadcastReq{}

// UnpackInterfaces implements the UnpackInterfacesMessage interface.
func (m BroadcastReq) UnpackInterfaces(unpacker codectypes.AnyUnpacker) error {
	return m.Tx.UnpackInterfaces(unpacker)
}

// BroadcastTxRequest implements a tx broadcasting handler that is responsible
// for broadcasting a valid and signed tx to a full node. The tx can be
// broadcasted via a sync|async|block mechanism.
func BroadcastTxRequest(clientCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req BroadcastReq

		body, err := io.ReadAll(r.Body)
		if rest.CheckBadRequestError(w, err) {
			return
		}
		// NOTE: amino is used intentionally here, don't migrate it!
		err = clientCtx.LegacyAmino.UnmarshalJSON(body, &req)
		if err != nil {
			err := fmt.Errorf("this transaction cannot be broadcasted via legacy REST endpoints, because it does not support"+
				" Amino serialization. Please either use CLI, gRPC, gRPC-gateway, or directly query the Tendermint RPC"+
				" endpoint to broadcast this transaction. The new REST endpoint (via gRPC-gateway) is POST /cosmos/tx/v1beta1/txs."+
				" Please also see the REST endpoints migration guide at %s for more info", clientrest.DeprecationURL)
			if rest.CheckBadRequestError(w, err) {
				return
			}
		}

		txBytes, err := ConvertAndEncodeStdTx(clientCtx.TxConfig, req.Tx)
		if rest.CheckInternalServerError(w, err) {
			return
		}

		clientCtx = clientCtx.WithBroadcastMode(req.Mode)

		res, err := clientCtx.BroadcastTx(txBytes)
		if rest.CheckInternalServerError(w, err) {
			return
		}
		rest.PostProcessResponseBare(w, clientCtx, res)
	}
}

// ConvertAndEncodeStdTx encodes the stdTx as a transaction in the format specified by txConfig
func ConvertAndEncodeStdTx(txConfig client.TxConfig, stdTx StdTx) ([]byte, error) {
	builder := txConfig.NewTxBuilder()

	var theTx sdk.Tx

	// check if we need a StdTx anyway, in that case don't copy
	if _, ok := builder.GetTx().(legacytx.StdTx); ok {
		theTx = stdTx
	} else {
		err := CopyTx(stdTx, builder, false)
		if err != nil {
			return nil, err
		}

		theTx = builder.GetTx()
	}

	return txConfig.TxEncoder()(theTx)
}

// CopyTx copies a Tx to a new TxBuilder, allowing conversion between
// different transaction formats. If ignoreSignatureError is true, copying will continue
// tx even if the signature cannot be set in the target builder resulting in an unsigned tx.
func CopyTx(tx signing.Tx, builder client.TxBuilder, ignoreSignatureError bool) error {
	err := builder.SetMsgs(tx.GetMsgs()...)
	if err != nil {
		return err
	}

	sigs, err := tx.GetSignaturesV2()
	if err != nil {
		return err
	}

	err = builder.SetSignatures(sigs...)
	if err != nil {
		if ignoreSignatureError {
			// we call SetSignatures() agan with no args to clear any signatures in case the
			// previous call to SetSignatures() had any partial side-effects
			_ = builder.SetSignatures()
		} else {
			return err
		}
	}

	builder.SetMemo(tx.GetMemo())
	builder.SetFeeAmount(tx.GetFee())
	builder.SetGasLimit(tx.GetGas())
	builder.SetTimeoutHeight(tx.GetTimeoutHeight())

	return nil
}
