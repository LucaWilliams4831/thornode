#!/bin/sh

SIGNER_NAME="${SIGNER_NAME:=thorchain}"
SIGNER_PASSWD="${SIGNER_PASSWD:=password}"
MASTER_ADDR="${BTC_MASTER_ADDR:=rltc1qj08ys4ct2hzzc2hcz6h2hgrvlmsjynawf4nr3r}"
BLOCK_TIME=${BLOCK_TIME:=1}
RPC_PORT=${RPC_PORT:=18443}

litecoind -regtest -rpcport="$RPC_PORT" -txindex -rpcuser="$SIGNER_NAME" -rpcpassword="$SIGNER_PASSWD" -rpcallowip=0.0.0.0/0 -rpcbind=127.0.0.1 -rpcbind="$(hostname)" &

# give time to litecoind to start
while true; do
  litecoin-cli -regtest -rpcport="$RPC_PORT" -rpcuser="$SIGNER_NAME" -rpcpassword="$SIGNER_PASSWD" generatetoaddress 100 "$MASTER_ADDR" && break
  sleep 5
done

# wait a bit while mocknet starts
sleep 30

# mine a new block every BLOCK_TIME
while true; do
  litecoin-cli -regtest -rpcport="$RPC_PORT" -rpcuser="$SIGNER_NAME" -rpcpassword="$SIGNER_PASSWD" generatetoaddress 1 "$MASTER_ADDR"
  sleep 2.5
done
