package evm

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// initSmartContracts load the erc20 contract and vault contract
func GetContractABI(routerContractABI, erc20ContractABI string) (*abi.ABI, *abi.ABI, error) {
	vault, err := abi.JSON(strings.NewReader(routerContractABI))
	if err != nil {
		return nil, nil, fmt.Errorf("fail to unmarshal vault abi: %w", err)
	}

	erc20, err := abi.JSON(strings.NewReader(erc20ContractABI))
	if err != nil {
		return nil, nil, fmt.Errorf("fail to unmarshal erc20 abi: %w", err)
	}
	return &vault, &erc20, nil
}
