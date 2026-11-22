# plans

Owns plans: the fields a plan page shows, and the prose in each language ([Plans](../../../docs/spec/plans.md), [Languages](../../../docs/spec/languages.md#plans)). The tables are `app.plans`, `app.plan_texts`, `app.plan_stops` and `app.plan_stop_texts` (`db/migrations/00004_cities_and_plans.sql`); no other package reads them.

So far plans are **read only**. A fixture seeds them in development and tests; making, reviewing and opening plans are later tickets'. A plan's states, its rounds and its record arrive with them, in tables keyed by the plan's id.

**Fields and prose.** What must be true on the day is a column of `app.plans` or `app.plan_stops`, the same in every language: the price (per person, whole units of the city's currency), the meeting address, the parts of the day, the start and end (`HH:MM` on the city's clock) and each stop's minutes and the mode and minutes of the leg to the next. The prose (title, description, what to look for at the meeting point, what to bring, each stop's name and note) is a row per language in `app.plan_texts` and `app.plan_stop_texts`, so a translation can't change a field and a city that opens with a new language is no migration.

## Using it

| Function | What |
|---|---|
| `New(pool)` | The service |
| `Service.List(ctx, city, language)` | The city's plans, earliest start first, with their prose in `language`. A city with none, or no such city, lists none: whether a city is listed is `cities`' to say |
| `Service.Get(ctx, id, language)` | One plan with its stops and legs, or `ErrNotFound`: for an id nobody has and for one that couldn't be a plan's alike, never an empty plan |
| `LoadFixture(ctx, db, data)` | Writes the `plans` of a fixture file (`../../fixtures`), each replaced whole, so a stop dropped from the file doesn't linger. The city must be there first. **Development and tests only**: `cmd/seed` and the tests call it; no process in the production image does. An unknown field is an error. `db` is a pool or a transaction (`Beginner`); given a transaction it writes in a savepoint, so `cmd/seed` can check and write in one transaction |

A plan with no prose in the language asked for reads in English, `FallbackLanguage` ([Languages](../../../docs/spec/languages.md): English is the fallback for every language), and its `Language` says so.

## Tests

Package tests go through the exported API on the throwaway Postgres (`internal/testdb`); `internal/rpc/plan_test.go` covers the same plans over HTTP.

| File | Proves |
|---|---|
| `plans_test.go` | The fixture seeds two plans and the city lists them by start time; a plan carries every field a plan page shows, stops and legs included; the prose comes back in the caller's language and the fields are the same in both; a plan with no prose in the language reads in English; a plan that doesn't exist, or an id that couldn't be one, is `ErrNotFound`; loading twice leaves two plans with their stops not doubled; a fixture with an unknown field is refused |
