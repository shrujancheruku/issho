#!/usr/bin/env bash
# What a dev deploy must have done, checked from outside, the way a visitor sees
# it: the API, the web and the admin each report the version just deployed (the
# short commit, which the API reports from its build), and the web's page is
# server-rendered. Run by .github/workflows/deploy-dev.yml after scripts/deploy-dev.sh.
#
# Environment (each has dev's value by default; the test points them at a stand-in):
#   WANT       the version to expect; default: this checkout's short commit
#   API_URL    https://api-dev.joinissho.com
#   WEB_URL    https://dev.joinissho.com
#   ADMIN_URL  https://admin-dev.joinissho.com
#   CHROME     a headless-capable Chrome; default google-chrome
set -euo pipefail

want=${WANT:-$(git rev-parse --short HEAD)}
api=${API_URL:-https://api-dev.joinissho.com}
web=${WEB_URL:-https://dev.joinissho.com}
admin=${ADMIN_URL:-https://admin-dev.joinissho.com}
chrome=${CHROME:-google-chrome}
fail=0
bad() { echo "FAIL $*"; fail=1; }

# version_at <base>: what the API's public health check reports through that address.
version_at() {
  curl -fsS --max-time 20 -X POST -H 'Content-Type: application/json' -d '{}' "$1/issho.v1.HealthService/Check" | jq -r .version
}

# The API on its own name, and on the web's and the admin's: the web's proxy
# routes the RPC paths to the API and the admin's Worker forwards them, which is
# how those two pages reach it with no CORS.
for pair in "API:$api" "API through the web's address:$web" "API through the admin's Worker:$admin"; do
  what=${pair%%:http*}
  base=http${pair#*:http}
  got=$(version_at "$base" || echo "no answer")
  [ "$got" = "$want" ] && echo "ok   $what reports $got" || bad "$what reports '$got', want '$want'"
done

# The page is built on the server: its words are in the HTML before any script runs.
if curl -fsS --max-time 20 "$web/plans" | grep -q 'No plans yet.'; then
  echo "ok   the web page is server-rendered"
else
  bad "the web page is not server-rendered: /plans has no 'No plans yet.' in its HTML"
fi

# And the pages show it: a real browser runs their script and dumps the result.
# --virtual-time-budget lets the page's request to the API finish first.
dom() {
  "$chrome" --headless=new --no-sandbox --disable-gpu --virtual-time-budget=20000 --dump-dom "$1" 2>/dev/null
}
for pair in "the web page:$web/you" "the admin page:$admin/"; do
  what=${pair%%:http*}
  url=http${pair#*:http}
  if dom "$url" | grep -q "API version $want"; then
    echo "ok   $what shows API version $want"
  else
    bad "$what ($url) does not show 'API version $want'"
  fi
done

[ "$fail" -eq 0 ] && echo "deploy-dev-verify: every address serves $want"
exit "$fail"
