#!/bin/bash

set -euo pipefail

# This script compares versioned functions to the develop branch to ensure no logic
# changes in historical versions that would cause consensus failure.

VERSION=$(awk -F. '{ print $2 }' version)
CI_MERGE_REQUEST_TITLE=${CI_MERGE_REQUEST_TITLE:-}

go run tools/versioned-functions/main.go --version="$VERSION" >/tmp/versioned-fns-current

git fetch https://gitlab.com/thorchain/thornode.git develop
git checkout FETCH_HEAD

git checkout - -- tools scripts
go run tools/versioned-functions/main.go --version="$VERSION" >/tmp/versioned-fns-develop
git checkout -

gofumpt -w /tmp/versioned-fns-develop /tmp/versioned-fns-current

if ! diff -u -F '^func' -I '^//' --color=always /tmp/versioned-fns-develop /tmp/versioned-fns-current; then
  echo "Detected change in versioned function."
  if [[ $CI_MERGE_REQUEST_TITLE == *"#check-lint-warning"* ]]; then
    echo "Merge request is marked unsafe."
  else
    echo 'Correct the change, add a new versioned function, or add "#check-lint-warning" to the PR description.'
    exit 1
  fi
fi
