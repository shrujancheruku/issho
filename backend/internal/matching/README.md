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

What was measured, on random rounds of the shape the tests generate (three languages, a quarter with the women-and-non-binary setting, a kept-apart pair for every three people, six times to tick), comparing each answer with the same search given ten times the work:

- **Exact with room to spare:** every round of up to 14 people ticking about half the times, and a crowd of any size who can all meet (60 at any of six times is settled in about a hundred steps). `TestTheSearchFinishesSmallAndOpenRounds` holds both.
- **Where it can fall short:** from 18 people up, when most people tick most of the times, and from 40 up even with few ticks. The first search (the most seated, then sizes and groups) reached its limit in 58 of the 360 rounds measured (14 to 60 people, three tick densities, 20 rounds each), about one in six. The date tie-break reached its limit in most rounds of 24 or more people; it only chooses among groupings already equal on those three, so it changes which date, not who is seated.
- **How much it cost:** of those 360 rounds, 20 gave a different split than ten times the work did, almost all in the sizes or the number of groups. Two of them seated one person fewer than the longer search found (24 people ticking nine times in ten, and 60 ticking three in ten). The longer search is itself a bound on the best, not a proof.
- **Time:** the slowest round took about 1.5 seconds. Matching runs when a round closes, not while someone waits.

Nothing here is exact beyond those, and this README does not claim it. When the search runs out, a person left out of the groups could still have been seated, so a caller who needs to explain "why wasn't I picked?" should not read an empty result at 18 or more people as proof. The seat order (milestone 4) is where leftovers are decided.

## Tests

`matching_test.go` has the examples through `Match`: the named sizes (3 through 12), 18, 60 and 61 people at one time, each rule, the date that seats the most, the earliest on a tie, seating the most across dates, duplicate signups and an unknown part. `property_test.go` generates rounds from a fixed seed and holds each rule on every group, that nobody left over could have formed a group, determinism and shuffle-invariance, the best grouping against a brute-force search both at one time and across three dates, Pilot-sized rounds, and that the package's non-test files import only `cmp`, `slices` and `policy`.
`exact_internal_test.go` holds the measured claim above: the search finishes small and open rounds within its limits.
