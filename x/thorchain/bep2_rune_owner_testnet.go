//go:build testnet || mocknet
// +build testnet mocknet

package thorchain

// BEP2RuneOwnerAddress is the owner of BEP2 testnet RUNE address,  during migration all upgraded BEP2 RUNE will be send to this owner address
// THORChain admin will burn those upgraded RUNE appropriately , It need to send to owner address is because only owner can burn it
const BEP2RuneOwnerAddress = "tbnb1lg9yns9zxay9jsf4gvdksn2vdps20q9pqzea03"
