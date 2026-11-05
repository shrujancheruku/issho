# Orchestration config

<!-- Read by /orchestrate:run and its agents. One source per rule: point at files that already hold a fact (Makefile, CI, instruction files), and write here only what lives nowhere else. -->

## Backlog

- **Tracker docs:** `docs/agents/issue-tracker.md` (operations, blocking, claiming) and `docs/agents/triage-labels.md` (label strings), from `/setup-matt-pocock-skills`. Takeable tickets are open, `ready-for-agent`, unassigned and unblocked (every GitHub "blocked by" dependency closed), with no open sub-issues and no `wayfinder:` label. Claim = `gh issue edit <n> --add-assignee @me`.
- **Order:** a run targets one parent issue (the user names it). Take takeable tickets in the parent's sub-issue order; blocking edges come first. Without a parent, ascending issue number. Grouping (several tickets in one PR, or one ticket split into several) comes from the parent's or the ticket's body, never from guesswork.
- **Domain docs:** `docs/agents/domain.md` (`GLOSSARY.md`, ADRs in `docs/adr/`, and the frozen `docs/decisions-2026.md`).

## Slots

- **Slots:** 2.
- **Exclusive holds:** at most one open PR adds or edits a file under `backend/db/migrations/`. Goose numbers migrations sequentially, so two would collide.

Tickets run in parallel only when their touch maps don't overlap outside the shared files.

## Verify

`make gen && make lint test`, from the repo root, for every ticket. Then `make secrets-scan` before pushing. Leave `TEST_DATABASE_URL` unset for those: DB integration tests skip there.

**Database tickets** (a touch map that includes `backend/db/**` or a `store.go`) also run `make test-db`. It starts a throwaway PostGIS container per run (unique name, free host port, removed on exit), so parallel worktrees can't clash. It needs Docker.

## Shared files

A **shared file** is one many PRs edit. The `merge` script resolves collisions in the files listed here by replaying each PR's lines onto main, so nobody merges them by hand; a conflict in any other file is handed to the worker.

- **Shared files:**
  - `backend/README.md`: edit only the sections for what the PR changes (endpoints, setup, testing), as `CLAUDE.md` requires. Other directory READMEs follow the same rule but aren't replayed.
- **Generated:** CI's `codegen` job fails on drift.
  - `backend/gen/**`: `make gen`
  - `packages/api-client/src/gen/**`: `make gen`
  - `docs/generated/**`: `make gen`
  - `backend/internal/uistrings/*.json`: `make gen`
  - `backend/internal/uistrings/human/**`: `make gen`
- **Orchestrator-owned edits:** the parent issue's body, and closing it once every sub-issue is closed.

Not shared, but every PR follows these:
- `docs/adr/`: a PR that settles a decision worth an ADR (see `docs/agents/domain.md`) adds the next number. Two PRs in one run can pick the same number; the second to merge renumbers. Never add rows to `docs/decisions-2026.md`.
- **Release-please files, never edited by hand:** `CHANGELOG.md`, `.release-please-manifest.json`, `release-please-config.json`, `version.txt`, `package.json` and `apps/mobile/package.json` versions, `apps/mobile/app.json` version.

## PRs and merging

- **Branch:** `<lane>/<number>-<slug>`, where lane is one of `ci`, `db`, `api`, `backend`, `mobile`, `admin` (from the scope in the ticket's title, e.g. `refactor(mobile)` → `mobile`; a scope with no lane of its own takes the closest, such as `notifications` → `backend`).
- **Commits:** Conventional Commits, scopes from `docs/contributing.md` (`commitlint.config.mjs`, hook `.githooks/commit-msg`).
- **PR title:** Conventional Commits. **Body:** `.github/pull_request_template.md`, with `Closes #<n>` (each issue for a grouped PR). No generated-by footer, Co-Authored-By or session link.
- **Method:** squash (the `main` ruleset allows only squash).
- **Orchestrator merges:** yes, except the merge gates below. Rule: the `main` ruleset's required checks green (`secrets`, `commits`, `codegen`, `backend`, `typescript`, `image`; strict, so the branch must be up to date; a job CI skips because the PR doesn't touch its paths reports as skipped, which counts as passing), plus `proto-breaking` unless the PR is labelled `breaking-ok`; every dependency merged; reviewer approved.

## Gates

- **Start gates:** none.
- **Merge gates:** any PR labelled `breaking-ok` (a breaking change). It waits for the owner's go-ahead.

## Human-only steps

Workers stop and report when they reach one:
- resetting or migrating `issho-dev` (or any hosted database)
- `supabase config push`
- EAS env writes (`eas env:set`)
- anything involving secrets: creating, rotating or uploading them
- force-push
- merging the release-please PR

## Environment

- **Setup per worktree:** `make setup` (pnpm install + git hooks). Go modules come from the shared cache.
- **Hosted, not local:** the local `.env*` files point at the hosted `issho-dev`, so workers never run `backend/cmd/tool`, `make migrate-up`, `make seed`, `make api` or anything else that connects to a database. The one exception is `make test-db`, which starts its own container, sets its own URLs and never reads `.env*`.
- **Collisions:** fixed ports (API `:8080`, admin Vite `5173` strictPort, Metro `8081`, local Supabase `54320–54329` under one `project_id`, one shared local database on `54322`). Workers don't start dev servers or local Supabase. Verify needs none of them.
- **`.worktreeinclude`** carries only `apps/mobile/expo-env.d.ts`, which is not a secret. It never carries `.env*`, `supabase/.env`, `supabase/signing_keys.json` or `google-services.json`.

## Models

| Role | Model | Effort |
|---|---|---|
| Orchestrator | the session's model | |
| Worker | sonnet; haiku for S tickets (at most 3 files, the **Tiers** file limit) | medium |
| Scout | haiku | low |
| Reviewer | S haiku · M sonnet · L sonnet (high effort) | medium |

## Review

- **Instruction files:** `CLAUDE.md`, `apps/mobile/AGENTS.md`, `docs/contributing.md`, `.github/pull_request_template.md`, the README of each directory changed, `GLOSSARY.md` for domain terms.
- **Risk paths** (any PR touching them is reviewed as L): `backend/db/migrations/**`, `backend/internal/identity/**`, `backend/internal/platform/auth/**`, `config/**`, `.kamal/**`, `.github/workflows/deploy-*.yml`, `backend/Dockerfile`, `supabase/config.toml`, `apps/*/wrangler.jsonc`, `site/wrangler.jsonc`, `scripts/deploy-web.sh`, `.github/workflows/ci.yml`, `scripts/ci-changes.sh` (a wrong path filter silently skips required checks). `proto/**` is left out: `buf breaking` and the `breaking-ok` merge gate cover it.
- **Tiers:** S is at most 50 changed lines in at most 3 files, none on a risk path; L touches a risk path or changes more than 400 lines; M is everything else. A `size:S|M|L` label on the ticket overrides.
- **Repo-specific checks:** `make gen` leaves no diff (run: `make gen && git diff --exit-code`); `docs/decisions-2026.md` untouched (run: `git diff --quiet origin/main...HEAD -- docs/decisions-2026.md`); migrations are new files with a Down; RPC auth is deny-by-default; no `public` schema tables, PostgREST grants or RLS-as-exposure; no secrets or personal data; `buf breaking` passes unless the PR is `breaking-ok`.
- **Post reviews:** yes. The reviewer posts each round on its PR as one GitHub review with event `COMMENT` (the owner's account can't approve its own PR): the verdict first in the body, findings inline on diff lines, anything off the diff in the body. Public repo: no local paths or session details.
