#!/usr/bin/env bash
# Proves native-metro.sh's `wait` is bounded: it returns 0 once the log shows a
# bundle load and fails (with the log's last lines) instead of waiting forever
# when none comes, that `stop` removes the pidfile and stops only the named
# emulator and simulator, and that the script refuses an unknown command. Metro
# and the devices themselves are checked by hand (see apps/app/README.md).
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

# stop, with stand-ins for android, xcrun and lsof first on PATH: they record
# their arguments and touch no device. Another emulator or simulator on the
# machine (a person's own, or another worktree's) must not be named.
BIN="$STATE/bin"; CALLS="$STATE/calls"
mkdir -p "$BIN"; : > "$CALLS"
for tool in android xcrun lsof; do
  printf '#!/bin/sh\necho "%s $*" >> "%s"\n' "$tool" "$CALLS" > "$BIN/$tool"
  chmod +x "$BIN/$tool"
done
sleep 300 & metro=$!
echo "$metro" > "$STATE/metro.pid"
PATH="$BIN:$PATH" NATIVE_AVD=test-avd NATIVE_IOS_DEVICE="Test Phone" "$SCRIPT" stop >/dev/null \
  || fail "stop failed"
[ ! -e "$STATE/metro.pid" ] || fail "stop left the pidfile"
sleep 0.2
if kill -0 "$metro" 2>/dev/null; then kill "$metro"; fail "stop left Metro's process running"; fi
grep -qx "android emulator stop test-avd" "$CALLS" || fail "stop did not stop NATIVE_AVD: $(cat "$CALLS")"
grep -qx "xcrun simctl shutdown Test Phone" "$CALLS" || fail "stop did not shut down NATIVE_IOS_DEVICE: $(cat "$CALLS")"
if grep -Eq "simctl shutdown all|emulator stop -all|shutdown booted" "$CALLS"; then fail "stop named every device: $(cat "$CALLS")"; fi
[ "$(grep -c '^android\|^xcrun' "$CALLS")" -eq 2 ] || fail "stop made calls beyond the two named devices: $(cat "$CALLS")"

echo "native-metro.test.sh: passed (wait is bounded; stop spares other devices; bad commands are refused)"
