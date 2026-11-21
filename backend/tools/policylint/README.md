# policylint

The check behind `make policy-lint`, which `make lint` runs. It holds the backend to the rule in [Architecture and stack](../../../docs/spec/architecture.md#the-seams): every number in the rules is a constant in [`internal/policy`](../../internal/policy/README.md), linked to the spec document that sets it.

```sh
make policy-lint                              # from the repo root
cd backend && go run ./tools/policylint -root ..
```

It prints one line per finding, as `file:line: kind: message`, and exits 1; exit 2 means it couldn't run (a package it can't parse).

## What it checks

| Kind | Fails when |
|---|---|
| `no-link` | A constant in `internal/policy` has no comment naming a file under `docs/spec/` (`docs/spec/outings.md`). The comment is the constant's own doc or trailing comment; a comment above a `const (` block doesn't count for the constants inside it. |
| `dead-link` | A constant's comment names a `docs/spec/` file that doesn't exist. The `#anchor` of a link isn't checked. |
| `literal` | Another backend package writes a policy number as a literal: an integer equal to one of the policy's integers, or a duration (`48 * time.Hour`, `time.Duration(7) * 24 * time.Hour`) equal to one of its durations. The message names the constant to use. |

The literal check leaves out `_test.go` files (a test states the spec's number on its own, so it can disagree with the constant), generated code, `testdata`, and the numbers 0 and 1. It sees only literal integers and `time` units, not a number written as a name or in another unit, so a round written as `AddDate(0, 0, 7)` isn't found; review catches those.

A number that only looks like a rule's (a page size of 3) is allowed with a comment on its line or the line above, and a reason: `//policylint:ignore a page size, not a group size`. Without a reason the comment doesn't count.

The spec says that if the literal check proves noisy in the first milestone, it becomes a review check ([The quality bar](../../../docs/spec/quality.md#what-ci-requires)). Delete `checkLiterals` then, and the `literal-*` fixtures with it.

## Tests and fixtures

`check_test.go` runs `Check` on small repos under `testdata/`, each with its own `docs/spec` and `backend/internal/policy`:

| Fixture | Proves |
|---|---|
| `clean` | Linked constants pass |
| `unlinked` | `no-link` |
| `deadlink` | `dead-link` |
| `literal-int` | `literal`, for an integer |
| `literal-duration` | `literal`, for durations, one that matches two constants |
| `literal-allowed` | Using the constants, a duration that is no rule's, tests, generated code and `//policylint:ignore` all pass |

`TestRepoPasses` runs the lint on this repo. Go ignores `testdata`, so the fixtures are parsed but never built; `gofmt` still reads them, so keep them formatted.
