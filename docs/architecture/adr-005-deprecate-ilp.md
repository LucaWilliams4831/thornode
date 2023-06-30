# ADR 005: Deprecate Impermanent Loss Protection

## Changelog

- December 12, 2022: Initial commit

## Status

Accepted

## Context

Having been necessary to bootstrap liquidity pools and attract capital during THORChain’s early-stage growth, Impermanent Loss Protection has served its purpose. The protocol has since evolved to offer Savings Vaults and Protocol Owned Liquidity (PoL), which give the protocol reserve the ability to take a more long-term outlook. Rather than subsidize LPs impermanent loss, the protocol reserve can take a stake in the pools directly via PoL. Paired with Savings Vaults, the need for dual-sided LP incentives becomes less apparent.

As demonstrated by Bancor’s rapid death spiral—attributed to their implementation of impermanent loss protection—we have seen how the feature can be dangerous at scale (particularly if offered on volatile assets). In THORChain’s case, a sudden loss of value of $RUNE price comparable to the price(s) of other assets may lead to a rapid, large-scale drawdown from the protocol reserve. Some external event (other than ILP being paid out over typical market cycles), such as an exploit or sanctions, could cause such a price drop. In such a scenario, dual-sided LPs may begin exiting and selling $RUNE-denominated impermanent loss protection (ILP) to cover losses, requiring an increasing amount of $RUNE to be pulled from the protocol reserve, further exacerbating the issue.

Therefore, it is necessary to re-evaluate the need for Impermanent Loss Protection.
While we have seen that THORChain’s Impermanent Loss Protection (ILP) has remained robust over bull (‘20-21) and bear markets (‘22+), impermanent loss protection remains an outstanding, potentially unbounded liability to the protocol.

## Proposed Change

1. Grandfather existing ILP liabilities. Existing depositors would remain covered in perpetuity. This ensures there is not a rush for the exit. Without grandfathering existing liabilities, LPs wanting to claim existing protections would withdraw. It is estimated that a $RUNE price of $6 negates all existing liabilities. Above that price, ILP liabilities would be effectively zero RUNE.
2. Thirty (30) days after the vote passes, Impermanent Loss Protection will be disabled for all new LPs. This gives prospective LPs the ability to lock-in ILP for the next until the cutoff date, which may attract new capital to the system.

## Alternatives Considered

An alternative to ILP was considered: “Deposit Protection”. The goal of Deposit Protection was to deprecate ILP, while protecting dual-sided LPs from negative LUVI (though not protecting them from impermanent loss). However, upon further consideration, the core team and Nine Realms determined that Deposit Protection does not achieve the stated goal, and that it would still create an unbounded liability to the protocol reserve. This was deemed unacceptable and therefore has been scrapped.

## References

- Deposit Protection: https://gitlab.com/thorchain/thornode/-/issues/1408
