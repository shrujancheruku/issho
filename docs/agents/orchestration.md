# Orchestration

How a spec gets built here. `/flow:build` follows this file. Tickets are GitHub issues, a build milestone is a GitHub Milestone, and `gh` is the only tracker tool (Orca has no GitHub issue commands).

A build milestone takes its tickets from the active **work parent** (see "Planning rules" in `docs/agents/issue-tracker.md`), in its sub-issue order, and nothing else. Grouping (several tickets in one change, or one ticket split) comes from the parent's or the ticket's body, never from guesswork.

The bar a ticket meets is [The quality bar and test strategy](../spec/quality.md); the order is [The build order](../spec/build-order.md).

## Layers

- **Procedure:** Matt Pocock's `/implement-spec`, unmodified. Read its `SKILL.md` and follow it.
- **Policy:** this file. Where it and `implement-spec` differ, this file wins.
- **Executor:** Orca. The implementer, merger, checker, fixer and reviewer are Orca workers. Short read-only jobs (exploration, `code-review`'s two reviewers) are subagents inside a worker. On an agent with no subagents, they become their own Orca workers.
- **Tracker:** GitHub Issues. See `docs/agents/issue-tracker.md`.
- **Measurement:** `${CLAUDE_PLUGIN_ROOT}/scripts/profile.py`.

`${CLAUDE_PLUGIN_ROOT}` is the installed `flow` plugin's folder. `/flow:build` expands it; outside it, run `claude plugin list` to find the path.

## Model routing

Before every worker, run `python3 ${CLAUDE_PLUGIN_ROOT}/scripts/route.py --role <role> --ticket <n> --lookup` and pass the `flags` (`--agent`, `--model`, `--effort`) and `task_title` it prints. Read the row from this table each time; never route from memory. The Model column names a profile in `models` in `docs/agents/flow.json`, which says which Orca agent and model id it runs on. Always pass `--model`: the user's own settings default to Opus.

| Key | Role | Model | Effort | Notes |
|---|---|---|---|---|
| coordinator | Coordinator | opus | medium | The session running `/flow:build`. |
| scout | Exploration or scout | haiku | low | A subagent inside a worker. |
| build.S | Implementer, size S, no `risk` | haiku | | Escalates once; see Escalation. |
| build.S.risk | Implementer, size S, `risk` | sonnet | medium | |
| build.M | Implementer, size M | sonnet | medium | |
| build.M.risk | Implementer, size M, `risk` | sonnet | high | |
| build.L | Implementer, size L | sonnet | high | |
| build.L.risk | Implementer, size L, `risk` | opus | | |
| build.escalated | Implementer after an escalation | sonnet | medium | A fresh worker on the same branch. |
| merge | Merger | haiku | low | |
| merge.conflict | Merger on a real conflict | sonnet | medium | |
| check | Spec check before a ticket merges into the integration branch | haiku | low | One per ticket. |
| review | Integration `code-review`, once per milestone | opus | | The same model as `review.risk`; both rows stay so `--risk` resolves. |
| review.risk | Integration `code-review`, milestone has `risk` tickets | opus | | The same model as `review`. |
| fix | Fixer | opus | | Fixes the integration review's findings, so it runs on the reviewer's model. |

Size comes from the issue's `size:S`, `size:M` or `size:L` label. The `risk` label, or a ticket that names a path in `risk_paths` in `docs/agents/flow.json`, raises the model to the `.risk` row. A ticket with no size label routes as M and `route.py` warns; run `estimate.py propose` for the founder to approve sizes.

For `review`, pass the integration PR's number as `--ticket`.

Put the model in the Orca task title: `#123 · build · haiku`. The profiler reads it.

## Starting a worker

1. `orca reference find https://github.com/shrujancheruku/issho/issues/<n> --json` first. If a workspace or agent is already attached to the issue, resume or read it instead of starting another. (Orca wants the issue URL; a bare number is not supported.)
2. `orca worktree create --issue <n> --name "<n>-<slug>" --base-branch integration/<milestone-slug> --no-parent --json` makes the ticket's worktree and links it to the issue.
3. `orca orchestration worker-start --worktree issue:<n> --task-title "<task_title from route.py>" <flags from route.py> --spec-file -`, with the brief on stdin. The brief holds context pointers: the issue URL, the spec, the exploration notes, the integration branch.

The merger, checker, fixer and reviewer start the same way: a `--task-title` of the shape `#<n> · <role> · <model>`, and the flags from `route.py`.

## Escalation

1. An S implementer on Haiku escalates once when any of these happens:
   - it stops and the stop cannot be resolved by an answer;
   - CI fails on its branch a second time;
   - review asks for changes a second time.
2. Start a fresh worker with `route.py --role build --variant escalated` on the same branch. Tell it what the first worker tried.
3. A second escalation goes to the founder: comment on the issue, add `needs-info`, remove `ready-for-agent`.

## Holds

Before dispatching a ticket, compare its touch map (the paths in its "Expected paths" section) with:

- open PRs,
- tickets in progress,
- exclusive holds the founder named.

Standing exclusive holds:

- At most one ticket in progress adds or edits a file under `backend/db/migrations/`. Goose numbers migrations sequentially, so two would collide.
- **The native slot:** at most one worker at a time runs anything on the iOS simulator, the Android emulator or Metro. Every worktree shares the simulator, the emulator and Metro's port 8081, so two at once interfere with each other (#602, #630).

Overlap means wait. Shared registries (one file per entry plus an index) do not count as overlap.

## Frontier

A ticket is on the frontier when it is open, labelled `ready-for-agent`, unassigned, has no Orca workspace on it (`orca reference find`), and every blocker is closed or labelled `integrated`. An `integrated` blocker counts as done: its work is already on the integration branch, though the issue stays open until the integration PR merges into main.

Blockers are the issue's native dependencies (`gh api repos/shrujancheruku/issho/issues/<n>/dependencies/blocked_by`), or the `Blocked by: #<n>` line at the top of the body where dependencies are off.

`implement-spec` warns that the frontier goes stale because the issues it waits on stay open. Track merges yourself: when a merger lands a ticket, add `integrated` to it and recompute.

## Branches and merging

- A build milestone is a GitHub Milestone. Its tickets carry that milestone.
- One integration branch (`integration/<milestone-slug>`) and one draft PR into main per milestone. Open the draft after the first merge, with `--milestone`. The Foundation milestone's branch is `integration/foundation`.
- The Haiku `check` worker passes a ticket before it merges into the integration branch. A failed check goes back to the implementer; it is not merged.
- Before a ticket merges, the merger merges the integration branch's tip into the ticket branch, runs the verify commands on the merged tree and pushes without force. A red merged tree goes back to the implementer, who fixes it on the ticket branch (the Foundation's `make lint` break after #596 was fixed that way, #597).
- A ticket's done-list proof lives in a resolution comment on the ticket: each criterion beside the test that proves it, and the verbatim last lines of each verify command. Foundation tickets have no PR of their own; [When a build ticket is done](../spec/quality.md#when-a-build-ticket-is-done) accepts a resolution comment in place of one.
- A ticket merged into the integration branch gets the `integrated` label (`gh issue edit <n> --add-label integrated`). Do not close it.
- The integration PR body has one `Closes #<n>` line per ticket, kept current as tickets land. When the founder merges the PR into main, GitHub closes those tickets.
- The founder merges integration PRs.

## Automerge

Off. The coordinator never merges an integration PR on its own.

The founder may turn it on for one run by telling the coordinator. Then the coordinator merges the integration PR when CI is green and the integration review passed. It ends with that run.

## Usage limits

On the first usage-limit error from any worker:

1. Stop dispatching.
2. Comment "Paused: usage limit, resets at <time>" on each ticket in progress.
3. Resume after the reset.

Wait out the limit on the same models. Switching model to dodge a limit breaks the profile.

## Stops

A worker stops and reports with `orca orchestration send --type escalation` for one of these reasons:

- `held-path`: the ticket needs a path someone else holds.
- `no-spec`: the ticket has no acceptance criteria.
- `ambiguous`: the ticket reads two ways.
- `adr-conflict`: the ticket contradicts an ADR in `docs/adr/`.
- `human-only`: a step only the founder can take: resetting or migrating `issho-dev` (or any hosted database), `supabase config push`, EAS env writes (`eas env:set`), anything involving secrets (creating, rotating, reading or uploading them), force-push, merging the release-please PR, or adding the `breaking-ok` label.
- `verify-outside-ticket`: proof needs work outside the ticket.
- `already-done`: the work is already on the integration branch.

## Verify

Every implementer, from the worktree root, before reporting done:

1. `make gen`, then `git diff --exit-code` must print nothing. Regeneration comes first, so lint and tests read current code.
2. `make secrets-scan`
3. `make lint` and `make test`. Each is a superset of its CI jobs; the required checks below are what CI runs.
4. `make setup` still works. A ticket that breaks it is not done.

Notes:

- **The database is never skipped.** `make test` starts its own throwaway Postgres 17 in Docker for each test binary and gives each test a fresh schema, or a database of its own for a migration test. It fails with Docker stopped, and no flag or variable skips it. Docker must be running.
- **`make lint` and `buf breaking` need `origin/main`.** `make lint` compares the proto with it and `make commits` reads the branch's commits from it. Fetch it first (`git fetch origin main`); with it missing, both fail and say so.
- **`buf breaking` prints "not run" until main holds v2's proto.** That is expected until the integration PR merges. No flag, variable or label switches it off locally; the `breaking-ok` label skips only CI's `proto-breaking` job (#595, #603).
- Confirm a push landed with `git log --oneline -1 origin/<branch>` before reporting it; don't pipe commit or push output through `tail`, which hides a commitlint rejection.

### The required checks

These are the 14 names set as required in the `main` ruleset (by the founder). Each job's id is its name, and each runs the make target of the same name in `.github/workflows/ci.yml`:

`secrets`, `commits`, `codegen`, `backend`, `boundaries`, `policy`, `rpc-auth`, `retention`, `proto-breaking`, `typescript`, `strings`, `e2e`, `native-bundle`, `image`.

A job skipped because its paths are untouched counts as passing. CI can only be seen running on the integration PR (#603): a workflow change is proven there, not on the branch.

## Environment

- **Setup per worktree:** `orca.yaml` runs `make setup` (pnpm install and the git hooks). `.worktreeinclude` copies nothing yet. Never list `.env*`, `supabase/.env` or `supabase/signing_keys.json` in it. Go modules come from the shared cache.
- **Local stack, never:** workers never start local Supabase, never run `make migrate-up` or `make migrate-down` against a local stack, never start a dev server, and never read `.env*`. Verify needs none of them.
- **Hosted, never from a worker:** workers never connect to `issho-dev` or any other hosted service, never deploy, and never read a secret's value from their machine. Secrets are read by CI at deploy time from Google Secret Manager (see `docs/secrets.md`).
- **Dev deploys from CI only:** `.github/workflows/deploy-dev.yml` deploys on a push to `main` or to `integration/foundation`, and on a manual run. The deploy identity signs in by Workload Identity Federation, and its sign-in is restricted to those two refs (#605). Each new milestone's integration branch is added to that condition by the founder, who is the one who can.
- **Native machine:** one native worker at a time (see Holds). The build machine needs Xcode with an iOS simulator runtime, JDK 17, and the Android SDK with platform-tools, the emulator, a platform, build-tools, a system image and at least one AVD, with `ANDROID_HOME` set (#629). iOS builds use CocoaPods, the decision recorded on #630. CocoaPods was not installed on the build machine when #629 was done, so the founder installs it before an iOS build runs.
- **Fixed ports:** the API listens on `:8080` by default, Metro on `8081`, and local Supabase's database on `54322`. Workers hold none of them; see "Local stack, never".

## Shared and generated files

Several tickets can add lines to the same shared file at once. Each adds its lines in its own block, without reordering or reformatting what is there, so the merger can keep both sides. The merger resolves conflicts in these by replaying each ticket's lines onto the integration branch; a conflict in any other file goes back to the implementer.

- **Shared:** the root `Makefile`, `README.md`, `backend/README.md`, `GLOSSARY.md`, `apps/app/README.md`, `apps/admin/README.md`. Edit only the sections for what the ticket changes, as `CLAUDE.md` requires.
- **Generated** (never hand-merged; regenerate after the merge, since CI's `codegen` job fails on drift): `backend/gen/**` and `packages/api-client/src/gen/**` with `make gen`; `backend/go.sum` with `go mod tidy` in `backend/`; `pnpm-lock.yaml` with `pnpm install --lockfile-only`.
- **ADRs:** a ticket that settles a decision worth an ADR (see `docs/agents/domain.md`) adds the next number in `docs/adr/`; if two land with the same number, the second renumbers.
- **Release-please files, never edited by hand:** `CHANGELOG.md`, `.release-please-manifest.json`, `release-please-config.json`, `version.txt`, the `package.json` and `apps/app/package.json` versions, and the `apps/app/app.json` version.

## Commits and PRs

- **Commits:** Conventional Commits, with the scope enum in `commitlint.config.mjs`. The hook `.githooks/commit-msg` runs the same rules. No generated-by footer, Co-Authored-By or session link.
- **Integration PR:** title in Conventional Commits; body from `.github/pull_request_template.md`, plus the `Closes #<n>` lines and the list of required checks. Note in the body that `proto-breaking` prints "not run" until main holds v2's proto. `main` allows only squash merges.
- **Required checks:** the 14 names above, on the `main` ruleset, strict: the branch must be up to date. `proto-breaking` is skipped by the `breaking-ok` label. A `breaking-ok` PR waits for the founder's go-ahead even with automerge on.

## Review checks

The `check` worker and the integration review read these as well as the ticket and the spec document it cites.

- **Instruction files:** `CLAUDE.md`, `.github/pull_request_template.md`, the README of each package directory the ticket changes (one per package directory, not per intermediate directory: #598), and `GLOSSARY.md` for domain terms.
- **Done list:** [When a build ticket is done](../spec/quality.md#when-a-build-ticket-is-done). Each criterion is listed beside its test; the verify commands' last lines are pasted; a screen change attaches each state, light and dark, Vietnamese and English, at phone width, from the web and from the simulator and emulator (see [Native along the way](../spec/quality.md#native-along-the-way)); the directory README is updated; anything the spec gets wrong is a spec PR or a stop, never a silent deviation.
- **Repo-specific:** `make gen` leaves no diff; migrations are new files with a working `Down`; every RPC declares an `access` rule, and the interceptor denies by default; no `public` schema tables, PostgREST grants or RLS-as-exposure; no secrets or personal data (the repo is public); `buf breaking` passes unless the PR is `breaking-ok`.
- **Risk paths:** `policy`, `matching`, `safety`, `retention`, migrations, the RPC access rules (`proto/**` and `backend/internal/rpc/**`), CI and deploy, as listed in `docs/agents/flow.json`. The owner reads code on these.
- **Hygiene:** comments explain why, not what; a behaviour change ships with a test that fails without it; a hard-to-reverse decision gets an ADR.
- **Post reviews:** the integration review posts each round on the integration PR as one GitHub review with event `COMMENT` (the founder's account can't approve its own PR): verdict first, findings inline. No local paths or session details.

## Profiling

At the end of each milestone, run:

```
python3 ${CLAUDE_PLUGIN_ROOT}/scripts/profile.py --run <run id> --coordinator <coordinator transcript> --pr <integration PR number> --apply
```

Read the HTML it writes. `--pr ... --apply` has the coordinator post the summary as a comment on the integration PR. The profile is unverified until the first real run.

## Slack

Agents write to GitHub only. The GitHub Slack app posts for `shrujancheruku/issho`: `#<prefix>-prs` hears pull requests, reviews and comments, and `#<prefix>-needs-you` hears issues labelled `ready-for-human` or `needs-info`.
