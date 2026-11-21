# e2e

End-to-end tests: Playwright drives a real browser against the real API binary and the web build, started together by `make e2e` ([Quality: End to end](../docs/spec/quality.md#end-to-end)). This is a pnpm workspace package, `@issho/e2e`.

```sh
make e2e   # from the repo root
```

Needs Go, `make setup` (pnpm install) and a network the first time, to download Playwright's Chromium. It needs neither Docker nor a database, and it never starts local Supabase.

## What `make e2e` does

`scripts/e2e.sh` does the work, then stops everything it started:

1. picks two free ports (never 8080, 8081 or 5173, so it runs beside a developer's servers);
2. builds `backend/cmd/api` with a version of its own (`e2e-<pid>`);
3. builds the admin (`pnpm --filter @issho/admin build`);
4. runs `playwright test`, whose config starts the API on the first port and `vite preview` of the admin on the second, with its proxy sending the API's paths to the API, and stops both at the end.

`playwright.config.ts` has no defaults for the ports or the binary: running `playwright test` by hand fails and says to use `make e2e`.

## The tests

| Test | Proves |
|---|---|
| `tests/version.spec.ts` | The web page loads and shows the API's version: the admin's page calls the API's public health check through the generated client, and the page shows the version this run built into the binary. A page that printed a fixed string would fail |

The page under test is the admin's, because it is the one web page that calls the API so far. The member app's web build joins when it has a screen that does; the four journeys in the spec come with the milestones that build them.

## Retries

CI has no retries, except one for Playwright: `retries: 1` in `playwright.config.ts`, the only retry setting in the repo (`scripts/ci-workflow.test.sh` checks both). A test that fails and then passes is reported as flaky, in the run's output and as a warning annotation in CI. The spec's rule is to fix or delete a flaky test in the same milestone.

On a failure, Playwright keeps a trace in `test-results/` (git-ignored), which CI uploads as the `e2e-traces` artifact.
