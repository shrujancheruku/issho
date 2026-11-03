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
check data "backend=true codegen=true image=true proto=false ts=true ts_filter=" data/activities.json
# Cross-tree reads (review round 1): each must keep its reader's job on.
check embedded-markdown "$go" backend/internal/translate/guide.md
check supabase-templates "backend=true codegen=false image=false proto=false ts=false ts_filter=" supabase/templates/source.json
check supabase-config "backend=true codegen=false image=false proto=false ts=false ts_filter=" supabase/config.toml
check supabase-other "$none" supabase/seed.sql
check api-client-gen "backend=false codegen=true image=false proto=false ts=true ts_filter=--affected" packages/api-client/src/gen/issho/v1/x_pb.ts
check api-client-package "backend=false codegen=true image=false proto=false ts=true ts_filter=--affected" packages/api-client/package.json
check go-mod "backend=true codegen=true image=true proto=true ts=false ts_filter=" backend/go.mod
check app-readme "$none" apps/mobile/README.md
# A rename shows up as two paths when the diff is run with --no-renames.
check rename "backend=true codegen=true image=true proto=false ts=true ts_filter=--affected" backend/internal/a.go apps/admin/src/b.ts
ts="backend=false codegen=false image=false proto=false ts=true ts_filter=--affected"
check admin-only "$ts" apps/admin/src/main.tsx
check mobile-only "$ts" apps/mobile/src/screens/Home.tsx
check api-client "$ts" packages/api-client/src/index.ts
check mixed-docs-and-admin "$ts" docs/ROADMAP.md apps/admin/src/a.ts
check mobile-i18n "backend=true codegen=true image=false proto=false ts=true ts_filter=--affected" apps/mobile/src/i18n/en.json
check generated-docs "backend=false codegen=true image=false proto=false ts=false ts_filter=" docs/generated/api.md
all="backend=true codegen=true image=true proto=true ts=true ts_filter="
check proto "$all" proto/issho/v1/x.proto
check workflow "$all" .github/workflows/ci.yml
check lockfile "$all" pnpm-lock.yaml
check makefile "$all" Makefile
check scripts "$all" scripts/db-checks.sh
check unknown "$all" something-new.toml
check admin-and-lockfile "$all" apps/admin/src/a.ts pnpm-lock.yaml

# The workflow diffs with --no-renames: moving a Go file into docs/ must still
# count as touching backend/ (a plain rename diff lists only the new path).
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
(
  cd "$tmp"
  git init -q .
  mkdir -p backend docs
  seq 1 50 > backend/a.go
  git add . && git -c user.name=t -c user.email=t@t commit -qm one
  git mv backend/a.go docs/a.txt
  git -c user.name=t -c user.email=t@t commit -qm two
  git diff --no-renames --name-only HEAD~1 HEAD
) | { ./ci-changes.sh | grep -qx 'backend=true'; } || { echo "FAIL rename via git diff"; fail=1; }

all_out=$(./ci-changes.sh --all </dev/null | tr '\n' ' ' | sed 's/ $//')
[[ "$all_out" == "$all" ]] || { echo "FAIL --all: $all_out"; fail=1; }

[[ $fail == 0 ]] && echo "ci-changes: all passed"
exit $fail
