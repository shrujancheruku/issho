#!/usr/bin/env bash
# Captures the running iOS simulator or Android emulator into
# apps/app/docs/screenshots/<name>-<platform>.png, for the screenshots a screen
# ticket attaches. Usage: scripts/native-screenshot.sh <ios|android> <name>
# The app must already be open on the device (make native-ios or native-android).
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: scripts/native-screenshot.sh <ios|android> <name>" >&2
  exit 2
fi
PLATFORM="$1"
NAME="$2"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="$ROOT/apps/app/docs/screenshots"
OUT="$OUT_DIR/$NAME-$PLATFORM.png"
mkdir -p "$OUT_DIR"

case "$PLATFORM" in
  ios)
    # simctl needs full Xcode, not only the Command Line Tools; `booted` is the
    # simulator that `expo run:ios` opened.
    if ! xcrun simctl help >/dev/null 2>&1; then
      echo "native-screenshot: iOS needs Xcode (xcrun simctl), not only the Command Line Tools" >&2
      exit 1
    fi
    xcrun simctl io booted screenshot "$OUT"
    ;;
  android)
    # exec-out streams the PNG without the CRLF rewriting that a plain shell
    # redirect of `adb shell screencap` would do.
    if ! command -v adb >/dev/null 2>&1; then
      echo "native-screenshot: adb not found; install the Android SDK platform-tools" >&2
      exit 1
    fi
    adb exec-out screencap -p >"$OUT"
    ;;
  *)
    echo "native-screenshot: platform must be ios or android, got '$PLATFORM'" >&2
    exit 2
    ;;
esac
echo "native-screenshot: wrote apps/app/docs/screenshots/$NAME-$PLATFORM.png"
