# season

`make season` plays the season ([`internal/season`](../../internal/season/README.md)) and prints its scorecard: how many signups there were, the share that got an outing (grouped, and going), and how many people got none. Run it to try a change to a number in `internal/policy` before making it ([The quality bar and test strategy](../../../docs/spec/quality.md#matching-and-the-rules)).

```
signups: 42
grouped: 34 (81%)
got an outing: 21 (50%)
people who signed up: 17
people who got none: 3
```

It starts its own throwaway Postgres 17 in Docker and removes it when it ends, so it needs Docker and takes no database URL or environment, and cannot reach any other database. Exit status: 0 when every invariant held and the log held none of the cast's emails or names, 1 when one broke (each is printed to stderr), 2 when it could not run.

It is not in the production image. `make test` runs the same season as a test (`internal/season`), so CI fails on a broken invariant whether or not anyone runs this.
