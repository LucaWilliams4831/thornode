package thorchain

import (
	"encoding/json"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type PreRegisterTHORName struct {
	Name    string
	Address string
}

func getPreRegisterTHORNames(ctx cosmos.Context, blockheight int64) ([]THORName, error) {
	var register []PreRegisterTHORName
	if err := json.Unmarshal(preregisterTHORNames, &register); err != nil {
		return nil, fmt.Errorf("fail to load preregistation thorname list,err: %w", err)
	}

	names := make([]THORName, 0)
	for _, reg := range register {
		addr, err := common.NewAddress(reg.Address)
		if err != nil {
			ctx.Logger().Error("fail to parse address", "address", reg.Address, "error", err)
			continue
		}
		name := NewTHORName(reg.Name, blockheight, []THORNameAlias{{Chain: common.THORChain, Address: addr}})
		acc, err := cosmos.AccAddressFromBech32(reg.Address)
		if err != nil {
			ctx.Logger().Error("fail to parse acc address", "address", reg.Address, "error", err)
			continue
		}
		name.Owner = acc
		names = append(names, name)
	}
	return names, nil
}
