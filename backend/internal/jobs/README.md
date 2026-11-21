# jobs

The River client the worker runs and the domain packages enqueue through. `NewWorkers(clk, pool, kinds)` is the bundle of workers and `New(pool, workers, log, clk)` builds the client; see "Jobs" in `backend/README.md`.

**Job log (`joblog.go`).** The client logs one `job` line per job worked: `kind`, `outcome` (`done` or `failed`) and `duration_ms` (from `clk`). Never the job's arguments, which hold what a person gave us, and never the error's text. River's own lines go through `logging.WithoutErrors`, so its "job errored" line keeps the kind and job ID but not the error. A nil `log` discards everything.

Tests: `jobs_test.go` (enqueue in a transaction, the queue's due job), `joblog_test.go` (a job whose arguments and error hold an email leaves it out of the log).
