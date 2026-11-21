# backend

One Go module (`github.com/shrujancheruku/issho/backend`), part of the root `go.work`. Later tickets add the database (#594) and the ConnectRPC API (#595); this README grows with them.

## Layout

| Path | What |
|---|---|
| `cmd/api` | The API process |
| `cmd/worker` | The background-job process |
| `internal/serve` | The lifecycle both commands share |
| `internal/testdb` | A schema per test in a throwaway Postgres |

## The commands

`api` and `worker` each log `started` and run until they get SIGINT or SIGTERM, then log `stopped` and exit 0. They do no other work yet. `cmd/cmd_test.go` builds each binary, signals it and checks the exit.

## Testing

Run from the repo root:

```sh
make test   # go test -race ./... in backend/
make lint   # gofmt and go vet
```

`make test` needs Docker running. The first `testdb.New(t)` in a test binary starts a Postgres 17 container (removed when the binary exits) and returns a pool whose `search_path` is a fresh schema, dropped when the test ends, so tests may write the same keys in parallel. With Docker stopped or unreachable (`DOCKER_HOST`) the test fails; no flag or environment variable skips the database. Tests use it directly rather than mocking stores.
