#!/usr/bin/env bash
# Metro (Expo's dev server) for a native run, as three commands that each return:
#   start  launches Metro in the background and returns once it answers
#   wait   returns once a device has loaded the app's bundle, or fails with the
#          log's last lines after NATIVE_WAIT seconds (default 180)
#   stop   stops Metro, the NATIVE_AVD emulator and the NATIVE_IOS_DEVICE
#          simulator, and no other device on the machine
# Why not `expo run:*` serving Metro itself: it stays in the foreground for as
# long as Metro runs, so a worker waiting on it waits forever. The build
# targets pass --no-bundler and exit when the app is installed.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP="$ROOT/apps/app"
STATE="${NATIVE_METRO_STATE:-$APP/.expo/native-metro}"
LOG="$STATE/metro.log"
PIDFILE="$STATE/metro.pid"
PORT=8081
mkdir -p "$STATE"

running() { [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; }

case "${1:-}" in
  start)
    if running; then echo "native-metro: already running"; exit 0; fi
    : > "$LOG"
    # CI=1 keeps Expo from prompting or watching the keyboard. setsid-less: the
    # pid is the pnpm process, and stop kills its whole group below.
    (cd "$APP" && CI=1 nohup pnpm exec expo start --dev-client --port "$PORT" >"$LOG" 2>&1 & echo $! >"$PIDFILE")
    for _ in $(seq 1 60); do
      if curl -fs "http://localhost:$PORT/status" 2>/dev/null | grep -q running; then
        echo "native-metro: Metro is up on port $PORT"
        exit 0
      fi
      sleep 1
    done
    echo "native-metro: Metro did not start in 60s; last lines:" >&2
    tail -n 20 "$LOG" >&2
    exit 1
    ;;
  wait)
    # Metro logs "Android Bundled ..." or "iOS Bundled ..." when a device
    # fetches the bundle.
    for _ in $(seq 1 "${NATIVE_WAIT:-180}"); do
      if grep -Eq "(Android|iOS) Bundled" "$LOG" 2>/dev/null; then
        echo "native-metro: the app loaded its bundle"
        exit 0
      fi
      sleep 1
    done
    echo "native-metro: no device loaded the bundle in ${NATIVE_WAIT:-180}s; last lines:" >&2
    tail -n 20 "$LOG" >&2
    exit 1
    ;;
  stop)
    if running; then
      pid="$(cat "$PIDFILE")"
      # Metro runs as children of the pnpm process; stop the children first.
      pkill -P "$pid" 2>/dev/null || true
      kill "$pid" 2>/dev/null || true
    fi
    rm -f "$PIDFILE"
    # Anything else still holding the port (a Metro started by hand).
    lsof -ti "tcp:$PORT" -sTCP:LISTEN 2>/dev/null | xargs kill 2>/dev/null || true
    android emulator stop "${NATIVE_AVD:-issho}" >/dev/null 2>&1 || true
    xcrun simctl shutdown "${NATIVE_IOS_DEVICE:-iPhone 17}" >/dev/null 2>&1 || true
    echo "native-metro: stopped Metro, the emulator and the simulator"
    ;;
  *)
    echo "usage: scripts/native-metro.sh <start|wait|stop>" >&2
    exit 2
    ;;
esac
