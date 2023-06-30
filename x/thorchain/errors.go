package thorchain

import (
	"fmt"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/hashicorp/go-multierror"
)

// THORChain error code start at 99
const (
	// CodeBadVersion error code for bad version
	CodeInternalError     uint32 = 99
	CodeTxFail            uint32 = 100
	CodeBadVersion        uint32 = 101
	CodeInvalidMessage    uint32 = 102
	CodeInvalidVault      uint32 = 104
	CodeInvalidMemo       uint32 = 105
	CodeInvalidPoolStatus uint32 = 107

	CodeSwapFail                 uint32 = 108
	CodeSwapFailNotEnoughFee     uint32 = 110
	CodeSwapFailInvalidAmount    uint32 = 113
	CodeSwapFailInvalidBalance   uint32 = 114
	CodeSwapFailNotEnoughBalance uint32 = 115

	CodeAddLiquidityFailValidation   uint32 = 120
	CodeFailGetLiquidityProvider     uint32 = 122
	CodeAddLiquidityMismatchAddr     uint32 = 123
	CodeLiquidityInvalidPoolAsset    uint32 = 124
	CodeAddLiquidityRUNEOverLimit    uint32 = 125
	CodeAddLiquidityRUNEMoreThanBond uint32 = 126

	CodeWithdrawFailValidation uint32 = 130
	CodeFailAddOutboundTx      uint32 = 131
	CodeFailSaveEvent          uint32 = 132
	CodeNoLiquidityUnitLeft    uint32 = 135
	CodeWithdrawWithin24Hours  uint32 = 136
	CodeWithdrawFail           uint32 = 137
	CodeEmptyChain             uint32 = 138
	CodeWithdrawLockup         uint32 = 139
)

var (
	errNotAuthorized                = fmt.Errorf("not authorized")
	errInvalidVersion               = fmt.Errorf("bad version")
	errBadVersion                   = se.Register(DefaultCodespace, CodeBadVersion, errInvalidVersion.Error())
	errInvalidMessage               = se.Register(DefaultCodespace, CodeInvalidMessage, "invalid message")
	errInvalidMemo                  = se.Register(DefaultCodespace, CodeInvalidMemo, "invalid memo")
	errFailSaveEvent                = se.Register(DefaultCodespace, CodeFailSaveEvent, "fail to save add events")
	errAddLiquidityFailValidation   = se.Register(DefaultCodespace, CodeAddLiquidityFailValidation, "fail to validate add liquidity")
	errAddLiquidityRUNEOverLimit    = se.Register(DefaultCodespace, CodeAddLiquidityRUNEOverLimit, "add liquidity rune is over limit")
	errAddLiquidityRUNEMoreThanBond = se.Register(DefaultCodespace, CodeAddLiquidityRUNEMoreThanBond, "add liquidity rune is more than bond")
	errInvalidPoolStatus            = se.Register(DefaultCodespace, CodeInvalidPoolStatus, "invalid pool status")
	errFailAddOutboundTx            = se.Register(DefaultCodespace, CodeFailAddOutboundTx, "prepare outbound tx not successful")
	errWithdrawFailValidation       = se.Register(DefaultCodespace, CodeWithdrawFailValidation, "fail to validate withdraw")
	errFailGetLiquidityProvider     = se.Register(DefaultCodespace, CodeFailGetLiquidityProvider, "fail to get liquidity provider")
	errAddLiquidityMismatchAddr     = se.Register(DefaultCodespace, CodeAddLiquidityMismatchAddr, "memo paired address must be non-empty and together with origin address match the liquidity provider record")
	errSwapFailNotEnoughFee         = se.Register(DefaultCodespace, CodeSwapFailNotEnoughFee, "fail swap, not enough fee")
	errSwapFail                     = se.Register(DefaultCodespace, CodeSwapFail, "fail swap")
	errSwapFailInvalidAmount        = se.Register(DefaultCodespace, CodeSwapFailInvalidAmount, "fail swap, invalid amount")
	errSwapFailInvalidBalance       = se.Register(DefaultCodespace, CodeSwapFailInvalidBalance, "fail swap, invalid balance")
	errSwapFailNotEnoughBalance     = se.Register(DefaultCodespace, CodeSwapFailNotEnoughBalance, "fail swap, not enough balance")
	errNoLiquidityUnitLeft          = se.Register(DefaultCodespace, CodeNoLiquidityUnitLeft, "nothing to withdraw")
	errWithdrawWithin24Hours        = se.Register(DefaultCodespace, CodeWithdrawWithin24Hours, "you cannot withdraw for 24 hours after providing liquidity for this blockchain")
	errWithdrawLockup               = se.Register(DefaultCodespace, CodeWithdrawLockup, "last add within lockup blocks")
	errWithdrawFail                 = se.Register(DefaultCodespace, CodeWithdrawFail, "fail to withdraw")
	errInternal                     = se.Register(DefaultCodespace, CodeInternalError, "internal error")
)

// ErrInternal return an error  of errInternal with additional message
func ErrInternal(err error, msg string) error {
	return se.Wrap(multierror.Append(errInternal, err), msg)
}
