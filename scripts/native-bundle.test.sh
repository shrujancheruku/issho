#!/usr/bin/env bash
# Proves native-bundle.sh fails on an import that resolves only on the web. The
# fixture route is copied into the app's routes for this run and removed on exit,
# even on failure, so the working tree is left as it was.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FIXTURE="$ROOT/apps/app/scripts/fixtures/native-bundle"
ROUTE="$ROOT/apps/app/src/app/__native_bundle_fixture.tsx"

trap 'rm -f "$ROUTE"' EXIT
cp "$FIXTURE/route.tsx" "$ROUTE"

# One run per platform: a run of both stops at the first failure, so iOS failing
# would hide whether Android does.
for platform in ios android; do
  if LOG="$("$ROOT/scripts/native-bundle.sh" "$platform" 2>&1)"; then
    echo "native-bundle.sh $platform passed with a web-only import in the app; it must fail" >&2
    exit 1
  fi

  # Match the resolver's own message for the fixture's import. A typo'd path would
  # also fail, with "None of these files exist" for a path that is not web-only.
  if ! grep -q "Unable to resolve module .*fixtures/native-bundle/web-only" <<<"$LOG"; then
    echo "native-bundle.sh $platform failed, but not on the web-only import:" >&2
    echo "$LOG" | tail -20 >&2
    exit 1
  fi
done

echo "native-bundle.test.sh: passed (a web-only import fails the iOS and the Android bundle)"
