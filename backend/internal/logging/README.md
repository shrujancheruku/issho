# logging

The one way a process makes a logger. Issho's logs carry the account's ID and the route and nothing else about a person: no IP address, email, request body or header ([Technical operations](../../../docs/spec/operations.md#errors-and-logs)).

- `New(w, cmd)` returns a `*slog.Logger` that writes one JSON object per line to `w`, tagged `"cmd":"api"` or `"cmd":"worker"`. Only `cmd/` calls it with `os.Stdout`; a test calls it with a buffer and so captures a whole run's output.
- `Discard()` is the logger for code that was given none.
- `Restricted(log, keep, drop)` is for a library (River) that logs through a logger it is given and attaches what it likes (an error's text, a panic's value). Each line keeps only the attributes named in `keep`, so one the library adds later is dropped rather than leaked; `drop` discards whole lines.
- `ServerErrorLog(log)` is an `http.Server`'s `ErrorLog`. Without one net/http writes to the standard logger, outside this helper, in free text that can name the client's address; each of its messages becomes a fixed `http server error` line with none of the text.

A package never builds a logger or writes to the process's output: it takes the `*slog.Logger` it is handed. `only_way_test.go` fails when a package under `internal/`, or `cmd/api` or `cmd/worker`, calls `slog.New`, `slog.NewLogLogger`, `slog.Default`, a package-level `slog` function, anything in `log`, `fmt.Print*`, the builtins `print` and `println`, touches `os.Stdout` or `os.Stderr`, or builds an `http.Server` without `ErrorLog`. The two commands' mains may name `os.Stdout`, to hand to `New`, and a test may capture the standard logger (`log.SetOutput`, `log.Writer`) to prove nothing reaches it.

What gets logged lives with the code that logs it: one line per API request is `internal/rpc` (`requestlog.go`), one line per job is `internal/jobs` (`joblog.go`). Tests: `logging_test.go`, `only_way_test.go`.
