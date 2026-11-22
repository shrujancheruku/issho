# fixtures

The data that seeds **development and tests**: one city, Da Nang–Hoi An, and two plans, which the thin loop runs on before plan-making exists ([The build order](../../docs/spec/build-order.md#the-milestones), [Architecture and stack](../../docs/spec/architecture.md#the-seams)). The package holds only the bytes (`fixtures.DaNangHoiAn`); `cities.LoadFixture` and `plans.LoadFixture` each read their own part.

`da-nang-hoi-an.json` is one object with two keys:

- `city`: `slug`, `state`, `time_zone`, `currency` and `names` by language.
- `plans`: each with `id` (fixed, so tests can name it), `city`, `price`, `meeting_address`, `parts_of_day`, `starts_at`, `ends_at`, `texts` by language (`title`, `description`, `meeting_point`, `what_to_bring`) and `stops` in order, each with `stay_minutes`, `leg` (the way to the next stop, `null` for the last) and `texts` by language (`name`, `note`).

Fields that must be true on the day are single values; only prose is per language. A key the loaders don't know fails the load, so a typo can't drop a field.

## Where it loads, and where it can't

- **Tests** call the loaders themselves.
- **Development:** `make seed` (`cmd/seed`) loads it into the local database after `make migrate-up`. `seed` refuses a database that isn't on this machine, before it connects.
- **Never in production.** `backend/Dockerfile` builds `api` and `worker`, and neither imports this package; `cmd/cmd_test.go` fails if `api`, `worker` or `migrate` is built from it or the image builds `seed`. A migration inserts none of it, so a hosted project has no city until one is made there.
