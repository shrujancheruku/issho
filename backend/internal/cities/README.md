# cities

Owns the cities: which are listed, and each one's time zone, currency and names ([Cities](../../../docs/spec/cities.md), [What varies by city](../../../docs/spec/city-settings.md)). The tables are `app.cities` and `app.city_names` (`db/migrations/00004_cities_and_plans.sql`); no other package reads them. A city in `app.cities` is a listed city: `people` asks this package whether an account's city is listed, and the database holds the same line with a foreign key from `app.accounts.city`.

So far a city has a slug, a state (`planned`, `open` or `closed`), a time zone, a currency and a name in each language. The other city settings, the areas and the calendar arrive with their own tickets, each in a table keyed by the slug. A city is made in development and tests by the fixture; nothing in production makes one yet (the admin's city settings will).

## Using it

| Function | What |
|---|---|
| `New(pool)` | The service |
| `Service.Get(ctx, slug)` | The city with that slug (`Slug`, `State`, `TimeZone` for `cityclock.New`, `Currency`, `Names` by language), or `ErrNotFound` |
| `LoadFixture(ctx, pool, data)` | Writes the `city` of a fixture file (`../../fixtures`), replacing a city of the same slug. **Development and tests only**: `cmd/seed` and the tests call it; no process in the production image does. An unknown field is an error |

## Tests

Package tests go through the exported API on the throwaway Postgres (`internal/testdb`).

| File | Proves |
|---|---|
| `cities_test.go` | The fixture seeds Da Nang–Hoi An with its time zone, currency and names in both languages; a city that isn't there is `ErrNotFound`; loading twice leaves one city; a fixture with a field the city doesn't know is refused |
