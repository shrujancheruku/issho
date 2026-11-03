#!/usr/bin/env bash
# The backend's database checks, run by CI's `backend` job and by `make test-db`
# so the steps and the package list exist once. Needs DATABASE_URL pointing at a
# migrated-or-empty PostGIS that is already accepting connections. Never reads .env*.
set -euo pipefail
cd "$(dirname "$0")/../backend"

: "${DATABASE_URL:?DATABASE_URL must point at a throwaway PostGIS}"

# Migrations apply, roll back and re-apply; the seed loads.
go run ./cmd/tool migrate up
go run ./cmd/tool migrate down
go run ./cmd/tool migrate up
go run ./cmd/tool seed ../data/citypacks/*.json

# DB integration tests and tool dry-runs (no Google calls).
TEST_DATABASE_URL="$DATABASE_URL" go test ./internal/geo/ ./internal/profile/ ./internal/plans/ ./internal/crews/ ./internal/chat/ ./internal/chatws/ ./internal/moderation/ ./internal/translations/ ./internal/notifications/ ./internal/cities/ ./internal/citypacks/ ./internal/curation/ -run Integration -v
go run ./cmd/tool build-matrix -dry-run tokyo
go run ./cmd/tool discover-venues -dry-run austin
go run ./cmd/tool discover-venues -dry-run da-nang-hoi-an  # areas with boundaries, read back from PostGIS
