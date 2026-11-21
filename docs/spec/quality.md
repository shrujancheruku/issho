# The quality bar and test strategy

From [The quality bar and test strategy](https://github.com/shrujancheruku/issho/issues/574), on the seams in [Architecture and stack](architecture.md#the-seams). It says what v2 is built to from its first commit and how that is checked. The build's own mechanics (verify commands, the required checks' names, the review checks, the `risk` paths) are in `docs/agents/orchestration.md`, rewritten to match this document by the first build milestone.

## The kinds of test

| Kind | Covers | Runs |
|---|---|---|
| Package tests | Each backend package through its exported Go API, on real Postgres, with the fake clock and the shared fakes | Every change |
| Unit tests | Pure code only: `matching`, `policy`, the city clock | Every change |
| Property tests | Matching's invariants on generated signups | Every change |
| The season | The rules across weeks and packages | Every change |
| Screen tests | The member app and the admin, rendered against a fake of the generated API client | Every change |
| End-to-end journeys | The real API, the Expo server and the app together in a browser | Every change |
| Model evals | The plan judge and plan translation | Only when a prompt, a model name or an adapter changes |
| The device checklist | What only a phone can show | By hand, before each release |

### Backend

- **A package is tested through its exported Go API.** That is the only seam another package may use, so the tests survive a refactor.
- **No mocked stores.** The rules live in SQL and transactions as much as in Go.
- **The database is never skipped.** One `make test` starts a throwaway Postgres and gives each test its own schema or a rolled-back transaction. It needs Docker, and it has no skip path locally, for build workers or in CI.
- **Built test-first.** A ticket that changes behaviour starts with a failing test through the seam. Its acceptance criteria each name a behaviour a test will prove.

### Matching and the rules

- **Property tests on the pure function.** Signups are generated at random and every result must hold these:
  - every group is 3 to 6;
  - a kept-apart pair is never grouped;
  - every group shares a language;
  - the "only women and non-binary people" setting holds;
  - the seat order is respected;
  - no date falls on a closed day.
- **The season.** v1's simulator returns as a deterministic run of about 60 scripted people and 12 plans over 8 weeks, through the real packages with the fake clock. It includes late cancels, no-shows, reports and account deletions.
  - CI fails if the season ends with a broken invariant.
  - A command prints the season's scorecard numbers (the share of signups that got an outing, people left waiting, pauses), so a change to a `policy` number can be tried before it is made.
  - The season's scripted members and plans also seed the [demo city](cities.md#a-demo-city).

### The apps

- **Screens are rendered against a fake of the generated API client** and assert what a person sees and can do. The app uses Jest and React Native Testing Library; the admin uses Vitest.
- The fake gives every state (waitlisted, paused, on the day) with no backend.
- No snapshot tests. A hook or component is tested alone only when it holds real logic.

### End to end

Playwright runs against the web build on the local stack. There are four journeys:

1. the core loop, from a join link to feedback;
2. a report that leads to a hold;
3. deleting an account;
4. a plan page read without JavaScript.

- The clock is moved by a test-only RPC, which is compiled out of the production image.
- No native end-to-end tool before the soft launch.
- **The device checklist** covers the native-only parts (push, on-device translation, the store build). It is written down and run by hand on a preview build before each release, on one iPhone, one Android phone and desktop Chrome.

### Model output

| Use | How it is evaluated |
|---|---|
| The plan judge | The eval set starts as the twelve plans the owner judged in the [prototype](plan-bar.md#what-the-prototype-showed) and gains every plan the owner reviews. A change may not lower agreement with the owner. The starting model is the one that agrees most. |
| The five accuracy checks | They are code, tested like any other against the fakes. |
| Plan translation | A fixed set of plans is translated and scored twice: by the judge's "says what the original says" check, and by a code check that the fact fields (times, prices, names) come through unchanged. A reader's corrections join the set. |
| The lead search | No offline eval. A contract test on a recorded response checks that a lead holds only our note, its links and a place ID, and that a run returns at most 10. Quality is judged only by the scorecard's share vouched ([Leads](leads.md)). |

Evals cost money and vary between runs, so they run only on a change that can move them. Their output is posted on the PR.

## Privacy as tests

Each of these is a named test, required from the milestone that first builds its subject.

| Promise | Test |
|---|---|
| No personal data reaches a model | The interfaces' types, and a test of each adapter's request |
| Logs and Sentry events carry only the account's ID | The season's log output is searched for its scripted members' emails and names |
| A deleted account leaves only the tombstone's listed fields | The season deletes accounts, and a test reads every table |
| The deletion ledger is replayed after a restore | The restore-test script runs in CI against the season's database |

## What CI requires

Every check fails the PR. A job skipped because its paths are untouched counts as passing.

| Check | What it catches |
|---|---|
| `secrets`, `commits` | A committed secret; a message that isn't a Conventional Commit |
| `codegen` | `make gen` leaves a diff |
| `backend` | Lint, the package tests, the property tests, the season, the privacy tests |
| `boundaries` | A package importing another's internals; a domain package importing an outside service's real implementation; anything but the city clock asking the system for the time |
| `policy` | A number in `policy` with no link to its document; a rule's number written as a literal outside `policy` |
| `rpc-auth` | An RPC with no declared access rule (public, member, Making, or admin with the second step) |
| `retention` | A table holding personal data with no registry entry |
| `proto-breaking` | `buf breaking`, unless the PR is labelled `breaking-ok` |
| `typescript` | Lint, the type check, the screen tests |
| `strings` | A display language missing a key; an unused key |
| `e2e` | The four journeys |
| `image` | The container doesn't build |
| `evals` | Only on prompt, model or adapter paths |

- If the `policy` literal lint proves noisy in the first milestone, it becomes a review check.
- **No coverage threshold.** Coverage is reported on the PR as information. The rule is that a behaviour change ships with a test that fails without it.
- **Flaky tests.** A test that fails without a code cause is fixed or deleted in the same milestone. CI has no retries and main has no skip markers. Playwright alone gets one retry, and the retry is reported.

## Bars that aren't tests

- **Accessibility:** every tappable thing has a label and a 44-point target, and text scales with the device's setting. Lint enforces it where a rule exists, and the per-ticket check otherwise.
- **Languages:** no string is written in a screen file. Dates and times go through one formatter.
- **Performance:** no budget, load test or accessibility audit before the soft launch.

## When a build ticket is done

All of these are in the PR or the ticket's resolution comment.

1. Each acceptance criterion is listed beside the name of the test that proves it.
2. `make gen`, `make secrets-scan` and `make lint test` pass from the worktree root, with the last lines of output pasted.
3. A ticket that changes a screen attaches screenshots of every state it touches: light and dark, Vietnamese and English, at phone width.
4. The directory's README is updated, and `GLOSSARY.md` if a term changed.
5. Anything in the spec the ticket found wrong or unbuildable is a spec PR or a stop. It is never a silent deviation.
6. A "Check in the build's first … ticket" item in the spec is answered in writing by the ticket that reaches it.

## Review

| Layer | When | What |
|---|---|---|
| The check | Per ticket, before it merges into the integration branch | The done list, against the ticket and the spec document it cites |
| The code review | Once per milestone, on the integration PR | Standards and spec, side by side, on a stronger model when the milestone has `risk` tickets |
| The owner | Before merging the integration PR | Reads the review, looks at the screenshots, and walks the milestone on dev from a short script the coordinator writes ("sign in as …, do …, expect …") |

The owner reads code only on the `risk` paths: `policy`, `matching`, `safety`, `retention`, migrations and the RPC access rules.
