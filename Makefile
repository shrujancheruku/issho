# Go tools are pinned in backend/go.mod via `tool` directives (go.work makes them usable from the root).
GOTOOL := go tool
GITLEAKS := go run github.com/zricethezav/gitleaks/v8@v8.30.1

# Optional env file for targeting another environment without exporting secrets in
# your shell, e.g. `make migrate-up seed ENV_FILE=.env.issho-dev`. Files matching
# .env.* are gitignored. Values are exported to commands, never echoed.
# make parses the file itself, so values must not contain '$' or '#'.
ifdef ENV_FILE
include $(ENV_FILE)
export
endif

.PHONY: setup secrets-scan docs gen gen-proto gen-sql lint test migrate-up migrate-status api seed matrix venues sim

# One-time setup after cloning: install JS deps and enable the secret-scanning pre-commit hook.
setup:
	pnpm install
	git config core.hooksPath .githooks

# Scan every commit on every branch for secrets. Gitignored local secrets
# (e.g. supabase/signing_keys.json) are intentionally not scanned; the pre-commit
# hook stops them if they are ever staged.
secrets-scan:
	$(GITLEAKS) git --redact --config .gitleaks.toml --log-opts=--all --no-banner .

# Regenerate docs/generated/api.md (part of gen-proto).
docs: gen-proto

gen: gen-proto gen-sql

gen-proto:
	rm -rf backend/gen/issho packages/api-client/src/gen docs/generated
	$(GOTOOL) buf generate

gen-sql:
	cd backend && go tool sqlc generate

lint:
	$(GOTOOL) buf lint proto
	cd backend && go vet ./... && go tool sqlc vet
	pnpm typecheck
	pnpm lint

test:
	cd backend && go test ./...
	pnpm test

migrate-up:
	cd backend && go run ./cmd/tool migrate up

migrate-status:
	cd backend && go run ./cmd/tool migrate status

api:
	cd backend && go run ./cmd/api

seed:
	cd backend && go run ./cmd/tool seed ../data/citypacks/*.json

# Google Maps data for one city (needs GOOGLE_MAPS_API_KEY; see docs/google-maps.md).
# Add ARGS=-dry-run to see the planned API usage without calling Google.
CITY ?= tokyo
matrix:
	cd backend && go run ./cmd/tool build-matrix $(ARGS) $(CITY)

venues:
	cd backend && go run ./cmd/tool discover-venues $(ARGS) $(CITY)

# Matching on synthetic users (no database or API keys): make sim [ARGS='-users 1000 -seed 2']
sim:
	cd backend && go run ./cmd/tool simulate $(ARGS) ../data/citypacks/*.json
