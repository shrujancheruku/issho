# v1's targets went with v1 (#587); v2's are added by the build tickets that
# need them. gitleaks is pinned in tools/go.mod via a `tool` directive (go.work
# makes it usable from the root).
GITLEAKS := go tool gitleaks

# The local Supabase database (`supabase start`). The migrate targets always use
# this, never DATABASE_URL from the environment: a shell or .env that exports it
# usually points at a hosted project. To migrate anything else, name it on the
# command line (make migrate-up DATABASE_URL=...).
DATABASE_URL = postgresql://postgres:postgres@127.0.0.1:54322/postgres

.PHONY: setup gen lint test rpc-auth migrate-up migrate-down tracker-check secrets-scan

# One-time setup after cloning: install JS deps (commitlint) and enable the git
# hooks (secret scan, commit-message lint).
setup:
	pnpm install
	git config core.hooksPath .githooks

# Code generation: sqlc compiles backend/db/queries into backend/gen/dbgen, and
# buf turns proto/ into the Go server stubs (backend/gen/issho) and the
# TypeScript client (packages/api-client/src/gen). buf's TypeScript plugin comes
# from pnpm install, so run `make setup` first.
gen:
	cd backend && go tool sqlc generate
	go tool buf generate

# gofmt and go vet over the backend, buf lint and buf breaking over proto/, the
# access-rule check and the type check of the generated client. The app and admin add their own
# checks in their tickets (#600, #601). buf breaking compares with main, but only
# once main holds v2's proto: until the Foundation merge, main's is v1's and every
# v2 file would read as a break (scripts/proto-breaking.sh).
lint: rpc-auth
	@test -z "$$(gofmt -l backend)" || { gofmt -l backend; echo "gofmt: files above need formatting"; exit 1; }
	cd backend && go vet ./...
	go tool buf lint
	scripts/proto-breaking.sh
	pnpm --filter @issho/api-client typecheck

# Needs Docker: the tests start a throwaway Postgres and give each test its own
# schema (backend/internal/testdb). There is no way to skip the database.
test:
	cd backend && go test -race -count=1 ./...
	pnpm --filter @issho/api-client test
	scripts/proto-breaking.test.sh

# Fails, naming the RPCs, when one in proto/ declares no access rule (the
# `access` option, see proto/README.md).
rpc-auth:
	cd backend && go run ./cmd/rpcauth ../proto

# Apply or roll back the goose migrations (backend/db/migrations) on the local
# database, which `supabase start` runs. migrate-down rolls back one migration.
migrate-up:
	cd backend && DATABASE_URL='$(DATABASE_URL)' go run ./cmd/migrate up

migrate-down:
	cd backend && DATABASE_URL='$(DATABASE_URL)' go run ./cmd/migrate down

# Scan every commit on every branch for secrets. Gitignored local secrets are
# intentionally not scanned; the pre-commit hook stops them if they are ever staged.
secrets-scan:
	$(GITLEAKS) git --redact --config .gitleaks.toml --log-opts=--all --no-banner .

# Tracker hygiene (#435): lists issues and PRs that break the planning rules in
# docs/agents/issue-tracker.md. Needs `gh` logged in; CI runs it weekly. Exits 1 on findings.
tracker-check:
	scripts/tracker-check.test.sh
	scripts/tracker-check.sh
