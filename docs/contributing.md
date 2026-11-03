# Contributing

## Workflow

1. Branch from `main`, e.g. `feat/sign-in`.
2. Make your change and keep it focused.
3. Run `make gen` if you touched `proto/` or `backend/db/queries/`.
4. Run `make lint test` locally.
5. **Update [ROADMAP.md](ROADMAP.md):**
   - Flip status markers.
   - Add a line to the Decision log if you made a choice.
   - Bump "Last updated".

   CI's `roadmap` job fails for code changes that don't touch it.
6. Open a PR using the template. Every CI job must pass.

## Commit messages: Conventional Commits

Write each message as `type(scope): summary`, in the imperative and lower-case, with no trailing period. Wrap body lines at 72 characters (the hard limit is 100). commitlint checks every message locally through the `commit-msg` hook (enabled by `make setup`), and again in CI for every commit in a PR.

- **Types:**
  - `feat`: new behaviour
  - `fix`: bug fix
  - `docs`
  - `refactor`
  - `test`
  - `perf`
  - `build`
  - `ci`
  - `chore`
- **Scopes** (optional; listed in `commitlint.config.mjs`): `backend`, `mobile`, `admin`, `api`, `db`, `matching`, `data`, `infra`, `ci`, `deps`, `release`.
- **Breaking changes:** use `feat(api)!: …` or add a `BREAKING CHANGE:` footer.

Examples:
```
feat(mobile): add email OTP sign-in screen
fix(backend): reject tokens without exp claim
docs: document city pack schema
```

## Versioning and releases: SemVer

- release-please watches `main` and keeps a **release PR** open. It contains the next version, worked out from commit types, and an updated `CHANGELOG.md`.
- **Merging the release PR** tags `vX.Y.Z` and publishes a GitHub release.
- Before 1.0.0: `feat` bumps the minor version and `fix` bumps the patch. Breaking changes bump the minor version.
- The version is synced to `version.txt`, `package.json`, `apps/mobile/package.json` and `apps/mobile/app.json`. Don't edit these by hand.

## Common tasks

### Add or change an API endpoint
1. Edit or add a `.proto` in `proto/issho/v1/`. **Document the RPC and every field in comments**; they become [generated/api.md](generated/api.md).
2. Run `make gen`.
3. Implement the handler in `backend/internal/<domain>/handler.go`, with the logic in `service.go`.
4. Register it in `backend/cmd/api/main.go`. New RPCs **require auth by default**. Only add one to the public list if it truly must be unauthenticated.
5. Add tests, then call it from the app via `api.<service>.<method>()`.
6. Don't make breaking changes to fields that shipped apps use. `buf breaking` checks this in CI.

### Add a database change
1. Add `backend/db/migrations/000NN_<name>.sql` with `-- +goose Up` and `-- +goose Down` sections. **Never edit a migration that has already been merged.**
2. Put app tables in the `app` schema.
3. Add queries in `backend/db/queries/*.sql`, then run `make gen`.
4. Test the round trip: `make migrate-up`, `go run ./cmd/tool migrate down`, `make migrate-up`. CI does the same.

### Add a city
See [../data/README.md](../data/README.md). It's a JSON file plus `make seed`, with no code changes.

### Add a dependency
- **Go:** `cd backend && go get <module>@<version>`. Tools go in with `go get -tool`.
- **Mobile:** `cd apps/mobile && npx expo install <pkg>`. This picks SDK-compatible versions.
- **Other TS packages:** `pnpm add <pkg>` in the package's folder.

## CI and branch protection

`.github/workflows/ci.yml` runs on every push to `main`, every pull request, and on demand (`workflow_dispatch`). A new push to a PR cancels its earlier run; runs on `main` always finish, because the deploys wait for them. Every job has a 15-minute timeout. There is deliberately no Turbo cache (it would save about 20s of `typescript` and risks a cache hit skipping a check).

- **What runs when:** a `changes` job (`scripts/ci-changes.sh`, tested by `scripts/ci-changes.test.sh`) maps the PR's changed paths to the jobs below, and `backend`, `codegen`, `image`, `typescript` and `proto-breaking` run only if their paths changed. A job skipped by its `if:` reports as skipped, which satisfies a required check (a workflow-level `paths:` filter would leave them pending and block the merge). `roadmap`, `secrets` and `commits` always run. Rough rules: docs and config only → none of the heavy jobs; `backend/`, `data/` → `backend`, `codegen`, `image`; `apps/` and `packages/` → `typescript`, with turbo `--affected` limiting it to the changed packages and their dependents (the mobile Jest suite doesn't run for an admin-only change); `proto/` → everything but a turbo filter; a change to the workflows, `Makefile`, `scripts/`, the lockfile or any path the script doesn't know → everything. Each group in the script says why its paths belong there. If you add a `go:embed` or a Go test or generator that reads files outside its own package, add those paths to the Go group in `scripts/ci-changes.sh` and a case to its test, or CI will skip the job that reads them. Pushes to `main` and dispatches run everything, so the deploy gate sees a full run. If `changes` itself fails, the other jobs run rather than skip.
- **Coverage** is reported in the `backend` and `typescript` job summaries (Go per package and total; Jest and Vitest totals) and never gated. CI runs `test:coverage` where local runs use `test`, so a package with a `test` script also needs a `test:coverage` one, or CI will not run its tests.
- **Required checks on `main`:** `roadmap`, `secrets`, `commits`, `codegen`, `backend`, `typescript`, `image`.
- **`proto-breaking` is not required.** It runs `buf breaking` against `main`. A PR with the `breaking-ok` label skips it (a deliberate override); without the label it shows red. `codegen`'s drift check runs either way.
- **Release PR:** a PR opened with `GITHUB_TOKEN` doesn't trigger workflows, so `release-please.yml` dispatches `ci.yml` on the release branch (`gh workflow run`, `actions: write`, no PAT or App). On a dispatch, `roadmap` compares against the merge-base with `main` and `commits` runs commitlint over the commits since `main`. A dispatched run is not a `push`, so it can't trigger a deploy.
- **Integration tests** run as `go test ./... -run Integration`, so a new package is picked up. They call `testfixture.DatabaseURL(t)`, which skips without `TEST_DATABASE_URL` locally but fails when `CI=true`.

### Settings that live on GitHub, not in the repo

The owner applies these once (they need admin rights). Create the label and the ruleset:

```sh
gh label create breaking-ok --repo shrujancheruku/issho --color D93F0B \
  --description "Intentional breaking proto change: skips proto-breaking"

gh api -X POST repos/shrujancheruku/issho/rulesets --input - <<'JSON'
{
  "name": "main",
  "target": "branch",
  "enforcement": "active",
  "conditions": { "ref_name": { "include": ["~DEFAULT_BRANCH"], "exclude": [] } },
  "rules": [
    { "type": "deletion" },
    { "type": "non_fast_forward" },
    { "type": "pull_request", "parameters": {
        "required_approving_review_count": 0,
        "dismiss_stale_reviews_on_push": false,
        "require_code_owner_review": false,
        "require_last_push_approval": false,
        "required_review_thread_resolution": false,
        "allowed_merge_methods": ["squash"] } },
    { "type": "required_status_checks", "parameters": {
        "strict_required_status_checks_policy": true,
        "required_status_checks": [
          { "context": "roadmap" }, { "context": "secrets" }, { "context": "commits" },
          { "context": "codegen" }, { "context": "backend" }, { "context": "typescript" },
          { "context": "image" } ] } }
  ]
}
JSON
```

That requires the checks, requires branches to be up to date, allows squash merges only, and blocks force-pushes and deletion. Add a bypass actor in the ruleset if the owner should be able to merge without a PR. Check it afterwards with `gh api repos/shrujancheruku/issho/rulesets`. The next release-please PR should show all seven checks green without anyone pushing to it; if `commits` or another check doesn't attach, look at the dispatched run in the Actions tab.

## Testing

| What | How | Where |
|---|---|---|
| Go unit tests | `make test` | `*_test.go` next to the code; table-driven style |
| Auth | Tests sign JWTs with a throwaway ES256 key | `internal/platform/auth/auth_test.go` |
| Go integration tests | `make test-db` (`go test ./... -run Integration`; `TestIntegration*` names) | CI `backend` job; they fail instead of skipping when `CI=true` |
| Migrations | Up, down, up, then seed against PostGIS | CI `backend` job; locally against `supabase start` |
| TypeScript | `pnpm typecheck`, `pnpm lint` (Turborepo) | all workspace packages |
| Mobile unit tests | `pnpm test` (Jest, `jest-expo` preset) | `apps/mobile/src/**/__tests__/*.test.ts` |
| End to end (manual for now) | Steps 3–4 of [onboarding.md](onboarding.md) | |

Tests must not call paid APIs (Google, Anthropic). Use fakes behind the provider interfaces, with recorded fixtures.

## AI agent skills

The official Supabase skills ([supabase/agent-skills](https://github.com/supabase/agent-skills), MIT) are installed at project scope in `.claude/skills/`, and tracked in `skills-lock.json`. Repo-specific rules that override them are in [`CLAUDE.md`](../CLAUDE.md), e.g. goose owns migrations and the Data API stays off.

Update them with a pinned CLI version that's at least two weeks old. Telemetry off:

```sh
DO_NOT_TRACK=1 npx -y skills@1.7.0 update
```

Review the diff before committing: skills run with full agent permissions. Check that nothing new conflicts with `CLAUDE.md`.

## Code style
- Go: `gofmt` and `go vet`. Packages are named after their domain. Errors are wrapped with `%w`.
- **Comment the why.** Any logic that isn't obvious from the code itself (security checks, SQL with subtle merge rules, matching heuristics) gets a comment explaining the reasoning.
- TypeScript: strict mode, and follow the existing patterns in each app.
