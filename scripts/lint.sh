#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "ERR: $*"
  exit 1
}

# check docs version
version=$(cat version)
if ! grep "^  version: ${version}" openapi/openapi.yaml; then
  die "docs version (openapi/openapi.yaml) does not match version file ${version}"
fi

# Check that no .pb.go files were added.
if git ls-files '*.go' | grep -q '.pb.go$'; then
  die "Do not add generated protobuf .pb.go files"
fi

if [ -n "$(git ls-files '*.go' | grep -v -e '^docs/' | xargs gofumpt -l 2>/dev/null)" ]; then
  git ls-files '*.go' | grep -v -e '^docs/' | xargs gofumpt -d 2>/dev/null
  die "Go formatting errors"
fi
go mod verify

./scripts/lint-handlers.bash

./scripts/lint-managers.bash

./scripts/lint-erc20s.bash
