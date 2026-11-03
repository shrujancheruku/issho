# Orchestration config

<!-- Read by /orchestrate:run and its agents. One source per rule: point at files that already hold a fact (Makefile, CI, instruction files), and write here only what lives nowhere else. -->

## Backlog

- **Sequence:** `docs/ROADMAP.md`, section "M10 · Tech debt": waves 0 → 1a → 1b → 2 → 3, in line order within a wave. Wave 3 also fills free lanes earlier. The section's "How it runs" paragraph is the source for grouping (#198 one PR per linter, #208 + #209 one PR, #163 may split per module).
- **Tracker:** GitHub Issues, shrujancheruku/issho (`docs/agents/issue-tracker.md`). Blocking = GitHub's native "blocked by" dependencies (`issue_dependencies_summary.blocked_by`), mirrored by a `## Blocked by` body section. Start a ticket only when every blocker is closed. Claim = `gh issue edit <n> --add-assignee @me`; an assigned ticket is taken.
- **Sizes:** the S/M/L after each link in the M10 wave lines. A ticket with no size is M.

## Lanes

A ticket's lane is the one its M10 wave line names (cities → backend, lint/CI and docs → ci, tests → the lane of the code under test, mobile/api → api if it changes `proto/`, else mobile). Paths are each lane's home, not a fence.

| Lane | Paths | Notes |
|---|---|---|
| ci | `.github/**`, `scripts/**`, `config/**`, `.kamal/secrets.example`, `Makefile`, `turbo.json`, `pnpm-workspace.yaml`, `go.work*`, `commitlint.config.mjs`, `.gitleaks.toml`, `release-please-config.json`, `backend/Dockerfile`, `supabase/config.toml`, `apps/*/wrangler.jsonc`, linter and Renovate config | |
| db | `backend/db/migrations/**`, `backend/db/queries/**`, `backend/sqlc.yaml`, `backend/gen/dbgen/**` | **Exclusive while #160 is open:** no other PR touches `backend/db/migrations` or `backend/db/queries`. |
| api | `proto/**`, `backend/gen/issho/**`, `packages/api-client/**`, `docs/generated/**`, RPC handlers and `backend/internal/platform/errs` | |
| backend | `backend/internal/**`, `backend/cmd/**`, `data/citypacks/**` | |
| mobile | `apps/mobile/**` | Read `apps/mobile/AGENTS.md`. |
| admin | `apps/admin/**` | |

At most one open PR per lane.

## Verify

`make gen && make lint test`, from the repo root, every lane. Then `make secrets-scan` before pushing. Leave `TEST_DATABASE_URL` unset for those: DB integration tests skip there and run in CI's `backend` job.

**Database tickets** (db lane, or a touch map that includes `backend/db/**` or a `store.go`) also run `make test-db` once #211 has merged. It runs the integration tests on a throwaway container of its own, so parallel worktrees can't clash (`testfixture` uses stable ids, so two runs sharing one database delete each other's rows). Until #211 merges, CI is the only place they run.

## Shared files

- `docs/ROADMAP.md`: each PR marks only its own M10 item, with ✅ right after the item's link and before its size (`[title](…/n) ✅ M`, plus a short note if the ticket asks for one). It never splits or restructures a line, and leaves the line's leading ⬜ alone. It may append a Decision-log row if it makes a decision (exactly three cells: date, decision, why). It never touches "Last updated" or the M10 heading. CI's `roadmap` job (`scripts/check-roadmap.sh`) needs the file to change in any code PR. On a conflict, merge `origin/main` in (no rebase) and keep both sides' lines.
- **Orchestrator owns:** "Last updated", the M10 heading status, and a line's leading ⬜ → ✅ once every item on it is done, all in one `docs:` PR at the end of a run.
- Directory READMEs: update the one for the directory changed, as `CLAUDE.md` requires.
- **Release-please files, never edited by hand:** `CHANGELOG.md`, `.release-please-manifest.json`, `version.txt`, `package.json` and `apps/mobile/package.json` versions, `apps/mobile/app.json` version.
- **Generated, never merged by hand:** `backend/gen/**`, `packages/api-client/src/gen/**`, `docs/generated/**`, `backend/internal/uistrings/*.json`. After a rebase, take either side, run `make gen`, and commit the result. CI's `codegen` job fails on drift.

## PRs and merging

- **Branch:** `<lane>/<number>-<slug>`
- **Commits:** Conventional Commits, scopes from `docs/contributing.md` (`commitlint.config.mjs`, hook `.githooks/commit-msg`).
- **PR title:** Conventional Commits. **Body:** `.github/pull_request_template.md`, with `Closes #<n>` (each issue for a grouped PR). No generated-by footer, Co-Authored-By or session link.
- **Method:** squash
- **Orchestrator merges:** yes, except the merge gates below. Rule: every `ci.yml` job green (main has no branch protection, so check them yourself), every dependency merged, branch up to date, reviewer approved.

## Gates

- **Start gates:** none.
- **Merge gates:** #160 (the owner resets `issho-dev`), and the `breaking-ok` PRs #166, #169, #177, #178. They wait for the owner's go-ahead.

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
- **Hosted, not local:** the local `.env*` files point at the hosted `issho-dev`, so workers never run `backend/cmd/tool`, `make migrate-up`, `make seed` or anything else that connects to a database. The one exception is `make test-db` (#211), which starts its own throwaway container, sets its own URLs and never reads `.env*`.
- **Collisions:** fixed ports (API `:8080`, admin Vite `5173` strictPort, Metro `8081`, local Supabase `54320–54329` under one `project_id`). Workers don't start dev servers or local Supabase. Verify needs none of them.
- **`.worktreeinclude`** carries only `apps/mobile/expo-env.d.ts`, which is not a secret. It never carries `.env*`, `supabase/.env` or `supabase/signing_keys.json`.

## Models

| Role | Model | Effort |
|---|---|---|
| Orchestrator | the session's model | |
| Worker | sonnet | medium |
| Scout, mechanic | haiku | low |
| Reviewer | S haiku · M sonnet · L sonnet, then an orchestrator read | medium |

## Review

- **On the PR:** every review is posted on its PR as one GitHub review with event `COMMENT` (the owner's account can't approve its own PR): the verdict first in the body, findings inline on diff lines, anything off the diff in the body. Reviewers are read-only (the plugin's guard blocks GitHub writes), so the reviewer returns the review as JSON and the orchestrator checks the line numbers and posts it. Public repo: no local paths or session details.
- **Instruction files:** `CLAUDE.md`, `apps/mobile/AGENTS.md`, `docs/contributing.md`, `.github/pull_request_template.md`, the README of each directory changed, `GLOSSARY.md` for domain terms.
- **Risk paths** (any PR touching them is reviewed as L): `backend/db/migrations/**`, `proto/**`, `backend/internal/identity/**`, `backend/internal/platform/auth/**`, `config/**`, `.kamal/**`, `.github/workflows/deploy-*.yml`, `backend/Dockerfile`, `supabase/config.toml`, `apps/*/wrangler.jsonc`.
- **Repo-specific checks:** `make gen` leaves no diff; ROADMAP changed only on the ticket's own M10 line (plus any Decision-log entry); migrations are new files with a Down; RPC auth is deny-by-default; no `public` schema tables, PostgREST grants or RLS-as-exposure; no secrets or personal data; `buf breaking` passes unless the ticket is `breaking-ok`.
