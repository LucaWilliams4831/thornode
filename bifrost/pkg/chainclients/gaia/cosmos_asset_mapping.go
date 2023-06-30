package gaia

import "strings"

type CosmosAssetMapping struct {
	CosmosDenom     string
	CosmosDecimals  int
	THORChainSymbol string
}

// CosmosAssetMappings maps a Cosmos denom to a THORChain symbol and provides the asset decimals
// CHANGEME: define assets that should be observed by THORChain here. This also acts a whitelist.
var CosmosAssetMappings = []CosmosAssetMapping{
	{
		CosmosDenom:     "uatom",
		CosmosDecimals:  6,
		THORChainSymbol: "ATOM",
	},
}

func GetAssetByCosmosDenom(denom string) (CosmosAssetMapping, bool) {
	for _, asset := range CosmosAssetMappings {
		if strings.EqualFold(asset.CosmosDenom, denom) {
			return asset, true
		}
	}
	return CosmosAssetMapping{}, false
}

func GetAssetByThorchainSymbol(symbol string) (CosmosAssetMapping, bool) {
	for _, asset := range CosmosAssetMappings {
		if strings.EqualFold(asset.THORChainSymbol, symbol) {
			return asset, true
		}
	}
	return CosmosAssetMapping{}, false
}
