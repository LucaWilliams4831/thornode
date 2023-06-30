package constants

// NewConstantValue get new instance of ConstantValue
func NewConstantValue() *ConstantVals {
	return &ConstantVals{
		int64values: map[ConstantName]int64{
			EmissionCurve:                       6,
			BlocksPerYear:                       5256000,
			MaxRuneSupply:                       -1,                 // max supply of rune. Default set to -1 to avoid consensus failure
			IncentiveCurve:                      100,                // configures incentive pendulum
			OutboundTransactionFee:              2_000000,           // TODO: remove me on hard fork
			OutboundTransactionFeeUSD:           2_000000,           // $0.02 fee on all swaps and withdrawals
			NativeTransactionFee:                2_000000,           // TODO: remove me on hard fork
			NativeTransactionFeeUSD:             2_000000,           // $0.02 fee on all on chain txs
			PoolCycle:                           43200,              // Make a pool available every 3 days
			StagedPoolCost:                      10_00000000,        // amount of rune to take from a staged pool on every pool cycle
			KillSwitchStart:                     0,                  // block height to start the kill switch of BEP2/ERC20 old RUNE
			KillSwitchDuration:                  5256000,            // number of blocks until swith no longer works
			MinRunePoolDepth:                    10000_00000000,     // minimum rune pool depth to be an available pool
			MaxAvailablePools:                   100,                // maximum number of available pools
			MinimumNodesForYggdrasil:            6,                  // No yggdrasil pools if THORNode have less than 6 active nodes
			MinimumNodesForBFT:                  4,                  // Minimum node count to keep network running. Below this, Ragnarök is performed.
			DesiredValidatorSet:                 100,                // desire validator set
			AsgardSize:                          40,                 // desired node operators in an asgard vault
			DerivedDepthBasisPts:                0,                  // Basis points to increase/decrease derived pool depth (10k == 1x)
			DerivedMinDepth:                     100,                // in basis points, min derived pool depth
			MaxAnchorSlip:                       1500,               // basis points of rune depth to trigger pausing a derived virtual pool
			MaxAnchorBlocks:                     300,                // max blocks to accumulate swap slips in anchor pools
			FundMigrationInterval:               360,                // number of blocks THORNode will attempt to move funds from a retiring vault to an active one
			ChurnInterval:                       43200,              // How many blocks THORNode try to rotate validators
			ChurnRetryInterval:                  720,                // How many blocks until we retry a churn (only if we haven't had a successful churn in ChurnInterval blocks
			BadValidatorRedline:                 3,                  // redline multiplier to find a multitude of bad actors
			LackOfObservationPenalty:            2,                  // add two slash point for each block where a node does not observe
			SigningTransactionPeriod:            300,                // how many blocks before a request to sign a tx by yggdrasil pool, is counted as delinquent.
			DoubleSignMaxAge:                    24,                 // number of blocks to limit double signing a block
			PauseBond:                           0,                  // pauses the ability to bond
			PauseUnbond:                         0,                  // pauses the ability to unbond
			MinimumBondInRune:                   1_000_000_00000000, // 1 million rune
			MaxBondProviders:                    6,                  // maximum number of bond providers
			MaxOutboundAttempts:                 0,                  // maximum retries to reschedule a transaction
			SlashPenalty:                        15000,              // penalty paid (in basis points) for theft of assets
			PauseOnSlashThreshold:               100_00000000,       // number of rune to pause the network on the event a vault is slash for theft
			FailKeygenSlashPoints:               720,                // slash for 720 blocks , which equals 1 hour
			FailKeysignSlashPoints:              2,                  // slash for 2 blocks
			LiquidityLockUpBlocks:               0,                  // the number of blocks LP can withdraw after their liquidity
			ObserveSlashPoints:                  1,                  // the number of slashpoints for making an observation (redeems later if observation reaches consensus
			ObservationDelayFlexibility:         10,                 // number of blocks of flexibility for a validator to get their slash points taken off for making an observation
			YggFundLimit:                        50,                 // percentage of the amount of funds a ygg vault is allowed to have.
			YggFundRetry:                        1000,               // number of blocks before retrying to fund a yggdrasil vault
			JailTimeKeygen:                      720 * 6,            // blocks a node account is jailed for failing to keygen. DO NOT drop below tss timeout
			JailTimeKeysign:                     60,                 // blocks a node account is jailed for failing to keysign. DO NOT drop below tss timeout
			NodePauseChainBlocks:                720,                // number of blocks that a node can pause/resume a global chain halt
			NodeOperatorFee:                     500,                // Node operator fee
			EnableDerivedAssets:                 0,                  // enable/disable swapping of derived assets
			MinSwapsPerBlock:                    10,                 // process all swaps if queue is less than this number
			MaxSwapsPerBlock:                    100,                // max swaps to process per block
			EnableOrderBooks:                    0,                  // enable order books instead of swap queue
			VirtualMultSynths:                   2,                  // pool depth multiplier for synthetic swaps
			VirtualMultSynthsBasisPoints:        10_000,             // pool depth multiplier for synthetic swaps (in basis points)
			MaxSynthPerAssetDepth:               3300,               // TODO: remove me on hard fork
			MaxSynthPerPoolDepth:                1700,               // percentage (in basis points) of how many synths are allowed relative to pool depth of the related pool
			MaxSynthsForSaversYield:             0,                  // percentage (in basis points) synth per pool where synth yield reaches 0%
			MinSlashPointsForBadValidator:       100,                // The minimum slash point
			FullImpLossProtectionBlocks:         1440000,            // number of blocks before a liquidity provider gets 100% impermanent loss protection
			MinCR:                               10_000,             // Minimum collateralization ratio (basis pts)
			MaxCR:                               60_000,             // Maximum collateralization ratio (basis pts)
			PauseLoans:                          1,                  // pause opening new loans and repaying loans
			LoanRepaymentMaturity:               0,                  // number of blocks before loan has reached maturity and can be repaid
			LendingLever:                        3333,               // This controls (in basis points) how much lending is allowed relative to rune supply
			MinTxOutVolumeThreshold:             1000_00000000,      // total txout volume (in rune) a block needs to have to slow outbound transactions
			TxOutDelayRate:                      25_00000000,        // outbound rune per block rate for scheduled transactions (excluding native assets)
			TxOutDelayMax:                       17280,              // max number of blocks a transaction can be delayed
			MaxTxOutOffset:                      720,                // max blocks to offset a txout into a future block
			TNSRegisterFee:                      10_00000000,        // TODO: remove me on hard fork
			TNSRegisterFeeUSD:                   10_00000000,        // registration fee for new THORName in USD
			TNSFeeOnSale:                        1000,               // fee for TNS sale in basis points
			TNSFeePerBlock:                      20,                 // TODO: remove me on hard fork
			TNSFeePerBlockUSD:                   20,                 // per block cost for TNS in USD
			PermittedSolvencyGap:                100,                // the setting is in basis points
			ValidatorMaxRewardRatio:             1,                  // the ratio to MinimumBondInRune at which validators stop receiving rewards proportional to their bond
			PoolDepthForYggFundingMin:           500_000_00000000,   // the minimum pool depth in RUNE required for ygg funding
			MaxNodeToChurnOutForLowVersion:      1,                  // the maximum number of nodes to churn out for low version per churn
			ChurnOutForLowVersionBlocks:         21600,              // the blocks after the MinJoinVersion changes before nodes can be churned out for low version
			POLMaxNetworkDeposit:                0,                  // Maximum amount of rune deposited into the pools
			POLMaxPoolMovement:                  100,                // Maximum amount of rune to enter/exit a pool per iteration - 1 equals one hundredth of a basis point of pool rune depth
			POLSynthUtilization:                 0,                  // TODO: remove me on hard fork
			POLTargetSynthPerPoolDepth:          0,                  // target synth per pool depth for POL (basis points)
			POLBuffer:                           0,                  // buffer around the POL synth utilization (basis points added to/subtracted from POLTargetSynthPerPoolDepth basis points)
			RagnarokProcessNumOfLPPerIteration:  200,                // the number of LP to be processed per iteration during ragnarok pool
			SynthYieldBasisPoints:               5000,               // amount of the yield the capital earns the synth holder receives if synth per pool is 0%
			SynthYieldCycle:                     0,                  // number of blocks when the network pays out rewards to yield bearing synths
			MinimumL1OutboundFeeUSD:             1000000,            // Minimum fee in USD to charge for LP swap, default to $0.01 , nodes need to vote it to a larger value
			MinimumPoolLiquidityFee:             0,                  // Minimum liquidity fee made by the pool,active pool fail to meet this within a PoolCycle will be demoted
			ILPCutoff:                           0,                  // the cutoff height for impermanent loss protection
			ChurnMigrateRounds:                  5,                  // Number of rounds to migrate vaults during churn
			AllowWideBlame:                      0,                  // allow for a wide blame, only set in mocknet for regression testing tss keysign failures
			MaxAffiliateFeeBasisPoints:          10_000,             // Max allowed affiliate fee basis points
			TargetOutboundFeeSurplusRune:        100_000_00000000,   // Target amount of RUNE for Outbound Fee Surplus: the sum of the diff between outbound cost to user and outbound cost to network
			MaxOutboundFeeMultiplierBasisPoints: 30_000,             // Maximum multiplier applied to base outbound fee charged to user, in basis points
			MinOutboundFeeMultiplierBasisPoints: 15_000,             // Minimum multiplier applied to base outbound fee charged to user, in basis points
			EnableUSDFees:                       0,                  // enable USD fees
		},
		boolValues: map[ConstantName]bool{
			StrictBondLiquidityRatio: true,
		},
		stringValues: map[ConstantName]string{
			DefaultPoolStatus: "Staged",
		},
	}
}
