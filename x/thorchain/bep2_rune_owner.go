//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package thorchain

// BEP2RuneOwnerAddress this is the BEP2 RUNE owner address , during migration all upgraded BEP2 RUNE will be send to this owner address
// THORChain admin will burn those upgraded RUNE appropriately , It need to send to owner address is because only owner can burn it
const BEP2RuneOwnerAddress = "bnb1e4q8whcufp6d72w8nwmpuhxd96r4n0fstegyuy"
