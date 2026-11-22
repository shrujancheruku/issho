# outings

Owns outings and their seats: the group matching formed on a date for a plan, and each person's place in it ([Outings](../../../docs/spec/outings.md), [Matching](../../../docs/spec/matching.md)). The tables are `app.outings` and `app.seats` (`db/migrations/00007_outings.sql`); no other package reads them. `plans` and `outings` load matching's inputs and store its results ([Architecture and stack](../../../docs/spec/architecture.md#the-seams)): `outings` reads a round's signups from `plans.Signups.Forming`, the languages, gender and "only women and non-binary people" setting of each person from `people.Service.Profile`, and calls the pure function in [`matching`](../matching/README.md).

So far it **forms outings and makes their invitations**. Answering an invitation, setting the outing at three yeses, going, check-in and feedback are later tickets'. The owner's approval of a *proposed* outing, the extra invitations and the seat order come with the outing in full (milestone 4), so an outing is formed **offered** (its invitations are out) and each person in a group gets one invited seat.

## Using it

| Function | What |
|---|---|
| `New(clock, pool, signups, people)` | The service |
| `Service.Form(ctx, plan)` | Groups the signups to the plan's running round and stores each group as an outing with a seat for each person, **in one transaction**. Returns how many outings it made. Forming again makes none for anyone who already has a seat in the round, so running it twice for the same signups makes no second outing. A plan that isn't there or isn't shown is `plans.ErrNotFound` |
| `Service.Mine(ctx, account)` | The account's own seats, earliest date first: `Seat` has the outing, the plan, the date, the part of the day, the plan's start time (`HH:MM`, the city's clock), the languages the group shares, the state and when the invitation went out and ends. It takes the account and holds no other person, so it can't return an outing the account has no seat in, and a name is never in it |
| `FormArgs`, `NewFormWorker(svc)`, `AddWorker(workers, svc)` | The River job `outings_form` that calls `Form` for a plan. Its arguments are a plan's id and nothing about a person. `cmd/worker` registers it. A plan that isn't there or isn't shown finishes the job without an error (retrying can't change it) |

**Invitations.** Each seat starts `invited`, with `invited_at` (the moment the group is formed: nothing is delivered yet, so that is when the invitation goes out) and `invitation_ends_at`, `policy.InvitationWindow` (a fixed 48 hours) later. An outing is formed `offered`. The date is never less than 3 days after the invitations go out: forming reads only the ticks that are still `policy.MinTickLeadDays` ahead now ([`plans`](../plans/README.md)).

**What an invited person is shown** is the date, the time (the part of the day the group ticked and the plan's start) and every language the group shares, never a name; `OutingService.ListMyOutings` is the RPC (`internal/rpc/outing.go`). It takes no account and no outing id. No RPC returns an outing to someone with no seat in it, and a later RPC that names an outing must check the caller's seat first.

**Two runs at once, and twice.** A run takes an advisory lock on the round, held to the end of its transaction, then reads who already has a seat; so River delivering the job twice, or two jobs at once, forms the round once. The database backs it: `app.seats` is unique on `(round_id, account_id)`, since nobody is in two groups of one round, so a run that did race fails rather than double-seats. A person who signs up after a run is formed on the next one, among the others who have no seat.

**What matching reads.** A signup's ticks, and the person's languages, gender and setting. Kept-apart pairs are an empty input until safety exists (milestone 6) and the city's closed dates are empty until the city calendar exists ([The build order](../../../docs/spec/build-order.md)). A person with no profile is left out (a signup needs one, so there is none). `people.Service.Profile` is read once for each person in a round; a round is tens of people, so there is no batch read.

**When it runs.** Nothing enqueues the job yet: rounds on the real clock are milestone 4's, and the end-to-end test's clock RPC (#622) enqueues `FormArgs` itself. A caller enqueues it with `client.InsertTx(ctx, tx, outings.FormArgs{PlanID: id}, nil)`, in the transaction that causes it.

## Retention

Both tables have a [registry](../retention/registry.json) entry: `app.outings` holds no one's data (the people are in `app.seats`) and `app.seats` is kept while the account exists ([Privacy](../../../docs/spec/privacy.md)).

## Tests

On the throwaway Postgres (`internal/testdb`), through the exported API, with a fake clock, the real `plans`, `people` and `matching`, on the fixture city.

| File | Proves |
|---|---|
| `outings_test.go` | Forming makes one outing with an invited seat for each person, with the date, the time, the shared languages and the time its 48 hours end; forming twice, or at the same time, makes no second outing; people who signed up later are formed on the next run, among themselves; matching's rules hold through the job (a shared language, the women-and-non-binary-only setting); a date less than 3 days after the invitations is never set; a run that fails part-way leaves no outing; the database holds one signup per person per round and a person who changed their times is one entry, the latest; a person with no seat is never shown an outing |
| `job_test.go` | The River job forms a plan's groups once however often it is delivered, and finishes for a plan that isn't there |

`../rpc/outing_test.go` covers the RPC over HTTP, and `../../db/migrate_test.go` the migration's down and up.
