//go:build !mocknet
// +build !mocknet

package tss

// MinimumMnemonicEntropy was determined by checking the minimum entropy of 1e8
// uniquely generated BIP39 mnemonics. In all likelihood, a validator mnemonic
// should not have a lower entropy value.
const MinimumMnemonicEntropy = 3.67
