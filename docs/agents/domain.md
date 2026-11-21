# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`GLOSSARY.md`** at the repo root.
- **ADRs in [`docs/adr/`](../adr/)**: read the ones that touch the area you're about to work in.
- **The spec in [`docs/spec/`](../spec/README.md)**: the document for the area you're about to work in. Each section links the ticket that holds its reasoning.
- **Resolution comments on closed issues**, where wayfinder tickets and triaged issues record what was decided. Search the tracker when an area's history matters.

If `GLOSSARY.md` doesn't exist, **proceed silently**. Don't flag its absence; don't suggest creating it upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates it lazily when terms actually get resolved.

## File structure

Single-context:

```
/
├── GLOSSARY.md
├── docs/adr/              ← the hard-to-reverse decisions
├── docs/spec/             ← the v2 spec, one document per area
├── apps/  backend/  packages/
```

## Recording a decision

Write ADRs in `docs/adr/` as the `/domain-modeling` skill describes, and only when its three tests hold (hard to reverse, surprising without context, a real trade-off). Most decisions don't need one: the resolution comment on the issue that settled it is enough. A product decision that changes the spec edits its document in `docs/spec/`.

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `GLOSSARY.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal: either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag decision conflicts

If your output contradicts an ADR or the spec, surface it explicitly rather than silently overriding:

> _Contradicts ADR 0002 (Issho's knowledge comes only from vouching), but worth reopening because…_
