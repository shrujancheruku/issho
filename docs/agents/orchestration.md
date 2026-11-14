# Orchestration

How a spec gets built here. `/flow:build` follows this file. Tickets are GitHub issues, a build milestone is a GitHub Milestone, and `gh` is the only tracker tool (Orca has no GitHub issue commands).

A build milestone takes its tickets from the active **work parent** (see "Planning rules" in `docs/agents/issue-tracker.md`), in its sub-issue order, and nothing else. Grouping (several tickets in one change, or one ticket split) comes from the parent's or the ticket's body, never from guesswork.

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
| review | Integration `code-review`, once per milestone | sonnet | high | |
| review.risk | Integration `code-review`, milestone has `risk` tickets | opus | | |
| fix | Fixer | sonnet | medium | |

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

Before dispatching a ticket, compare its touch map (the paths in its "What to build" section) with:

- open PRs,
- tickets in progress,
- exclusive holds the founder named.

Standing exclusive hold: at most one ticket in progress adds or edits a file under `backend/db/migrations/`. Goose numbers migrations sequentially, so two would collide.

Overlap means wait. Shared registries (one file per entry plus an index) do not count as overlap.

## Frontier

A ticket is on the frontier when it is open, labelled `ready-for-agent`, unassigned, has no Orca workspace on it (`orca reference find`), and every blocker is closed or labelled `integrated`. An `integrated` blocker counts as done: its work is already on the integration branch, though the issue stays open until the integration PR merges into main.

Blockers are the issue's native dependencies (`gh api repos/shrujancheruku/issho/issues/<n>/dependencies/blocked_by`), or the `Blocked by: #<n>` line at the top of the body where dependencies are off.

`implement-spec` warns that the frontier goes stale because the issues it waits on stay open. Track merges yourself: when a merger lands a ticket, add `integrated` to it and recompute.

## Branches and merging

- A build milestone is a GitHub Milestone. Its tickets carry that milestone.
- One integration branch (`integration/<milestone-slug>`) and one draft PR into main per milestone. Open the draft after the first merge, with `--milestone`.
- The Haiku `check` worker passes a ticket before it merges into the integration branch. A failed check goes back to the implementer; it is not merged.
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
- `adr-conflict`: the ticket contradicts an ADR in `docs/adr/` or the frozen `docs/decisions-2026.md`.
- `human-only`: a step only the founder can take: resetting or migrating `issho-dev` (or any hosted database), `supabase config push`, EAS env writes (`eas env:set`), anything involving secrets (creating, rotating or uploading them), force-push, merging the release-please PR, or adding the `breaking-ok` label.
- `verify-outside-ticket`: proof needs work outside the ticket.
- `already-done`: the work is already on the integration branch.

## Verify

Every implementer, from the worktree root, before reporting done. Leave `TEST_DATABASE_URL` unset: DB integration tests skip there.

1. `make gen` (regenerates code before lint and tests read it)
2. `make secrets-scan`
3. `make lint test`

A ticket whose touch map includes `backend/db/**` or a `store.go` also runs `make test-db`. It starts a throwaway PostGIS container per run (unique name, free host port, removed on exit), so parallel worktrees can't clash. It needs Docker.

Confirm a push landed with `git log --oneline -1 origin/<branch>` before reporting it; don't pipe commit or push output through `tail`, which hides a commitlint rejection.

## Environment

- **Setup per worktree:** `orca.yaml` runs `make setup` (pnpm install + git hooks), and Orca copies `.worktreeinclude` (only `apps/mobile/expo-env.d.ts`, never `.env*`, `supabase/.env`, `supabase/signing_keys.json` or `google-services.json`). Go modules come from the shared cache.
- **Hosted, not local:** the local `.env*` files point at the hosted `issho-dev`, so workers never run `backend/cmd/tool`, `make migrate-up`, `make seed`, `make api` or anything else that connects to a database. The one exception is `make test-db`, which sets its own URLs and never reads `.env*`.
- **Fixed ports:** API `:8080`, admin Vite `5173`, Metro `8081`, local Supabase `54320–54329`. Workers don't start dev servers or local Supabase; verify needs none of them.

## Shared and generated files

The merger resolves conflicts in these by replaying each ticket's lines onto the integration branch; a conflict in any other file goes back to the implementer.

- **Shared:** `backend/README.md`, `docs/architecture.md`, `GLOSSARY.md`, `apps/mobile/README.md`, `apps/admin/README.md`. Edit only the sections for what the ticket changes, as `CLAUDE.md` requires.
- **Generated** (never hand-merged; regenerate after the merge, CI's `codegen` job fails on drift): `backend/gen/**`, `packages/api-client/src/gen/**`, `docs/generated/**`, `backend/internal/uistrings/*.json` and `backend/internal/uistrings/human/**` with `make gen`; `pnpm-lock.yaml` with `pnpm install --lockfile-only`.
- **ADRs:** a ticket that settles a decision worth an ADR (see `docs/agents/domain.md`) adds the next number in `docs/adr/`; if two land with the same number, the second renumbers. Never add rows to `docs/decisions-2026.md`.
- **Release-please files, never edited by hand:** `CHANGELOG.md`, `.release-please-manifest.json`, `release-please-config.json`, `version.txt`, the `package.json` and `apps/mobile/package.json` versions, the `apps/mobile/app.json` version.

## Commits and PRs

- **Commits:** Conventional Commits, scopes from `docs/contributing.md` (`commitlint.config.mjs`, hook `.githooks/commit-msg`). No generated-by footer, Co-Authored-By or session link.
- **Integration PR:** title in Conventional Commits; body from `.github/pull_request_template.md` plus the `Closes #<n>` lines. `main` allows only squash merges.
- **Required checks:** the `main` ruleset's `secrets`, `commits`, `codegen`, `backend`, `typescript`, `image` (strict: the branch must be up to date; a job CI skips for untouched paths counts as passing), plus `proto-breaking` unless the PR is labelled `breaking-ok`. A `breaking-ok` PR waits for the founder's go-ahead even with automerge on.

## Review checks

The `check` worker and the integration review read these as well as the ticket.

- **Instruction files:** `CLAUDE.md`, `apps/mobile/AGENTS.md`, `docs/contributing.md`, `.github/pull_request_template.md`, the README of each directory changed, `GLOSSARY.md` for domain terms.
- **Repo-specific:** `make gen && git diff --exit-code` is clean; `git diff --quiet origin/main...HEAD -- docs/decisions-2026.md`; migrations are new files with a Down; RPC auth is deny-by-default; no `public` schema tables, PostgREST grants or RLS-as-exposure; no secrets or personal data (the repo is public); `buf breaking` passes unless the PR is `breaking-ok`.
- **Hygiene:** comments explain why, not what; a behaviour change ships with a test that fails without it and updates the directory README, `docs/architecture.md` and `GLOSSARY.md`; a hard-to-reverse decision gets an ADR.
- **Post reviews:** the integration review posts each round on the integration PR as one GitHub review with event `COMMENT` (the founder's account can't approve its own PR): verdict first, findings inline. No local paths or session details.

## Profiling

At the end of each milestone, run:

```
python3 ${CLAUDE_PLUGIN_ROOT}/scripts/profile.py --run <run id> --coordinator <coordinator transcript> --pr <integration PR number> --apply
```

Read the HTML it writes. `--pr ... --apply` has the coordinator post the summary as a comment on the integration PR. The profile is unverified until the first real run.

## Slack

Agents write to GitHub only. The GitHub Slack app posts for `shrujancheruku/issho`: `#<prefix>-prs` hears pull requests, reviews and comments, and `#<prefix>-needs-you` hears issues labelled `ready-for-human` or `needs-info`.
