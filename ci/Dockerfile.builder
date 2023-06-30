FROM golang:1.20.3


# hadolint ignore=DL3008,DL4006
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    curl git jq make protobuf-compiler xz-utils sudo python3-pip \
    && rm -rf /var/cache/apt/lists \
    && go install mvdan.cc/gofumpt@v0.3.0
