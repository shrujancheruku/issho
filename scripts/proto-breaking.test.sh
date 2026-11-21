#!/usr/bin/env bash
# Tests for proto-breaking.sh in a throwaway git repo that holds copies of the
# real proto/ and buf.yaml. Run from anywhere: scripts/proto-breaking.test.sh
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
export BUF="$(cd "$repo" && go tool -n buf)"
script="$repo/scripts/proto-breaking.sh"
fail=0

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"
git init -q -b work
git config user.email test@example.test
git config user.name test
git config commit.gpgsign false
cp "$repo/buf.yaml" .
cp -R "$repo/proto" .

# run <ref>: the script's output in $out and its exit status in $status.
run() { status=0; out=$("$script" "$@" 2>&1) || status=$?; }
# expect <what> <status> <substring>
expect() {
  [ "$status" -eq "$2" ] || { echo "FAIL $1: exit $status, want $2:"; echo "$out"; fail=1; return; }
  grep -qF -- "$3" <<<"$out" || { echo "FAIL $1: output lacks '$3':"; echo "$out"; fail=1; }
}

# A ref that isn't fetched: not run, and said so.
run refs/remotes/origin/nothing
expect "absent ref" 0 "buf breaking not run"

# A main that still holds v1's proto (no access.proto): not run, and said so.
git add -A && git commit -qm "v2"
git branch v2main
git checkout -q -b v1main
git rm -qr proto && mkdir -p proto/issho/v1 && echo 'syntax = "proto3"; package issho.v1;' > proto/issho/v1/admin.proto
git add -A && git commit -qm "v1"
git checkout -q work
run v1main
expect "main with v1's proto" 0 "does not hold v2's proto"

# A main that holds v2's proto, and a change that keeps it compatible: runs and passes.
run v2main
expect "compatible change" 0 ""
! grep -qF "not run" <<<"$out" || { echo "FAIL compatible change: the check did not run:"; echo "$out"; fail=1; }

# The same main, and a change that removes a field: runs and fails.
sed -i.bak '/string version = 1;/d' proto/issho/v1/health.proto && rm proto/issho/v1/health.proto.bak
run v2main
expect "breaking change" 100 "version"

[ "$fail" -eq 0 ] && echo "proto-breaking: ok"
exit "$fail"
