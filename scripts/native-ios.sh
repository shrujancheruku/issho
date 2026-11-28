#!/usr/bin/env bash
# Builds and opens the app on the NATIVE_IOS_DEVICE simulator (make native-ios).
# Usage: scripts/native-ios.sh <device name>
#
# Xcode 27 replaced Simulator.app with Device Hub, inside Xcode.app. Expo CLI
# 58.1.21 knows that and tries `open -a DeviceHub` and the devices:// URL, but
# Launch Services does not know an app that lives inside Xcode.app until it is
# opened once by path, so both fail and Expo then reports "Simulator app did not
# open fast enough" after it has already built and installed the app. Opening
# Device Hub by path first makes Expo's own check find it. Drop this step when an
# Expo CLI release opens Device Hub by path (re-check when @expo/cli moves past
# 58.1.21). On Xcode 26 and earlier there is no Device Hub, so nothing happens.
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: scripts/native-ios.sh <device name>" >&2
  exit 2
fi

# `|| true`: with no developer directory selected there is no Device Hub to open,
# and pipefail would otherwise end the script before Expo can say what is wrong.
HUB="$({ xcode-select -p 2>/dev/null || true; } | sed 's|/Contents/Developer.*||')/Contents/Applications/DeviceHub.app"
# DEVELOPER_DIR, when set, wins over xcode-select, as it does for xcodebuild.
if [ -n "${DEVELOPER_DIR:-}" ]; then
  HUB="${DEVELOPER_DIR%/Contents/Developer*}/Contents/Applications/DeviceHub.app"
fi
if [ -d "$HUB" ]; then
  open "$HUB"
fi

exec pnpm --filter @issho/app exec expo run:ios --device "$1" --no-bundler
