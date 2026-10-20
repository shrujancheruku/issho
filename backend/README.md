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
| `internal/health` | `HealthService` |
| `internal/identity` | `IdentityService`: app account on top of Supabase users |
| `internal/catalog` | `CatalogService`: activity catalogue with city overrides |
| `internal/cities` | City-pack JSON format and seeding |
| `db/migrations/` | goose SQL migrations (embedded via `db/embed.go`) |
| `db/queries/` | sqlc queries |
| `gen/` | **Generated.** `gen/issho` (protobuf/Connect), `gen/dbgen` (sqlc). Don't edit; run `make gen`. |

## Configuration

| Variable | Required | Example (local) | Notes |
|---|---|---|---|
| `DATABASE_URL` | yes | `postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable` | Supabase local DB. In production, use the transaction pooler for the API. |
| `SUPABASE_URL` | yes | `http://127.0.0.1:54321` | The JWKS is fetched from `$SUPABASE_URL/auth/v1/.well-known/jwks.json`. The token issuer must be `$SUPABASE_URL/auth/v1`. |
| `API_ADDR` | no | `:8080` | Listen address |
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
| `POST /issho.v1.IdentityService/GetMe` | bearer | Returns (and lazily creates) the caller's account |
| `POST /issho.v1.CatalogService/ListActivities` | bearer | Activities enabled in a city, localised |

Example:
```sh
curl -s -X POST localhost:8080/issho.v1.CatalogService/ListActivities \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" \
  -d '{"cityId":"tokyo","locale":"ja"}'
```

Errors use Connect codes, e.g. `{"code":"unauthenticated","message":"missing bearer token"}` with HTTP 401.

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
- goose records its versions in `app_goose_db_version`.
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

CI also runs migrations up → down → up and seeds against a PostGIS container. Tests must never call paid external APIs.

## Code generation

```sh
make gen   # from root: buf (Go + TS + docs) and sqlc
```

The code generators are pinned as `tool` directives in `go.mod`, and `../go.work` makes `go tool` work from the repo root.
