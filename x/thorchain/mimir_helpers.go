package thorchain

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func isAdmin(acc cosmos.AccAddress) bool {
	for _, admin := range ADMINS {
		addr, err := cosmos.AccAddressFromBech32(admin)
		if acc.Equals(addr) && err == nil {
			return true
		}
	}
	return false
}

func isAdminAllowedForMimir(key string) bool {
	for _, denyPattern := range adminMimirDenyList {
		if denyPattern.MatchString(key) {
			return false
		}
	}
	return true
}
