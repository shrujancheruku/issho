# e2eclock

The test-only RPCs of the end-to-end build ([#622](https://github.com/shrujancheruku/issho/issues/622); [Quality: End to end](../../../docs/spec/quality.md#end-to-end)). Every file here has the `e2e` build tag, so **the package is not compiled into the production `api` or `worker`**: `make test` (`cmd/cmd_test.go`) and `make image` (`scripts/image.sh`) both fail if either binary holds the `issho.e2e.v1` package name.

- `ClockService` (`proto/issho/e2e/v1/clock.proto`): `GetTime`, `SetTime(now)` and `AdvanceTime(by)` move the one `clock.Fake` that the API's handlers, its token verifier and its in-process worker all read. `SetTime` with no instant and `AdvanceTime` with a negative duration are `InvalidArgument`.
- `FormingService.FormPlan(plan_id)`: inserts the River job `outings.FormArgs` for the plan and returns when the worker has completed it (30 seconds at most), so a test that moves on has a formed plan. Nothing schedules forming yet (rounds on the real clock are milestone 4's); this does what the schedule will. An empty `plan_id` is `InvalidArgument`.

`Mounts(clk, former)` returns the two services as `rpc.Mount`s, which `cmd/api/e2e_on.go` puts in `rpc.Config.Mounts`. `Former` is the seam for forming (`Form(ctx, planID)`); `NewRiverFormer(client)` is the real one, which subscribes to the job's end events before inserting it, so it cannot miss the answer.

Both services declare `ACCESS_PUBLIC` like any RPC (`make rpc-auth` checks it): the build holds a throwaway database and is never deployed.

Tests (`go test -tags e2e ./internal/e2eclock/`; `make test` runs them): the clock RPCs move the clock the API reads and refuse what they cannot mean; `FormPlan` runs the job so each signup has a seat when it returns, and refuses a request with no plan.
