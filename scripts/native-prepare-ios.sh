#!/usr/bin/env bash
# Prepares apps/app's iOS project for Swift Package Manager, then leaves it ready
# for `expo run:ios`. Run by `make native-ios`; safe to run again.
#
# Why this is more than `expo run:ios`: the app turns on
# experiments.swiftPackageManager, and with that flag Expo's prebuild still
# writes a CocoaPods Podfile into ios/. React Native's SPM setup refuses to
# inject into a project with that Podfile unless `pod` is installed, and we
# don't install CocoaPods. So the Podfile is removed before the setup runs.
#
# Why the scaffold step runs every time: two dependencies (react-native-screens
# and @react-native-masked-view/masked-view) ship no Package.swift. Scaffold
# writes one into node_modules, which `pnpm install` wipes, so it is not kept
# in git and runs again here on each machine.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/apps/app"

# The autolinking command the template Podfile would pass; React Native's
# default (@react-native-community/cli) is not installed in an Expo app.
AUTOLINK='["node","--no-warnings","--eval","require('"'"'expo/bin/autolinking'"'"')","expo-modules-autolinking","react-native-config","--json","--platform","ios"]'
SETUP=node_modules/react-native/scripts/setup-apple-spm.js

if [ ! -d ios ]; then
  # --no-install skips SPM setup here; it runs below, after the Podfile is gone.
  pnpm exec expo prebuild --platform ios --no-install
fi
rm -f ios/Podfile ios/Podfile.properties.json

node "$SETUP" scaffold --yes --config-command "$AUTOLINK"

# The marker means the project was already injected; `update` refreshes it and
# `add` injects a fresh project. Neither needs CocoaPods once the Podfile is gone.
if compgen -G "ios/*.xcodeproj/.spm-injected.json" > /dev/null; then
  node "$SETUP" update --yes --config-command "$AUTOLINK"
else
  node "$SETUP" add --yes --config-command "$AUTOLINK"
fi
echo "native-prepare-ios: iOS project is set up for Swift Package Manager"
