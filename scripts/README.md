# scripts

Repo-level helper scripts, run by the root `Makefile`. Each script has a header comment with its usage.

| Script | Run by | What it does |
|---|---|---|
| `native-bundle.sh` | `make native-bundle` (also `make lint`) | `expo export` of `apps/app` for iOS and Android; fails on an import that resolves only on the web |
| `native-bundle.test.sh` | `make native-bundle-test` (also `make test`) | Proves the bundle check fails on a web-only import, using the fixture in `apps/app/scripts/fixtures/native-bundle/` |
| `native-metro.sh` | `make native-metro`, `make native-wait`, `make native-stop` | Metro for a native run as three commands that return: starts it in the background, waits (bounded) until a device loads the bundle, and stops Metro with the build's emulator and simulator ([Native builds](../apps/app/README.md#native-builds)) |
| `native-metro.test.sh` | `make native-screenshot-test` (also `make test`) | Proves `wait` is bounded and the script refuses an unknown command |
| `native-screenshot.sh` | `make native-screenshot-android`, `make native-screenshot-ios` | Captures the running emulator or simulator to `apps/app/docs/screenshots/<name>-<platform>.png` |
| `native-screenshot.test.sh` | `make native-screenshot-test` (also `make test`) | Proves `native-screenshot.sh` refuses bad arguments before it touches a device |
| `proto-breaking.sh` | `make lint`, `make proto-breaking` | `buf breaking` against `origin/main`, once main holds v2's proto |
| `proto-breaking.test.sh` | `make test`, `make backend` (CI's `backend` job) | Tests `proto-breaking.sh` in a throwaway git repo |
| `ci-changes.sh` | CI's `changes` job | Which of CI's path-filtered jobs have something to check, from the changed files on stdin; a skipped job reports as passing ([Workflows](../.github/workflows/README.md)) |
| `ci-changes.test.sh` | `make ci-test` (also `make test` and CI's `codegen` job) | Tests `ci-changes.sh` |
| `ci-workflow.test.sh` | `make ci-lint` (also `make lint` and CI's `codegen` job) | Checks `ci.yml` keeps its required checks, skips, pins, least privilege and no-retry rule, that what CI runs on touches every job's paths, and that every command in `make lint test` runs in some job |
| `e2e.sh` | `make e2e` | Starts a throwaway Docker stack (Postgres, Supabase Auth, a mail catcher and a gateway, on a network of its own and ports Docker picks), builds the API (with the test-only RPCs) and the web, runs Playwright against them, then removes what it started ([e2e](../e2e/README.md)) |
| `image.sh` | `make image` | Builds `backend/Dockerfile` and checks the image holds `api` and `worker`; builds `apps/web/Dockerfile` (with stand-in sign-in values as build secrets), checks the bundle holds them, that a rebuild with other values bundles the new ones and that an empty value fails; runs it and checks `/plans` is server-rendered; then removes the images |
| `deploy-dev.sh` | `make deploy-dev` (CI's `deploy-dev` workflow only) | Builds and pushes the two images, migrates `issho-dev`, then deploys the API, the worker, the Expo server and the admin, in that order; a failed step stops the rest. It first checks that every secret `.kamal/secrets.dev` names exists (`gcloud secrets versions access`, value discarded) and that the two Supabase values are not empty, gives the web build a digest of them, and stops, naming the missing ones, before Kamal runs |
| `deploy-dev.test.sh` | `make deploy-test` (also `make test` and CI's `codegen` job) | Tests that order with stand-ins for `kamal`, `gcloud`, `go` and `pnpm`: a failed migration or build deploys nothing. It also checks that no secret's value is printed, runs the script both off and on GitHub Actions, and on Actions allows the value only in the `::add-mask::` line the runner keeps out of the log |
| `deploy-dev-verify.sh` | `make deploy-dev-verify` (CI's `deploy-dev` workflow) | Checks from outside that the API, the web and the admin each report the commit just deployed and that the web page is server-rendered |
| `deploy-dev-verify.test.sh` | `make deploy-test` | Tests it against a stand-in server and browser |
| `dev-host.sh` | CI's `deploy-dev` workflow | Prints the dev VM's address, asked of Google Cloud (the reserved address `issho-dev-ip`); the workflow masks it |
| `dev-host.test.sh` | `make deploy-test` | Tests it with a stand-in for `gcloud` |
| `dev-ssh-pin.sh` | CI's `deploy-dev` workflow | Writes the VM's address and its host key (`dev-ssh-host-key`) as the only known host, and an ssh config that refuses any other key, for Kamal |
| `dev-ssh-pin.test.sh` | `make deploy-test` | Tests what it writes with a stand-in for `gcloud`, then, against a real sshd in Docker and through Kamal's net-ssh, that the pinned key is accepted and a changed key or an unknown host refused |
| `deploy-config.test.sh` | `make deploy-test` | Loads `config/deploy*.yml` and `.kamal/secrets.dev` through Kamal's own loader, in Docker, with a stand-in `gcloud` ([config](../config/README.md)) |
| `tracker-check.sh` | `make tracker-check` (weekly CI) | The issue-tracker hygiene check ([issue-tracker.md](../docs/agents/issue-tracker.md)) |
| `tracker-check.test.sh` | `make tracker-check` | Tests `tracker-check.sh` against the fixtures in `testdata/tracker-check/` |
| `brand/*.py` | by hand, when the brand changes | Builds the logo SVGs and the app's icons, splash and PNG exports from `assets/brand/` (see each script's header for its tools) |

`testdata/` holds fixtures for the tests above.
