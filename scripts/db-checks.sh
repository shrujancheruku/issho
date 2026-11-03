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

# DB integration tests and tool dry-runs (no Google calls).
# -p 1: the packages share one database and their fixtures use stable ids, so
# packages running in parallel delete each other's rows.
TEST_DATABASE_URL="$DATABASE_URL" go test -p 1 ./internal/geo/ ./internal/profile/ ./internal/plans/ ./internal/crews/ ./internal/chat/ ./internal/chatws/ ./internal/moderation/ ./internal/translations/ ./internal/notifications/ ./internal/cities/ ./internal/citypacks/ ./internal/curation/ -run Integration -v
go run ./cmd/tool build-matrix -dry-run tokyo
go run ./cmd/tool discover-venues -dry-run austin
go run ./cmd/tool discover-venues -dry-run da-nang-hoi-an  # areas with boundaries, read back from PostGIS
