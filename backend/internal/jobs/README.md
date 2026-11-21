# jobs

The River client the worker runs and the domain packages enqueue through. `NewWorkers(clk, pool, kinds)` is the bundle of workers and `New(pool, workers, log)` builds the client; see "Jobs" in `backend/README.md`.

**Job log (`joblog.go`).** The client logs exactly one `job` line per job worked: `kind` and `outcome` (`done`, `failed` or `panicked`), nothing else. Never the job's arguments, which hold what a person gave us, nor the error's text or a panic's value. River's own lines go through `logging.Restricted`: its lines about a job below a warning ("Job errored", "Job panicked") and its panic-recovery line are dropped, since they repeat the job line; its warnings and errors that aren't the job's outcome (a stuck job, one it couldn't mark complete) and its lines about the client stay, with only `job_id`, `job_kind`, `kind`, `queue` and `client_id`. A nil `log` discards everything.

Tests: `jobs_test.go` (enqueue in a transaction, the queue's due job), `joblog_test.go` (a job that succeeds, one that fails and one that panics, each with an email in its arguments, error or panic, log one line each and the email nowhere).
