# boundaries

The check behind `make boundaries`, which `make lint` runs. It holds the backend to two rules in [Architecture and stack](../../../docs/spec/architecture.md#the-seams): a package reaches another only through its exported Go API, and outside services are wired only in `cmd/`.

```sh
make boundaries                                    # from the repo root
cd backend && go run ./tools/boundaries -root ..
```

It prints one line per finding, as `file:line: kind: message`, and exits 1; exit 2 means it couldn't run.

## What it checks

| Kind | Fails when |
|---|---|
| `internal` | A package imports a package below another seam's top (`internal/places/cache`, or another package's sqlc `store`, `internal/staff/store`) from outside that seam's tree. `internal/places` is the seam; `internal/places/cache` is its internals. Go's own `internal` rule doesn't see this, since all of it is under `backend/internal` |
| `outside-service` | Anything outside `cmd/` imports a package marked `//boundaries:outside-service`, which declares a real implementation of an outside service. The seam's own interface package may not import it either |

`cmd/` may import anything: it is where the process is wired. Test files are checked like the rest. Generated code (`gen/`) and `testdata` are left out. The marker is a comment line of its own in a non-test file of the package, as in `internal/email/resend/resend.go`.

The third boundary rule in the spec, that only the city clock asks the system for the time, is the clock ticket's.

## Tests and fixtures

`check_test.go` runs `Check` on small backends under `testdata/`, each with its own `backend/go.mod`:

| Fixture | Proves |
|---|---|
| `clean` | A domain package using an interface, `cmd/` importing the real implementation and a package using its own subpackage all pass |
| `internal-import` | `internal` |
| `real-impl` | `outside-service` |
| `store-import` | `internal`, for a package importing another's sqlc `store` (each package's queries generate under it); the owner importing its own passes |

`TestRepoPasses` runs the check on this repo.
