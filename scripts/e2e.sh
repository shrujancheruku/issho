#!/usr/bin/env bash
# The end-to-end tests: Playwright against the real API binary and the web build,
# started here on free ports. Usage: scripts/e2e.sh   (from anywhere)
#
# Never starts local Supabase and touches no database: the API has no store yet.
# It chooses three free ports rather than the dev defaults (8080, 8081, 5173), so
# it can run beside a developer's servers; Playwright starts both servers and
# stops them when the run ends (e2e/playwright.config.ts).
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# A free port: bind 0 and ask which one we got. Another process could take it
# before the server starts; Playwright then fails to start, which is loud enough.
free_port() { node -e 'const s=require("net").createServer().listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})'; }

# A version no real build has, so the page can only show it by asking the API.
export E2E_API_VERSION="e2e-$$"
export E2E_API_BINARY="$work/api"
export E2E_API_PORT=$(free_port)
export E2E_WEB_PORT=$(free_port)
export E2E_APP_PORT=$(free_port)
# The Expo server's build (the member app's web export) and where Playwright finds it.
export E2E_APP_DIST="$work/app-dist"

(cd "$repo/backend" && go build -ldflags "-X main.version=$E2E_API_VERSION" -o "$E2E_API_BINARY" ./cmd/api)
pnpm --filter @issho/admin build
# The member app, built as the web image builds it: for its own origin (the Expo
# server forwards the API's paths, as in deployment), and 127.0.0.1 counts as a
# dev API, so the page shows the API's version. Nobody is signed in, so /you sends
# the page to sign-in, which shows the version too. The Supabase values are
# stand-ins, so the build is "set up" and shows no configuration error: nothing
# calls them (no session is stored, no code is sent), and no real project is named.
(cd "$repo/apps/app" && CI=1 EXPO_PUBLIC_API_URL="http://127.0.0.1:$E2E_APP_PORT" \
  EXPO_PUBLIC_SUPABASE_URL="http://127.0.0.1:1" EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY="e2e-stand-in-key" \
  pnpm exec expo export --platform web --output-dir "$E2E_APP_DIST")
pnpm --filter @issho/e2e exec playwright install chromium

pnpm --filter @issho/e2e test
