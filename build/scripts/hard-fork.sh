#!/bin/sh

set -ex

HARDFORK_BLOCK_HEIGHT="${HARDFORK_BLOCK_HEIGHT:--1}"
CHAIN_ID="${CHAIN_ID:-}"
NEW_GENESIS_TIME="${NEW_GENESIS_TIME:-}"
if [ -z "$CHAIN_ID" ]; then
  echo "CHAIN_ID is empty"
  exit 1
fi
if [ -z "$NEW_GENESIS_TIME" ]; then
  echo "NEW_GENESIS_TIME is empty"
  exit 1
fi
DATE=$(date +%s)
echo "new chain id: $CHAIN_ID , genesis_time:$NEW_GENESIS_TIME"

# backup first
cp -r ~/.thornode/config ~/.thornode/config."$DATE".bak

# export genesis file
thornode export --height "$HARDFORK_BLOCK_HEIGHT" >thorchain_genesis_export."$DATE".json

# reset the database
thornode unsafe-reset-all

# update chain id
jq --arg CHAIN_ID "$CHAIN_ID" --arg NEW_GENESIS_TIME "$NEW_GENESIS_TIME" '.chain_id=$CHAIN_ID | .genesis_time=$NEW_GENESIS_TIME' thorchain_genesis_export."$DATE".json >temp.json
# copied exported genesis file to the config directory
cp temp.json ~/.thornode/config/genesis.json
