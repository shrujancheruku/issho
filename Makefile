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

# Code generation: sqlc compiles each backend package's queries into its store, and
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
# v2 file would read as a break. It takes no arguments or environment, so nothing
# given to make lint switches it off, and it fails if origin/main isn't fetched
# (scripts/proto-breaking.sh).
lint: rpc-auth
	@test -z "$$(gofmt -l backend)" || { gofmt -l backend; echo "gofmt: files above need formatting"; exit 1; }
	cd backend && go vet ./...
	cd backend && go run ./tools/clockcheck
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

# Every number in the rules is a constant in backend/internal/policy, linked to
# its spec document (#596). make lint runs this; the rule is stated once, in
# backend/tools/policylint.
.PHONY: policy-lint
policy-lint:
	cd backend && go run ./tools/policylint -root ..

lint: policy-lint

# The admin (#601): its build (type check, then static files in apps/admin/dist)
# is a lint, and its screen tests run under make test.
.PHONY: admin-lint admin-test
lint: admin-lint
test: admin-test
admin-lint:
	pnpm --filter @issho/admin build
admin-test:
	pnpm --filter @issho/admin test

# The member app, apps/app (#600): expo lint (which bans string literals in
# screen files), the strings check (en.json and vi.json hold the same keys, none
# unused) and the type check; and its Jest tests.
.PHONY: app-lint app-test
app-lint:
	pnpm --filter @issho/app lint
	pnpm --filter @issho/app typecheck

app-test:
	pnpm --filter @issho/app test

lint: app-lint
test: app-test

# A package reaches another only through its exported Go API, and outside
# services are wired only in cmd/ (#599), and only the city clock asks the system
# for the time (tools/clockcheck, which `backend` runs too). make lint runs this;
# the rules are stated once, in backend/tools/boundaries and backend/tools/clockcheck.
.PHONY: boundaries
boundaries:
	cd backend && go run ./tools/boundaries -root ..
	cd backend && go run ./tools/clockcheck

lint: boundaries

# Native checks (#602). `native-bundle` bundles the app for iOS and Android and
# fails on a web-only import; its test proves that with a fixture. The development
# builds on the simulator and emulator are the block below (#630).
.PHONY: native-bundle native-bundle-test

native-bundle:
	scripts/native-bundle.sh

native-bundle-test:
	scripts/native-bundle.test.sh

lint: native-bundle
test: native-bundle-test

# Local development builds on the iOS simulator and the Android emulator (#630).
# Nothing goes to EAS or a store. Metro's port (8081) and the simulator and
# emulator are shared, so one native run at a time per machine. NATIVE_AVD is an
# AVD name from `android emulator list`; NATIVE_IOS_DEVICE a simulator name from
# `xcrun simctl list devices available`.
NATIVE_IOS_DEVICE ?= iPhone 17
NATIVE_AVD ?= issho

.PHONY: native-metro native-wait native-stop native-ios native-android native-screenshot-ios native-screenshot-android native-screenshot-test

# These three return. `expo run:*` would stay in the foreground as Metro, so the
# build targets pass --no-bundler and Metro is its own target (scripts/native-metro.sh).
native-metro:
	scripts/native-metro.sh start

native-wait:
	scripts/native-metro.sh wait

native-stop:
	NATIVE_AVD="$(NATIVE_AVD)" NATIVE_IOS_DEVICE="$(NATIVE_IOS_DEVICE)" scripts/native-metro.sh stop

native-ios:
	pnpm --filter @issho/app exec expo run:ios --device "$(NATIVE_IOS_DEVICE)" --no-bundler

native-android:
	android emulator start $(NATIVE_AVD)
	pnpm --filter @issho/app exec expo run:android --no-bundler

native-screenshot-ios:
	scripts/native-screenshot.sh ios $(or $(NAME),screen)

native-screenshot-android:
	scripts/native-screenshot.sh android $(or $(NAME),screen)

test: native-screenshot-test
native-screenshot-test:
	scripts/native-screenshot.test.sh
	scripts/native-metro.test.sh

# CI's required checks (#603). Each job in .github/workflows/ci.yml is a thin call
# to one of these (or to a target above), so what CI runs is what runs here.
# Their names are the checks' names; scripts/ci-workflow.test.sh keeps the two
# together. `make lint test` stays the superset to run before a push.
.PHONY: commits backend codegen retention proto-breaking typescript strings image e2e ci-lint ci-test

# Every commit since the branch left COMMITS_BASE is a Conventional Commit, by
# the config the commit-msg hook uses (commitlint.config.mjs). The base is
# origin/main unless given (fetch it first); CI gives a PR's base. Merge commits
# are skipped, as commitlint's defaults skip them.
COMMITS_BASE ?= origin/main
commits:
	@base=$$(git merge-base '$(COMMITS_BASE)' HEAD) || { echo "commits: no merge base with $(COMMITS_BASE); fetch it first"; exit 1; }; \
	pnpm exec commitlint --config commitlint.config.mjs --from "$$base" --to HEAD

# The backend job: formatting, vet, the clock lint, buf lint, every Go test (the
# package tests, the property tests, the season and the privacy tests, as they
# arrive) and proto-breaking.sh's own tests. It is `make lint test`'s Go half
# without buf breaking, which has its own job: `breaking-ok` skips that job, and
# make lint can't skip the check. The script's tests stay here so the label
# can't skip them too.
backend:
	@test -z "$$(gofmt -l backend)" || { gofmt -l backend; echo "gofmt: files above need formatting"; exit 1; }
	cd backend && go vet ./...
	cd backend && go run ./tools/clockcheck
	go tool buf lint
	cd backend && go test -race -count=1 ./...
	scripts/proto-breaking.test.sh

# `make gen` leaves no diff: nothing changed and nothing new. Needs `make setup`
# (buf's TypeScript plugin comes from pnpm install).
codegen: gen
	git diff --exit-code
	@test -z "$$(git status --porcelain)" || { git status --short; echo "codegen: make gen left files that are not committed"; exit 1; }

# Every table has an entry in backend/internal/retention/registry.json. Part of
# make test as well; this is the check on its own. Needs Docker.
retention:
	cd backend && go test -count=1 ./internal/retention/

# buf breaking against origin/main (fetch it first). make lint runs it too.
proto-breaking:
	scripts/proto-breaking.sh

# The TypeScript half: the client's type check and tests, the app's lint (with
# the strings check), type check and tests, the admin's build and tests, and the
# e2e package's type check.
typescript: app-lint app-test admin-lint admin-test
	pnpm --filter @issho/api-client typecheck
	pnpm --filter @issho/api-client test
	pnpm --filter @issho/e2e typecheck

# The app's strings: en.json and vi.json hold the same keys and none is unused.
strings:
	pnpm --filter @issho/app strings

# The backend's container image builds and holds api and worker, and the web's
# builds and serves a server-rendered page (apps/web). Needs Docker.
image:
	scripts/image.sh

# Playwright against the API binary and the web build, on free ports. Starts
# neither Supabase nor a database. See e2e/README.md.
e2e:
	scripts/e2e.sh

# The workflow files are valid (actionlint, pinned in tools/go.mod) and
# ci.yml keeps its required checks, skips and no-retry rule.
ci-lint:
	go tool actionlint
	scripts/ci-workflow.test.sh

# The path filter that lets a job skip and still report as passing.
ci-test:
	scripts/ci-changes.test.sh

lint: ci-lint
test: ci-test

# Dev's deploy (#605), run by .github/workflows/deploy-dev.yml and nowhere else:
# nothing that deploys is installed on a laptop. deploy-test is what can be tested
# without a server: the order (a failed migration stops the deploy), the Kamal
# config (with Kamal itself, in Docker) and the checks the deploy ends with. It
# is part of make test and, through ci-test, of CI's codegen job.
.PHONY: deploy-dev deploy-dev-verify deploy-test
deploy-dev:
	scripts/deploy-dev.sh

deploy-dev-verify:
	scripts/deploy-dev-verify.sh

deploy-test:
	scripts/deploy-config.test.sh
	scripts/deploy-dev.test.sh
	scripts/deploy-dev-verify.test.sh
	scripts/dev-host.test.sh
	scripts/dev-ssh-pin.test.sh
	cd apps/web && node --test

test: deploy-test
ci-test: deploy-test

# Load the development fixture (the city Da Nang–Hoi An and its two plans, backend/fixtures)
# into the local database that `make migrate-up` migrated. It refuses a database that
# isn't on this machine and is not in the production image (backend/cmd/seed).
.PHONY: seed
seed:
	cd backend && DATABASE_URL='$(DATABASE_URL)' go run ./cmd/seed

# Seed the hosted dev database (issho-dev) only. The founder runs it, once, after dev is
# migrated; no worker or CI job does. DATABASE_URL (dev's migration connection) and
# SEED_DEV_PROJECT_REF (the dev project's ref) come from the shell's environment, not this
# file; the seed refuses any target that isn't that project's, or whose stored
# issho.environment setting isn't 'dev' (set once by the founder on dev). See docs/deploy.md.
.PHONY: seed-dev
seed-dev:
	cd backend && go run ./cmd/seed --hosted-dev
