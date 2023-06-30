#!/bin/bash

echo "Linting managers.go file"

inited=$(grep "return new" x/thorchain/managers.go | awk '{print $2}' | awk -F "(" '{print $1}')
created=$(grep --exclude "*_test.go" "func new" x/thorchain/manager_* | awk '{print $2}' | awk -F "(" '{print $1}')
missing=$(echo -e "$inited\n$created" | grep -Ev 'Dummy|Helper|newStoreMgr' | sort -n | uniq -u)
echo "$missing"

[ -z "$missing" ] && echo "OK" && exit 0

[[ -n $missing ]] && echo "Not OK" && exit 1
