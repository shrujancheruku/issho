#!/usr/bin/env bash
# The backend's database checks, run by CI's `backend` job and by `make test-db`
# so the steps and the package list exist once. Needs DATABASE_URL pointing at a
# migrated-or-empty Supabase Postgres (PostGIS in `extensions`) that is already accepting connections. Never reads .env*.
set -euo pipefail
cd "$(dirname "$0")/../backend"

: "${DATABASE_URL:?DATABASE_URL must point at a throwaway PostGIS}"

# Down-migrations are destructive: only ever run against a local database. Only
# postgres:// URLs with userinfo on localhost/127.0.0.1 are accepted (percent-encode
# an '@' in a password), and query parameters other than sslmode=disable are refused:
# pgx lets host=/hostaddr= in the query override the host.
re='^postgres://[^/?@]*@(localhost|127\.0\.0\.1)(:[0-9]+)?(/[^?]*)?(\?sslmode=disable)?$'
if ! [[ "$DATABASE_URL" =~ $re ]]; then
  echo "db-checks.sh refuses this DATABASE_URL (must be postgres://user:pass@localhost or 127.0.0.1, no query but sslmode=disable)." >&2
  exit 1
fi

# Migrations apply, roll back and re-apply; the seed loads. The glob is deliberate:
# new city packs get seeded (and tested) without editing this script.
go run ./cmd/tool migrate up
go run ./cmd/tool migrate down
go run ./cmd/tool migrate up
go run ./cmd/tool seed ../data/citypacks/*.json

# One database per package: build a migrated, seeded template once, and
# testfixture clones it for each package's tests (and drops the clone), so the
# packages run in parallel (no -p 1). The role needs CREATEDB, which Supabase's
# `postgres` has. The sweep drops the template and any clone a killed run left.
rest="${DATABASE_URL#postgres://}"
query=""; [[ "$DATABASE_URL" == *\?* ]] && query="?${DATABASE_URL#*\?}"
TEST_DATABASE_URL="postgres://${rest%%/*}/issho_template$query"
export TEST_DATABASE_URL
trap 'go run ./internal/testfixture/cmd/testdb sweep "$TEST_DATABASE_URL"' EXIT
go run ./internal/testfixture/cmd/testdb create "$TEST_DATABASE_URL" "$DATABASE_URL"
DATABASE_URL="$TEST_DATABASE_URL" go run ./cmd/tool migrate up
DATABASE_URL="$TEST_DATABASE_URL" go run ./cmd/tool seed ../data/citypacks/*.json

# DB integration tests and tool dry-runs (no Google calls).
# ./... instead of a package list: the list had silently left out metrics, and
# a new package would be left out too. Every test that needs the database is
# named TestIntegration* (checked: all 53 tests that skip without a database
# match), so -run Integration finds exactly those.
# -count=1: the test cache keys on Go sources and env, not on the database's
# contents, so a cached pass could hide a migration or view change (#492).
go test ./... -run Integration -count=1 -v
go run ./cmd/tool build-matrix -dry-run tokyo
go run ./cmd/tool discover-venues -dry-run austin
go run ./cmd/tool discover-venues -dry-run da-nang-hoi-an  # areas with boundaries, read back from PostGIS
