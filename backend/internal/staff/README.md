# staff

Owns the queue: the one list of by-hand steps, each an **item** with a due time and a written default ([Running Issho](../../../docs/spec/running-issho.md#the-queue), [Architecture and stack](../../../docs/spec/architecture.md#the-seams)). The table is `app.queue_items` (`db/migrations/00002_queue.sql`); no other package reads it, and `staff` imports no domain package.

This is the queue's mechanism only. The kinds of item, who may take each, alerts and the admin's screens come with the tickets that need them; staff roles, the staff log and the scorecard are later too.

## Using it

A package that opens items registers each **kind** at startup, with the function that applies its default. The kind and its default are Go, not data:

```go
kinds := staff.NewKinds()
kinds.Register("plan-review", func(ctx context.Context, tx pgx.Tx, item staff.Item) error {
    // The written default: leave the plan closed. Runs in tx, with the
    // item's own state change.
    return nil
})
queue := staff.NewQueue(clk, kinds, riverClient)

item, err := queue.Open(ctx, tx, staff.OpenParams{Kind: "plan-review", Subject: planID, DueAt: due})
```

| Function | What |
|---|---|
| `Kinds.Register(kind, apply)` | Names a kind and its default. Panics on a duplicate, an empty name or a nil function: it's wiring, so it fails at startup |
| `Queue.Open(ctx, tx, params)` | Inserts the item and enqueues its due job, both in `tx`. `ErrUnknownKind` for a kind nobody registered |
| `Queue.Resolve(ctx, tx, id, reason)` | A person closes an open item with a reason (`ErrReasonRequired`; `ErrNotOpen` for one that is missing or already closed) |
| `Queue.ListOpen(ctx, db)` | The open items, soonest due first; `db` is a pool or a transaction |
| `AddWorker(workers, clk, pool, kinds)` | Registers the due worker; `jobs.NewWorkers` calls it |

`Open` and `Resolve` take the caller's transaction, so a rolled-back caller leaves no item and no job. The `Inserter` the queue enqueues through is a River client; an insert-only one (no workers) is enough, which is what the API process has. `cmd/worker` registers the kinds on one `Kinds` and passes it to `jobs.NewWorkers`.

An item is `open`, then `resolved` (a person acted, with a reason) or `defaulted` (the due job applied the default). `Subject` is whatever the opening package needs to find the thing again; staff never reads it.

## The due job

`Open` enqueues a River job, `staff_queue_item_due`, scheduled at the due time. When it runs it locks the item's row, and then:

- an item that is no longer open (resolved, or defaulted by an earlier delivery) is left alone, so the default is applied once however many times River delivers the job;
- an item that isn't due yet by the injected clock is left open and the job is snoozed until it is. River schedules by the database's clock; this check is what lets tests move time;
- otherwise the kind's default runs in the same transaction that marks the item `defaulted`. If it returns an error the transaction rolls back, the item stays open and River retries.

Resolving an item doesn't cancel its job; the job finds the item closed and finishes.

## Tests

`staff_test.go` goes through the exported API on the throwaway Postgres (`internal/testdb`), with a `clock.Fake`; it runs the due worker with River's `rivertest`. `internal/jobs/jobs_test.go` runs the same path through a started client.
