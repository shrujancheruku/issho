# season

The season ([The quality bar and test strategy](../../../docs/spec/quality.md#matching-and-the-rules)): a deterministic run of scripted people and plans over two weeks, through the real packages on the fake clock. CI fails if it ends with a broken invariant, and its log output is searched for the scripted members' emails and names. This is the season's first form: quality.md describes 60 people over 8 weeks, and this is 20 over two weeks. It grows in each later milestone, from join links to feedback; late cancels' costs, reports and account deletions arrive with the packages that own them.

`Run(ctx, pool, Options)` migrates the empty database behind `pool`, loads the fixture city and its two plans, and plays the script. It uses only the packages' exported APIs (`tools/boundaries` enforces that), so what it knows of the world it was told by them; it reads no table.

## The cast and the script

Twenty people, invented: made-up names (none a word a log line holds), emails on `example.com`, fixed account ids. Seventeen are admitted: the founder by the seed's admit (`people.AdmitByEmail`), the rest by join links in a tree, each member bringing in at most three (a member has three links in all; a used one does not return). Three never are: one tries a code that is nobody's, one a used code, one nothing, and all three try to sign up.

Over the plans' first two rounds (5 to 18 January 2026) the clock steps from event to event:

- **Joining**, the first evening: the account, the join link, the profile.
- **Signing up**, each round, at night, as the API does: admitted, then a profile, then `Signups.Set` with three to five dates from 3 days ahead to a round past the round's end. In the second round some tick a time they are already holding.
- **Forming**, each morning: the season calls `outings.Service.Form` itself, since forming isn't scheduled yet, where the spec's rules would: early when five people with a language between them share a time, and otherwise on the round's last day.
- **Answering**: each invited person says yes, no or nothing, at some hour of the 48; at 48 hours the season calls `Expire` (what the River job does: River's queue is paused, since its jobs run on the real clock, which would race the fake one).
- **Going**: some cancel the day before, some don't come, the rest check in when it opens; everyone who was going at the start answers feedback once it opens, and a no-show sometimes says they went.

Every draw comes from one seeded generator and is taken in an order that depends on nothing random, so two runs make the same outings. A fresh database and the seed are the whole input.

## What it checks

`CheckInvariants` on what the season saw (`World`), at the end:

- every outing that was set had 3 to 6 people going, who share a language and had ticked its date and part of the day;
- nobody was going to two outings at one time, checked at each sweep (after every forming and every answer) on the seats going at that moment, so a double booking is caught even if one seat is cancelled later;
- nobody who isn't admitted holds a signup;
- what feedback says of attendance is what `Attended` says (checked in or said so; their own no wins).

`Broken` builds a world with one of these broken, for the test that proves each check can fail. Refusals the packages must make are counted too, and the test fails if one never happens: a fourth join link, a used or made-up code, a signup by someone not admitted, a yes at a time already taken.

## The log search

The run's logger (`logging.New`) goes to every package that logs, River's client included, and its output is searched (`Search`) for each member's email, name, first name and last name, in any case. The test also captures the standard logger, which slog's default writes through. A hit names the member and which detail, never the detail. `Options.Plant` lets a test log a member's name partway through to prove the run reports it.

## The scorecard

`Scorecard` is the share of signups (a member's signup to a plan in a round) that got a group, and that got an outing (the person was going at some point), and how many people got none. `make season` prints it (`cmd/season`).

## Testing

`season_test.go`: each criterion of the first form, the determinism of two runs, the invariants and their failure on purpose (a double booking at a sweep is caught though a seat is cancelled later; a seat cancelled by a sweep is not counted), and the log search with its planted names. The whole package runs in about 10 seconds against the throwaway Postgres `make test` starts.
