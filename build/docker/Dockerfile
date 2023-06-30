#
# THORChain
#

#
# Build
#
FROM registry.gitlab.com/thorchain/thornode:builder-v4@sha256:a58b06a98485bcef78d7733cc6d66e8c62a306b1ec388a032469c592c5a71841 AS build

ENV GOBIN=/go/bin
ENV GOPATH=/go
ENV CGO_ENABLED=0
ENV GOOS=linux

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TAG=mainnet
RUN make protob install

#
# Main
#
FROM registry.gitlab.com/thorchain/thornode:runner-base-v1@sha256:8fc433efa3e853b59cafe920654705e37c94228ae99cba6892933a61594c11f3

# Copy the compiled binaries over.
COPY --from=build /go/bin/generate /go/bin/thornode /go/bin/bifrost /usr/bin/

COPY build/scripts /scripts

# default to mainnet
ARG TAG=mainnet
ENV NET=$TAG

# default to fullnode
CMD ["/scripts/fullnode.sh"]
