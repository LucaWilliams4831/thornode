#!/bin/bash
set -euo pipefail

echo "Linting handlers file"

get_instances() {
  PATTERN="$1"
  shift
  # Files with no matches cause grep to return error code.
  grep --no-filename -E -o "$PATTERN" "$@" || true
}

handlers=$(find x/thorchain/ -name "handler_*.go")

for f in $handlers; do
  if [[ $f == *"_test"* ]]; then
    continue
  fi
  if [[ $f == *"_archive"* ]]; then
    continue
  fi
  base=${f%.*}
  archive="$base"_archive.go
  files="$f"
  if [[ -f $archive ]]; then
    files+=" $archive"
  fi

  # trunk-ignore(shellcheck/SC2086): word split is desired behavior
  validate_init=$(get_instances " validateV[0-9]+" $files)
  validate_call=$(get_instances "\.validateV[0-9]+" "$f")

  # trunk-ignore(shellcheck/SC2086): word split is desired behavior
  handler_init=$(get_instances " handleV[0-9]+" $files)
  handler_call=$(get_instances "\.handleV[0-9]+" "$f")

  missing=$(echo -e "$validate_init\n$validate_call\n$handler_init\n$handler_call" | sed -e 's/^.//g' | sort -n | uniq -u)

  if [[ -n $missing ]]; then
    echo "Handler: $f... Failed"
    echo "Detected but not used:"
    echo "$missing"
    exit 1
  fi
  echo "Handler: $f... OK"
done

# Check that no change has removed a handler registration.
REMOVED=$(git diff origin/develop | grep -E '^\-\s*Register\(\s*"[0-9\.]+' || true)

if [[ -n $REMOVED ]]; then
  cat <<EOF
Error: Handler registrations were removed:
$REMOVED
EOF
  exit 1
fi
