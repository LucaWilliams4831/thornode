//go:build testnet
// +build testnet

// For Public TestNet
package constants

func init() {
	int64Overrides = map[ConstantName]int64{
		PoolCycle:             1000,
		MinRunePoolDepth:      100_00000000,
		AsgardSize:            15,
		DesiredValidatorSet:   30,
		ChurnInterval:         240,
		MinimumBondInRune:     10000_00000000, // 10K rune
		LiquidityLockUpBlocks: 0,
		StagedPoolCost:        10_00000000,
	}
	boolOverrides = map[ConstantName]bool{
		StrictBondLiquidityRatio: false,
	}
}
