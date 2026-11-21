# v1's targets went with v1 (#587); v2's are added by the build tickets that
# need them. gitleaks is pinned in tools/go.mod via a `tool` directive (go.work
# makes it usable from the root).
GITLEAKS := go tool gitleaks

.PHONY: setup tracker-check secrets-scan

# One-time setup after cloning: install JS deps (commitlint) and enable the git
# hooks (secret scan, commit-message lint).
setup:
	pnpm install
	git config core.hooksPath .githooks

# Scan every commit on every branch for secrets. Gitignored local secrets are
# intentionally not scanned; the pre-commit hook stops them if they are ever staged.
secrets-scan:
	$(GITLEAKS) git --redact --config .gitleaks.toml --log-opts=--all --no-banner .

# Tracker hygiene (#435): lists issues and PRs that break the planning rules in
# docs/agents/issue-tracker.md. Needs `gh` logged in; CI runs it weekly. Exits 1 on findings.
tracker-check:
	scripts/tracker-check.test.sh
	scripts/tracker-check.sh
