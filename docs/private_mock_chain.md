# How to run a private chain(mocknet)

mocknet is a special mode in THORNode which is used for test purpose, in mocknet mode, Binance daemon will be running a custom implementation which mock binance interface,refer to [here](https://gitlab.com/thorchain/bepswap/mock-binance) for the source code
BTC/LTC/BCH will be running in regtest mode

In order to run mocknet in a private chain , we will need four linux machine , these days , it is very easy to get that from any of the cloud platform. It can by AWS, DigitalOcean , Google cloud etc.

Here I assume you already have 4 linux machine

## Prerequisite

Each of the linux machine you will have the following installed

- Docker - you will need user docker to build the image that will be running on the machine
- Docker compose
- [Jq](https://stedolan.github.io/jq/)

If your linux machine doesn't have these , you can run the following command to install it

```bash
apt update
apt -y install make docker.io jq
systemctl start docker
systemctl enable docker

curl -L "https://github.com/docker/compose/releases/download/1.25.3/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose
```

Besides the above software , you will also need to open some ports , so the node can talk to each other.

```bash
iptables -A INPUT -p tcp  --match multiport --dports 1317,26656,26657,26660,8000,8080,6040,5040,4040,18443,18333,18334,18332 -j ACCEPT
iptables -A INPUT -p tcp  --match multiport --dports 8545,30301,30303,28443 -j ACCEPT
```

## Clone thornode

```bash
git clone --single-branch --branch develop https://gitlab.com/thorchain/thornode.git ~/thornode
```

This will get from master branch , likely it is not the one you want to use , so checkout the `branch` you would like to test. Here let's assume you will testing `develop` branch

```bash
cd ~/thornode
git checkout develop
```

## Build docker images

Run the following command in ~/thornode folder , this will build your docker image locally

```bash
BUILDTAG=mocknet BRANCH=mocknet make docker-gitlab-build
```

## Start genesis node

In order to run a private chain , you will have to genesis, and choose one of the machine as your genesis machine.

I assume you have 4 linux machine , their ip addresses are

```bash
192.168.0.1
192.168.0.2
192.168.0.3
192.168.0.4
```

Let's use `192.168.0.1` as the genesis node, run the following command in ~/thornode folder

```bash
BLOCK_TIME=2m make -C build/docker reset-mocknet-standalone
```

This will start the node as genesis node , also run Bitcoin / Bitcoin Cash / Litecoin / Mock Binance / ETH chain in test mode

## Start validator nodes

Create a seed environment variable , with all the node's ip address , like the following, so thornode will use each other as seeds

```bash
export SEEDS=192.168.0.1,192.168.0.2,192.168.0.3,192.168.0.4
```

Start validator node

```bash
ETH_HOST=http://192.168.0.1:8545 SEEDS=$SEEDS PEER=192.168.0.1 BINANCE_HOST=http://192.168.0.1:26660 BTC_HOST=192.168.0.1:18443 BCH_HOST=192.168.0.1:28443 LTC_HOST=192.168.0.1:38443 make -C build/docker reset-mocknet-validator
```

Now you will have a private chain (MockNet) working , the validator node will automatically bond itself , set ip address , and node keys etc etc.

## Create some pools, and add some fund

Here is an example to create some pool on BNB chain . Since we are running a mock binance node , which allow us to create asset from thin air.

If you want to run the following script from your local machine , make sure you have `thornode` binary available on your PATH

```bash
#!/bin/sh
set -ex

if [ -z $1 ]; then
    echo "Missing mock binance address (address:port)"
    exit 1
fi
for i in $(seq 1 1 1)
do
SIGNER_PASSWD=password
SIGNER_NAME="whatever$i"
THOR_ADDRESS=$(printf "$SIGNER_PASSWD\n" | thornode keys show $SIGNER_NAME --keyring-backend=file --output json | jq -r '.address')
if [ -z $THOR_ADDRESS ]; then
    printf "$SIGNER_PASSWD\n$SIGNER_PASSWD\n" | thornode keys add $SIGNER_NAME --keyring-backend=file
fi
THOR_ADDRESS=$(printf "$SIGNER_PASSWD\n" | thornode keys show $SIGNER_NAME --keyring-backend=file --output json | jq -r '.address')
PUBKEY=$(printf "$SIGNER_PASSWD\n" | thornode keys show $SIGNER_NAME --keyring-backend=file --output json | jq -r '.pubkey')
BNB_ADDRESS=$(NET=testnet pubkey2address -p $PUBKEY | grep tbnb | awk '{ print $NF }')
echo $BNB_ADDRESS
POOL_ADDRESS=$(curl -s $1:1317/thorchain/inbound_addresses | jq -r '.[]|select(.chain=="BNB") .address')

curl -v -s -X POST -d "[{
  \"from\": \"tbnb1lltanv67yztkpt5czw4ajsmg94dlqnnhrq7zqm\",
  \"to\": \"$POOL_ADDRESS\",
  \"coins\":[
      {\"denom\": \"RUNE-67C\", \"amount\": 1000000000000000}
  ],
  \"memo\": \"switch:$THOR_ADDRESS\"
}]" $1:26660/broadcast/easy

sleep 10s

# add BNB.BNB
# send RUNE
printf "$SIGNER_PASSWD\n$SIGNER_PASSWD\n" | thornode tx thorchain deposit 200000000000000 rune add:BNB.BNB:$BNB_ADDRESS --chain-id thorchain --node tcp://$1:26657 --from $SIGNER_NAME --keyring-backend=file --yes
sleep 5s

# add BNB-LOK-3C0
printf "$SIGNER_PASSWD\n$SIGNER_PASSWD\n" | thornode tx thorchain deposit 50000000000 rune add:BNB.LOK-3C0:$BNB_ADDRESS --chain-id thorchain --node tcp://$1:26657 --from $SIGNER_NAME --keyring-backend=file --yes
sleep 5s

# add BTCB-101
printf "$SIGNER_PASSWD\n$SIGNER_PASSWD\n" | thornode tx thorchain deposit 150000000000 rune add:BNB.BTCB-101:$BNB_ADDRESS --chain-id thorchain --node tcp://$1:26657 --from $SIGNER_NAME --keyring-backend=file --yes
sleep 5s

# send in BNB
curl -vvv -s -X POST -d "[{
  \"from\": \"$BNB_ADDRESS\",
  \"to\": \"$POOL_ADDRESS\",
  \"coins\":[
      {\"denom\": \"BNB\", \"amount\": 100000000000}
  ],
  \"memo\": \"add:BNB.BNB:$THOR_ADDRESS\"
}]" $1:26660/broadcast/easy

# send in LOK
curl -vvv -s -X POST -d "[{
  \"from\": \"$BNB_ADDRESS\",
  \"to\": \"$POOL_ADDRESS\",
  \"coins\":[
      {\"denom\": \"LOK-3C0\", \"amount\": 40000000000}
  ],
  \"memo\": \"add:BNB.LOK-3C0:$THOR_ADDRESS\"
}]" $1:26660/broadcast/easy

# send in BTCB-101
curl -vvv -s -X POST -d "[{
  \"from\": \"$BNB_ADDRESS\",
  \"to\": \"$POOL_ADDRESS\",
  \"coins\":[
      {\"denom\": \"BTCB-101\", \"amount\": 40000000000}
  ],
  \"memo\": \"add:BNB.BTCB-101:$THOR_ADDRESS\"
}]" $1:26660/broadcast/easy

done
```

## How to check logs?

- thornode - `docker logs -f thornode`
- bifrost - `docker logs -f bifrost`
