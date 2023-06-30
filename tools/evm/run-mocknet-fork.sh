#!/usr/bin/env bash

# NOTE: This script should only be entered by the Makefile target.

set -euo pipefail

# set chain rpc to local hardhat
UPPER_CHAIN=$(echo "$1" | tr '[:lower:]' '[:upper:]')

# get chain rpc
case $1 in
"avax")
  CHAIN_RPC="https://rpc.ankr.com/avalanche"
  export "${UPPER_CHAIN}_HOST"=http://host.docker.internal:5458/ext/bc/C/rpc
  ;;
"eth")
  CHAIN_RPC="https://rpc.ankr.com/eth"
  export "${UPPER_CHAIN}_HOST"=http://host.docker.internal:5458
  ;;
"bsc")
  CHAIN_RPC="https://rpc.ankr.com/bsc"
  export "${UPPER_CHAIN}_HOST"=http://host.docker.internal:5458
  ;;
*)
  echo "Unsupported chain: $1"
  exit 1
  ;;
esac

# start mocknet
docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard up -d

set -m

# start hardhat in background
cd tools/evm
npm install
npx hardhat node --fork $CHAIN_RPC --port 5458 --hostname 0.0.0.0 &

# bootstrap usdc balance
node init.js

# attach to hardhat
fg
