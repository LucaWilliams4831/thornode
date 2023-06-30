//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package thorchain

var (
	ethUSDTAsset = `ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7`

	// https://etherscan.io/address/0x3624525075b88B24ecc29CE226b0CEc1fFcB6976
	ethOldRouter = `0x3624525075b88B24ecc29CE226b0CEc1fFcB6976`
	// https://etherscan.io/address/0xD37BbE5744D730a1d98d8DC97c42F0Ca46aD7146
	ethNewRouter = `0xD37BbE5744D730a1d98d8DC97c42F0Ca46aD7146`

	avaxOldRouter = ``
	// https://snowtrace.io/address/0x8F66c4AE756BEbC49Ec8B81966DD8bba9f127549#code
	avaxNewRouter = `0x8F66c4AE756BEbC49Ec8B81966DD8bba9f127549`

	// BSC Router
	bscOldRouter = ``
	bscNewRouter = `0xb30ec53f98ff5947ede720d32ac2da7e52a5f56b`
)
