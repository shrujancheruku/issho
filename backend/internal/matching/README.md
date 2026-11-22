# matching

Matching forms groups and sets dates among a round's signups. It is a pure function: no database, no clock and no outside service ("Matching is a pure function" in [Architecture and stack](../../../docs/spec/architecture.md#the-seams); the rules are in [Matching](../../../docs/spec/matching.md)). The `plans` and `outings` packages load its inputs and store its results.

This is the thin loop's matching: only the rules that are never broken. The seat order, preferences, rounds, the weather and the extra invitations come with the outing in full ([The build order](../../../docs/spec/build-order.md)).

```go
groups := matching.Match(matching.Input{
	Signups: []matching.Signup{{
		Person:    "p1",
		Ticks:     []matching.Slot{{Date: "2026-12-07", Part: matching.Morning}},
		Languages: []string{"vi", "en"},
		Gender:    matching.Woman,
		// WomenAndNonBinaryOnly: true,
	}, /* ... */},
	KeptApart:   []matching.Pair{{A: "p1", B: "p7"}},
	ClosedDates: []string{"2026-12-09"},
})
// groups[0]: Date, Part, People (by ID) and Languages (every one they all share)
```

Dates are the city's calendar dates (`"2006-01-02"`, as `cityclock.Date` prints them) and `Part` carries `cityclock.Part`'s values, so the caller converts with `matching.Part(p)`. Turning an instant into a part of the day, and a closed day into a date, is the caller's job with `cityclock`; this package imports no `time`.

## What it guarantees

Every group:

- is 3 to 6 people (`policy.MinGroupSize`, `policy.MaxGroupSize`), all of whom ticked its date and part of the day;
- has at least one spoken language its people all share, and lists every one of them. It never builds a two-language group and prefers no language;
- holds no kept-apart pair, in either order;
- is not on a closed date;
- is made up only of women and non-binary people when any of its people has that setting on. A person with the setting on whose gender is neither is never grouped.

Nobody is in two groups. A person who appears twice in the signups counts once, by their first entry, and a tick whose part isn't morning, afternoon or evening is ignored.

## How it chooses

1. **The date that seats the most wins**, and a tie goes to the earliest (date, then morning, afternoon, evening). Its groups are formed, those people leave the pool, and the next date is chosen the same way until none seats anyone.
2. **At one date and part of the day**, the groups seat the most people first, then prefer sizes nearest 4 or 5 (the sum of each group's distance from that range, `policy.PreferredGroupSizeLow` and `High`), then fewer groups. So 6 make one group of 6, 7 make 4 and 3, 8 make 4 and 4, 9 make 5 and 4.

The same input always gives the same output, whatever the order of the signups, ticks or kept-apart pairs: people are ordered by ID and ties go to what the search found first.

The search at one time is exact for rounds of the size the Pilot has (the tests compare it with a brute-force search). It stops at a fixed count of steps (`searchSteps`, not a clock) and at a fixed number of candidate groups for a person (`groupsPerNode`) so a crowd of a few dozen at one time still returns; past those, it returns valid groups that may not be the best.

## Tests

`matching_test.go` has the examples through `Match`: the named sizes (3 through 12), each rule, the date that seats the most, the earliest on a tie, a duplicate person and an unknown part. `property_test.go` generates rounds from a fixed seed and holds each rule on every group, that nobody left over could have formed a group, determinism and shuffle-invariance, the best grouping against a brute-force search of every way to seat up to eight people at one time, and that the package's non-test files import only `cmp`, `slices`, `sort` and `policy`.
