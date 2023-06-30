# ADR 009: Reserve Income and Fee Overhaul

## Changelog

- 17 Apr 23: drafted

## Status

The acceptance of ADR 008 ^(1) necessitates an overhaul of Reserve Income and Fees; this ADR.

## Context

ADR 008 seeks to reduce L1 Outbound Fees to a minimum (1:1 gas spent) to make ETH and BTC swaps cheaper, thus drive up L1 swap adoption.
In past this fee (the multiplier charged on top of L1 outbounds), drove ~500k of annual income to the RESERVE (2022 terms). ^(2)
The L1 Outbound Fee lower bound is priced in USD, but other fees are priced in RUNE.

The community wish to overhaul fees to make them easier to understand, fairer, and more appropriate for the purposes of TC.

Three Goals

1. Overhaul fee price denomination
2. Revamp fees to source 500k in annual income to make up for ADR 008
3. Overhaul the role of the RESERVE in fees

** THORChain Fees **

| Fee                    | Description              | Amount                                                                             | Recipient                                |
| ---------------------- | ------------------------ | ---------------------------------------------------------------------------------- | ---------------------------------------- |
| Liquidity Fee          | Paid on every swap       | Proportional to slip                                                               | 100% to Network participants intra-block |
| L1 Outbound Fee        | L1 Outbounds             | Ideally 1:1 gas spent, but a minimum of $1.00 is enforced to pay for TSS resources | Reserve                                  |
| Native Outbound Fee    | RUNE and synth outbounds | 0.02 RUNE                                                                          | Reserve                                  |
| Native Transaction Fee | RUNE and synth transfers | 0.02 RUNE                                                                          | Reserve                                  |
| TNS Fees               | Fees to register TNS     | 10 RUNE + 10 RUNE per year                                                         | Reserve                                  |

## Proposal

** USD Pricing **
All fees users directly pay should be delineated in USD terms using the internal USD price feed.

- `MinimumL1OutboundFeeUSD :1_0000_0000` -> `MinimumL1OutboundFeeUSD : 2_0000_0000`
- `OutboundTransactionFee : 200_0000` -> `OutboundTransactionFeeUSD : 2000_0000` (20c)
- `NativeTransactionFee : 200_0000` -> `NativeTransactionFeeUSD : 2000_0000` (20c)
- `TNSRegisterFee: 10_0000_0000`, -> `TNSRegisterFeeUSD: 10_0000_0000`, ($10)
- `TNSFeePerBlock: 20`, -> `TNSFeePerBlockUSD: 20`, ($10 per year)

** 500k Extra Income **
To source another 500k in income, the Native Outbound and Transaction fees should be increased from ~0.02R (3c) to 20c (as above),
and the MinimumL1OutboundFeeUSD should be repriced from $1.00 to $2.00.

** Role of Reserve **
The RESERVE is a large pool of capital that is used

- to pay out to Network participants on a smoothing function (reduce volatility)
- fund ILP (deprecated)
- fund Protocol Owned Liquidity (a profit-seeking facility and LP-of-last-resort)

One of the draw-backs from paying fees intra-block is volatility - yield for Savers, LPs and Nodes can fluctuate depending on the daily economic activity of the chain.
This begs the question - why not pay all fees into the Reserve and slightly increase the Emissions?
This means ALL income goes into a smoothing function and yield would be fairly constant even over periods of 3-6 months.
The yield computed daily, monthly or even yearly would be very similiar, thus frontends and wallets would align much closer when displaying APR.

## Decision

PENDING

## Detailed Design

Implementation Requirements

- revamp fees to use USD pricing
- divert 100% of liquidity fees to the RESERVE

Mimir Requirements

- Set all new fees
- Yield will drop by ~25% for network participants, so `EmissionCurve` should be changed from `8` to `6`, which will increase it back by +25%

## Consequences

### Positive

- Network participants will enjoy smoothed yield that doesn't fluctuate monthly, but is the same magnitude
- Reserve Income is re-established
- Arbs will have much better PnL tracking since fees are priced in USD

### Negative

- Arbs will pay 20c per synth swap, which may erode synth arb volume

### Nuetral

- Exchanges will need to be notified that the transfer fee has increased and how much
- Block Rewards will increase

## References

(1) https://gitlab.com/thorchain/thornode/-/blob/develop/docs/architecture/adr-008-implement-dynamic-outbound-fee-multiplier.md
(2) https://flipsidecrypto.xyz/Multipartite/reserve-cumulative-income-health-rOUjF2
