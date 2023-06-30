#!/bin/sh

set -o pipefail

. "$(dirname "$0")/core.sh"

if [ "$NET" = "mocknet" ] || [ "$NET" = "testnet" ]; then
  echo "Loading unsafe init for mocknet and testnet..."
  . "$(dirname "$0")/core-unsafe.sh"
  . "$(dirname "$0")/testnet/state.sh"
fi

NODES="${NODES:=1}"
SEED="${SEED:=thornode}" # the hostname of the master node
ETH_HOST="${ETH_HOST:=http://ethereum:8545}"
AVAX_HOST="${AVAX_HOST:=http://avalanche:9650}"
THOR_BLOCK_TIME="${THOR_BLOCK_TIME:=5s}"
CHAIN_ID=${CHAIN_ID:=thorchain}

# this is required as it need to run thornode init, otherwise tendermint related command doesn't work
if [ "$SEED" = "$(hostname)" ]; then
  if [ ! -f ~/.thornode/config/priv_validator_key.json ]; then
    init_chain
    # remove the original generate genesis file, as below will init chain again
    rm -rf ~/.thornode/config/genesis.json
  fi
fi

create_thor_user "$SIGNER_NAME" "$SIGNER_PASSWD" "$SIGNER_SEED_PHRASE"

VALIDATOR=$(thornode tendermint show-validator | thornode pubkey --bech cons)
NODE_ADDRESS=$(echo "$SIGNER_PASSWD" | thornode keys show thorchain -a --keyring-backend file)
NODE_PUB_KEY=$(echo "$SIGNER_PASSWD" | thornode keys show thorchain -p --keyring-backend file | thornode pubkey)
VERSION=$(fetch_version)

if [ "$SEED" = "$(hostname)" ]; then
  echo "Setting THORNode as genesis"
  if [ ! -f ~/.thornode/config/genesis.json ]; then
    # add ourselves to the genesis state
    NODE_IP_ADDRESS=${EXTERNAL_IP:=$(curl -s http://whatismyip.akamai.com)}

    init_chain "$NODE_ADDRESS"
    add_node_account "$NODE_ADDRESS" "$VALIDATOR" "$NODE_PUB_KEY" "$VERSION" "$NODE_ADDRESS" "$NODE_PUB_KEY_ED25519" "$NODE_IP_ADDRESS"

    # disable default bank transfer, and opt to use our own custom one
    disable_bank_send

    # for mocknet, add initial balances
    echo "Using NET $NET"
    if [ "$NET" = "mocknet" ]; then
      echo "Setting up accounts"

      # smoke test accounts
      add_account tthor1z63f3mzwv3g75az80xwmhrawdqcjpaekk0kd54 rune 5000000000000
      add_account tthor1wz78qmrkplrdhy37tw0tnvn0tkm5pqd6zdp257 rune 25000000000100
      add_account tthor18f55frcvknxvcpx2vvpfedvw4l8eutuhku0uj6 rune 25000000000100
      add_account tthor1xwusttz86hqfuk5z7amcgqsg7vp6g8zhsp5lu2 rune 5090000000000

      # local cluster accounts (2M RUNE)
      add_account tthor1uuds8pd92qnnq0udw0rpg0szpgcslc9p8lluej rune 200000000000000 # cat
      add_account tthor1zf3gsk7edzwl9syyefvfhle37cjtql35h6k85m rune 200000000000000 # dog
      add_account tthor13wrmhnh2qe98rjse30pl7u6jxszjjwl4f6yycr rune 200000000000000 # fox
      add_account tthor1qk8c8sfrmfm0tkncs0zxeutc8v5mx3pjj07k4u rune 200000000000000 # pig

      reserve 22000000000000000

      # deploy evm contracts
      deploy_evm_contracts

    else
      echo "ETH Contract Address: $CONTRACT"
      set_eth_contract "$CONTRACT"

    fi

    if [ "$NET" = "testnet" ]; then
      # mint 1m RUNE to reserve for testnet
      reserve 100000000000000

      # add testnet account and balances
      testnet_add_accounts
    fi

    echo "Genesis content"
    cat ~/.thornode/config/genesis.json
    thornode validate-genesis --trace
  fi
fi

# setup peer connection, typically only used for some mocknet configurations
if [ "$SEED" != "$(hostname)" ]; then
  if [ ! -f ~/.thornode/config/genesis.json ]; then
    echo "Setting THORNode as peer not genesis"

    init_chain "$NODE_ADDRESS"
    NODE_ID=$(fetch_node_id "$SEED")
    echo "NODE ID: $NODE_ID"
    export THOR_TENDERMINT_P2P_PERSISTENT_PEERS="$NODE_ID@$SEED:$PORT_P2P"

    cat ~/.thornode/config/genesis.json
  fi
fi

# render tendermint and cosmos configuration files
thornode render-config

export SIGNER_NAME
export SIGNER_PASSWD
exec "$@"
