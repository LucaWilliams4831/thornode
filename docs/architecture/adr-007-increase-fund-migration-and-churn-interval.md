# ADR 007: Increase Fund Migration and Churn Interval

## Changelog

- March 27, 2023: Initial commit

## Status

Proposed

## Context

Currently churns are roughly 3 days and migrations take about 7 hours to complete after keygen for the new vaults has succeeded. Nine Realms has been working hard with many wallets to integrate Thorchain as a backend swap provider, and there have been a few cases where wallet bugs resulted in inbounds sent to retired vaults.

One recent example of a significant instance was a retired vault (https://blockstream.info/address/bc1q7zgtsnxhu7q4v467aqfj646s2ksps2rumhq0pz) that had almost a full BTC sent to it within a few hours of being retired - and the wallet claimed to the user that the lost funds were Thorchain's responsibility. While we still maintain that this is a wallet error and responsibility, in the ideal scenario we would still handle the inbound. The general perspective is that there is high likelihood over coming years that wallet bugs may lead to retired vault inbounds and lost user funds, and we should make the system as forgiving as possible to protect those relations.

The simplest way to ensure late inbounds to old vaults are handled is by increasing the time the vault stays retiring. While this may be a small inconveneince for nodes, providing the largest time window possible is a proactive step to prevent unfortunate experiences for onboarding wallets and end users.

## Proposed Change

Nine Realms proposes rolling out this change in 2 stages:

1. Increase `FundMigrationInterval` to `3600` (5x current), which will result in the time to churn out taking roughly half of a current churn cycle.
1. Allow nodes a few weeks to prepare for the change in cadence and then increase the churn interval to `129600` (3x current) and `FundMigrationInterval` to `14400`, which will result in churns approximately every 9 days with vaults remaining retiring for approximately 7 days. If nodes want to churn out and then back in within one round, they will have roughtly 2 days at the end of the churn to do so.

## Alternatives Considered

1. Protocol changes to continue observing retired vaults and make a best effort to refund inbounds if a quorum of nodes for the retiring vault remain.
1. Manual coordination of nodes to reconstitute funds in retired vaults along with manual payout from the treasury on a subjective basis.

## Consequences

### Positive

- Inbounds that were stuck or sent to an improperly cached address will have a large time buffer to be observed.
- Larger churn interval reduces gas spend for migrations, which will slightly reduce outbound fees for users with the advent of https://gitlab.com/thorchain/thornode/-/merge_requests/2835.

### Negative

- Churns will happen less frequently and take longer - if nodes want to churn out for only one round to perform maintenance and then immediately churn back in, they must act quickly.
