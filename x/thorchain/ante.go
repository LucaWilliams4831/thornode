package thorchain

import (
	"github.com/blang/semver"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type AnteDecorator struct {
	keeper keeper.Keeper
}

func NewAnteDecorator(keeper keeper.Keeper) AnteDecorator {
	return AnteDecorator{
		keeper: keeper,
	}
}

func (ad AnteDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	version, hasVersion := ad.keeper.GetVersionWithCtx(ctx)
	if !hasVersion || version.LT(semver.MustParse("1.106.0")) {
		// TODO remove on hard fork
		// skip custom ante handling before v106
		return next(ctx, tx, simulate)
	}

	if err := ad.rejectMultipleDepositMsgs(ctx, tx.GetMsgs()); err != nil {
		return ctx, err
	}

	// run the message-specific ante for each msg, all must succeed
	for _, msg := range tx.GetMsgs() {
		if err := ad.anteHandleMessage(ctx, version, msg); err != nil {
			return ctx, err
		}
	}

	return next(ctx, tx, simulate)
}

// rejectMultipleDepositMsgs only one deposit msg allowed per tx
func (ad AnteDecorator) rejectMultipleDepositMsgs(ctx cosmos.Context, msgs []cosmos.Msg) error {
	hasDeposit := false
	for _, msg := range msgs {
		switch msg.(type) {
		case *types.MsgDeposit:
			if hasDeposit {
				return cosmos.ErrUnknownRequest("only one deposit msg per tx")
			}
			hasDeposit = true
		default:
			continue
		}
	}
	return nil
}

// anteHandleMessage calls the msg-specific ante handling for a given msg
func (ad AnteDecorator) anteHandleMessage(ctx sdk.Context, version semver.Version, msg cosmos.Msg) error {
	// ideally each handler would impl an ante func and we could instantiate
	// handlers and call ante, but handlers require mgr which is unavailable
	switch m := msg.(type) {

	// consensus handlers
	case *types.MsgBan:
		return BanAnteHandler(ctx, version, ad.keeper, *m)
	case *types.MsgErrataTx:
		return ErrataTxAnteHandler(ctx, version, ad.keeper, *m)
	case *types.MsgNetworkFee:
		return NetworkFeeAnteHandler(ctx, version, ad.keeper, *m)
	case *types.MsgObservedTxIn:
		return ObservedTxInAnteHandler(ctx, version, ad.keeper, *m)
	case *types.MsgObservedTxOut:
		return ObservedTxOutAnteHandler(ctx, version, ad.keeper, *m)
	case *types.MsgSolvency:
		return SolvencyAnteHandler(ctx, version, ad.keeper, *m)
	case *types.MsgTssPool:
		return TssAnteHandler(ctx, version, ad.keeper, *m)
	case *types.MsgTssKeysignFail:
		return TssKeysignFailAnteHandler(ctx, version, ad.keeper, *m)

	// cli handlers (non-consensus)
	case *types.MsgSetIPAddress:
		return IPAddressAnteHandler(ctx, version, ad.keeper, *m)
	case *types.MsgMimir:
		return MimirAnteHandler(ctx, version, ad.keeper, *m)
	case *types.MsgNodePauseChain:
		return NodePauseChainAnteHandler(ctx, version, ad.keeper, *m)
	case *types.MsgSetNodeKeys:
		return SetNodeKeysAnteHandler(ctx, version, ad.keeper, *m)
	case *types.MsgSetVersion:
		return VersionAnteHandler(ctx, version, ad.keeper, *m)

	// native handlers (non-consensus)
	case *types.MsgDeposit:
		return DepositAnteHandler(ctx, version, ad.keeper, *m)
	case *types.MsgSend:
		return SendAnteHandler(ctx, version, ad.keeper, *m)

	default:
		return cosmos.ErrUnknownRequest("invalid message type")
	}
}
