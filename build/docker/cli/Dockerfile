FROM registry.gitlab.com/thorchain/devops/node-launcher:bitcoin-daemon-22.0 as bitcoin
FROM registry.gitlab.com/thorchain/devops/node-launcher:bitcoin-cash-daemon-24.0.0 as bitcoin-cash
FROM registry.gitlab.com/thorchain/devops/node-launcher:dogecoin-daemon-1.14.5 as dogecoin
FROM registry.gitlab.com/thorchain/devops/node-launcher:litecoin-daemon-0.18.1 as litecoin

FROM golang:1.20.3 AS build

RUN apt-get update && \
  apt-get install --no-install-recommends -y protobuf-compiler=3.12.4-1 && \
  rm -rf /var/lib/apt/lists/*

# build geth since the image is alpine based
RUN go install github.com/ethereum/go-ethereum/cmd/geth@v1.11.5

# copy chain clis
COPY --from=bitcoin /usr/local/bin/bitcoin-cli /usr/local/bin/bitcoin-cli
COPY --from=bitcoin-cash /usr/local/bin/bitcoin-cli /usr/local/bin/bitcoin-cash-cli
COPY --from=dogecoin /usr/local/bin/dogecoin-cli /usr/local/bin/dogecoin-cli
COPY --from=litecoin /usr/local/bin/litecoin-cli /usr/local/bin/litecoin-cli

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TAG=mocknet
RUN make protob install
