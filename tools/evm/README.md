# EVM Tools

We provide convenience tools to run mocknet against forked mainnet EVM chains for the testing of aggregator contracts. The following instructions assume the working directory is the repo root.

## Contract Testing Prerequisites

1. The contract must exist in the mocknet whitelist at `x/thorchain/aggregators/dex_mocknet.go`.
1. If using `evm-tool.py`, the default account (`0x8db97c7cece249c2b98bdc0226cc4c2a57bf52fc`) must contain token balances for the swap-in token (see section below for fake balances).

## Fake Token Balances

The `init.js` script is run during intialization of the EVM fork and uses impersonation APIs to create a fake 100K balance of the chain-native USDC token on all fork chains. The tokens used at the time of writing are:

```javascript
const usdTokens = {
  ETH: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // ETH.USDC
  AVAX: "0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E", // AVAX.USDC
  BSC: "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", // BSC.USDC
};
```

## DEX Aggregator Testing

The following instructions will be performed for Ethereum, but apply in the same manner to other chains. Note that the swap in ABI is not necessarily consistent across all contracts, so the `swap_in` function in `build/scripts/evm/evm-tool.py` may need to be slightly modified to support alternate function signatures.

### Setup

1. Start mocknet with forked Ethereum:

```bash
make reset-mocknet-fork-eth
```

2. Watch logs and wait until Bifrost startup:

```bash
make logs-mocknet
```

3. Create ETH pool:

```bash
# deposit (1 ETH) 3 times for confirmation counting (only once for other EVM chains)
for i in 1 2 3; do
  python3 build/scripts/evm/evm-tool.py --chain ETH --rpc http://localhost:5458 --action deposit
done
```

Then deposit the RUNE side to create the pool - run the following in the `docker compose -f build/docker/docker-compose.yml run cli` shell to create the RUNE side of the pools:

```bash
thornode tx thorchain deposit 10000000000 rune ADD:ETH.ETH:0x8db97c7cece249c2b98bdc0226cc4c2a57bf52fc --from cat $TX_FLAGS
```

### Swap In

Note that swap in may not be consistent across all DEX aggregators. Testing alternative implementations can be done by overwriting `build/scripts/evm/aggregator-abi.json` and updating the arguments passed in the `swap_in` function of `evm-tool.py`. Call `swapIn` on the aggregator contract using the USDC minted to the `evm-tool.py` admin account:

```bash
python3 build/scripts/evm/evm-tool.py --chain ETH --rpc http://localhost:5458 --token-address 0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48 --action swap-in --agg-address 0xBd68cBe6c247e2c3a0e36B8F0e24964914f26Ee8
```

The other EVM chains do not require confirmations, but for Ethereum we'll send 2 dummy deposits to force confirmation of the swap in:

```bash
for i in 1 2; do
  python3 build/scripts/evm/evm-tool.py --chain ETH --rpc http://localhost:5458 --action deposit
done
```

The account balance at http://localhost:1317/cosmos/bank/v1beta1/balances/tthor1uuds8pd92qnnq0udw0rpg0szpgcslc9p8lluej should include the RUNE output from the swap in.

### Swap Out

Run the following in the `docker compose -f build/docker/docker-compose.yml run cli` shell to swap out USDC (or another token of choice) to the default `evm-tool` address (or an address of choice and use the `--address` flag below to check the balance):

```bash
thornode tx thorchain deposit 10000000000 rune SWAP:ETH.ETH:0x8db97c7cece249c2b98bdc0226cc4c2a57bf52fc::::0xbd68cbe6c247e2c3a0e36b8f0e24964914f26ee8:0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48  --from cat $TX_FLAGS
```

Check the token balance on the target account:

```bash
python3 build/scripts/evm/evm-tool.py --chain ETH --rpc http://localhost:5458 --token-address 0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48 --action token-balance
```
