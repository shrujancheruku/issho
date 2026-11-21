#!/usr/bin/env bash
# Tests scripts/dev-host.sh with stand-ins for gcloud and curl: it prints the
# address Cloudflare holds for the API's name, sends the token to Cloudflare
# without putting it in an argument, and fails when Cloudflare says no.
# Usage: scripts/dev-host.test.sh
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
script="$repo/scripts/dev-host.sh"
fail=0
bad() { echo "FAIL $*"; fail=1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir "$work/bin"

# gcloud: the stored token, with the newline it really ends in.
cat > "$work/bin/gcloud" <<'STUB'
#!/bin/sh
echo "gcloud $*" >> "$LOG"
printf 'token-value\n'
STUB
# curl: Cloudflare's two answers, chosen by the URL (the last argument that looks like one).
# The header it was given arrives in --config, which it reads, as curl does.
cat > "$work/bin/curl" <<'STUB'
#!/bin/bash
echo "curl $*" >> "$LOG"
cfg=""; url=""
while [ $# -gt 0 ]; do
  case "$1" in
    --config) cfg=$(cat "$2"); shift ;;
    http*) url=$1 ;;
  esac
  shift
done
echo "header: $cfg" >> "$LOG.headers"
zones=${ZONES:-'{"success":true,"result":[{"id":"zone1"}]}'}
records=${RECORDS:-'{"success":true,"result":[{"content":"203.0.113.7"}]}'}
case "$url" in
  */zones\?name=joinissho.com) echo "$zones" ;;
  */zones/zone1/dns_records*) echo "$records" ;;
  *) echo '{"success":false}'; exit 22 ;;
esac
STUB
chmod +x "$work/bin/gcloud" "$work/bin/curl"

run() { : > "$work/log"; : > "$work/log.headers"; PATH="$work/bin:$PATH" LOG="$work/log" "$script" 2> "$work/err"; }

[ "$(run)" = "203.0.113.7" ] || bad "did not print the address"
grep -q 'name=api-dev.joinissho.com' "$work/log" || bad "did not look up the API's name"
grep -q 'Bearer token-value' "$work/log.headers" || bad "did not send the token, without its newline, as a bearer"
grep -q 'token-value' "$work/log" && bad "the token is in a command's arguments"

if RECORDS='{"success":true,"result":[]}' run >/dev/null; then bad "succeeded with no DNS record"; fi
if ZONES='{"success":true,"result":[]}' run >/dev/null; then bad "succeeded with no zone"; fi
if RECORDS='{"success":true,"result":[{"content":"not an address"}]}' run >/dev/null; then bad "accepted something that is not an address"; fi

[ "$fail" -eq 0 ] && echo "dev-host: ok"
exit "$fail"
