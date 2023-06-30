package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/x/gov/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// NewSendHandler create a new instance of SendHandler
func NewSendHandler(mgr Manager) BaseHandler[*MsgSend] {
	return BaseHandler[*MsgSend]{
		mgr:    mgr,
		logger: MsgSendLogger,
		validators: NewValidators[*MsgSend]().
			Register("1.87.0", MsgSendValidateV87).
			Register("0.1.0", MsgSendValidateV1),
		handlers: NewHandlers[*MsgSend]().
			Register("1.112.0", MsgSendHandleV112).
			Register("1.108.0", MsgSendHandleV108).
			Register("0.1.0", MsgSendHandleV1),
	}
}

func MsgSendValidateV87(ctx cosmos.Context, mgr Manager, msg *MsgSend) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// disallow sends to modules, they should only be interacted with via deposit messages
	if msg.ToAddress.Equals(mgr.Keeper().GetModuleAccAddress(AsgardName)) ||
		msg.ToAddress.Equals(mgr.Keeper().GetModuleAccAddress(BondName)) ||
		msg.ToAddress.Equals(mgr.Keeper().GetModuleAccAddress(ReserveName)) ||
		msg.ToAddress.Equals(mgr.Keeper().GetModuleAccAddress(ModuleName)) {
		return errors.New("cannot use MsgSend for Module transactions, use MsgDeposit instead")
	}

	return nil
}

func MsgSendLogger(ctx cosmos.Context, msg *MsgSend) {
	ctx.Logger().Info("receive MsgSend", "from", msg.FromAddress, "to", msg.ToAddress, "coins", msg.Amount)
}

func MsgSendHandleV112(ctx cosmos.Context, mgr Manager, msg *MsgSend) (*cosmos.Result, error) {
	if mgr.Keeper().IsChainHalted(ctx, common.THORChain) {
		return nil, fmt.Errorf("unable to use MsgSend while THORChain is halted")
	}

	nativeTxFee := mgr.Keeper().GetNativeTxFee(ctx)
	gas := common.NewCoin(common.RuneNative, nativeTxFee)
	gasFee, err := gas.Native()
	if err != nil {
		return nil, ErrInternal(err, "fail to get gas fee")
	}

	totalCoins := cosmos.NewCoins(gasFee).Add(msg.Amount...)
	if !mgr.Keeper().HasCoins(ctx, msg.FromAddress, totalCoins) {
		return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
	}

	// send gas to reserve
	sdkErr := mgr.Keeper().SendFromAccountToModule(ctx, msg.FromAddress, ReserveName, common.NewCoins(gas))
	if sdkErr != nil {
		return nil, fmt.Errorf("unable to send gas to reserve: %w", sdkErr)
	}

	sdkErr = mgr.Keeper().SendCoins(ctx, msg.FromAddress, msg.ToAddress, msg.Amount)
	if sdkErr != nil {
		return nil, sdkErr
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent(
			cosmos.EventTypeMessage,
			cosmos.NewAttribute(cosmos.AttributeKeyModule, types.AttributeValueCategory),
		),
	)

	return &cosmos.Result{}, nil
}

// SendAnteHandler called by the ante handler to gate mempool entry
// and also during deliver. Store changes will persist if this function
// succeeds, regardless of the success of the transaction.
func SendAnteHandler(ctx cosmos.Context, v semver.Version, k keeper.Keeper, msg MsgSend) error {
	return nil
}
