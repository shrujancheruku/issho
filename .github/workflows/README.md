# Workflows

| Workflow | What |
|---|---|
| `ci.yml` | The required checks, below |
| `release-please.yml` | Keeps the release PR open from the Conventional Commits since the last tag |
| `tracker-check.yml` | The weekly issue-tracker hygiene check (`make tracker-check`) |
| `deploy-dev.yml` | Deploys dev on a push to `main` (and, while the Foundation is built, to `integration/foundation`), or by hand for a rollback. [docs/deploy.md](../../docs/deploy.md) says how |

`deploy-dev.yml` is not a check and has none of `ci.yml`'s rules (`scripts/ci-workflow.test.sh` reads `ci.yml` only): it needs `id-token: write` and `packages: write`, and uses Google's auth action, pinned to a commit like the rest. `go tool actionlint` (the `codegen` job) lints it, and `make deploy-test` (also run by `codegen`) tests what it runs.

## The required checks

Every job in `ci.yml` is a check, and its job id is its name: the founder sets these names in branch protection ([The quality bar](../../docs/spec/quality.md#what-ci-requires)).

| Check | Runs | What it catches |
|---|---|---|
| `secrets` | `make secrets-scan` | A committed secret, in any commit on any branch |
| `commits` | `make commits` | A commit since the PR's base that isn't a Conventional Commit (`commitlint.config.mjs`, the commit-msg hook's config) |
| `codegen` | `make codegen ci-lint ci-test` | `make gen` leaves a diff or an untracked file; and CI's own guards: actionlint, the workflow's shape (`scripts/ci-workflow.test.sh`) and the path filter (`scripts/ci-changes.test.sh`) |
| `backend` | `make backend` | gofmt, `go vet`, the clock lint, `buf lint`, every Go test on a throwaway Postgres, and `proto-breaking.sh`'s own tests |
| `boundaries` | `make boundaries` | A package importing another's internals; an outside service's real implementation outside `cmd/` |
| `policy` | `make policy-lint` | A number in `policy` with no link to its document; a rule's number as a literal elsewhere |
| `rpc-auth` | `make rpc-auth` | An RPC with no declared access rule |
| `retention` | `make retention` | A table with no entry in `backend/internal/retention/registry.json` |
| `proto-breaking` | `make proto-breaking` | `buf breaking` against `main`. Skipped when the PR carries the `breaking-ok` label |
| `typescript` | `make typescript` | Lint, type check and tests of the app, the admin, the client and the e2e package |
| `strings` | `make strings` | A display language missing a key; an unused key |
| `e2e` | `make e2e` | The end-to-end tests (`e2e/README.md`) |
| `native-bundle` | `make native-bundle native-bundle-test native-screenshot-test` | The app not bundling for iOS or Android; the native scripts' own tests (the bundle's web-only import check, the screenshot and Metro scripts) failing |
| `image` | `make image` | A container image not building; the backend's not holding `api` and `worker`; the web's not serving a server-rendered page |

`make lint test` before a push covers all of these except `commits`, `image`, `e2e` and the `breaking-ok` skip, and also runs `buf breaking`, so on a `breaking-ok` change `make lint` fails locally on purpose. The other way round, every command `make lint test` runs is run by some job: `scripts/ci-workflow.test.sh` fails otherwise, so no check exists only on a laptop.

## A job that skips still passes

A check that never reports blocks the PR, so a workflow-level `paths:` filter can't be used. Instead the `changes` job lists the files the PR touches (`scripts/ci-changes.sh`, tested by `scripts/ci-changes.test.sh`), and each filtered job skips itself when none of its paths changed. A skipped job reports as passing. If `changes` itself fails, the jobs run instead of skipping. A change to anything every job runs on touches every job's paths: the workflow, `.github/actions/`, the Makefile, `scripts/ci-*`, the root `package.json`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`, any `go.mod` or `go.sum`, `go.work` and `tools/`. Otherwise a break in one would skip the jobs it breaks, and each would report as passing. `secrets`, `commits` and `codegen` always run, which is why `codegen` also runs CI's own guards. On a push to `main` or a dispatched run, every job runs.

A new job: add its target to the Makefile, its paths to `scripts/ci-changes.sh` and its test case to `scripts/ci-changes.test.sh`, and `needs: changes` with the same `if:` as its neighbours. `make lint` (`scripts/ci-workflow.test.sh` and actionlint) and the `codegen` job fail if the job is missing the skip, a timeout or a pinned action, uses an action other than checkout, the toolchains or upload-artifact, or doesn't call the make target of its own name. A new local action is in the shared paths already (`.github/actions/`).

## Rules the workflow keeps

- **No retries.** Nothing in CI retries or sets `continue-on-error`. The one exception is Playwright's single retry (`e2e/playwright.config.ts`), reported as "flaky" in its output and as a warning on the run.
- **Least privilege.** The token is `contents: read` for the whole workflow; no job has permissions of its own and none uses a secret.
- **Actions pinned to a commit SHA**, with the release in a comment. The local action `.github/actions/setup` installs Go and pnpm/Node for the jobs that need them.
- **`make lint` fetches `main`.** `buf breaking` fails without `origin/main`, so `proto-breaking` checks out the full history. The `backend` job calls `make backend`, the Go half of `make lint` without `buf breaking`, so the `breaking-ok` label skips that one check and no other.
