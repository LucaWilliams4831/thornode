# ADR 008: Implement a Dynamic Outbound Fee Multiplier (DOFM)

## Changelog

- March 30, 2023: Implementation of DOFM merged to go out in v108
- April 4, 2023: Decision to open discussion/ADR before bringing functionality live
- April 6, 2023: ADR Opened to get formal decision from NOs/community

## Status

Proposed

## Context

Currently, users are charged a 3x constant on the gas_rate for outbounds on external L1 blockchains, while the vaults only spend 1.5x the gas_rate when signing/broadcasting the outbound tx. The difference between what the users are charged and what the vaults spend (i.e. the "spread") is pocketed by the reserve as an income stream. Since the start of Multi-chain Chaosnet, the reserve has made about 1 million $RUNE in total from this difference, or about 0.6% of the current Reserve balance. The below dashboards have more information:

https://flipsidecrypto.xyz/Multipartite/reserve-cumulative-income-health-rOUjF2

This constant 3x multiplier of the outbound fee effects all end users of THORChain: it makes swapping more expensive, and eats into the profits of both LPers and Savers. For swappers, especially smaller swappers and those transacting on historically expensive chains like ETH and BTC, this 3x multiplier becomes a major deterrant to using the network: they simply could use a centralized service and get better price execution. For Savers, espcially savers in lower yield vaults like BTC, this 3x multiplier eats into profits and increases the time-to-break-even.

At the same time, this "spread" is a constant source of income for the reserve, amounting to 0.6% in aggregate of the total reserve balance at the time of writing. Modifying the outbound fee system to make it cheaper for the end user would mean effectively removing this income source for the reserve.

## Alternative Approaches

To make swaps and withdraws cheaper for the end user there are not a lot of other options - the outbound fee is the clear place to reduce fees. Other fees include the liquidity fee, which is determined by the "slippage" formula that is a cornerstone of THORChain's CLP/AMM design; modifying this formula would be a major change in THORChain's economic design and is not adivisable.

In terms of reserve income, another option is to create a new source of income for the reserve to replace the lost income from the outbound fee "spread". Two possible options are:

- Have the reserve take a small % of liquidity fees from swaps. This wouldn't add any cost to the end user, but would take yield from LPs & Nodes.
- Increase the base network fee of 0.02 $RUNE, or make this fee dynamic. This would increase costs to swappers.

## Decision

> Pending

## Detailed Design

Create a dynamic "outbound fee multiplier" that moves between a `max_multiplier` and a `min_multiplier` based on the current outbound fee "surplus" of the network in relation to a "target" surplus. The "surplus" is the difference between the gas users are charged and the gas the network has spent. As the network's surplus grows in relation to the target surplus, the outbound multiplier will decrease from the `max_multiplier`, to the `min_multiplier`. The outbound fee multiplier will then be a "sliding scale" instead of being a constant 3x.

### New Mimirs

`TargetOutboundFeeSurplusRune`: target amount of $RUNE to have as a surplus. Suggested initial value 100_000_00000000 (100,000 $RUNE)
`MaxOutboundFeeMultiplierBasisPoints`: max multiplier in basis points. Suggested initial value: 30_000
`MinOutboundFeeMultiplierBasisPoints`: min multiplier in basis points. Suggested initial value: 15_000

### New Network Properties

`outbound_gas_spent_rune`: Sum of $RUNE spent by the network on outbounds
`outbound_gas_withheld_rune`: Sum of $RUNE withheld from the user for outbounds

current surplus = `outbound_gas_withheld_rune - outbound_gas_spent_rune`

The current surplus is compared with the target surplus, and the outbound fee multiplier is adjusted accordingly on a sliding scale: If surplus => target, use the min multiplier. If surplus = 0 use the max multiplier. If surplus > 0 && surplus < target, return the basis points value in between min and max multiplier that represents the "progress" to the target surplus.

## Consequences

If the proposed design is implemented and activated, this would slowly decrease the outbound fees for end users, which would have two major consequences. First, swapping and withdrawing from THORChain will become cheaper (up to 2x cheaper when considering outbound fee costs). Secondly, overtime the income that the reserve makes on upcharging users on outbound fees will trend to 0. As mentioned above, the total income that the reserve has made on this system since the start of MCCN amounts to 0.6% of the total reserve balance. **Note: the proposed change ensures that the Reserve will never lose money on outbound fee/churn costs.**

## References

- Dynamic Outbound Fee Implementation: https://gitlab.com/thorchain/thornode/-/merge_requests/2835
- Reserve income dashboard: https://flipsidecrypto.xyz/Multipartite/reserve-cumulative-income-health-rOUjF2
