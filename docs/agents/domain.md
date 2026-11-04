# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`GLOSSARY.md`** at the repo root.
- **ADRs in [`docs/adr/`](../adr/)**: read the ones that touch the area you're about to work in.
- **The frozen decision log in [`docs/decisions-2026.md`](../decisions-2026.md)** (newest at the bottom): every decision made before 2026-11-04. Read the entries for your area too; don't add rows.
- **Resolution comments on closed issues**, where wayfinder tickets and triaged issues record what was decided. Search the tracker when an area's history matters.

If `GLOSSARY.md` doesn't exist, **proceed silently**. Don't flag its absence; don't suggest creating it upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates it lazily when terms actually get resolved.

## File structure

Single-context:

```
/
├── GLOSSARY.md
├── docs/adr/              ← decisions from 2026-11-04 on
├── docs/decisions-2026.md ← frozen decision log (before 2026-11-04)
├── apps/  backend/  packages/
```

## Recording a decision

Write ADRs in `docs/adr/` as the `/domain-modeling` skill describes, and only when its three tests hold (hard to reverse, surprising without context, a real trade-off). Most decisions don't need one: the resolution comment on the issue that settled it is enough. Never add rows to `docs/decisions-2026.md`.

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `GLOSSARY.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal: either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag decision conflicts

If your output contradicts an ADR or an entry in the decision log, surface it explicitly rather than silently overriding:

> _Contradicts the decision log entry of 2026-10-24 (city packs are edited through GitHub pull requests), but worth reopening because…_
