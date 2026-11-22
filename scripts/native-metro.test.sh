#!/usr/bin/env bash
# Proves native-metro.sh's `wait` is bounded: it returns 0 once the log shows a
# bundle load and fails (with the log's last lines) instead of waiting forever
# when none comes, and the script refuses an unknown command. Metro and the
# devices themselves are checked by hand (see apps/app/README.md).
set -euo pipefail

SCRIPT="$(cd "$(dirname "$0")" && pwd)/native-metro.sh"
STATE="$(mktemp -d)"
trap 'rm -rf "$STATE"' EXIT
export NATIVE_METRO_STATE="$STATE" NATIVE_WAIT=1

fail() { echo "native-metro.test.sh: $1" >&2; exit 1; }

status=0; "$SCRIPT" nonsense >/dev/null 2>&1 || status=$?
[ "$status" -eq 2 ] || fail "unknown command: expected exit 2, got $status"

echo "Metro waiting on http://localhost:8081" > "$STATE/metro.log"
status=0; out="$("$SCRIPT" wait 2>&1)" || status=$?
[ "$status" -eq 1 ] || fail "wait with no bundle load: expected exit 1, got $status"
grep -q "Metro waiting" <<<"$out" || fail "wait's failure did not show the log's last lines"

echo "Android Bundled 796ms entry.js (1443 modules)" >> "$STATE/metro.log"
"$SCRIPT" wait >/dev/null || fail "wait did not return 0 after a bundle load"

echo "native-metro.test.sh: passed (wait is bounded; bad commands are refused)"
