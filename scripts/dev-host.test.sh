#!/usr/bin/env bash
# Tests scripts/dev-host.sh with a stand-in for gcloud: it asks Google Cloud for
# the reserved address issho-dev-ip, prints only that, and fails on anything
# that is not an address.
# Usage: scripts/dev-host.test.sh
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
script="$repo/scripts/dev-host.sh"
fail=0
bad() { echo "FAIL $*"; fail=1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir "$work/bin"

cat > "$work/bin/gcloud" <<'STUB'
#!/bin/sh
echo "gcloud $*" >> "$LOG"
[ -z "${FAIL:-}" ] || exit 1
printf '%s\r\n' "${ADDRESS-203.0.113.7}"
STUB
chmod +x "$work/bin/gcloud"

run() { : > "$work/log"; PATH="$work/bin:$PATH" LOG="$work/log" "$script" 2> "$work/err"; }

[ "$(run)" = "203.0.113.7" ] || bad "did not print the address"
grep -q 'compute addresses describe issho-dev-ip' "$work/log" || bad "did not ask for issho-dev-ip"
grep -q -- '--region asia-southeast1' "$work/log" || bad "did not name the region"
grep -q -- '--project issho-dev-512213' "$work/log" || bad "did not name the project"

if ADDRESS='' run >/dev/null 2>&1; then bad "succeeded with no address"; fi
if ADDRESS='not an address' run >/dev/null 2>&1; then bad "accepted something that is not an address"; fi
if ADDRESS='host.example.com' run >/dev/null 2>&1; then bad "accepted a host name"; fi
if FAIL=1 run >/dev/null 2>&1; then bad "succeeded although gcloud failed"; fi

[ "$fail" -eq 0 ] && echo "dev-host: ok"
exit "$fail"
