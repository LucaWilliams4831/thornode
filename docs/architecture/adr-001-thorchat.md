# ADR 001: ThorChat

## Changelog

- 2022-05-07: Created

## Status

Paused

## Context

Node operator communications are currently conducted over Discord (a centralized service) and are asymmetrical in nature. In order to preserve privacy NOs are encouraged to use `make relay` which uses a relay bot to post messages into a Discord channel. This leads to a suboptimal communication style.

This document outlines a replacement for that communication channel: ThorChat. ThorChat is a standard opensource chat daemon (MatterMost, IRC, etc.) accessible only via a Tor Hidden Service. Node operators will authenticate using a `make go-chat` that automatically creates an account based on the node pubkey, and have special access to public channel where only NOs/core community members can talk, for coordinating network operation.

## Alternative Approaches

- [Tox](https://tox.chat/about.html) - Use a group text chat on Tox.
  - Pros:
    - Even more decentralized.
  - Cons:
    - Less featureful UX.
    - No browser [client](https://tox.chat/clients.html), requires new app on user side (most written in C).
    - Higher bar for outside community members to jump on and observe.

## Decision

TBD

## Detailed Design

### Architecture

ThorChat will be a four+ pod deployment:

- Tor relay for terminating the Hidden Service.
- Nginx for locking down handler paths & caching.
- Chat server (Mattermost or IRC)
- Database/storage pod(s) - primary/secondaries as needed for scale.

### New development

There are only a few greenfield components needed:

- Auth plugin for taking signed message from `make go-chat` and creating/updating specially tagged node operator account in the backend.
- Audit of chosen chat server codebase for security suitability.

### Risks

A chat service's primary operational risks are:

- Spam/DoS control
- Account takeover
- RCE

These pose an additional level of risk in the proposed application, as takeover of the chat server here (by large scale account takeover, or RCE) allows for possible social engineering of Thorchain decisions.

### Benefits

This system would allow for simple and more seamless node operator communications, with a minimal burden on participants. Only a Tor-capable browser is required (TorBrowser, the Tor feature in Brave, etc.) and anonymity is not only preserved, it is enforced.

More significantly, node operators will have a significantly better comms experience - reply notifications, presence status, ability to use reacts. Simple polls can be taken with reacts, or plugins could be added for polling/feedback collection, etc.

### Operations

The operational burden would be the cost of running the aforementioned services, plus the requisite observability/uptime/on-call duties.

### Coordination

As this is a new component separate from the existing thornode network, coordination for rollout of this service is minimal. It would primarily be social, helping make node operators aware of the new primary comms channel and how to access it.

### Open Questions

#### Choice of chat server

MatterMost or an IRC daemon? The tradeoff between these two is feature set vs. security footprint, resepectively. Author leans towards MatterMost as it best recreates the current Discord UX, but acknowledges it will require more work to audit and lock down.

#### Maintenance of Discord

The dependent questions:

- Shift to using this new chat system for all community discussion vs. just node operator discussion?
- Maintain the current Discord in parallel or only use as a hot spare?

## Consequences

### Positive

- Increased engagement & interaction between/with node operators.
- Reduced centralization threat.
- Reduced dependency on corporate infrastructure.

### Negative

- Additional system ops/maintenance burden.
- Increased attack surface area.

### Neutral

- Split communications, if Discord is still preferred for the general community channels.

## References

- TODO: list of Discord outages.
