#!/usr/bin/env bash
# Writes Go coverage as a markdown table: one row per package, then the total.
# Usage: go-coverage-summary.sh <go test output> <coverprofile>   (run from backend/)
# Reported only, never gated (#192): this script cannot fail the job on a number.
set -uo pipefail
log=$1 profile=$2
echo "### Go coverage"
echo
echo "| Package | Statements |"
echo "| --- | --- |"
# `go test -cover` prints "ok<TAB>pkg<TAB>time<TAB>coverage: 12.1% of statements"
# (or "<TAB>pkg<TAB><TAB>coverage: 0.0%..." for a package without tests).
awk -F'\t' '/coverage: / {
  pkg = ($1 == "ok" || $1 == "FAIL" || $1 == "") ? $2 : $1
  sub(/^github.com\/shrujancheruku\/issho\/backend\//, "", pkg)
  pct = $0; sub(/.*coverage: /, "", pct); sub(/ of statements.*/, "", pct)
  printf "| %s | %s |\n", pkg, pct
}' "$log" | sort
echo
echo "**Total: $(go tool cover -func="$profile" | awk '/^total:/ {print $3}')** (statements; integration tests are excluded, they need the database)"
