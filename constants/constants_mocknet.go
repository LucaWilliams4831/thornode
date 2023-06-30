//go:build mocknet
// +build mocknet

// For internal testing and mockneting
package constants

import (
	"os"
	"strconv"
)

func init() {
	int64Overrides = map[ConstantName]int64{
		// ArtificialRagnarokBlockHeight: 200,
		DesiredValidatorSet:                 12,
		ChurnInterval:                       60, // 5 min
		ChurnRetryInterval:                  30,
		MinimumBondInRune:                   100_000_000, // 1 rune
		ValidatorMaxRewardRatio:             3,
		FundMigrationInterval:               40,
		LiquidityLockUpBlocks:               0,
		MaxRuneSupply:                       500_000_000_00000000,
		JailTimeKeygen:                      10,
		JailTimeKeysign:                     10,
		AsgardSize:                          6,
		MinimumNodesForYggdrasil:            4,
		VirtualMultSynthsBasisPoints:        20_000,
		MinTxOutVolumeThreshold:             2000000_00000000,
		TxOutDelayRate:                      2000000_00000000,
		PoolDepthForYggFundingMin:           500_000_00000000,
		MaxSynthPerPoolDepth:                3_500,
		MaxSynthsForSaversYield:             5000,
		PauseLoans:                          0,
		AllowWideBlame:                      1,
		TargetOutboundFeeSurplusRune:        10_000_00000000,
		MaxOutboundFeeMultiplierBasisPoints: 20_000,
		MinOutboundFeeMultiplierBasisPoints: 15_000,
	}
	boolOverrides = map[ConstantName]bool{
		StrictBondLiquidityRatio: false,
	}
	stringOverrides = map[ConstantName]string{
		DefaultPoolStatus: "Available",
	}

	if os.Getenv("CHURN_MIGRATION_ROUNDS") != "" {
		int64Overrides[ChurnMigrateRounds], _ = strconv.ParseInt(os.Getenv("CHURN_MIGRATION_ROUNDS"), 10, 64)
	}

	if os.Getenv("FUND_MIGRATION_INTERVAL") != "" {
		int64Overrides[FundMigrationInterval], _ = strconv.ParseInt(os.Getenv("FUND_MIGRATION_INTERVAL"), 10, 64)
	}
}
