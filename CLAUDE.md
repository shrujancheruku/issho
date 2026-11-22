# Agent instructions for Issho

Project-specific rules for AI coding agents. They **take precedence over generic skills** in `.claude/skills/` (e.g. the Supabase skills) where they conflict. Humans: see [README.md](README.md).

## Read first
- [GitHub Issues](https://github.com/shrujancheruku/issho/issues): plan and status (see `docs/agents/issue-tracker.md`)
- [docs/spec/](docs/spec/README.md): the v2 spec, in the terms of the root `GLOSSARY.md`. v1's code is not in this repo; it stays readable at the `v1-final` tag. An amendment to the spec is a PR that edits its document.
- [docs/adr/](docs/adr/): why things are the way they are
- The README in the directory you're changing.
- [docs/agents/orchestration.md](docs/agents/orchestration.md) when building a milestone in parallel on Orca workers with the [flow plugin](https://github.com/shrujancheruku/orca-flow) (`/flow:build`): model routing, holds, verify commands, review checks and the merge rule.

## Every change
- Work from an issue and close it from the PR (`Closes #<n>`). Record a decision in the issue's resolution comment, or as an ADR in `docs/adr/` when it's hard to reverse, surprising without context and a real trade-off. The v1 roadmap and its decision log are retired; never recreate either.
- Planned work lives under the one active **work parent** issue (sub-issue order is run order); see "Planning rules" in `docs/agents/issue-tracker.md`.
- Keep the directory READMEs accurate (endpoints, setup, testing).
- Comment non-obvious logic with the *why*.
- Run `make gen` (then `git diff --exit-code` must be clean), `make secrets-scan`, `make lint` and `make test` from the repo root. `make test` starts its own Postgres in Docker, so Docker must be running.
- Commit messages follow Conventional Commits (`feat(scope): …`); hooks and CI enforce this.

## Supabase: how this repo differs from the generic Supabase skills
- **Migrations are owned by goose**, in `backend/db/migrations/*.sql` (`make migrate-up`). **Never** use `supabase db push`, `supabase db pull`, `supabase migration new` or `supabase/migrations/`. The CLI's migrations and seed are disabled in `supabase/config.toml`.
- **App tables live in the `app` schema; goose's bookkeeping lives in `goose`.** Don't create tables in `public`.
- **The Data API (PostgREST) is disabled**, locally and on hosted projects. Clients never query the database directly; everything goes through the Go API (ConnectRPC). Don't grant `anon`/`authenticated` access or write RLS policies as a way to expose data. Add an RPC instead.
- **Supabase Auth is used for sign-in only** (email OTP). The Go API verifies ES256/RS256 tokens against the project JWKS and never uses the JWT secret.
- **Hosted config**, once a hosted project's overrides exist in `[remotes.<project>]` in `supabase/config.toml`, is pushed with `supabase config push`. Always run `supabase config diff` and review it before pushing. (No `[remotes]` section exists yet.)
- **The Supabase MCP server is not configured.** Don't create `.mcp.json` unless the user asks.

## Secrets (the repo is public)
- Never commit secrets. Real values live in gitignored files (`.env.*` and the others `.gitignore` lists) or in the deployment's secret store. See [docs/secrets.md](docs/secrets.md).
- Never print secrets in command output: no `cat` of env files, and filter CLI output so only public values (e.g. the publishable key) are shown.
- `EXPO_PUBLIC_*` values are bundled into the app and are public.
- `make secrets-scan` before pushing; the pre-commit hook also scans.

## Agent skills

### Issue tracker

GitHub Issues on shrujancheruku/issho via `gh` (public repo: no secrets or personal data in issues). See `docs/agents/issue-tracker.md`.

### Triage labels

The five default labels: needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: a root `GLOSSARY.md`; decisions in `docs/adr/` and in the spec under `docs/spec/`. See `docs/agents/domain.md`.

<!-- flow:begin orchestration -->
### Orchestration

Build a spec with `/flow:build`: Matt's `/implement-spec` run on Orca workers, with models routed by `route.py` from the table in `docs/agents/orchestration.md`. A build milestone is a GitHub Milestone; the founder merges the integration PR. See `docs/agents/orchestration.md`.
<!-- flow:end orchestration -->
