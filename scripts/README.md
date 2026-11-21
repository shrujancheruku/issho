# scripts

Repo-level helper scripts, run by the root `Makefile`. Each script has a header comment with its usage.

| Script | Run by | What it does |
|---|---|---|
| `native-bundle.sh` | `make native-bundle` (also `make lint`) | `expo export` of `apps/app` for iOS and Android; fails on an import that resolves only on the web |
| `native-bundle.test.sh` | `make native-bundle-test` (also `make test`) | Proves the bundle check fails on a web-only import, using the fixture in `apps/app/scripts/fixtures/native-bundle/` |
| `proto-breaking.sh` | `make lint`, `make proto-breaking` | `buf breaking` against `origin/main`, once main holds v2's proto |
| `proto-breaking.test.sh` | `make test`, `make backend` (CI's `backend` job) | Tests `proto-breaking.sh` in a throwaway git repo |
| `ci-changes.sh` | CI's `changes` job | Which of CI's path-filtered jobs have something to check, from the changed files on stdin; a skipped job reports as passing ([Workflows](../.github/workflows/README.md)) |
| `ci-changes.test.sh` | `make ci-test` (also `make test` and CI's `codegen` job) | Tests `ci-changes.sh` |
| `ci-workflow.test.sh` | `make ci-lint` (also `make lint` and CI's `codegen` job) | Checks `ci.yml` keeps its required checks, skips, pins, least privilege and no-retry rule, that what CI runs on touches every job's paths, and that every command in `make lint test` runs in some job |
| `e2e.sh` | `make e2e` | Playwright against the API binary and the web build, on free ports ([e2e](../e2e/README.md)) |
| `image.sh` | `make image` | Builds `backend/Dockerfile` and checks the image holds `api` and `worker`, then removes it |
| `tracker-check.sh` | `make tracker-check` (weekly CI) | The issue-tracker hygiene check ([issue-tracker.md](../docs/agents/issue-tracker.md)) |
| `tracker-check.test.sh` | `make tracker-check` | Tests `tracker-check.sh` against the fixtures in `testdata/tracker-check/` |
| `brand/*.py` | by hand, when the brand changes | Builds the logo SVGs and the app's icons, splash and PNG exports from `assets/brand/` (see each script's header for its tools) |

`testdata/` holds fixtures for the tests above.
