//go:build !mocknet
// +build !mocknet

package thorchain

import (
	re "regexp"
)

var adminMimirDenyList = []*re.Regexp{
	re.MustCompile("(?i)EmissionCurve"),
	re.MustCompile("(?i)IncentiveCurve"),
	re.MustCompile("(?i)BlocksPerYear"),
	re.MustCompile("(?i)MinimumBondInRune"),
	re.MustCompile("(?i)NumberOfNewNodesPerChurn"),
	re.MustCompile("(?i)AsgardSize"),
	re.MustCompile("(?i)FullImpLossProtectionBlocks"),
	re.MustCompile("(?i)DesiredValidatorSet"),
	re.MustCompile("(?i)MinimumNodesForBFT"),
	re.MustCompile("(?i)ObserveSlashPoints"),
	re.MustCompile("(?i)FailKeysignSlashPoints"),
	re.MustCompile("(?i)FailKeygenSlashPoints"),
	re.MustCompile("(?i)StagedPoolCost"),
	re.MustCompile("(?i)Ragnarok.*"),
	re.MustCompile("(?i)MinTxOutVolumeThreshold"),
	re.MustCompile("(?i)MaxTxOutOffset"),
	re.MustCompile("(?i)TxOutDelayRate"),
	re.MustCompile("(?i)TxOutDelayMax"),
	re.MustCompile("(?i)MaxRuneSupply"),
}
