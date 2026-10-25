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

.PHONY: setup secrets-scan docs gen gen-proto gen-sql gen-strings email-templates ui-bundle lint test migrate-up migrate-status api worker seed matrix venues sim

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

gen: gen-proto gen-sql gen-strings

gen-proto:
	rm -rf backend/gen/issho packages/api-client/src/gen docs/generated
	$(GOTOOL) buf generate

gen-sql:
	cd backend && go tool sqlc generate

# The app's plain English UI text is the source; the API embeds a copy and
# only ever translates strings in it (internal/uistrings). Human versions
# (i18n/human/<lang>.json) are checked, stamped with the English they were
# written for, and copied too, as are the notes for translators.
gen-strings:
	cd backend && go run ./cmd/tool ui-strings ../apps/mobile/src/i18n
	cp apps/mobile/src/i18n/en.json backend/internal/uistrings/en.json
	cp apps/mobile/src/i18n/notes.json backend/internal/uistrings/notes.json
	rm -rf backend/internal/uistrings/human
	cp -R apps/mobile/src/i18n/human backend/internal/uistrings/human

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

# Background jobs (River): hourly matching passes per city.
worker:
	cd backend && go run ./cmd/worker

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

# Sign-in emails in every language listed in supabase/templates/source.json
# (needs DATABASE_URL for the translation cache and GOOGLE_TRANSLATE_API_KEY).
# Rewrites supabase/templates/*.html and the subjects in supabase/config.toml.
email-templates:
	cd backend && go run ./cmd/tool email-templates

# The city packs' languages, as the API at API_URL serves them, bundled into
# the app so a fresh install shows them before its first fetch. Run against
# the API the app build will use (production before a release).
ui-bundle:
	cd backend && go run ./cmd/tool ui-bundle -api $(API_URL)
