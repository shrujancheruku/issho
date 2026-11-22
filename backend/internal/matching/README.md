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

Nobody is in two groups. A tick whose part isn't morning, afternoon or evening is ignored.

A person who appears more than once is one person if the entries say the same (ticks and languages in any order count as the same), and is never grouped if the entries differ. Matching can't know which of two entries a hard rule (a language, the setting) should come from, so it picks neither; the result is the same whatever the order. The caller sends one signup for each person.

## How it chooses

The whole assignment is ranked, not one date at a time ("Sizes" in [Matching](../../../docs/spec/matching.md#rounds)):

1. **The most people seated**, over every date together. Seating the most on one date never wins over seating more across two: Monday {p1, p2, p3, p4} with Tuesday {p5, p6, p7} seat 7, where taking Tuesday's five (with p1 and p2) first would seat 5.
2. **Then sizes nearest 4 or 5**: the sum of each group's distance from that range (`policy.PreferredGroupSizeLow` and `High`).
3. **Then fewer groups.** So 6 make one group of 6, 7 make 4 and 3, 8 make 4 and 4, 9 make 5 and 4, and 61 make nine of 5 and four of 4.
4. **Then the date that seats the most wins, and a tie goes to the earliest** (date, then morning, afternoon, evening); then the next date is weighed the same way.
5. Anything still equal is settled by the order the search tries things in, which depends only on the people and the rules.

The same input always gives the same output, whatever the order of the signups, ticks or kept-apart pairs.

## How exact it is

The search is a branch and bound over the whole assignment, and it is exact except when it runs out of work. Its limits (`seatWork`, `dateWork`, `groupsPerNode`) count search nodes and candidate groups, never a clock, so a result is the same on every machine. When it runs out it returns the best valid groups it has found: every rule still holds, but a different grouping might seat more people or sit nearer 4 or 5.

What was measured, by `measure_internal_test.go` (below), on random rounds from a fixed seed, of the shape the property tests generate (three languages, a quarter with the women-and-non-binary setting, a kept-apart pair for every three people, six times to tick). The figures are what one run printed, not a proof for every round:

- **Small rounds:** in 300 random rounds of up to 14 people ticking about half the times, none ran out of work. `TestTheSearchFinishesSmallAndOpenRounds` holds the same claim for another 300 such rounds (without kept-apart pairs) in every `make test`.
- **A crowd who can all meet:** 60 people who can all meet at any of six times were settled in 295 steps for the most seated, without running out of work (the same test).
- **Larger rounds:** 360 rounds of 14 to 60 people (14, 18, 24, 40, 50 and 60 people, each at three tick densities of 0.3, 0.6 and 0.9, 20 rounds each), each searched as `Match` searches and again with ten times the work.
  - The first search (the most seated, then sizes and groups) ran out of work in 69 of the 360 rounds: in none of the 60 at 14 people, in 5 of the 20 at 18 people ticking 0.9 (and none of the other 40 at 18), in 5 of the 60 at 24 people, and in 2 to 14 of every 20 at 40, 50 or 60 people (14 of 20 at 60 people ticking 0.3 is the most).
  - 33 of the 360 gave a different answer than ten times the work did, almost all in the sizes or the number of groups. One of them seated one person fewer than the longer search found (40 people ticking 0.9). The longer search is itself a bound on the best, not a proof.
  - Cost is counted in steps, not seconds (the repo's clock rule keeps `time.Now` out of tests too): the most any one round's first search took was 400,289 steps, which is its limit (`seatWork`, 400,000) plus the last candidates weighed. Matching runs when a round closes, not while someone waits.

The rounds in the Pilot are far smaller than the dense, larger ones where it fell short ([Matching](../../../docs/spec/matching.md#rounds), "Sizes").

Nothing here is exact beyond those, and this README does not claim it. When the search runs out, a person left out of the groups could still have been seated, so a caller who needs to explain "why wasn't I picked?" should not read an empty result at 18 or more people as proof. The seat order (milestone 4) is where leftovers are decided.

## Tests

`matching_test.go` has the examples through `Match`: the named sizes (3 through 12), 18, 60 and 61 people at one time, each rule, the date that seats the most, the earliest on a tie, seating the most across dates, duplicate signups and an unknown part. `property_test.go` generates rounds from a fixed seed and holds each rule on every group, that nobody left over could have formed a group, determinism and shuffle-invariance, Pilot-sized rounds, that the package's non-test files import only `cmp`, `slices` and `policy`, and the best grouping against a brute-force search that tries every assignment: at one time, across three dates, and across the parts of a day. Those brute-force rounds draw closed dates too, so a closed day has to seat nobody.

`exact_internal_test.go` holds the small rounds and the open crowd of 60 above: the search finishes them within its limits.

`measure_internal_test.go` prints the larger-round figures above. It is slow (about three minutes), so it is behind the `measure` build tag and not part of `make test`; run it on request:

```sh
cd backend && go test -tags measure -run TestMeasure -v ./internal/matching
```
