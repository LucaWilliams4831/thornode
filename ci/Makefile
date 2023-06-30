
help: ## Help message
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

builder: ## Build the CI builder image
	docker build -t registry.gitlab.com/thorchain/thornode:builder-v4 -f Dockerfile.builder .

runner: ## Build the base runner image
	docker build -t registry.gitlab.com/thorchain/thornode:runner-base-v1 -f Dockerfile.runner .

push-builder: ## Push the CI builder image
	docker push registry.gitlab.com/thorchain/thornode:builder-v4

push-runner: ## Push the base runner image
	docker push registry.gitlab.com/thorchain/thornode:runner-base-v1
