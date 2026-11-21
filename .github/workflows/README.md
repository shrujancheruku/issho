# Workflows

| Workflow | What |
|---|---|
| `ci.yml` | The required checks, below |
| `release-please.yml` | Keeps the release PR open from the Conventional Commits since the last tag |
| `tracker-check.yml` | The weekly issue-tracker hygiene check (`make tracker-check`) |

Deploys are #605's and arrive as their own workflow.

## The required checks

Every job in `ci.yml` is a check, and its job id is its name: the founder sets these names in branch protection ([The quality bar](../../docs/spec/quality.md#what-ci-requires)).

| Check | Runs | What it catches |
|---|---|---|
| `secrets` | `make secrets-scan` | A committed secret, in any commit on any branch |
| `commits` | commitlint | A message that isn't a Conventional Commit |
| `codegen` | `make codegen` | `make gen` leaves a diff or an untracked file |
| `backend` | `make backend` | gofmt, `go vet`, the clock lint, `buf lint` and every Go test on a throwaway Postgres |
| `boundaries` | `make boundaries` | A package importing another's internals; an outside service's real implementation outside `cmd/` |
| `policy` | `make policy-lint` | A number in `policy` with no link to its document; a rule's number as a literal elsewhere |
| `rpc-auth` | `make rpc-auth` | An RPC with no declared access rule |
| `retention` | `make retention` | A table with no entry in `backend/internal/retention/registry.json` |
| `proto-breaking` | `make proto-breaking` | `buf breaking` against `main`. Skipped when the PR carries the `breaking-ok` label |
| `typescript` | `make typescript` | Lint, type check and tests of the app, the admin, the client and the e2e package |
| `strings` | `make strings` | A display language missing a key; an unused key |
| `e2e` | `make e2e` | The end-to-end tests (`e2e/README.md`) |
| `native-bundle` | `make native-bundle native-bundle-test` | The app not bundling for iOS or Android |
| `image` | `make image` | The container image not building, or not holding `api` and `worker` |

`make lint test` before a push covers all of these except `image`, `e2e` and the `breaking-ok` skip, and also runs `buf breaking`, so on a `breaking-ok` change `make lint` fails locally on purpose.

## A job that skips still passes

A check that never reports blocks the PR, so a workflow-level `paths:` filter can't be used. Instead the `changes` job lists the files the PR touches (`scripts/ci-changes.sh`, tested by `scripts/ci-changes.test.sh`), and each filtered job skips itself when none of its paths changed. A skipped job reports as passing. If `changes` itself fails, the jobs run instead of skipping. `secrets`, `commits` and `codegen` always run. On a push to `main` or a dispatched run, every job runs.

A new job: add its target to the Makefile, its paths to `scripts/ci-changes.sh` and its test case to `scripts/ci-changes.test.sh`, and `needs: changes` with the same `if:` as its neighbours. `make lint` (`scripts/ci-workflow.test.sh` and actionlint) fails if the job is missing the skip, a timeout or a pinned action.

## Rules the workflow keeps

- **No retries.** Nothing in CI retries or sets `continue-on-error`. The one exception is Playwright's single retry (`e2e/playwright.config.ts`), reported as "flaky" in its output and as a warning on the run.
- **Least privilege.** The token is `contents: read` for the whole workflow; no job has permissions of its own and none uses a secret.
- **Actions pinned to a commit SHA**, with the release in a comment. The local action `.github/actions/setup` installs Go and pnpm/Node for the jobs that need them.
- **`make lint` fetches `main`.** `buf breaking` fails without `origin/main`, so `proto-breaking` checks out the full history. The `backend` job calls `make backend`, the Go half of `make lint` without `buf breaking`, so the `breaking-ok` label skips that one check and no other.
