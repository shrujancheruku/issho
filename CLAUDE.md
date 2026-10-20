# Agent instructions for Issho

Project-specific rules for AI coding agents. They **take precedence over generic skills** in `.claude/skills/` (e.g. the Supabase skills) where they conflict. Humans: see [docs/onboarding.md](docs/onboarding.md).

## Read first
- [docs/ROADMAP.md](docs/ROADMAP.md): plan, status and decision log
- [docs/architecture.md](docs/architecture.md): how the pieces fit together
- The README in the directory you're changing. `apps/mobile/AGENTS.md` has Expo-specific rules.

## Every change
- Update **docs/ROADMAP.md** (status, decision log, "Last updated"). CI fails code changes without it.
- Keep the directory READMEs accurate (endpoints, setup, testing).
- Comment non-obvious logic with the *why*.
- Run `make gen` after editing `proto/` or `backend/db/queries/`, then `make lint test`.
- Commit messages follow Conventional Commits (`feat(scope): …`); hooks and CI enforce this.

## Supabase: how this repo differs from the generic Supabase skills
- **Migrations are owned by goose**, in `backend/db/migrations/*.sql` (`make migrate-up`). **Never** use `supabase db push`, `supabase db pull`, `supabase migration new` or `supabase/migrations/`. The CLI's migrations and seed are disabled in `supabase/config.toml`.
- **App tables live in the `app` schema; goose's bookkeeping lives in `goose`.** Don't create tables in `public`.
- **The Data API (PostgREST) is disabled**, locally and on hosted projects. Clients never query the database directly; everything goes through the Go API (ConnectRPC). Don't grant `anon`/`authenticated` access or write RLS policies as a way to expose data. Add an RPC instead.
- **Supabase Auth is used for sign-in only** (email OTP). The Go API verifies ES256/RS256 tokens against the project JWKS and never uses the JWT secret.
- **Hosted config** is pushed from `supabase/config.toml` with `supabase config diff` / `config push`, using the overrides in `[remotes.<project>]`. Always run `config diff` and review it before pushing.
- **The Supabase MCP server is not configured.** Don't create `.mcp.json` unless the user asks.

## Secrets (the repo is public)
- Never commit secrets. Real values live in gitignored files (`.env.*`, `supabase/.env`, `supabase/signing_keys.json`) or in the deployment's secret store. See [docs/secrets.md](docs/secrets.md).
- Never print secrets in command output: no `cat` of env files, and filter CLI output so only public values (e.g. the publishable key) are shown.
- `EXPO_PUBLIC_*` values are bundled into the app and are public.
- `make secrets-scan` before pushing; the pre-commit hook also scans.
