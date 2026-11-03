# Orchestration config

<!-- Read by /orchestrate:run and its agents. One source per rule: point at files that already hold a fact (Makefile, CI, instruction files), and write here only what lives nowhere else. -->

## Backlog

- **Tracker docs:** `docs/agents/issue-tracker.md` (operations, blocking, claiming) and `docs/agents/triage-labels.md` (label strings), from `/setup-matt-pocock-skills`. Takeable tickets are open, `ready-for-agent`, unassigned and unblocked (every GitHub "blocked by" dependency closed), with no open sub-issues and no `wayfinder:` label. Claim = `gh issue edit <n> --add-assignee @me`.
- **Order:** not ascending issue number. Take takeable tickets in `docs/ROADMAP.md` "M10 · Tech debt" order: waves 0 → 1a → 1b → 2 → 3, in line order within a wave. Wave 3 also fills free slots earlier. The section's "How it runs" paragraph is the source for grouping (#198 one PR per linter, #208 + #209 one PR, #163 may split per module).
- **Domain docs:** `docs/agents/domain.md` (`GLOSSARY.md`, and the Decision log in `docs/ROADMAP.md`; there is no `docs/adr/`).

## Slots

- **Slots:** 2.
- **Exclusive holds:** at most one open PR adds or edits a file under `backend/db/migrations/`. Goose numbers migrations sequentially, so two would collide.

Tickets run in parallel only when their touch maps don't overlap outside the shared files.

## Verify

`make gen && make lint test`, from the repo root, for every ticket. Then `make secrets-scan` before pushing. Leave `TEST_DATABASE_URL` unset for those: DB integration tests skip there.

**Database tickets** (a touch map that includes `backend/db/**` or a `store.go`) also run `make test-db`. It starts a throwaway PostGIS container per run (unique name, free host port, removed on exit), so parallel worktrees can't clash. It needs Docker.

## Shared files

- `docs/ROADMAP.md`: each PR marks only its own M10 item, with ✅ right after the item's link and before its size (`[title](…/n) ✅ M`, plus a short note if the ticket asks for one). It never splits or restructures a line, and leaves the line's leading ⬜ alone. It may append a Decision-log row if it makes a decision (exactly three cells: date, decision, why). It never touches "Last updated" or the M10 heading. CI's `roadmap` job (`scripts/check-roadmap.sh`) only checks that the file changed in any non-exempt PR; the own-line rule is a reviewer check. On a conflict, merge `origin/main` in (no rebase) and keep both sides' lines.
- **Orchestrator owns:** "Last updated", the M10 heading status, and a line's leading ⬜ → ✅ once every item on it is done, all in one `docs:` PR at the end of a run.
- Directory READMEs: update the one for the directory changed, as `CLAUDE.md` requires.
- **Release-please files, never edited by hand:** `CHANGELOG.md`, `.release-please-manifest.json`, `release-please-config.json`, `version.txt`, `package.json` and `apps/mobile/package.json` versions, `apps/mobile/app.json` version.
- **Generated, never merged by hand:** `backend/gen/**`, `packages/api-client/src/gen/**`, `docs/generated/**`, `backend/internal/uistrings/*.json`. After a rebase or merge, take either side, run `make gen`, and commit the result. CI's `codegen` job fails on drift.

## PRs and merging

- **Branch:** `<lane>/<number>-<slug>`, where lane is one of `ci`, `db`, `api`, `backend`, `mobile`, `admin` (the area the ticket's M10 line names).
- **Commits:** Conventional Commits, scopes from `docs/contributing.md` (`commitlint.config.mjs`, hook `.githooks/commit-msg`).
- **PR title:** Conventional Commits. **Body:** `.github/pull_request_template.md`, with `Closes #<n>` (each issue for a grouped PR). No generated-by footer, Co-Authored-By or session link.
- **Method:** squash (the `main` ruleset allows only squash).
- **Orchestrator merges:** yes, except the merge gates below. Rule: the `main` ruleset's required checks green (`roadmap`, `secrets`, `commits`, `codegen`, `backend`, `typescript`, `image`; strict, so the branch must be up to date), plus `proto-breaking` unless the PR is labelled `breaking-ok`; every dependency merged; reviewer approved.

## Gates

- **Start gates:** none.
- **Merge gates:** any PR labelled `breaking-ok`, and the PRs closing #169 and #178 (breaking changes). They wait for the owner's go-ahead.

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
| Worker | sonnet | medium |
| Scout, mechanic | haiku | low |
| Reviewer | S haiku · M sonnet · L sonnet, then an orchestrator read | medium |

## Review

- **Instruction files:** `CLAUDE.md`, `apps/mobile/AGENTS.md`, `docs/contributing.md`, `.github/pull_request_template.md`, the README of each directory changed, `GLOSSARY.md` for domain terms.
- **Risk paths** (any PR touching them is reviewed as L): `backend/db/migrations/**`, `proto/**`, `backend/internal/identity/**`, `backend/internal/platform/auth/**`, `config/**`, `.kamal/**`, `.github/workflows/deploy-*.yml`, `backend/Dockerfile`, `supabase/config.toml`, `apps/*/wrangler.jsonc`.
- **Tiers:** S is at most 50 changed lines in at most 3 files, none on a risk path; L touches a risk path or changes more than 400 lines; M is everything else. The S/M/L after the ticket's link on its M10 line overrides (the repo has no `size:` labels).
- **Repo-specific checks:** `make gen` leaves no diff; ROADMAP changed only on the ticket's own M10 line (plus any Decision-log entry); migrations are new files with a Down; RPC auth is deny-by-default; no `public` schema tables, PostgREST grants or RLS-as-exposure; no secrets or personal data; `buf breaking` passes unless the PR is `breaking-ok`.
- **Post reviews:** yes. Every round is posted on its PR as one GitHub review with event `COMMENT` (the owner's account can't approve its own PR): the verdict first in the body, findings inline on diff lines, anything off the diff in the body. Reviewers are read-only (the plugin's guard blocks GitHub writes), so the reviewer returns the review as JSON and the orchestrator checks the line numbers and posts it. Public repo: no local paths or session details.
