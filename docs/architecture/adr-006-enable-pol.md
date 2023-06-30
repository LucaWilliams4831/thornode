# ADR 006: Enable POL

## Changelog

- February 17, 2022: Initial commit

## Status

Proposed

## Context

Protocol Owned Liquidity is a mechanism whereby the protocol utilizes the Protocol Reserve to deposit $RUNE asymmetrically into liquidity pools. In effect, it is taking the RUNE-side exposure in dual-sided LPs, reducing synth utilization, so that Savers Vaults can grow. Protocol Owned Liquidity may generate profit or losses to the Protcol Reserve, and care should be taken to determine the timing, assets and amount of POL that is deployed to the network.

A vote is currently underway to raise the `MAXSYNTHPERPOOLDEPTH` from `5000` to `6000`. Nodes have already been instructed that raising the vote to `6000` comes with an implicit understanding that Protocol Owned Liquidity (POL) will be activated as a result (https://discord.com/channels/838986635756044328/839001804812451873/1074682919886528542). This ADR serves to codify the exact parameters being proposed to enable POL.

## Proposed Change

- `POLTargetSynthPerPoolDepth` to `4500`: POL will continue adding RUNE to a pool until the synth depth of that pool is 45%.
- `POLBuffer` to `500`: Synth utilization must be >5% from the target synth per pool depth in order to add liquidity / remove liquidity. In this context, liquidity will be withdrawn below 40% synth utilization and deposited above 50% synth utilization.
- `POLMaxPoolMovement` to `1`: POL will move the pool price at most 0.01% in one block
- `POLMaxNetworkDeposit` to `1000000000000`: start at 10,000 RUNE, with authorization to add up to 10,000,000 RUNE on an incremental basis at developer's discretion. After 10m RUNE, a new vote must be called to further raise the `POLMaxNetworkDeposit`.
- `POL-BTC-BTC` to `1`: POL will start adding to the BTC pool immediately, as the pool has reached its synth cap at the time of publication.
- `POL-ETH-ETH` to `1`: POL will start adding to the ETH pool once it has reached the its synth cap.

The threshold for this ADR to pass are as follows, in chronological order:

- `MAXSYNTHPERPOOLDEPTH` to `6000` achieves 2/3rds node vote consensus
- If the author requests a Motion to Bypass and fewer than 16% of nodes dissent within 7 days (by setting `DISSENTPOL` to `1`)
- `ENABLEPOL` to `1` achieves 2/3rds node vote consensus

## Alternatives Considered

The pros/cons and alternatives to Protocol Owned Liquidity have been discussed on Discord ad neauseum. Check the [#economic-design](https://discord.com/channels/838986635756044328/839002361749438485) channel for discussion, as most topics have been covered there. The benefits and risks of POL are complex and cannot be summarized impartially by the author of this ADR. Get involved in the discussion and do your own research.

## References

- [GitLab Issue](https://gitlab.com/thorchain/thornode/-/issues/1342#protocol-owned-liquidity-pol)
- [GrassRoots Crypto](https://www.youtube.com/watch?v=Up2-arSzH5k)
