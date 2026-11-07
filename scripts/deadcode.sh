#!/usr/bin/env bash
# Dead-code check (#436): Go `deadcode -test` over backend/, then knip over the
# TypeScript workspaces. Run by `make deadcode` and by CI, so the two can't drift.
#
# REPORT-ONLY for now: findings are printed (and, in CI, added to the job
# summary) but the script exits 0. It starts gating once the phase 1 cuts are
# done (#433): then set DEADCODE_STRICT=1 in the CI step (or drop the default).
#
# Tools are pinned here and fetched on demand, so the check adds no dependency
# to go.mod or package.json. Bump them on purpose.
set -uo pipefail

DEADCODE_VERSION=v0.52.0 # golang.org/x/tools
KNIP_VERSION=6.44.1

# Parked parts (ADR 0002) are exempt by this named list: extended regexes
# matched against deadcode's `file:line:col: unreachable func: ...` lines.
# knip's own parked list is in knip.jsonc.
PARKED_GO=(
  'backend/internal/matching/sim/' # the matching simulator
  'backend/internal/tagging/'      # interest tagging
  'backend/internal/repoedit/'     # the editing side of repoedit (the read side is kept; narrow this once the read side is split out)
)

cd "$(dirname "$0")/.."
summary="${GITHUB_STEP_SUMMARY:-/dev/null}"
status=0

echo "== Go: deadcode -test $DEADCODE_VERSION =="
# deadcode prints paths relative to the working directory (backend/), so put
# the prefix back to match the list above. Default -filter is the main module.
go_out=$(cd backend && go run "golang.org/x/tools/cmd/deadcode@$DEADCODE_VERSION" -test ./...) || status=1
go_out=$(printf '%s\n' "$go_out" | sed -e '/^$/d' -e 's|^|backend/|')
for p in "${PARKED_GO[@]}"; do
  go_out=$(printf '%s\n' "$go_out" | grep -Ev -- "$p")
done
go_out=$(printf '%s\n' "$go_out" | sed -e '/^$/d')
[ -n "$go_out" ] && status=1
echo "${go_out:-none}"

echo
echo "== TypeScript: knip $KNIP_VERSION =="
ts_out=$(pnpm dlx "knip@$KNIP_VERSION" --no-progress 2>&1 | grep -v '^\.\.\.') || status=1
echo "${ts_out:-none}"

{
  echo "### Dead code (report-only)"
  echo '```'
  echo "Go:"; echo "${go_out:-none}"
  echo; echo "knip:"; echo "${ts_out:-none}"
  echo '```'
} >>"$summary"

if [ "$status" -ne 0 ]; then
  if [ "${DEADCODE_STRICT:-}" = 1 ]; then
    echo "dead code found (strict)"; exit 1
  fi
  echo "dead code found (report-only: not failing)"
fi
exit 0
