#!/usr/bin/env bash
# Tests scripts/deploy-dev-verify.sh against a stand-in server that answers like
# dev does, and a stand-in browser: it passes when everything shows the commit's
# version and fails, naming what, when one thing shows another. Needs python3.
# Usage: scripts/deploy-dev-verify.test.sh
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
script="$repo/scripts/deploy-dev-verify.sh"
fail=0
bad() { echo "FAIL $*"; fail=1; }

work=$(mktemp -d)
server_pid=""
trap '[ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null; rm -rf "$work"' EXIT

# One server stands for the API, the web and the admin: the version it reports
# is read from a file, so a test can change it between runs, and the web page's
# words are fixed.
cat > "$work/server.py" <<'PY'
import http.server, json, sys

class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args): pass
    def version(self):
        return open(sys.argv[2]).read().strip()
    def do_POST(self):
        if self.path == "/issho.v1.HealthService/Check":
            body = json.dumps({"version": self.version()}).encode()
            self.send_response(200); self.send_header("content-type", "application/json")
        else:
            body = b"{}"; self.send_response(404)
        self.end_headers(); self.wfile.write(body)
    def do_GET(self):
        if self.path == "/plans":
            body = open(sys.argv[3]).read().encode()
            self.send_response(200); self.send_header("content-type", "text/html")
        else:
            body = b""; self.send_response(404)
        self.end_headers(); self.wfile.write(body)

s = http.server.HTTPServer(("127.0.0.1", 0), Handler)
print(s.server_address[1], flush=True)
s.serve_forever()
PY
echo abc1234 > "$work/version"
echo '<html><body>No plans yet.</body></html>' > "$work/plans.html"
python3 "$work/server.py" _ "$work/version" "$work/plans.html" > "$work/port" &
server_pid=$!
for _ in $(seq 1 50); do [ -s "$work/port" ] && break; sleep 0.1; done
base="http://127.0.0.1:$(cat "$work/port")"

# The stand-in browser prints what the page would hold once its script ran: the
# page's version comes from $DOM_VERSION.
cat > "$work/chrome" <<'STUB'
#!/bin/sh
echo "<html><body><p>API version ${DOM_VERSION}</p></body></html>"
STUB
chmod +x "$work/chrome"

run() { # <served version> <version in the browsers' pages> [want]
  echo "$1" > "$work/version"
  TRIES=2 SECONDS_BETWEEN=0 DOM_VERSION=$2 API_URL=$base WEB_URL=$base ADMIN_URL=$base CHROME="$work/chrome" WANT="${3:-abc1234}" "$script" > "$work/out" 2>&1
}

run abc1234 abc1234 || { cat "$work/out"; bad "failed although everything shows the commit's version"; }

if run old9999 abc1234; then bad "passed although the API serves an old version"; fi
grep -q 'API' "$work/out" || bad "the failure doesn't say it was the API"

if run abc1234 old9999; then bad "passed although the pages show an old version"; fi
grep -qi 'page' "$work/out" || bad "the failure doesn't say it was a page"

echo '<html><body>Internal Server Error</body></html>' > "$work/plans.html"
if run abc1234 abc1234; then bad "passed although the web page is not rendered on the server"; fi
grep -q 'server-rendered' "$work/out" || bad "the failure doesn't say the web page is not server-rendered"

[ "$fail" -eq 0 ] && echo "deploy-dev-verify: ok"
exit "$fail"
