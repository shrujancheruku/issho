# policy

Every number in the spec's rules, as a constant. None is in the database, and none varies by city ([Architecture and stack](../../../docs/spec/architecture.md#the-seams)). Other backend packages import these names and never write the number again.

| Constant | Value | Set by |
|---|---|---|
| `MinGroupSize`, `MaxGroupSize` | 3 and 6 | [Matching](../../../docs/spec/matching.md) |
| `PreferredGroupSizeLow`, `PreferredGroupSizeHigh` | 4 and 5 | [Matching](../../../docs/spec/matching.md) |
| `InvitationWindow` | 48 hours | [Outings](../../../docs/spec/outings.md) |
| `RoundLength` | 7 days | [Matching](../../../docs/spec/matching.md) |
| `CheckInOpensBeforeStart` | 30 minutes | [Outings](../../../docs/spec/outings.md) |
| `FeedbackWindow` | 7 days | [Outings](../../../docs/spec/outings.md) |
| `MinAge` | 18 | [People](../../../docs/spec/people.md) |
| `NameMinLength`, `NameMaxLength` | 1 and 40 characters | [People](../../../docs/spec/people.md) |
| `MinTickLeadDays` | 3 days | [Matching](../../../docs/spec/matching.md) |

These are the numbers the thin loop needs. A later ticket adds a number to this package when it builds the rule that uses it.

## Changing a number

A number changes only by a PR that edits the spec document and the constant together. If the two disagree, the document wins and the constant is the bug.

## Adding a number

Give the constant a comment that names the spec document that sets it, as a path from the repo root: `// docs/spec/outings.md`. The lint below fails the build without one, or when the file isn't there. `policy_test.go` checks each value against the spec's own words.

## The lint

`make policy-lint` (run by `make lint`) is [`tools/policylint`](../../tools/policylint/README.md). It fails on a constant here with no link to a file under `docs/spec/`, on a link to a file that doesn't exist, and on one of these numbers written as a literal in another backend package.
