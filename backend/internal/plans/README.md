# plans

Owns plans: the fields a plan page shows, and the prose in each language ([Plans](../../../docs/spec/plans.md), [Languages](../../../docs/spec/languages.md#plans)). The tables are `app.plans`, `app.plan_texts`, `app.plan_stops` and `app.plan_stop_texts` (`db/migrations/00004_cities_and_plans.sql`), and for signups `app.plan_rounds`, `app.signups` and `app.signup_ticks` (`00006_signups.sql`); no other package reads them.

So far plans are **read only**, and only signups to them are written. A fixture seeds them in development and tests; making, reviewing and opening plans are later tickets'. A plan's states, its rounds and its record arrive with them, in tables keyed by the plan's id.

**Fields and prose.** What must be true on the day is a column of `app.plans` or `app.plan_stops`, the same in every language: the price (per person, whole units of the city's currency), the meeting address, the parts of the day, the start and end (`HH:MM` on the city's clock) and each stop's minutes and the mode and minutes of the leg to the next. The prose (title, description, what to look for at the meeting point, what to bring, each stop's name and note) is a row per language in `app.plan_texts` and `app.plan_stop_texts`, so a translation can't change a field and a city that opens with a new language is no migration.

## Using it

| Function | What |
|---|---|
| `New(clock, pool, cities)` | The service, reading "today" from the injected clock on each plan's city clock |
| `Service.List(ctx, city, language)` | The city's plans, earliest start first, with their prose in `language`. A city with none, no such city or a closed one lists none, and a plan that isn't shown yet (below) is left out |
| `Service.Get(ctx, id, language)` | One plan with its stops and legs, or `ErrNotFound`: for an id nobody has, one that couldn't be a plan's, and a plan that isn't shown (below) alike, never an empty plan |
| `LoadFixture(ctx, db, data)` | Writes the `plans` of a fixture file (`../../fixtures`), each replaced whole, so a stop dropped from the file doesn't linger. The city must be there first. **Development and tests only**: `cmd/seed` and the tests call it; no process in the production image does. An unknown field is an error. `db` is a pool or a transaction (`Beginner`); given a transaction it writes in a savepoint, so `cmd/seed` can check and write in one transaction |

**When a plan is shown.** One definition, `shown` in `plans.go`, used by `List`, `Get` and `Signups.Set`: the plan's city isn't closed (a closed city's plans are kept but not shown) and today on the city's calendar is its `opens_on` day or later. The spec has no "opening soon" state, so a plan before its opening day is as if it weren't there: `List` leaves it out, and `Get` and `Set` answer `ErrNotFound`, the same error as a missing plan. The opening day itself is shown and takes signups.

A plan with no prose in the language asked for reads in English, `FallbackLanguage` ([Languages](../../../docs/spec/languages.md): English is the fallback for every language), and its `Language` says so.

## Signups

A signup is a person's **ticks** on a plan: parts of the day (morning, afternoon, evening, in the city's clock hours) on real dates, from 3 days ahead. It commits them to nothing ([Matching](../../../docs/spec/matching.md#rounds), [The core loop](../../../docs/spec/core-loop.md)). `plans.Signups` is its API; the RPCs are `SignupService` in `internal/rpc`.

| Function | What |
|---|---|
| `NewSignups(clock, pool, cities)` | The service, reading "today" from the injected clock on each plan's city clock |
| `Signups.Set(ctx, account, plan, ticks)` | Signs the account up, or replaces the ticks of the signup it has: changing times is signing up again. Ticks come back in time order, repeats once. The whole call or nothing: one refused tick saves none and opens no round |
| `Signups.Withdraw(ctx, account, plan)` | Ends the account's signup in the plan's running round. A person with none withdraws nothing, with no error |
| `Signups.Mine(ctx, account)` | The account's own signups to rounds that haven't ended, earliest hear-by first, each with its `HearBy`. It takes the account, so it can't return another's |
| `Signups.Forming(ctx, plan)` | The plan's running round as forming groups reads it: the round's id (empty while nobody has signed up), the plan's start time and one `Entry` (account and ticks) per person, by account. Only the ticks still `policy.MinTickLeadDays` ahead **now** on the city's clock are read, so an outing's date is never less than 3 days after its invitations go out, and a person with none left isn't an entry. A plan that isn't shown is `ErrNotFound`. `outings` is its one caller |
| `Signups.Count(ctx, plans)` | How many are signed up to each plan's running round: a number, never who |

A tick is refused when it is less than `policy.MinTickLeadDays` (3) calendar days ahead **on the city's clock** (`ErrTickTooSoon`), on a part of the day the plan doesn't allow (`ErrTickPartNotAllowed`), or later than a round (`policy.RoundLength`) after the round's last day (`ErrTickTooFar`, "from 3 days ahead to 7 days after the round's last day"). A signup with no ticks, a date that isn't `YYYY-MM-DD` or a part that isn't one of the three is `ErrInvalid`. A plan that isn't there or isn't shown (its city is closed, or it hasn't opened yet) takes no signups: `ErrNotFound`, the same error whichever. A closed city's signups can still be withdrawn.

**Rounds, so far.** A plan's rounds are fixed by the day it opened, not by who signs up when ([Matching](../../../docs/spec/matching.md#rounds)). `app.plans.opens_on` is that city day (the fixture sets it; `00006_signups.sql` backfills a plan already there with the day it ran); round *k* covers `opens_on + 7k` to `opens_on + 7k + 6` (`policy.RoundLength` is 7 days, the first and last day included) and the last day is the **hear-by date** every signup in it shares. A signup goes into the round that holds today on the city's calendar. The dates are derived; the `app.plan_rounds` row (number, first and last day) is only stored by the first signup into the round, unique on `(plan_id, number)`, so signups racing to be first store one. Before its opening day a plan is not shown, has no round and takes no signups (`ErrNotFound`). Once a round has ended, its signups are no longer the member's current ones, the plan's count starts again from 0, and the day after its last day is the next round. The rest of the rounds (a state, their own clock) is milestone 4's ("Rounds on the real clock"); it adds columns to `app.plan_rounds`, not a second table. Re-loading the fixture replaces a plan whole, signups included.

**One signup per person per plan round** is the database's rule: `app.signups` is keyed `(round_id, account_id)`, so a second entry is refused with a unique violation (`signups_pkey`) whatever code writes it. Matching leaves out a person whose entries differ rather than erroring ([matching](../matching/README.md)), so the outings job can rely on never meeting one: `Forming` returns one entry per person, and `outings`' tests hold the constraint from that side too. `app.signup_ticks` hangs off it, so withdrawing, or deleting the account, takes the ticks with it.

**Admission.** `SignUp` doesn't check that the account was admitted: admission is `people`' to keep (join links, #609, which makes a signup from an account that isn't admitted refused). **Profile.** The RPC, not this package, requires the profile (`rpc.RequireProfile`).

The retention registry holds all three tables (`../retention/registry.json`): the signups and their ticks are kept while the account exists ([Privacy](../../../docs/spec/privacy.md)).

## Tests

Package tests go through the exported API on the throwaway Postgres (`internal/testdb`); `internal/rpc/plan_test.go` covers the same plans over HTTP.

| File | Proves |
|---|---|
| `plans_test.go` | The fixture seeds two plans and the city lists them by start time; a plan carries every field a plan page shows, stops and legs included; the prose comes back in the caller's language and the fields are the same in both; a plan with no prose in the language reads in English; a plan that doesn't exist, or an id that couldn't be one, is `ErrNotFound`; loading twice leaves two plans with their stops not doubled; a fixture with an unknown field is refused |

`signups_test.go` is the signups, through `plans.Signups` on the fixture city with a fake clock: a signup carries its ticks and the day the person hears by, a tick under 3 days ahead is refused (counted on the city's clock, not UTC's), so is one on a part of the day the plan doesn't allow, one too far past the round, and a malformed one; a refused signup leaves nothing; signing up again changes the ticks, withdrawing removes the signup (twice, or with none, is no error); one person signs up to both fixture plans; `Mine` is the caller's own; `Count` is a number per plan; a round lasts 7 days on the city's clock and the next signup opens the next; a closed city's plan takes no signups, nor does one that hasn't opened, which `List` and `Get` also hide, until midnight of its opening day on the city's clock (`List`, `Get` and `Set` agree); the database itself refuses a second signup of one person to one round; and `Forming` reads the running round one entry per person with the latest ticks, none for a plan nobody signed up to or one that isn't shown, and leaves out a tick that is now less than 3 days ahead.
