# New Chain Integrations

Integrating a new chain into THORChain is an inherently risky process. THORChain inherits the risks (and value) from each chain it connects. Node operators take on risk and cost by adding new chains. Chains should be economically-significant, acceptable risk, and reasonable cost to be considered.

## **Phase I: Data Gathering and Initial Proposal**

Chains should meet a minimum standard to remain listed on THORChain.

- meet initial listing standards
- meet pool depth, volume, LP count requirements

A chain that changes its characteristics for the worse, or drops in uptake on THORChain may cause the following issues:

- become centralised and introduce a risk to the network
- lose adoption and thus be costly to subsidise for the network

Chains should meet a minimum standard of the following before being listed on THORChain.

- decentralisation
- ossification
- economic value
- developer support
- community support

### **Chain Consequences:**

A chain that fails on THORChain may have the following affects:

- Infinite Mint bug causes theft of pooled assets from LPs
- Impact to reliability of THORNodes (poor sync, halted churn, double-spend txOuts)
- Poor LP uptake causes low fee revenue for that chain
- Waste of developer resources to support the chain
- Disruption to THORChain when Ragnaroking the chain

### **Detailed Requirements:**

A new chain to be added should meet the following requirements:

#### _Decentralisation_

- Must not be controlled by a single entity that can pause the network or freeze accounts.
- Must not be controlled by a multisig < 10 signatories
- If PoS, should have more than 10 Validators

#### _Ossification_

- Must not be younger (since genesis) than 2 years
- Must not be hard-forking more than once per 6 months

#### _Economic Value_

- Must not be less than 10% of THORChain's FDV
- Must have existing daily volumes not less than 10% of $RUNE volumes
- If PoW, must not take longer than 1hour to conf-count a $1k swap

#### _Developer Support_

- Must demonstrate organic developer support
- Must have functioning node client + wallet js client

#### _Community_

- Must have users that exceed 10% of THORChain's on-chain users

### **Removing Chains**

Chains should meet a minimum standard to remain listed on THORChain.

- meet initial listing standards
- meet pool depth, volume, LP count requirements

A chain that changes its characteristics for the worse, or drops in uptake on THORChain may cause the following issues:

- become centralised and introduce a risk to the network
- lose adoption and thus be costly to subsidise for the network

A chain should be purged from THORChain if any of the following are sustained over a 6 month period:

- Breach any of New Chain Standards set out above
- Have a base asset pool depth that drops below `MINRUNEPOOLDEPTH`
- Have daily volumes that drop below $1k for an entire `POOLCYCLE`
- Have less than 100 LPs

### **Proposal of a New Chain:**

New chain is proposed in #propose-a-chain, and a new channel created under “Community Chains” in Discord. This is an informal proposal, and should loosely follow the template under [Chain Proposal Template](#chain-proposal-template).

### **Node Mimir Vote:**

Prompt Node Operators to vote on `Halt<Proposed-Chain>Chain=1` view Node Mimir. If a 50% consensus is reached then development of the chain client can be started.

## **Phase II: Development, Testing, and Auditing**

1. _Chain Client Development Period:_ Community devs of the Proposed Chain build the Bifrost Chain Client, and open a PR to [`thornode`](https://gitlab.com/thorchain/thornode) (referencing the Gitlab issue created in the discussion phase), and [`node-launcher`](https://gitlab.com/thorchain/node-launcher) repos.

   1. All PRs should meet the public requirements set forth in [Technical Requirements and Guidelines](#technical-requirements-and-guidelines).

1. _Stagenet Merge/Baking Period:_ Community devs are incentivized to test all necessary functionality as it relates to the new chain integration. Any chain on stagenet that is to be considered for Mainnet will have to go through a defined baking/hardening process set forth

_Functionality to be tested:_

- Swapping to/from the asset
- Adding/withdrawing assets on the chain
- Minting/burning synths
- Registering a thorname for the chain
- Vault funding
- Vault churning
- Inbound addresses returned correctly
- Insolvency on the chain halts the chain
- Unauthorised tx on the chain (double-spend) halts the chain
- Chain client does not sign outbound when `HaltSigning<Chain>` is enabled

_Usage requirements:_

- 100 inbound transactions on stagenet
- 100 outbound transactions on stagenet
- 100 RUNE of aggregate add liquidity transactions on stagenet
- 100 RUNE of aggregate withdraw liquidity transactions on stagenet

3. **Chain Client Audit:** An expert of the chain (that is not the author) must independently review the chain client and sign off on the safety and validity of its implementation. The final audit must be included in the chain client Pull Request under `bifrost/pkg/chainclients/<chain-name>`.

## **Phase III: Mainnet Release**

The following steps will be performed by the core team and Nine Realms for the final rollout of the chain.

1. _Admin Mimir:_ Halt the new chain and disable trading until rollout is complete.

1. _Daemon Release and Sync:_ Announcement will be made to NOs to `make install` in order to start the sync process for the new chain daemon.

1. _Enable Bifrost Scanning:_ The final `node-launcher` PR will be merged, and NOs instructed to perform a final `make install` to enable Bifrost scanning.

1. _Admin Mimir:_ Unhalt the chain to enable Bifrost scanning.

1. _Admin Mimir:_ Enable trading once nodes have scanned to the tip on the new chain.

---

## Technical Requirements and Guidelines

A new Chain Integration must include a pull request to [`thornode`](https://gitlab.com/thorchain/thornode) (referencing the Gitlab issue created in the discussion phase) and [`node-launcher`](https://gitlab.com/thorchain/node-launcher).

### **Thornode PR Requirements**

1. Ensure a "mocknet" (local development network) service for the chain daemon is be added (`build/docker/docker-compose.yml`).
1. Ensure **70% or greater** unit test coverage.
1. Ensure a `<chain>_DISABLED` environment variable is respected in the Bifrost initialization script at `build/scripts/bifrost.sh`.
1. Lead a live walkthrough (PR author) with the core team, Nine Realms, and any other interested community members. During the walkthrough the author must be able to speak to the questions in (#chain-client-implementation-considerations).
1. Can an inbound transaction be "spoofed" - i.e. can the Chain Client be tricked into thinking value was transferred into THORChain, when it actually was not?
1. Does the chain client properly whitelist valid assets and reject invalid assets?
1. Does the chain client properly convert asset values to/from the 8 decimal point standard of thornode?
1. Is gas reporting deterministic? Every Bifrost must agree, or THORChain will not reach consensus.
1. Does the chain client properly report solvency of Asgard vaults?

#### **Node Launcher PR Requirements**

There should be 3 PRs in the node-launcher repo - the first to add the Docker image for the chain daemon, the second to add the service, the third to enable scanning in Bifrost. The first must be merged first so that hashes from the image builds may be pinned in the second.

1. **Image PR**
   1. Add a Dockerfile at `ci/images/<chain>/Dockerfile`.
   2. Ensure all source versions in the Dockerfile are pinned to a specific git hash.
2. **Services PR**
   1. Use an existing chain directory as a template for the new chain daemon configuration, reference the PR for the last added chain.
   2. Ensure the resource request sizes for the daemon are slightly over-provisioned (~20%) to the average expected utilization under normal operation.
   3. Extend the `get_node_service` function in `scripts/core.sh` with the service so that it is available for the standard make targets.
   4. Extend the `deploy_fullnode` function in `scripts/core.sh` with `--set <daemon-name>.enabled=false` in both the diff and install commands.
   5. Ensure the `<chain>_DISABLED` environment variable is used to disable the chain via a variable in `bifrost/values.yaml`.
3. **Enable PR**
   1. Update `bifrost/values.yaml` to enable the chain.

### Chain Proposal Template

```yaml
Chain Name:
Chain Type: EVM/UTXO/Cosmos/Other
Hardware Requirements: Memory and Storage
Year started:
Market Cap:
CoinMarketCap Rank:
24hr Volume:
Current DEX Integrations:
Other relevant dApps:
Number of previous hard forks:
```
