#!/usr/bin/env bash
# Tests for ci-changes.sh. Run from anywhere: scripts/ci-changes.test.sh
set -euo pipefail

script=$(cd "$(dirname "$0")" && pwd)/ci-changes.sh
fail=0

# jobs <files…> prints the names of the jobs that run, space-separated, for a
# change to those files.
jobs() { printf '%s\n' "$@" | "$script" | sed -n 's/=true$//p' | tr '\n' ' ' | sed 's/ $//'; }
# want <what> <expected> <files…>
want() {
  local what=$1 expected=$2
  shift 2
  local got
  got=$(jobs "$@")
  [ "$got" = "$expected" ] || { echo "FAIL $what: ran '$got', want '$expected'"; fail=1; }
}

all='backend boundaries e2e image native-bundle policy proto-breaking retention rpc-auth strings typescript'

want "a change to no file runs nothing" "" 
want "an ADR runs nothing" "" docs/adr/0001-example.md
want "the README runs nothing" "" README.md
want "a spec document, which the policy lint reads" "backend boundaries policy retention" docs/spec/quality.md
want "a backend package" "backend boundaries e2e image policy retention rpc-auth" backend/internal/staff/staff.go
want "the app" "e2e native-bundle strings typescript" apps/app/src/app/index.tsx
want "the admin" "e2e typescript" apps/admin/src/HealthPage.tsx
want "a proto file" "backend boundaries e2e native-bundle policy proto-breaking retention rpc-auth typescript" proto/issho/v1/health.proto
want "the e2e package" "e2e typescript" e2e/tests/version.spec.ts
want "the Makefile runs every job" "$all" Makefile
want "the workflow runs every job" "$all" .github/workflows/ci.yml
want "another workflow runs nothing" "" .github/workflows/release-please.yml
# What CI itself depends on runs every job: a break in any of these would
# otherwise skip the jobs it breaks, and a skipped job reports as passing.
want "the setup action runs every job" "$all" .github/actions/setup/action.yml
want "another local action runs every job" "$all" .github/actions/other/action.yml
want "a CI script runs every job" "$all" scripts/ci-changes.sh
want "the pnpm lockfile runs every job" "$all" pnpm-lock.yaml
want "the pnpm workspace runs every job" "$all" pnpm-workspace.yaml
want "the root package.json runs every job" "$all" package.json
want "a tool pin runs every job" "$all" tools/go.mod
want "the tools' checksums run every job" "$all" tools/go.sum
want "the backend's go.sum runs every job" "$all" backend/go.sum
want "the backend's go.mod runs every job" "$all" backend/go.mod
want "the workspace's go.work.sum runs every job" "$all" go.work.sum
want "proto-breaking.sh runs its check and the backend job, which runs its tests" "backend proto-breaking" scripts/proto-breaking.sh
want "an app's own package.json is not CI's" "e2e native-bundle strings typescript" apps/app/package.json
want "two changes run both's jobs" "e2e native-bundle strings typescript" apps/app/a.ts apps/app/b.ts

# --all is a push to main, a dispatch, or a base that can't be diffed.
got=$(printf '' | "$script" --all | sed -n 's/=true$//p' | tr '\n' ' ' | sed 's/ $//')
[ "$got" = "$all" ] || { echo "FAIL --all: ran '$got'"; fail=1; }

# Every job is reported, true or false, so a workflow can read any of them.
count=$(printf 'README.md\n' | "$script" | wc -l | tr -d ' ')
[ "$count" -eq 11 ] || { echo "FAIL: $count lines for a docs change, want one per job (11)"; fail=1; }

# Nothing but --all is accepted.
if "$script" --bogus </dev/null >/dev/null 2>&1; then echo "FAIL: accepted an unknown argument"; fail=1; fi

[ "$fail" -eq 0 ] && echo "ci-changes: ok"
exit "$fail"
