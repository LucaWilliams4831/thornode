# THORNode Docker

## Fullnode

The default image will start a fullnode:

```bash
docker run \
  -e CHAIN_ID=thorchain-mainnet-v1 \
  -e NET=mainnet \
  registry.gitlab.com/thorchain/thornode:chaosnet-multichain
```

The above command will result in syncing chain state to ephemeral storage within the container, in order to persist data across restarts simply mount a local volume:

```bash
mkdir thornode-data
docker run \
  -v $(pwd)/thornode-data:/root/.thornode \
  -e CHAIN_ID=thorchain-mainnet-v1 \
  -e NET=mainnet \
  registry.gitlab.com/thorchain/thornode:chaosnet-multichain
```

Nine Realms provides snapshots taken from a statesync recovery which can be rsync'd without need for a high memory (80G at time of writing) machine to recover the statesync snapshot. Ensure `gsutil` is installed, and pull the latest statesync snapshot via:

```bash
mkdir -p thornode-data/data
HEIGHT=$(
  curl -s 'https://storage.googleapis.com/storage/v1/b/public-snapshots-ninerealms/o?delimiter=%2F&prefix=thornode/pruned/' |
  jq -r '.prefixes | map(match("thornode/pruned/([0-9]+)/").captures[0].string) | map(tonumber) | sort | reverse[0]'
)
gsutil -m rsync -r -d "gs://public-snapshots-ninerealms/thornode/pruned/$HEIGHT/" thornode-data/data
docker run \
  -v $(pwd)/thornode-data:/root/.thornode \
  -e CHAIN_ID=thorchain-mainnet-v1 \
  -e NET=mainnet \
  registry.gitlab.com/thorchain/thornode:chaosnet-multichain
```

Since this image tag contains the latest version of THORNode, the node can auto update by simply placing this in a loop to re-pull the image on exit:

```bash
while true; do
  docker pull registry.gitlab.com/thorchain/thornode:chaosnet-multichain
  docker run \
    -v $(pwd)/thornode-data:/root/.thornode \
    -e NET=mainnet \
    registry.gitlab.com/thorchain/thornode:chaosnet-multichain
do
```

The above commands also apply to `testnet` and `stagenet` by simply using the respective image (in these cases `-e NET=...` is not required):

```code
testnet  => registry.gitlab.com/thorchain/thornode:testnet
stagenet => registry.gitlab.com/thorchain/thornode:stagenet
```

## Validator

Officially supported deployments of THORNode validators require a working understanding of Kubernetes and related infrastructure. See the [Cluster Launcher](https://gitlab.com/thorchain/devops/cluster-launcher) repo for cluster Terraform resources, and the [Node Launcher](https://gitlab.com/thorchain/devops/node-launcher) repo for deployment utilities which internally leveraging Helm.

## Mocknet

The development environment leverages Docker Compose V2 to create a mock network - this is included in the latest version of Docker Desktop for Mac and Windows, and can be added as a plugin on Linux by following the instructions [here](https://docs.docker.com/compose/cli-command/#installing-compose-v2).

The mocknet configuration is vanilla, leveraging Docker Compose profiles which can be combined at user discretion. The following profiles exist:

```code
thornode => thornode only
bifrost  => bifrost and thornode dependency
midgard  => midgard and thornode dependency
mocknet  => all mocknet dependencies
```

### Keys

We leverage the following keys for testing and local mocknet setup, created with a simplified mnemonic for ease of reference. We refer to these keys by the name of the animal used:

```text
cat => cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat crawl
dog => dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog fossil
fox => fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox filter
pig => pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig quick
```

### Examples

Example commands are provided below for those less familiar with Docker Compose features:

```bash
# start a mocknet with all dependencies
docker compose --profile mocknet up -d

# multiple profiles are supported, start a mocknet and midgard
docker compose --profile mocknet --profile midgard up -d

# check running services
docker compose ps

# tail the logs of all services
docker compose logs -f

# tail the logs of only thornode and bifrost
docker compose logs -f thornode bifrost

# enter a shell in the thornode container
docker compose exec thornode sh

# copy a file from the thornode container
docker compose cp thornode:/root/.thornode/config/genesis.json .

# rebuild all buildable services (thornode and bifrost)
docker compose build

# export thornode genesis
docker compose stop thornode
docker compose run thornode -- thornode export
docker compose start thornode

# hard fork thornode
docker compose stop thornode
docker compose run /docker/scripts/hard-fork.sh

# stop mocknet services
docker compose --profile mocknet down

# clear mocknet docker volumes
docker compose --profile mocknet down -v
```

## Multi-Node Mocknet

The Docker Compose configuration has been extended to support a multi-node local network. Starting the multinode network requires the `mocknet-cluster` profile:

```bash
docker compose --profile mocknet-cluster up -d
```

Once the mocknet is running, you can open open a shell in the `cli` service to access CLIs for interacting with the mocknet:

```bash
docker compose run cli

# increase default 60 block churn (keyring password is "password")
thornode tx thorchain mimir CHURNINTERVAL 1000 --from dog $TX_FLAGS

# set limit to 1 new node per churn (keyring password is "password")
thornode tx thorchain mimir NUMBEROFNEWNODESPERCHURN 1 --from dog $TX_FLAGS
```

## EVM Tool

The `evm-tool.py` script is leveraged during mocknet init in the `thornode` container to create the router contract and test token, but may also be run directly for additional convenience targets.

### Create Gas and Token Pools

Note that the token address used in this command is the same as the address output in the logs during `thornode` init (created by `evm-tool.py --action deploy`). Run the following within the `docker compose exec thornode sh` shell to create the asset side of the pools:

```bash
python3 /scripts/evm/evm-tool.py --chain ETH --action deposit
python3 /scripts/evm/evm-tool.py --chain ETH --token-address 0x52C84043CD9c865236f11d9Fc9F56aa003c1f922 --action deposit-token
```

Run the following in the `docker compose run cli` shell to create the RUNE side of the pools:

```bash
thornode tx thorchain deposit 10000000000 rune ADD:ETH.ETH:0x8db97c7cece249c2b98bdc0226cc4c2a57bf52fc --from cat $TX_FLAGS
thornode tx thorchain deposit 10000000000 rune ADD:ETH.TKN-0X52C84043CD9C865236F11D9FC9F56AA003C1F922:0x8db97c7cece249c2b98bdc0226cc4c2a57bf52fc --from cat $TX_FLAGS
```

## Local Mainnet Fork of EVM Chain

There are scripts for creation of a mocknet using forked mainnet EVM chains for testing of aggregator contracts. See `[tools/evm/README.md](../../tools/evm/README.md)` for documentation.

## Bootstrap Mocknet Data

You can leverage the smoke tests to bootstrap local vaults with a subset of test data. Run:

```bash
make bootstrap-mocknet
```
