# Issho

Issho proposes real-world outings, like a photo walk that ends at a café, to small groups of nearby people who share interests. After each outing, people privately pick who they'd meet again. Mutual picks shape future groups and recurring crews.

## Layout

| Path | What |
|---|---|
| `proto/` | API contract (protobuf, ConnectRPC). The source of truth for Go and TypeScript types. |
| `backend/` | Go modular monolith. `cmd/api` serves the API, `cmd/tool` runs migrations and seeding. Domain code lives in `internal/<domain>`. |
| `backend/db/` | goose migrations and sqlc queries. Generated code goes in `backend/gen/`. |
| `apps/mobile/` | Expo (SDK 58) app using Expo Router. |
| `packages/api-client/` | Generated Connect-ES client, plus `createApi()`. |
| `supabase/` | Local Supabase config (`supabase start`). Postgres + PostGIS, Auth (ES256 signing key in the gitignored `signing_keys.json`), Storage and Mailpit. goose, not the Supabase CLI, owns migrations. |
| `data/` | Global activity catalogue and per-city packs (`data/citypacks/*.json`). To add a city, add a pack; no code changes. |

## Prerequisites

- Go (version pinned in `backend/go.mod`), Node 24, pnpm.
- buf, sqlc and goose come from `go tool`, pinned in `backend/go.mod`. Nothing to install globally.
- To run locally: Docker plus the [Supabase CLI](https://supabase.com/docs/guides/cli). They provide Postgres + PostGIS, Auth and Storage.

## Common tasks

```sh
pnpm install
make gen          # regenerate protobuf (Go + TS) and sqlc code
make test         # Go tests
make lint         # buf lint, go vet, sqlc vet, TypeScript typecheck

# local stack (Docker via OrbStack). First time only: echo '[]' > supabase/signing_keys.json && supabase gen signing-key --algorithm ES256
supabase start
export $(grep -v '^#' .env.example | xargs)   # local DATABASE_URL / SUPABASE_URL
make migrate-up
make seed         # activity catalogue + Tokyo city pack
make api          # http://localhost:8080

cd apps/mobile && npx expo start
```

The API verifies Supabase access tokens with the project's JWKS. In the Supabase project, enable **asymmetric JWT signing keys**.

## Commits and releases

- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org), for example `feat(mobile): add sign-in screen` or `fix(backend): reject expired tokens`. Breaking changes use `feat!:` or a `BREAKING CHANGE:` footer. CI checks the commits on every PR.
- Versions follow [SemVer](https://semver.org). [release-please](https://github.com/googleapis/release-please) opens a release PR on `main` with the next version and `CHANGELOG.md`. Merging it tags `vX.Y.Z` and creates the GitHub release.
- While the version is below 1.0.0, `feat` bumps the minor version and `fix` bumps the patch.
- The version is kept in sync in `version.txt`, `package.json`, `apps/mobile/package.json` and `apps/mobile/app.json` (`expo.version`).
