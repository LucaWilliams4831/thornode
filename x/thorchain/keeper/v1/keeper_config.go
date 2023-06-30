package keeperv1

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// GetConstants returns the constant values
func (k KVStore) GetConstants() constants.ConstantValues {
	return k.constAccessor
}

// GetConfigInt64 returns the mimir value for the key if set, otherwise the constant value
func (k KVStore) GetConfigInt64(ctx cosmos.Context, key constants.ConstantName) int64 {
	val, err := k.GetMimir(ctx, key.String())
	if val < 0 || err != nil {
		val = k.GetConstants().GetInt64Value(key)
		if err != nil {
			ctx.Logger().Error("fail to get mimir", "key", key.String(), "error", err)
		}
	}
	return val
}
