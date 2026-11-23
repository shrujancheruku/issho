# e2e

End-to-end tests: Playwright drives a real browser against the real API binary, the web build and a throwaway local stack, started together by `make e2e` ([Quality: End to end](../docs/spec/quality.md#end-to-end)). This is a pnpm workspace package, `@issho/e2e`.

```sh
make e2e   # from the repo root
```

## What it needs

- **Go and `make setup`** (pnpm install), and a network the first time: for Playwright's Chromium and for Docker to pull the images below.
- **Docker, running.** The journeys need a database, an Auth server that mails a code and a mail catcher, so `make e2e` now starts them as **throwaway containers** (the Go tests' Postgres is the same kind of thing). It is the only part of `make e2e` that needs Docker.
- It **never** starts, stops, resets or connects to the developer's local Supabase (`supabase start`, port 54322) or anything hosted, and it reads no `.env*`. The database password, the Auth JWT secret and the Auth signing key are made fresh for each run (in `scripts/e2e.sh`), live only in that process and its containers, and are gone with them. `make secrets-scan` stays clean with no allowlist entry.

## What `make e2e` does

`scripts/e2e.sh` does the work, then removes everything it started:

1. starts four containers on a Docker network of their own, named `issho-e2e-<pid>-*` and labelled with the pid, each published on `127.0.0.1` only, on a port Docker picks:
   - `postgres` (`postgres:17-alpine`, the image the Go tests use), with the role and schema Auth's own migrations expect;
   - `mail`, Mailpit, a mail catcher: Auth sends through its SMTP and the test reads its JSON API;
   - `auth`, Supabase Auth (GoTrue), set up as `supabase/config.toml` sets it for the sign-in (6-digit email code, confirmations on) and sending the repo's own `supabase/templates/code.html`. Its tokens are ES256, signed by a key made for the run and published at the key set the API's verifier reads, and last a year of real time, because the API checks a token's expiry against the clock a journey moves days ahead;
   - `gateway`, nginx, which does what Supabase's gateway does: serves Auth under `/auth/v1`, answers the CORS preflight the app needs and serves the email template to Auth;
2. builds `backend/cmd/api` **with the `e2e` build tag** and a version of its own (`e2e-<pid>`), and `migrate` and `seed`; runs goose up on the throwaway database, and loads the fixture (the city and its two plans);
3. builds the admin (`pnpm --filter @issho/admin build`) and the member app's web export (`expo export`, for its own origin as the web image builds it, with the run's gateway as its Supabase URL and a stand-in publishable key: Auth does not check the key);
4. picks three free ports and runs `playwright test`, whose config starts the API on the first port, `vite preview` of the admin on the second and the Expo server (`apps/web/server.js`) on the third, serving the app's export and forwarding the API's paths, and stops all three at the end;
5. removes the containers and the network (also when interrupted). A run that was killed outright leaves containers whose pid label names a process that is gone; the next run removes them.

The images are pinned by digest at the top of `scripts/e2e.sh`. Renovate does not read a shell script, so a bump is by hand.

`playwright.config.ts` has no defaults for any of these: running `playwright test` by hand fails and says to use `make e2e`.

## The end-to-end API build

The API under test is built with `-tags e2e` (`backend/cmd/api/e2e_on.go`; the production build uses `e2e_off.go`). It differs from the production one in three ways, all compiled out of the production image:

- its clock is a fake that starts at the system's time and is **moved by test-only RPCs**: `issho.e2e.v1.ClockService` (`GetTime`, `SetTime`, `AdvanceTime`; `proto/issho/e2e/v1/clock.proto`);
- its **worker runs in the same process**, on the same fake clock, so one `SetTime` moves what the API and the jobs read. (One process, because a fake clock can only be shared in one; a second process would need its own way to read the API's time.) River's own scheduling of a job for a later instant reads the database's clock, not the fake, so a job scheduled for later does not run when the fake passes it: the journey answers every invitation, so none is waited on;
- **`issho.e2e.v1.FormingService.FormPlan`** inserts the River job `outings.FormArgs` for a plan and returns when the worker has run it. Nothing schedules forming yet (rounds on the real clock are milestone 4's), so the test does what the schedule will.

Each of these RPCs declares an `access` rule like every RPC (`ACCESS_PUBLIC`: the build holds a throwaway database and is never deployed) and `make rpc-auth` checks it. They are in their own proto package, `issho.e2e.v1`, whose generated Go package the production api and worker never import. **A check proves the production image does not hold them:** `backend/cmd/cmd_test.go` (`TestTestOnlyRPCsAreInTheE2EBuildAndNotInProduction`, part of `make test` and CI's `backend` job) builds `api` and `worker` as the Dockerfile does and fails if either holds `issho.e2e.v1`, and builds `api` with the tag to show the check can see it; `scripts/image.sh` (`make image`, CI's `image` job) makes the same search in the binaries inside the built image.

## The tests

| Test | Proves |
|---|---|
| `tests/version.spec.ts` | The web page loads and shows the API's version: the admin's page calls the API's public health check through the generated client, and the page shows the version this run built into the binary. A page that printed a fixed string would fail |
| `tests/app-version.spec.ts` | The member app opened at `/you` signed out (the guard sends it to sign-in, which shows the version), served by the Expo server from the app's web export, shows the same version: the page calls the API on its own origin, the forwarder sends the call on, and the generated client reads the answer |
| `tests/core-loop.spec.ts` | **Journey 1**, from a join link to feedback (below) |

`support/stack.ts` is what the journeys use of the stack: the test-only RPCs (as Connect JSON, straight to the API), the sign-in code read from the mail catcher (a poll with a deadline, never a sleep), a sign-in through Auth's own API for the owner, and the seed (which loads the fixture again, replacing each plan and its signups whole, so a retry starts clean, and admits the owner as `make seed ADMIT=` does).

### Journey 1: the core loop

Three browser contexts at phone width (390 wide), each with its own clock, as three people, with the API's clock moved to match. First the scene: an owner is signed in through Auth's API, admitted, and makes three join links. Then, as steps of one test:

1. **Each arrives by a join link**, is sent to sign in, asks for a code, reads it from the mail catcher, signs in and is let in.
2. **Each opens the plan, gives a profile and signs up** with the first evening on offer, the same for all three.
3. **Forming** (`FormPlan`): each is invited, and sees counts and languages and **no names**: the test asserts the page's text and **the body of every outings response the page received** hold none of the other two names, and `othersGoing` and `checkedInNames` are empty.
4. **Each says yes**; held until the third, whose yes sets the outing.
5. **Those going see who they are going with** (first names, exactly the other two) and "Check-in opens".
6. **The clock moves to a minute before check-in opens**, then to the instant it does: no "I'm here" before, "I'm here" at it; each checks in and sees only those who came before.
7. **The clock moves past the plan's end**: each is asked for feedback, goes through it (the others ticked by default, a pick) and saves; no screen says who picked them.

The browsers' clocks are Playwright's (`context.clock`), moved with the server's, because the app works the check-in and feedback windows out from the device's time.

The owner's **walk script** for the same journey, on the web, the iOS simulator and the Android emulator, is [`docs/walks/core-loop.md`](../docs/walks/core-loop.md).

## Retries

CI has no retries, except one for Playwright: `retries: 1` in `playwright.config.ts`, the only retry setting in the repo (`scripts/ci-workflow.test.sh` checks both). A test that fails and then passes is reported as flaky, in the run's output and as a warning annotation in CI. The spec's rule is to fix or delete a flaky test in the same milestone. Journeys run one at a time (`workers: 1`): they share one stack and one clock.

On a failure, Playwright keeps a trace in `test-results/` (git-ignored), which CI uploads as the `e2e-traces` artifact.

## Not done

- The journey is not run on a simulator or an emulator (no native end-to-end tool before the soft launch). Part 2 of the walk script (forming to feedback) cannot be walked on dev either, until forming is scheduled and a round can run on the real clock.
- The three other journeys (a report that leads to a hold, deleting an account, a plan page read without JavaScript) come with the milestones that build them.
