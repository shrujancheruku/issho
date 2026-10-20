# backend

The Go backend for Issho: a modular monolith serving a ConnectRPC API. It uses Postgres (Supabase) through pgx and sqlc, and goose for migrations. Architecture notes are in [../docs/architecture.md](../docs/architecture.md).

## Layout

| Path | Contents |
|---|---|
| `cmd/api/` | API server: wires config, DB pool, the JWKS verifier and handlers, then serves HTTP/1.1 and h2c on `API_ADDR` |
| `cmd/tool/` | Ops CLI: `migrate <goose command>`, `seed [-catalog file] <citypack.json>...` |
| `internal/platform/config` | Environment variables → `Config` |
| `internal/platform/db` | pgx pool and goose migration runner |
| `internal/platform/auth` | Supabase JWT verification (JWKS) and the Connect auth interceptor |
| `internal/platform/i18n` | Resolves localised JSON text (`{"en": …, "ja": …}`) |
| `internal/platform/errs` | Connect interceptor: logs unexpected errors and returns a generic `internal error` (no leaked details) |
| `internal/platform/supabase` | Supabase admin API client (account deletion), using the secret key |
| `internal/health` | `HealthService` |
| `internal/identity` | `IdentityService`: app account on top of Supabase users (get, delete) |
| `internal/catalog` | `CatalogService`: activity catalogue with city overrides |
| `internal/cities` | City-pack JSON format and seeding |
| `db/migrations/` | goose SQL migrations (embedded via `db/embed.go`) |
| `db/queries/` | sqlc queries |
| `gen/` | **Generated.** `gen/issho` (protobuf/Connect), `gen/dbgen` (sqlc). Don't edit; run `make gen`. |

## Configuration

| Variable | Required | Example (local) | Notes |
|---|---|---|---|
| `DATABASE_URL` | yes | `postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable` | Supabase local DB. In production, use the **session pooler** (port 5432) or the direct connection, with `sslmode=require`. **Not** the transaction pooler (port 6543): pgx's prepared statements and River's LISTEN/NOTIFY don't work through it. |
| `SUPABASE_URL` | yes | `http://127.0.0.1:54321` | The JWKS is fetched from `$SUPABASE_URL/auth/v1/.well-known/jwks.json`. The token issuer must be `$SUPABASE_URL/auth/v1`. |
| `API_ADDR` | no | `:8080` | Listen address |
| `SUPABASE_SECRET_KEY` | no | `SECRET_KEY` from `supabase status` | **Server-only secret.** Enables admin calls (account deletion). Without it, `DeleteAccount` returns `failed_precondition`. Never commit it; see [secrets](../docs/secrets.md). |
| `ALLOWED_ORIGINS` | no | `https://admin.example.com` | Comma-separated browser origins. **Empty = no browser origins allowed.** |

Local defaults are in `../.env.example`: `export $(grep -v '^#' ../.env.example | xargs)`.

## Run

```sh
supabase start                 # from repo root; see ../supabase/README.md
make migrate-up seed api       # from repo root
```

Or from this directory: `go run ./cmd/tool migrate up`, `go run ./cmd/tool seed ../data/citypacks/tokyo.json`, `go run ./cmd/api`.

## API endpoints

The full reference, generated from the proto comments, is in **[../docs/generated/api.md](../docs/generated/api.md)**. All RPCs are `POST /<package>.<Service>/<Method>` with a JSON body (`Content-Type: application/json`) or binary protobuf.

| Endpoint | Auth | Purpose |
|---|---|---|
| `POST /issho.v1.HealthService/Check` | public | Database ping and build version |
| `POST /issho.v1.IdentityService/GetMe` | bearer | Returns (and lazily creates) the caller's account. `unauthenticated: account deleted` for tokens of deleted users. |
| `POST /issho.v1.IdentityService/DeleteAccount` | bearer | Permanently deletes the caller (Supabase auth user → cascades to app data). Safe to retry. |
| `POST /issho.v1.CatalogService/ListActivities` | bearer | Activities enabled in a city, localised |

Example:
```sh
curl -s -X POST localhost:8080/issho.v1.CatalogService/ListActivities \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" \
  -d '{"cityId":"tokyo","locale":"ja"}'
```

Errors use Connect codes, e.g. `{"code":"unauthenticated","message":"missing bearer token"}` with HTTP 401.

**Error hygiene:** handlers return `connect.NewError(code, err)` for anything the caller should see. Any other error is logged server-side (`unhandled error` with the procedure name) and the client receives only `{"code":"internal","message":"internal error"}`. So a raw database or upstream error never leaks to callers.

### Authentication
- Clients send `Authorization: Bearer <Supabase access token>`.
- Tokens must:
  - be signed **ES256 or RS256** with a key from the project JWKS (HS256 is rejected)
  - have issuer `$SUPABASE_URL/auth/v1`
  - have audience `authenticated`
  - have an `exp` claim that hasn't passed
- **Deny-by-default:** every RPC requires auth unless it's listed as public in `cmd/api/main.go`.

## Migrations

```sh
go run ./cmd/tool migrate up        # apply all
go run ./cmd/tool migrate down      # roll back one
go run ./cmd/tool migrate status
```

- Write new files as `db/migrations/000NN_name.sql` with `-- +goose Up` / `-- +goose Down` sections. Wrap multi-statement blocks (`DO $$ … $$`) in `-- +goose StatementBegin/End`.
- goose records its versions in `goose.db_version`, in a schema of its own. Not `public`, which Supabase's Data API would expose. Not `app`, which migration 00001's Down drops. Older databases with `public.app_goose_db_version` are moved automatically on the next run.
- Supabase-only statements (the `auth.users` foreign key, revoking `anon`) are guarded with `IF EXISTS` checks, so the same migrations also run on plain Postgres in CI.

## Seeding

`cmd/tool seed` upserts `../data/activities.json`, then each city pack given. It's idempotent. Note that removing an area from the JSON does **not** delete it from the database. See [../data/README.md](../data/README.md).

## Testing

```sh
go test ./...          # or `make test` from the root
```

| Test | Covers |
|---|---|
| `internal/platform/auth/auth_test.go` | Valid token; expired, missing `exp`, wrong issuer/audience/key/subject; HS256 rejected |
| `internal/platform/i18n/i18n_test.go` | Locale fallback chain |
| `cmd/api/cors_test.go` | CORS allowlist; empty list denies all |
| `internal/platform/errs/errs_test.go` | Raw errors become generic `internal`; Connect errors pass through |
| `internal/platform/supabase/admin_test.go` | Admin delete: request shape, 404 treated as already deleted, other errors surfaced |
| `internal/identity/service_test.go` | Account creation keeps an existing email; deletion order (auth first, app data kept if that fails) |

CI also runs migrations up → down → up and seeds against a PostGIS container. Tests must never call paid external APIs.

## Code generation

```sh
make gen   # from root: buf (Go + TS + docs) and sqlc
```

The code generators are pinned as `tool` directives in `go.mod`, and `../go.work` makes `go tool` work from the repo root.
