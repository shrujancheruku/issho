# v1's targets went with v1 (#587); v2's are added by the build tickets that
# need them. gitleaks is pinned in tools/go.mod via a `tool` directive (go.work
# makes it usable from the root).
GITLEAKS := go tool gitleaks

.PHONY: setup gen lint test tracker-check secrets-scan

# One-time setup after cloning: install JS deps (commitlint) and enable the git
# hooks (secret scan, commit-message lint).
setup:
	pnpm install
	git config core.hooksPath .githooks

# Code generation. Nothing generates yet: sqlc (#594) and the ConnectRPC client
# (#595) hook in here.
gen:
	@echo "nothing to generate yet"

# gofmt and go vet over the backend. The app and admin add their own checks in
# their tickets (#600, #601).
lint:
	@test -z "$$(gofmt -l backend)" || { gofmt -l backend; echo "gofmt: files above need formatting"; exit 1; }
	cd backend && go vet ./...

# Needs Docker: the tests start a throwaway Postgres and give each test its own
# schema (backend/internal/testdb). There is no way to skip the database.
test:
	cd backend && go test -race -count=1 ./...

# Scan every commit on every branch for secrets. Gitignored local secrets are
# intentionally not scanned; the pre-commit hook stops them if they are ever staged.
secrets-scan:
	$(GITLEAKS) git --redact --config .gitleaks.toml --log-opts=--all --no-banner .

# Tracker hygiene (#435): lists issues and PRs that break the planning rules in
# docs/agents/issue-tracker.md. Needs `gh` logged in; CI runs it weekly. Exits 1 on findings.
tracker-check:
	scripts/tracker-check.test.sh
	scripts/tracker-check.sh
