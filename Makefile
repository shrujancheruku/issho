# Go tools are pinned in backend/go.mod via `tool` directives (go.work makes them usable from the root).
GOTOOL := go tool
GITLEAKS := go run github.com/zricethezav/gitleaks/v8@v8.30.1

.PHONY: setup secrets-scan docs gen gen-proto gen-sql lint test migrate-up migrate-status api seed

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
	cd backend && go run ./cmd/tool seed ../data/citypacks/tokyo.json
