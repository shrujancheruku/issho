# v2 is written from scratch

Issho v1 was built fast, without its product decisions being questioned first. Before the Pilot we stopped, decided the whole product again from the root ([Wayfinder: Issho v2, from scratch](https://github.com/shrujancheruku/issho/issues/532)) and wrote the result down as the spec in [`docs/spec/`](../spec/README.md). v2 is built from that spec in this repo, with v1's code deleted rather than changed to fit. The fault was in what v1 had decided more than in how it was coded, and the spec overturned enough of it (how plans are made, what Issho stores about places, matching, the profile, the stages of launch) that changing v1 in place would have meant rewriting most of it under the weight of its old shape. v1 had no users and no data, so nothing was lost.

This reverses an earlier verdict. Days before that, every part of v1 was audited and costed both ways, and the answer was "nothing is rebuilt": keep most parts, renovate the ones with the most debt. That audit compared the cost of reaching a code-quality bar with the product held fixed. Once the product itself was open, its premise was gone.

## Considered options

- **Renovate v1 to the new spec.** Rejected: the audit's cost advantage rested on v1's domain fit, and the spec removed that fit in the largest parts (plans, the catalogue, cities, matching).
- **Keep v1 in a legacy folder.** Rejected: it would still be linted, updated and copied from.
- **A new repo.** Rejected: the spec's tickets, research branches and tracker setup are here and link to each other by issue number.

## Consequences

- The last v1 commit is tagged `v1-final` and the build's first PR deletes v1. Anyone may read v1 at the tag; copying anything needs a build ticket with the line "Ports `<path>` at `v1-final`".
- No data, city data or open ticket carries over. Every v1 issue was closed unplanned, and the build's tickets are written from the spec ([Where v2 lives and what carries over](../spec/repo-and-carry-over.md)).
- v1's ADRs and its decision log were deleted with it. Two decisions were carried into this folder, rewritten for v2: the tombstone ([ADR 0003](0003-a-deleted-account-leaves-a-tombstone.md)) and the owner as operator ([ADR 0004](0004-the-owner-operates-issho-until-the-public-launch.md)). The old files are in history at [`2bd1c7f`](https://github.com/shrujancheruku/issho/tree/2bd1c7f/docs).
- v2's first release is `2.0.0`.
