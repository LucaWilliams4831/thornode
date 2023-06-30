########################################################################################
# Environment Checks
########################################################################################

CHECK_ENV:=$(shell ./scripts/check-env.sh)
ifneq ($(CHECK_ENV),)
$(error Check environment dependencies.)
endif

########################################################################################
# Config
########################################################################################

.PHONY: build test tools export healthcheck run-mocknet build-mocknet stop-mocknet halt-mocknet ps-mocknet reset-mocknet logs-mocknet openapi

# compiler flags
NOW=$(shell date +'%Y-%m-%d_%T')
COMMIT:=$(shell git log -1 --format='%H')
VERSION:=$(shell cat version)
TAG?=testnet
ldflags = -X gitlab.com/thorchain/thornode/constants.Version=$(VERSION) \
		  -X gitlab.com/thorchain/thornode/constants.GitCommit=$(COMMIT) \
		  -X gitlab.com/thorchain/thornode/constants.BuildTime=${NOW} \
		  -X github.com/cosmos/cosmos-sdk/version.Name=THORChain \
		  -X github.com/cosmos/cosmos-sdk/version.AppName=thornode \
		  -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
		  -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
		  -X github.com/cosmos/cosmos-sdk/version.BuildTags=$(TAG)

# golang settings
TEST_DIR?="./..."
BUILD_FLAGS := -ldflags '$(ldflags)' -tags ${TAG}
TEST_BUILD_FLAGS := -parallel=1 -tags=mocknet
GOBIN?=${GOPATH}/bin
BINARIES=./cmd/thornode ./cmd/bifrost ./tools/generate
BIFROST_UTXO_CLIENT_PKGS := ./bifrost/pkg/chainclients/dogecoin/... \
		  ./bifrost/pkg/chainclients/bitcoin/... \
		  ./bifrost/pkg/chainclients/bitcoincash/... \
		  ./bifrost/pkg/chainclients/litecoin/...

# docker tty args are disabled in CI
ifndef CI
DOCKER_TTY_ARGS=-it
endif

# pull branch name from CI if unset and available
ifdef CI_COMMIT_BRANCH
	BRANCH?=$(shell echo ${CI_COMMIT_BRANCH})
	BUILDTAG?=$(shell echo ${CI_COMMIT_BRANCH})
endif

# image build settings
BRANCH?=$(shell git rev-parse --abbrev-ref HEAD)
GITREF=$(shell git rev-parse --short HEAD)
BUILDTAG?=$(shell git rev-parse --abbrev-ref HEAD)

########################################################################################
# Targets
########################################################################################

# ------------------------------ Generate ------------------------------

SMOKE_PROTO_DIR=test/smoke/thornode_proto

protob:
	@./scripts/protocgen.sh

protob-docker:
	@docker run --rm -v $(shell pwd):/app -w /app \
		registry.gitlab.com/thorchain/thornode:builder-v4@sha256:a58b06a98485bcef78d7733cc6d66e8c62a306b1ec388a032469c592c5a71841 \
		make protob

openapi:
	@docker run --rm \
		--user $(shell id -u):$(shell id -g) \
		-v $$PWD/openapi:/mnt \
		openapitools/openapi-generator-cli:v6.0.0@sha256:310bd0353c11863c0e51e5cb46035c9e0778d4b9c6fe6a7fc8307b3b41997a35 \
		generate -i /mnt/openapi.yaml -g go -o /mnt/gen
	@rm openapi/gen/go.mod openapi/gen/go.sum

# ------------------------------ Build ------------------------------

build:
	go build ${BUILD_FLAGS} ${BINARIES}

install:
	go install ${BUILD_FLAGS} ${BINARIES}

tools:
	go install -tags ${TAG} ./tools/generate
	go install -tags ${TAG} ./tools/pubkey2address

# ------------------------------ Housekeeping ------------------------------

format:
	@git ls-files '*.go' | grep -v -e '^docs/' | xargs gofumpt -w

lint:
	@./scripts/lint.sh
	@go run tools/analyze/main.go ./common/... ./constants/... ./x/...
	@./scripts/trunk check --no-fix --upstream origin/develop

lint-ci:
	@./scripts/lint.sh
	@go run tools/analyze/main.go ./common/... ./constants/... ./x/...
	@./scripts/trunk check --all --ci -j8
	@./scripts/lint-versions.bash

# ------------------------------ Testing ------------------------------

test-coverage:
	@go test ${TEST_BUILD_FLAGS} -v -coverprofile=coverage.txt -covermode count ${TEST_DIR}
	sed -i '/\.pb\.go:/d' coverage.txt

coverage-report: test-coverage
	@go tool cover -html=coverage.txt

test-coverage-sum:
	@go run gotest.tools/gotestsum --junitfile report.xml --format testname -- ${TEST_BUILD_FLAGS} -v -coverprofile=coverage.txt -covermode count ${TEST_DIR}
	sed -i '/\.pb\.go:/d' coverage.txt
	@GOFLAGS='${TEST_BUILD_FLAGS}' go run github.com/boumenot/gocover-cobertura < coverage.txt > coverage.xml
	@go tool cover -func=coverage.txt
	@go tool cover -html=coverage.txt -o coverage.html

test:
	@CGO_ENABLED=0 go test ${TEST_BUILD_FLAGS} ${TEST_DIR}
	# network specific tests
	@CGO_ENABLED=0 go test -tags stagenet ./common
	@CGO_ENABLED=0 go test -tags testnet ./common ${BIFROST_UTXO_CLIENT_PKGS}
	@CGO_ENABLED=0 go test -tags mainnet ./common ${BIFROST_UTXO_CLIENT_PKGS}

test-race:
	@go test -race ${TEST_BUILD_FLAGS} ${TEST_DIR}

# ------------------------------ Test Regressions ------------------------------

test-regression:
	@DOCKER_BUILDKIT=1 docker build -t thornode-regtest -f ci/Dockerfile.regtest .
	@docker run --rm ${DOCKER_TTY_ARGS} \
		-e DEBUG -e RUN -e EXPORT -e TIME_FACTOR -e PARALLELISM \
		-e UID=$(shell id -u) -e GID=$(shell id -g) \
		-p 1317:1317 -p 26657:26657 \
		-v $(shell pwd)/test/regression/mnt:/mnt \
		-v $(shell pwd)/test/regression/suites:/app/test/regression/suites \
		-v $(shell pwd)/test/regression/templates:/app/test/regression/templates \
		-w /app thornode-regtest sh -c 'make _test-regression'

test-regression-coverage:
	@go tool cover -html=test/regression/mnt/coverage/coverage.txt

# internal target used in docker build
_build-test-regression:
	@go install -ldflags '$(ldflags)' -tags=mocknet,regtest ./cmd/thornode
	@go build -ldflags '$(ldflags)' -cover -tags=mocknet,regtest -o /regtest/cover-thornode ./cmd/thornode
	@go build -ldflags '$(ldflags)' -tags mocknet -o /regtest/regtest ./test/regression/cmd

# internal target used in test run
_test-regression:
	@rm -rf /mnt/coverage && mkdir -p /mnt/coverage
	@cd test/regression && /regtest/regtest
	@go tool covdata textfmt -i /mnt/coverage -o /mnt/coverage/coverage.txt
	@grep -v -E -e archive.go -e 'v[0-9]+.go' -e openapi/gen /mnt/coverage/coverage.txt > /mnt/coverage/coverage-filtered.txt
	@go tool cover -func /mnt/coverage/coverage-filtered.txt > /mnt/coverage/func-coverage.txt
	@awk '/^total:/ {print "Regression Coverage: " $$3}' /mnt/coverage/func-coverage.txt
	@chown -R ${UID}:${GID} /mnt

# ------------------------------ Test Sync ------------------------------

test-sync-mainnet:
	@BUILDTAG=mainnet BRANCH=mainnet $(MAKE) docker-gitlab-build
	@docker run --rm -e CHAIN_ID=thorchain-mainnet-v1 -e NET=mainnet registry.gitlab.com/thorchain/thornode:mainnet

test-sync-stagenet:
	@BUILDTAG=stagenet BRANCH=stagenet $(MAKE) docker-gitlab-build
	@docker run --rm -e CHAIN_ID=thorchain-stagenet-v2 -e NET=stagenet registry.gitlab.com/thorchain/thornode:stagenet

test-sync-testnet:
	@BUILDTAG=testnet BRANCH=testnet $(MAKE) docker-gitlab-build
	@docker run --rm -e CHAIN_ID=thorchain-testnet-v2 -e NET=testnet registry.gitlab.com/thorchain/thornode:testnet

# ------------------------------ Docker Build ------------------------------

docker-gitlab-login:
	docker login -u ${CI_REGISTRY_USER} -p ${CI_REGISTRY_PASSWORD} ${CI_REGISTRY}

docker-gitlab-push:
	./build/docker/semver_tags.sh registry.gitlab.com/thorchain/thornode ${BRANCH} $(shell cat version) \
		| xargs -n1 | grep registry | xargs -n1 docker push
	docker push registry.gitlab.com/thorchain/thornode:${GITREF}

docker-gitlab-build:
	docker build -f build/docker/Dockerfile \
		$(shell sh ./build/docker/semver_tags.sh registry.gitlab.com/thorchain/thornode ${BRANCH} $(shell cat version)) \
		-t registry.gitlab.com/thorchain/thornode:${GITREF} --build-arg TAG=${BUILDTAG} .


# ------------------------------ Single Node Mocknet ------------------------------

cli-mocknet:
	@docker compose -f build/docker/docker-compose.yml run --rm cli

run-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard up -d

stop-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard down -v

# Halt the Mocknet without erasing the blockchain history, so it can be resumed later.
halt-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard down

build-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard build

bootstrap-mocknet: $(SMOKE_PROTO_DIR)
	@docker run ${SMOKE_DOCKER_OPTS} \
		registry.gitlab.com/thorchain/thornode:smoke \
		python scripts/smoke.py --bootstrap-only=True

ps-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard images
	@docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard ps

logs-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet logs -f thornode bifrost

reset-mocknet: stop-mocknet run-mocknet

# ------------------------------ Mocknet EVM Fork ------------------------------

reset-mocknet-fork-%: stop-mocknet
	@./tools/evm/run-mocknet-fork.sh $*

# ------------------------------ Multi Node Mocknet ------------------------------

run-mocknet-cluster:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard up -d

stop-mocknet-cluster:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard down -v

halt-mocknet-cluster:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard down

build-mocknet-cluster:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard build

ps-mocknet-cluster:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard images
	@docker compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard ps

reset-mocknet-cluster: stop-mocknet-cluster build-mocknet-cluster run-mocknet-cluster
