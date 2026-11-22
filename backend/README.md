# backend

One Go module (`github.com/shrujancheruku/issho/backend`), part of the root `go.work`. The database baseline (#594) and the ConnectRPC API (#595) are in, and this README grows with each ticket.

## Layout

| Path | What |
|---|---|
| `cmd/api` | The API process: serves ConnectRPC |
| `cmd/worker` | The background-job process: runs River |
| `cmd/rpcauth` | `make rpc-auth`: fails when an RPC in `proto/` declares no access rule |
| `cmd/migrate` | `migrate up` and `migrate down`, behind `make migrate-up` and `make migrate-down` |
| `db` | The goose migrations (`db/migrations`), embedded, and the `Up` and `Down` that run them; the schema sqlc reads |
| `internal/<pkg>/store` | The Go that sqlc generates from `internal/<pkg>/queries`, one pair per package (today `internal/staff`). Generated: never edit it |
| `internal/clock` | The injected clock: `Clock`, `Real` (wired only in `cmd/`) and the test `Fake` |
| `internal/cityclock` | The one city-clock helper: a city's date, "HH:MM on a date", part of the day, midnight-crossing windows |
| `tools/clockcheck` | The lint check that fails on `time.Now`, `time.Since` and `time.Until` outside `internal/clock`, and on `clock.Real` outside `cmd/` |
| `gen/issho` | The Go that buf generates from `proto/`: the messages and the ConnectRPC service stubs. Generated: never edit it |
| `internal/rpc` | The API's services and the access interceptor; `rpc/rpcauth` is the check behind `cmd/rpcauth` |
| `internal/email` | `email.Sender`, its in-memory fake, and `email/resend`, the real one |
| `internal/logging` | `logging.New`, the one way a process makes a logger, and the test that keeps it the only way (see "Logs") |
| `internal/jobs` | The River client the worker runs and the domain packages enqueue through |
| `internal/policy` | Every number in the spec's rules, as a constant linked to its document (see "Policy") |
| `internal/matching` | Matching as a pure function: a round's signups, the kept-apart pairs and the closed dates in, groups with a date and part of the day out (the thin loop's hard rules; see its README) |
| `internal/staff` | The queue: items with a due time and a written default, and the River job that applies the default (see "The queue") |
| `internal/retention` | The registry of every table's personal data and lifetime, and the check that none is missing (see "Retention") |
| `internal/serve` | The lifecycle both commands share |
| `internal/testdb` | A schema, or a whole database, per test in a throwaway Postgres |
| `Dockerfile` | One image holding `api` and `worker` (see "The image") |
| `tools/policylint` | The check behind `make policy-lint` |
| `tools/boundaries` | The check behind `make boundaries` |

## The commands

`api` and `worker` each log `started` (as JSON, one object per line; see "Logs") and run until they get SIGINT or SIGTERM, then log `stopped` and exit 0 (`api` lets requests in flight finish first). `api` listens on `API_ADDR` (default `:8080`) and logs the address it bound, so `API_ADDR=127.0.0.1:0` takes any free port. `worker` connects to `WORKER_DATABASE_URL` and runs River (see "Jobs"); without it, the worker logs the error and exits 1. With `RESEND_API_KEY` and `EMAIL_FROM` set it can send email (see "Outside services"); with neither it runs and every send fails; a key without `EMAIL_FROM` exits 1. `cmd/cmd_test.go` builds each binary, signals it and checks the exit.

## The API

The API is ConnectRPC over protobuf: the contract is `proto/` (see its README), `make gen` turns it into `gen/issho`, and `internal/rpc` implements it. `rpc.NewHandler` mounts every service behind one interceptor that enforces the `access` option each RPC declares in its proto (public, member, Making, or admin with the second sign-in step). It denies by default: an RPC with no rule is refused for everyone with `internal`, a missing, malformed or unverifiable token on a member, Making or admin RPC is `unauthenticated`, and a verified caller without the role (or, for admin, the second step) is `permission_denied`.

Who a token belongs to comes from an `rpc.Authenticator`, which turns a bearer token into an `rpc.Identity` (subject, second step, Making, admin). The Supabase JWKS verifier and the staff-role lookup are #608's; until they exist `cmd/api` passes none and every token is refused, so only public RPCs work. A handler reads the caller with `rpc.IdentityFrom(ctx)`.

The one RPC so far is `HealthService.Check`, public, which reports the build's version. `cmd/api`'s `version` is set when building (`go build -ldflags "-X main.version=1.2.3"`) and is `dev` otherwise.

Tests go through HTTP with a generated Connect client. `internal/rpc/access_test.go` serves a throwaway service, built in the test with one RPC per rule plus one with none, behind the real interceptor; `internal/rpc/rpcauth` tests the access-rule check against fixture protos in its `testdata`.

## Logs

Logs carry the account's ID and the route and nothing else about a person ([Technical operations](../docs/spec/operations.md#errors-and-logs)). Each command builds its logger once with `logging.New(os.Stdout, "api")`, one JSON object per line, and hands it down; no package makes a logger of its own or writes to stdout, which `internal/logging/only_way_test.go` enforces, so a test that builds one on a buffer captures a whole run.

- **API:** one `request` line per request with `route`, `status`, `duration_ms` and, when the caller is signed in, `account`. It is built from those fields, never from the request, so an email in a body, a forwarded-for header or the caller's address can't reach it (`internal/rpc/requestlog_test.go`). A path no service owns logs as `unmatched`; a handler that panics logs as a 500, without the caller's address or the panic's value, and net/http's own errors go through `logging.ServerErrorLog`, which keeps none of their text (`internal/rpc/panic_test.go`).
- **Worker:** exactly one `job` line per job with `kind` and `outcome` (`done`, `failed` or `panicked`); never the arguments, the error's text or a panic's value, and River's own lines about the job are dropped (`internal/jobs/joblog_test.go`).

See `internal/logging/README.md`, `internal/rpc/README.md` and `internal/jobs/README.md`.

## Migrations

goose owns the schema, as SQL files in `db/migrations/` (`00001_baseline.sql`, `00002_….sql`), each with a `-- +goose Up` and a working `-- +goose Down`. App tables are in the `app` schema, River's in `river`, and goose's own bookkeeping in `goose` (`goose.goose_db_version`); nothing is ever created in `public`. Don't use the Supabase CLI's migrations (`supabase db push`, `supabase migration new`): they are disabled in `supabase/config.toml`.

```sh
supabase start      # the local database, on 127.0.0.1:54322
make migrate-up     # apply every pending migration
make migrate-down   # roll back the latest one
```

Both targets run `cmd/migrate`, which connects to the local database and ignores `DATABASE_URL` in the environment; to migrate another database, name it: `make migrate-up DATABASE_URL=...`. A migration is a new file, never an edit of one that has landed on `main`; its Down undoes it exactly, so `up`, `down`, `up` works (`db/migrate_test.go` runs that, checks `public` stays empty and goose's table is in `goose`). Production migrates forward only: a mistake is fixed by a new migration.

The baseline writes out River's own schema (River v0.51.2) as SQL, so that goose is the only thing that migrates. When an upgrade of River adds a migration, `db/river_test.go` fails until a new goose migration holds its SQL.

## Queries

SQL is written by hand in each package's `queries/*.sql` and compiled by sqlc into that package's `store` (config: `sqlc.yaml`, one entry per package, which reads `db/migrations` as the schema). A package's queries are its internals: `tools/boundaries` refuses any other package that imports its `store`, so no package reads another's tables. A new package adds its own `sqlc.yaml` entry, never a shared output. After changing a query or a migration, run `make gen` from the repo root and commit the result; CI fails if `make gen` leaves a diff. Call the generated methods with a pgx pool or transaction: `store.New(tx).ListOpenQueueItems(ctx)`. `timestamptz` columns come out as `time.Time` (`*time.Time` when nullable), set by the overrides in `sqlc.yaml`.

## Jobs

Background work is River's, in Postgres. `internal/jobs.New` builds the client; `cmd/worker` starts it on a direct (or session-pooler) connection in `WORKER_DATABASE_URL`, since River listens for new jobs with LISTEN/NOTIFY, which a transaction pooler breaks. A job is enqueued with `client.InsertTx` in the transaction that causes it: it is worked only after that transaction commits and never if it rolls back (`internal/jobs/jobs_test.go`). Each domain package defines its own job args and worker and registers the worker on the bundle from `jobs.NewWorkers(clk, pool, kinds)`, taking the time from `clk` (see "Time"). The bundle holds the queue's due worker (see "The queue"); a domain package's job is added beside it.

## Time

Code never asks the system for the time. It takes a `clock.Clock`: `cmd/worker` builds `clock.Real{}` and passes it to `jobs.NewWorkers`, and tests pass a `clock.NewFake(start)` that stands still until `Set` or `Advance` moves it. `cmd/api` builds its own `clock.Real{}`, which times the request log. `make lint` runs `tools/clockcheck`, which fails on `time.Now`, `time.Since` and `time.Until` in any package but `internal/clock`, and on `clock.Real` outside `cmd/` (tests included). See `internal/clock/README.md`, `internal/cityclock/README.md` and `tools/clockcheck/README.md`.

Every time on a city's clock goes through `cityclock.New(zone, dayParts)`, built from the city's IANA time zone and its clock hours of morning, afternoon and evening (a city setting). Its methods take an instant, so they are pure and testable with a fake clock: `Date` (the city's date), `At` (an instant from a date and "HH:MM"), `PartOfDay` and `InWindow` (a from-to window that may cross midnight, from inclusive, to exclusive). `At` puts a time the clocks skip after the gap and takes the first of a time they repeat.

## The queue

Every by-hand step is an item in one queue, with a due time and a written default; `internal/staff` owns its table (`app.queue_items`, migration `00002`). A package registers each kind of item with the function that applies its default, then opens items in its own transaction with `Queue.Open`, which also enqueues a River job for the due time. When the job runs, an item still open at its due time gets the default and is marked `defaulted`, once; an item resolved first is left alone. `cmd/worker` builds the `staff.Kinds` registry and passes it to `jobs.NewWorkers`. See `internal/staff/README.md`.

## Policy

Every number in the spec's rules (a group of 3 to 6, a 48-hour invitation) is a constant in `internal/policy`, with a comment linking the spec document that sets it; none is in the database. A number changes only by a PR that edits the document and the constant together. `make policy-lint`, which `make lint` runs, fails on a constant with no link or a dead one, and on a policy number written as a literal in another package. See `internal/policy/README.md` and `tools/policylint/README.md`.

## Outside services

Each outside service (email, then push, Places and the models) follows the pattern `internal/email` set:

1. **One interface in its own package**, named for what the code needs (`email.Sender`), and that package's exported API is all a domain package imports.
2. **One shared fake in the same package** (`email.NewFake()`), which records what was sent and enforces the same message rules as the real one. Every test uses it; none writes its own.
3. **The real implementation in a subpackage** (`email/resend`) marked `//boundaries:outside-service`, tested with a transport that serves a recorded response from `testdata`: no network, no key.
4. **It is constructed only in `cmd/`** (`cmd/worker/email.go`) from environment variables, and handed to the packages that take the interface. With no key set it is a `NotConfigured` stand-in that fails each send, so a local process still runs.

`make boundaries` (part of `make lint`) fails a package that imports another seam's internals, and anything outside `cmd/` that imports a marked implementation. See `tools/boundaries/README.md`.

## Retention

`internal/retention/registry.json` lists every table with what personal data it holds and its lifetime, and `make retention` (CI's `retention` check, also part of `make test`) fails on a table with no entry, or an entry for a table that is gone. A migration that adds a table adds its entry in the same PR. See `internal/retention/README.md`.

## The image

`Dockerfile` builds `api` and `worker` into one image at `/usr/local/bin`, on a distroless base that runs as a non-root user; the command defaults to `api`, and the deploy runs the worker by naming its command. `make image` builds it, checks both binaries are in it and removes it again (`scripts/image.sh`). `--build-arg VERSION=1.2.3` is what the API's health check reports.

## Testing

Run from the repo root:

```sh
make test   # go test -race ./... in backend/, and the api-client's tests
make lint   # gofmt, go vet, buf lint, buf breaking, rpc-auth and the api-client's type check
make policy-lint   # the policy lint, which make lint also runs
make boundaries    # the seam check, which make lint also runs
```

`make test` needs Docker running. The first `testdb.New(t)` in a test binary starts a Postgres 17 container (removed when the binary exits) and returns a pool whose `search_path` is a fresh schema, dropped when the test ends, so tests may write the same keys in parallel. A test that runs the migrations, which use fixed schema names, calls `testdb.NewDatabase(t)` instead for a database of its own, then `db.Up`. With Docker stopped or unreachable (`DOCKER_HOST`) the test fails; no flag or environment variable skips the database. Tests use it directly rather than mocking stores.
