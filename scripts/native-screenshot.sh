#!/usr/bin/env bash
# Captures the running iOS simulator or Android emulator into
# apps/app/docs/screenshots/<name>-<platform>.png, the files a screen ticket
# attaches. Usage: scripts/native-screenshot.sh <ios|android> <name>
# The app must already be open on the device (make native-ios or native-android).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if [ "$#" -ne 2 ]; then
  echo "usage: scripts/native-screenshot.sh <ios|android> <name>" >&2
  exit 2
fi
PLATFORM="$1"
NAME="$2"

case "$PLATFORM" in
  ios|android) ;;
  *)
    echo "native-screenshot: platform must be ios or android, got '$PLATFORM'" >&2
    exit 2
    ;;
esac

# A name is a plain file stem: no path separators, so a screenshot always lands
# in docs/screenshots/.
if [[ ! "$NAME" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
  echo "native-screenshot: name must be lowercase letters, digits and hyphens, got '$NAME'" >&2
  exit 2
fi

OUT="$ROOT/apps/app/docs/screenshots/$NAME-$PLATFORM.png"
mkdir -p "$(dirname "$OUT")"

if [ "$PLATFORM" = ios ]; then
  # simctl needs full Xcode, not only the Command Line Tools. "booted" is the
  # simulator that `expo run:ios` opened; simctl fails if none is booted.
  if ! xcrun simctl help >/dev/null 2>&1; then
    echo "native-screenshot: iOS needs Xcode (xcrun simctl), not only the Command Line Tools" >&2
    exit 1
  fi
  xcrun simctl io booted screenshot --type=png "$OUT"
else
  # The android CLI, not adb, takes the capture; it picks the device on its own
  # when only one is connected (the emulator, for a local run).
  android screen capture --output="$OUT"
fi

echo "native-screenshot: wrote apps/app/docs/screenshots/$NAME-$PLATFORM.png"
