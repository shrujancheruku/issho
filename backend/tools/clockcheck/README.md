# clockcheck

A lint check, run by `make lint`, that keeps the backend on the injected clock (`internal/clock`). It fails when:

- any Go file outside `internal/clock` reads the system clock with `time.Now`, `time.Since` or `time.Until`, whether called or passed as a value, and under an aliased import;
- any Go file outside `cmd/` refers to `clock.Real`: packages take a `clock.Clock`, and only the commands wire in the real one.

Tests count: they use `clock.NewFake`. A failure prints one `path:line: message` per finding and a hint, and exits 1:

```
internal/leak/leak.go:9: time.Now reads the system clock
clockcheck: take a clock.Clock instead; only internal/clock reads the system clock and only cmd/ builds clock.Real
```

It parses every `.go` file under `backend/` and skips directories named `testdata` (the go tool ignores them too). Run it from `backend/`: `go run ./tools/clockcheck`.

The fixtures in `testdata/` prove it, through `clockcheck_test.go`: `bad` must fail with exactly its five findings (aliased import, a function taken as a value, a test file), `ok` must pass (a local variable named `time`, mentions in strings), `exempt` stands for the clock's own directory, and `wiring` holds a `cmd/` that may build `clock.Real` and a package that may not. A dot-import of `time` (`Now()` unqualified) is not caught.
