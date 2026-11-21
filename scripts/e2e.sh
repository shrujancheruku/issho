#!/usr/bin/env bash
# The end-to-end tests: Playwright against the real API binary and the web build,
# started here on free ports. Usage: scripts/e2e.sh   (from anywhere)
#
# Never starts local Supabase and touches no database: the API has no store yet.
# It chooses two free ports rather than the dev defaults (8080, 8081, 5173), so
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

(cd "$repo/backend" && go build -ldflags "-X main.version=$E2E_API_VERSION" -o "$E2E_API_BINARY" ./cmd/api)
pnpm --filter @issho/admin build
pnpm --filter @issho/e2e exec playwright install chromium

pnpm --filter @issho/e2e test
