# e2e

End-to-end tests: Playwright drives a real browser against the real API binary and the web build, started together by `make e2e` ([Quality: End to end](../docs/spec/quality.md#end-to-end)). This is a pnpm workspace package, `@issho/e2e`.

```sh
make e2e   # from the repo root
```

Needs Go, `make setup` (pnpm install) and a network the first time, to download Playwright's Chromium. It needs neither Docker nor a database, and it never starts local Supabase.

## What `make e2e` does

`scripts/e2e.sh` does the work, then stops everything it started:

1. picks three free ports (never 8080, 8081 or 5173, so it runs beside a developer's servers);
2. builds `backend/cmd/api` with a version of its own (`e2e-<pid>`);
3. builds the admin (`pnpm --filter @issho/admin build`) and the member app's web export (`expo export`, for its own origin, as the web image builds it);
4. runs `playwright test`, whose config starts the API on the first port, `vite preview` of the admin on the second, with its proxy sending the API's paths to the API, and the Expo server (`apps/web/server.js`) on the third, serving the app's export and forwarding the API's paths, and stops all three at the end.

`playwright.config.ts` has no defaults for the ports, the binary or the app's export: running `playwright test` by hand fails and says to use `make e2e`.

## The tests

| Test | Proves |
|---|---|
| `tests/version.spec.ts` | The web page loads and shows the API's version: the admin's page calls the API's public health check through the generated client, and the page shows the version this run built into the binary. A page that printed a fixed string would fail |
| `tests/app-version.spec.ts` | The member app opened at `/you` signed out (the guard sends it to sign-in, which shows the version), served by the Expo server from the app's web export with stand-in Supabase values that nothing calls, shows the same version: the page calls the API on its own origin, the forwarder sends the call on, and the generated client reads the answer |

The admin's page and the member app's You tab are the two web pages that call the API so far, so both are covered: the second goes through the Expo server and its forwarder, as deployed. The four journeys in the spec come with the milestones that build them.

## Retries

CI has no retries, except one for Playwright: `retries: 1` in `playwright.config.ts`, the only retry setting in the repo (`scripts/ci-workflow.test.sh` checks both). A test that fails and then passes is reported as flaky, in the run's output and as a warning annotation in CI. The spec's rule is to fix or delete a flaky test in the same milestone.

On a failure, Playwright keeps a trace in `test-results/` (git-ignored), which CI uploads as the `e2e-traces` artifact.
