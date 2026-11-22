# retention

The registry of every table in the database: what personal data it holds about a person and how long a row is kept ([Technical operations](../../../docs/spec/operations.md#retention-and-deletion), [Privacy](../../../docs/spec/privacy.md#what-is-kept-and-for-how-long)). The `retention` CI check (`make retention`) fails when a table has no entry. The nightly deletion job and the account-deletion path are milestone 6's; they will read this registry.

## The registry

`registry.json` is a list of entries, one per table, named `schema.table` (every schema but Postgres's own: `app`, `river`, `goose`, and `public`, which should hold none):

```json
{"table": "app.queue_items", "personal_data": "none"}
{"table": "app.signups", "personal_data": "the account's ID and chosen times", "lifetime": "while the account exists"}
```

- `personal_data` is required. Say `none` when the table holds none; otherwise name what it holds.
- `lifetime` is required when the table holds personal data, in the words of the spec's retention table.
- An unknown field, a table with no schema, or a table listed twice makes the file fail to load.

A table with **no entry fails the check whatever it holds**: the registry is how a table is said to hold no personal data, so a table nobody has described counts as holding some. An entry for a table that no longer exists fails too, so the registry follows the schema. A migration that adds a table adds its entry in the same PR.

`personal_data: "none"` is a claim about the columns: when a later ticket puts a person's data in a table already marked `none` (an account's ID in `app.queue_items.subject`, say), it changes the entry in the same PR. The check can't see inside a column.

`river.river_job` is `none` for what the code puts in it, not for everything River writes there. Its `args` hold only what a job type carries (today `staff.DueArgs`, an item's ID); a job type that carried a person's data would change the entry. But its `errors` and `metadata` columns are River's, and a failed job's error text can quote a person's data (`backend/internal/jobs/joblog.go` keeps that text out of the logs for the same reason). The registry treats that text as the job's working state, not as data kept about a person, and so doesn't list it: River deletes a job with its errors 24 hours after it completes or is cancelled and 7 days after it is discarded (its defaults; `internal/jobs` sets no others), and a job still retrying keeps its errors until then. The nightly retention job (milestone 6) is where a shorter bound would go.

## The API

| | |
|---|---|
| `Load()` | The registry embedded in the binary |
| `Parse(r)` | A registry from JSON, rejecting an entry that says nothing useful |
| `Tables(ctx, pool)` | The `schema.table` of every table in the database |
| `Registry.Check(tables)` | One line per disagreement; none when the registry and the schema agree |

## Tests

`retention_test.go`, on a throwaway Postgres with every migration run:

| Test | Proves |
|---|---|
| `TestRegistryCoversEveryMigratedTable` | The registry in the repo and the migrated schema agree. This is the check CI's `retention` job runs |
| `TestTableWithPersonalDataAndNoEntryFails` | The fixture `testdata/unregistered_table.sql`, a table of email addresses with no entry, fails the check and the message names it |
| `TestEntryForATableThatIsGoneFails` | A stale entry fails |
| `TestParseRejectsAnEntryThatSaysNothingUseful`, `TestParseRejectsTheSameTableTwice` | The file's rules above |
