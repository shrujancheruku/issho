# clock

The one place the backend reads the system's time.

- `Clock` is the interface code takes: `Now() time.Time`.
- `Real{}` reads the system clock. Only `cmd/` builds it (`cmd/worker` passes it to `jobs.NewWorkers`) and hands it down as a `Clock`.
- The end-to-end build of the API (`-tags e2e`, `cmd/api/e2e_on.go`) builds a `Fake` instead, that the test-only RPCs in `internal/e2eclock` move; the production build never does.
- `NewFake(start)` is for tests. It stands still until the test calls `Set(t)` (jump to any instant, forwards or back) or `Advance(d)`, and is safe to read from other goroutines.

```go
clk := clock.NewFake(time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC))
clk.Advance(48 * time.Hour) // an invitation's 48 hours pass
```

`make lint` runs `tools/clockcheck`, which fails if any package but this one calls `time.Now`, `time.Since` or `time.Until`, or if anything outside `cmd/` builds `clock.Real`. Tests: `clock_test.go`.
