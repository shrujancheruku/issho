#!/usr/bin/env bash
# Tests for tracker-check.sh against recorded gh JSON. Run: scripts/tracker-check.test.sh
set -euo pipefail
cd "$(dirname "$0")"

data=testdata/tracker-check
export TRACKER_CHECK_NOW=2026-11-07T00:00:00Z
fail=0

got=$(./tracker-check.sh "$data/issues.json" "$data/prs.json" || true)
status=0
./tracker-check.sh "$data/issues.json" "$data/prs.json" >/dev/null || status=$?

# want <substring>: the output has a line with it.
want() { grep -qF -- "$1" <<<"$got" || { echo "FAIL missing: $1"; fail=1; }; }
# wantnot <substring>: the output has no line with it.
wantnot() { ! grep -qF -- "$1" <<<"$got" || { echo "FAIL unexpected: $1"; fail=1; }; }

want "#2 [triage-label: 2"
want "#3 [triage-label: 0"
want "#4 [closed-with-triage-label: ready-for-agent]"
want "#6 [retired-roadmap"
want "#7 [retired-roadmap"
want "#8 [no-parent"
want "#9 [no-size"
want "#11 [pr-only-label: go]"
want "PR #101 [merged-pr-closes-no-issue]"

wantnot "#1 "     # clean, with a "Moved from ROADMAP" provenance line
wantnot "#5 "     # closed wontfix is allowed
wantnot "#10 "    # Pilot bug without a parent
wantnot "#12 "    # wayfinder is exempt from the triage-label rule
wantnot "#269 "   # Dependency Dashboard
wantnot "#360 "   # Ideas
wantnot "PR #100 "
wantnot "PR #102 " # outside the window
wantnot "PR #103 " # release-please
wantnot "PR #104 " # renovate

[ "$status" -eq 1 ] || { echo "FAIL exit status $status, want 1"; fail=1; }

# A clean tracker exits 0.
clean=$(mktemp)
trap 'rm -f "$clean"' EXIT
echo '[]' >"$clean"
./tracker-check.sh "$clean" "$clean" >/dev/null || { echo "FAIL clean input exited non-zero"; fail=1; }

[ "$fail" -eq 0 ] && echo "tracker-check tests passed"
exit "$fail"
