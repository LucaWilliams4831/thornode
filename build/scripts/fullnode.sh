#!/bin/sh

set -o pipefail

export SIGNER_NAME="${SIGNER_NAME:=thorchain}"
export SIGNER_PASSWD="${SIGNER_PASSWD:=password}"

. "$(dirname "$0")/core.sh"

if [ ! -f ~/.thornode/config/genesis.json ]; then
  init_chain
  rm -rf ~/.thornode/config/genesis.json # set in thornode render-config
fi

# render tendermint and cosmos configuration files
thornode render-config

exec thornode start
