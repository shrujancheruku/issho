# logging

The one way a process makes a logger. Issho's logs carry the account's ID and the route and nothing else about a person: no IP address, email, request body or header ([Technical operations](../../../docs/spec/operations.md#errors-and-logs)).

- `New(w, cmd)` returns a `*slog.Logger` that writes one JSON object per line to `w`, tagged `"cmd":"api"` or `"cmd":"worker"`. Only `cmd/` calls it with `os.Stdout`; a test calls it with a buffer and so captures a whole run's output.
- `Discard()` is the logger for code that was given none.
- `WithoutErrors(log)` drops the `error` attribute from every line, for a library (River) that reports a failure with the error's text, which can quote what a person gave us.

A package never builds a logger or writes to the process's output: it takes the `*slog.Logger` it is handed. `only_way_test.go` fails when a package under `internal/`, or `cmd/api` or `cmd/worker`, calls `slog.New`, `slog.Default`, a package-level `slog` function, anything in `log`, `fmt.Print*` or touches `os.Stdout` or `os.Stderr` (the two commands' mains may name `os.Stdout`, to hand to `New`).

What gets logged lives with the code that logs it: one line per API request is `internal/rpc` (`requestlog.go`), one line per job is `internal/jobs` (`joblog.go`). Tests: `logging_test.go`, `only_way_test.go`.
