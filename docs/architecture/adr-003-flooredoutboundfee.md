# ADR 003: FLOORED OUTBOUND FEE

## Changelog

- {date}: {changelog}

## Status

Accepted

## Context

Recently [ADR-002 Remove Ygg Vaults](https://gitlab.com/thorchain/thornode/-/blob/develop/docs/architecture/adr-002-removeyggvaults.md) was passed, which deprecated ygg-vaults and made all outbound TX go thru TSS. This has increased the computational cost on the network to process L1 swaps. Additionally, concerns about chain bloat remain extant; each L1 swap requires a minimum of twice the number of nodes in witness tx, with some large tx up to 3 times. This is around 0.1kb in blockstate (each tx around 300bytes, (3 \* 120 \* 300 = 0.1kb)), which is kept around indefinitely until the chain hard forks and flattens history (once a year).

Thus an L1 Swap "cost" to the network is:

- up to 360 state tx
- 14 nodes in TSS for 10 seconds

The vast majority of swaps on THORChain are arbitrage across pools (this is to be expected with liquidity pools). Synth swaps are available to arbitrage agents and are a single transaction. In comparison a synth swap is at least 360 times cheaper in computation cost than an L1 swap, and should be the preferred swap for the majority of arbs.

Thus an L1 swap should have a minimum fee of 10-100 times more than a synth swap, since it has a cost to the network two orders of magnitude more than a synth swap. A synth swap costs 0.02 RUNE (the cost to perform a tx on THORChain), so an L1 swap should be 1-2 RUNE minimum.

Currently, BNB L1 swaps are _cheaper_ to swap than its own synth, around (0.000075 \* $200 = 1.5c), compared to a synth swap of (0.02 \* $2 = 4c). It has been observed (and confirmed after discussions with some known arb teams on THORChain) that is this is one of the reasons why BNB L1 swaps are done over BNB synth swaps.

## Proposal

Floor the `outboundFee` to a value which is:

1. 10-100 times higher than a synth swap
2. Commensurate with the state storage costs for 0.1kb for 12 months, as well as the compute costs for 14 nodes in TSS for 10 seconds

The value that achieves both is around $1.00. Since the cost of an L1 swap is not linked to RUNE value, it makes more sense to use THORChain's USD-sensing logic to peg this fee to a fixed USD value.

In most other chains the `outboundFee` charged typically lands in excess of $1.00, so this proposal will only affect the cheap chains, such as BNB.

- BTC, 10sat/byte, $1.5
- ETH, 10gwei, $2

** Implementation **

The existing `OutboundTransactionFee` is used to charge the RUNE value for toRUNE swaps, as well as the synth value for toSYNTH swaps and is currently set to 0.02RUNE. This can be kept.

A new constant of `minimumL1OutboundFeeUSD` should be added to be 1_0000_0000, which would be read as $1.00.

Charge the outboundFee, then

- convert to USD value
- if it is less than `minimumL1OutboundFeeUSD`, raise it

Use THORChain's USD sensing logic to determine what $1.00 is.

## Decision

The following stakeholders and their potential perspectives are discussed:

** Nodes **
Nodes should support, since the network is now raising fees to match their costs (compute and long-term storage), as well as push more swaps to synths and thus reduce demand on L1 witnessing (less slashes for missing observations)

** LPs **
Long-term LPs should support, since the network is raising fees to bolster the RESERVE (their long-term revenue support)

** Transient Swappers **
Transient Swappers should largely be unaffected, since outboundFees on most chains are higher than $1.00, and $1.00 is a very cheap minimum fee to pay for decentralised swaps.

** Arbers **
Arbitrage agents should largely be nuetral, since they have the option to switch to using synths which has much lower fees (and is faster).
