#!/usr/bin/env bash
# Proves native-screenshot.sh refuses bad arguments before it touches a device,
# so a typo can't write a file outside docs/screenshots/ or capture the wrong
# thing. The device capture itself is checked by hand (see apps/app/README.md).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="$ROOT/scripts/native-screenshot.sh"

expect_refusal() {
  local label="$1"; shift
  local status=0
  local log
  log="$("$SCRIPT" "$@" 2>&1)" || status=$?
  if [ "$status" -ne 2 ]; then
    echo "native-screenshot.test.sh: $label: expected exit 2, got $status" >&2
    echo "$log" >&2
    exit 1
  fi
}

expect_refusal "no arguments"
expect_refusal "one argument" ios
expect_refusal "unknown platform" web plans
expect_refusal "name with a path separator" ios "../plans"
expect_refusal "name with uppercase" ios "Plans"

echo "native-screenshot.test.sh: passed (bad arguments are refused before a device is touched)"
