# clockcheck

Fails `make lint` when any backend Go file reads the system clock with `time.Now`, `time.Since` or `time.Until`, whether called or passed as a value, and under an aliased import. Only `internal/clock` is exempt; everything else takes a `clock.Clock` so tests can set the time.

It parses every `.go` file under `backend/`, tests included, and skips directories named `testdata` (the go tool ignores them too). `testdata/bad` is the fixture that proves it fails, `testdata/ok` proves look-alikes (a local variable named `time`, strings) pass, and `testdata/exempt` proves the clock's own directory can be skipped. `clockcheck_test.go` runs them.

Run it from `backend/`: `go run ./tools/clockcheck`.
