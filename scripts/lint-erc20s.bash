#!/bin/bash
set -euo pipefail

# Ethereum tokens
git show origin/develop:common/tokenlist/ethtokens/eth_mainnet_V95.json |
  jq -r '.tokens[] | .address | ascii_downcase' | sort -n | uniq -u >/tmp/orig_erc20_token_list.txt

jq -r '.tokens[] | .address | ascii_downcase' <common/tokenlist/ethtokens/eth_mainnet_V95.json |
  uniq -u >/tmp/modified_erc20_token_list.txt

cat /tmp/orig_erc20_token_list.txt /tmp/modified_erc20_token_list.txt |
  sort -n | uniq -d >/tmp/union_erc20_token_list.txt

diff /tmp/orig_erc20_token_list.txt /tmp/union_erc20_token_list.txt || exit 1

# AVAX Tokens
git show origin/develop:common/tokenlist/avaxtokens/avax_mainnet_V95.json |
  jq -r '.tokens[] | .address | ascii_downcase' | sort -n | uniq -u >/tmp/orig_avax_erc20_token_list.txt

jq -r '.tokens[] | .address | ascii_downcase' <common/tokenlist/avaxtokens/avax_mainnet_V95.json |
  uniq -u >/tmp/modified_avax_erc20_token_list.txt

cat /tmp/orig_avax_erc20_token_list.txt /tmp/modified_avax_erc20_token_list.txt |
  sort -n | uniq -d >/tmp/union_avax_erc20_token_list.txt

diff /tmp/orig_avax_erc20_token_list.txt /tmp/union_avax_erc20_token_list.txt || exit 1

echo "OK"
