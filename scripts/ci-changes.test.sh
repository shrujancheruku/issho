#!/usr/bin/env bash
# Tests for ci-changes.sh. Run: scripts/ci-changes.test.sh
set -euo pipefail
cd "$(dirname "$0")"

fail=0
# check <name> <expected output on one line> <paths...>
check() {
  local name=$1 want=$2; shift 2
  local got
  got=$( { [[ $# -gt 0 ]] && printf '%s\n' "$@"; true; } | ./ci-changes.sh | tr '\n' ' ' | sed 's/ $//')
  if [[ "$got" != "$want" ]]; then
    echo "FAIL $name"; echo "  want: $want"; echo "  got:  $got"; fail=1
  fi
}

none="backend=false codegen=false image=false proto=false ts=false ts_filter="
check docs-only "$none" docs/ROADMAP.md README.md
check no-files "$none"
check claude-and-site "$none" .claude/orchestration.md site/index.html config/deploy.yml
go="backend=true codegen=true image=true proto=false ts=false ts_filter="
check go-only "$go" backend/internal/walks/walks.go
check data "$go" data/activities.json
ts="backend=false codegen=false image=false proto=false ts=true ts_filter=--affected"
check admin-only "$ts" apps/admin/src/main.tsx
check mobile-only "$ts" apps/mobile/src/screens/Home.tsx
check api-client "$ts" packages/api-client/src/index.ts
check mixed-docs-and-admin "$ts" docs/ROADMAP.md apps/admin/src/a.ts
check mobile-i18n "backend=false codegen=true image=false proto=false ts=true ts_filter=--affected" apps/mobile/src/i18n/en.json
check generated-docs "backend=false codegen=true image=false proto=false ts=false ts_filter=" docs/generated/api.md
all="backend=true codegen=true image=true proto=true ts=true ts_filter="
check proto "$all" proto/issho/v1/x.proto
check workflow "$all" .github/workflows/ci.yml
check lockfile "$all" pnpm-lock.yaml
check makefile "$all" Makefile
check scripts "$all" scripts/db-checks.sh
check unknown "$all" something-new.toml
check admin-and-lockfile "$all" apps/admin/src/a.ts pnpm-lock.yaml

all_out=$(./ci-changes.sh --all </dev/null | tr '\n' ' ' | sed 's/ $//')
[[ "$all_out" == "$all" ]] || { echo "FAIL --all: $all_out"; fail=1; }

[[ $fail == 0 ]] && echo "ci-changes: all passed"
exit $fail
