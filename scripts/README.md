# scripts

Repo-level helper scripts, run by the root `Makefile`. Each script has a header comment with its usage.

| Script | Run by | What it does |
|---|---|---|
| `native-bundle.sh` | `make native-bundle` (also `make lint`) | `expo export` of `apps/app` for iOS and Android; fails on an import that resolves only on the web |
| `native-bundle.test.sh` | `make native-bundle-test` (also `make test`) | Proves the bundle check fails on a web-only import, using the fixture in `apps/app/scripts/fixtures/native-bundle/` |
| `proto-breaking.sh` | `make lint` | `buf breaking` against `origin/main`, once main holds v2's proto |
| `proto-breaking.test.sh` | `make lint` | Tests `proto-breaking.sh` in a throwaway git repo |
| `tracker-check.sh` | `make tracker-check` (weekly CI) | The issue-tracker hygiene check ([issue-tracker.md](../docs/agents/issue-tracker.md)) |
| `tracker-check.test.sh` | `make tracker-check` | Tests `tracker-check.sh` against the fixtures in `testdata/tracker-check/` |
| `brand/*.py` | by hand, when the brand changes | Builds the logo SVGs and the app's icons, splash and PNG exports from `assets/brand/` (see each script's header for its tools) |

`testdata/` holds fixtures for the tests above.
