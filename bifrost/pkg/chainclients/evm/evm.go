package evm

import _ "embed"

//go:embed abi/router.json
var routerContractABI string

//go:embed abi/erc20.json
var erc20ContractABI string

const (
	MaxContractGas = 80000

	defaultDecimals = 18 // evm chains consolidate all decimals to 18 (wei)
	tenGwei         = 10000000000
)
