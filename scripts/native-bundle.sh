#!/usr/bin/env bash
# Bundles the member app for iOS and Android with `expo export`, which runs Metro
# with the native resolver. It fails on an import that resolves only on the web
# (a *.web.* file with no native twin), which the web build would hide. Nothing is
# kept: the bundles go to a temp directory that is removed on exit.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$(mktemp -d)"
trap 'rm -rf "$OUT"' EXIT

cd "$ROOT/apps/app"
for platform in ios android; do
  pnpm exec expo export --platform "$platform" --output-dir "$OUT/$platform"
done
echo "native-bundle: iOS and Android bundles built"
