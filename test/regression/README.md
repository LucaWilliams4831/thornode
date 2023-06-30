# THORNode Regression Testing Framework

Thornode is increasingly complex and the most valuable feature we can provide at this stage is security. We aim to ensure that all changes made to the code base only increase the level of security and confidence in the code for all future features, maintenance, fixes, and optimizations. Unit tests are of high importance, but it is often not possible to cleanly map state changes from user actions into meaningful unit tests. Smoke tests enable us to test the full scope of state changes from user actions, but the mechanics make them slow to run and difficult to author/review - this has resulted in a test suite that does not extensively verify the boundaries of the system, especially with ongoing feature additions.

We aim to create a regression testing harness that focuses specifically on Thornode logic (no Bifrost coverage) for state changes triggered by user operations in a human readable format - making these test cases easy to define for the author, and easy to reason about as a reviewer considering additional boundaries.

## Design

The high level structure of these tests is straightforward - each test case is defined in a YAML file consisting of start state and a set of interleaved transactions and checks. These test cases are organized hierarchically into directories as suites for testing specific features and boundary conditions. In order to avoid raciness in the test cases, we prevent blocks from being created and expose a special operation type to trigger the creation of blocks.

### Directory Structure

Test cases should be organized into directories as “suites” consisting of test cases for specific conditions or features.

```none
suites/
  pools/
    create-pool-rune-first.yaml
    create-pool-same-block.yaml
    create-pool-asset-first.yaml
  core/
    initialize.yaml
  lending/
    borrow-repay.yaml
  mimir/
    killswitch.yaml
    ilp-cutoff.yaml
    max-rune-supply.yaml
  synths/
    mint-burn.yaml
...
```

### Test Structure

The simplest of test structures may look something like:

```yaml
# yaml state deep merged with genesis
---
# create-block
---
# check: thornode endpoint + jq conditions to assert
---
# transaction: observed, deposit, mimir, send
---
# create-block
---
# check: thornode endpoint + jq conditions to assert
---
# ...
```

### Dynamic Values

In order to preserve the human-readability of test cases, the harness will populate embedded variables at runtime for addresses and transaction IDs. These values will be expressed as Go template functions and can be used in the test cases like:

- `{{ addr_bnb_dog }}` (the bnb address for the "dog" mnemonic)
- `{{ observe_txid 1 }}` (deterministic tx id for first observed tx)
- `{{ native_txid 1 }}` (the txid of the first native transaction)
- `{{ native_txid -1 }}` (the txid of the most recent native transaction)
- `{{ pubkey_dog }}` (pubkey for the "dog" mnemonic)
- `{{ template "default-state.yaml" }}` (default go template, embed the contents of the template `default-state.yaml`)

Addresses will be generated for each chain from the following mnemonics which are already used for the nodes in mocknet and each will be referenced by the corresponding animal:

```none
dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog fossil
cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat crawl
fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox filter
pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig quick
```

The `dog` mnemonic is a special case and will be used as the mnemonic for the default simulation validator, and as the mimir admin.

### State Definition

The state definition can be any valid data to deep merge with the default generated genesis file before initializing the simulation validator. There can be multiple `state` operations at the beginning of a test case. In practice this like just contains some pools and LP records which can be easily copied from some network state to simplify the starting state for the simulation:

```yaml
type: state
genesis:
  app_state:
    thorchain:
      pools:
        - LP_units: "428885140806810"
          asset: "BTC.BTC"
          balance_asset: "94662702495"
          balance_rune: "1204803370651200"
          pending_inbound_asset: "0"
          pending_inbound_rune: "0"
          status: "Available"
```

### Check Definition

The check definition contains an endpoint, optional query parameters, and a set of `jq` query assertions against the endpoint response:

```yaml
type: check
endpoint: /thorchain/pools
params: {}
asserts:
  - .|length == 1
  - .[0]|.asset == "BTC.BTC"
  - .[0].LP_units == "428885140806810"
```

### Transaction Definitions

There are multiple types of transactions that may be defined, which map to the protobuf types and should be self-explanatory by example:

```yaml
type: tx-observed-in
signer: {{ addr_thor_dog }}
txs:
  - tx:
      id: "{{ txid 1 }}"
      chain: BTC
      from_address: {{ addr_btc_cat }}
      to_address: {{ addr_btc_dog }}
      coins:
        - amount: "100000000"
          asset: "BTC.BTC"
          decimals: 8
      gas:
        - amount: "10000"
          asset: "BTC.BTC"
      memo: "+:BTC.BTC:{{ addr_thor_cat }}"
    block_height: 1
    finalise_height: 1
    observed_pub_key: {{ pubkey_dog }}
---
type: tx-mimir
key: FullImpLossProtectionBlocks
value: 1
signer: {{ addr_thor_dog }}
---
type: tx-deposit
signer: {{ addr_thor_fox }}
coins:
  - amount: "200000000"
    asset: "rune"
memo: "+:BTC.BTC"
---
type: tx-send
from_address: {{ addr_thor_fox }}
to_address: {{ addr_thor_cat }}
amount:
  - denom: "rune"
    amount: "200000000"
```

All transactions can optionally set a `sequence` parameter to override the transaction sequence - this is required if attempting to send multiple transactions from the same address in the same block.

### Create Blocks Definition

In order to allow the defining of test cases that are sensitive to the timing of blocks and placement of transactions therein, we expose the ability to explicitly trigger the creation of blocks during the test simulation:

```yaml
type: create-blocks
count: 1
```

If a specific transaction should cause the process to exit, an optional `exit` parameter will verify `thornode` exits with the provided code.

## Tips for Writing Tests

The simplest way to approach test creation is to define state changes and transactions and keep the following operation at the end of the test file:

```yaml
type: check
endpoint: <endpoint>
asserts:
  - "false"
```

This assertion will always fail and print the endpoint response to the console for inspecting the current state of a given endpoint after the test run up that point. Remember that the Cosmos and Thorchain APIs are available on port `1317` and the Tendermint APIs are available on port `26657`:

- https://v1.cosmos.network/rpc/v0.45.1
- https://thornode.ninerealms.com/thorchain/doc/#/
- https://docs.tendermint.com/master/rpc/#/

Pass the `RUN` environment variable the name of your test to avoid running all suites (it will also match a regex):

```bash
RUN=my-test make test-regression
```

If stuck set `DEBUG=1` to output the entire log output from the `thornode` process and pause execution at the end of the test to inspect endpoints:

```bash
DEBUG=1 RUN=my-test make test-regression
```

Setting `EXPORT=1` will force overwrite the exported genesis after the test:

```bash
EXPORT=1 make test-regression
```

Setting `PARALLELISM=<parallelism>` will run tests with the provided parallelism.

```bash
PARALLELISM=4 make test-regression
```

### Conventions

We attempt to seed pools based on the following value ratios to keep reasoning simpler:

```none
BTC == 1000 RUNE
ETH == 100 RUNE
<all-others> == 1 RUNE
```

### Coverage

We leverage functionality in Golang 1.20 to track code coverage on the `thornode` binary during live execution. Every run of the regression tests will generate a coverage percentage with archived, versioned, and generated code filtered - the value will be output to the console at the end of the test run. Coverage data is cleared after each run and a convenience target exists to open the coverage data from the last test run in the browser.

```bash
make test-regression-coverage
```

### Flakiness

The nature of these tests should be more predictable than the existing smoke tests in the repo, but there are still some caveats. Since block creation acquires a lock in process that will prevent query handling, all checks between blocks must complete within the block time - this block time defaults to `1s`. Additionally there is some raciness between the return of the application `EndBlock` and the time at which Tendermint, Cosmos, and Thorchain endpoints will execute against the new blocks data - we have a default sleep after the return of `EndBlock` set to `200ms`.

In order to avoid raciness more conveniently while running on resource constrained hardware, all time values above can optionally be increased by an integer factor defined in the `TIME_FACTOR` environment variable. If you find tests are hitting timeouts or returning inconsistent data, simply increase this factor (this will slow down the test run):

```bash
TIME_FACTOR=2 make test-regression
```
