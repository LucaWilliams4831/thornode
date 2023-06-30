package utxo

import (
	"fmt"

	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/common"
)

func GetAsgardAddress(chain common.Chain, maxAsgardAddresses int, bridge thorclient.ThorchainBridge) ([]common.Address, error) {
	vaults, err := bridge.GetAsgardPubKeys()
	if err != nil {
		return nil, fmt.Errorf("fail to get asgards : %w", err)
	}

	newAddresses := make([]common.Address, 0)
	for _, v := range vaults {
		addr, err := v.PubKey.GetAddress(chain)
		if err != nil {
			continue
		}
		newAddresses = append(newAddresses, addr)
		if len(newAddresses) > maxAsgardAddresses {
			break
		}
	}
	return newAddresses, nil
}
