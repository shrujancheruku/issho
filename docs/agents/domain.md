# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`GLOSSARY.md`** at the repo root.
- **The Decision log in [`docs/ROADMAP.md`](../ROADMAP.md)** (the table under "Decision log", newest at the bottom): read the entries that touch the area you're about to work in. This repo has no `docs/adr/`; the decision log is its record of decisions, and CI requires ROADMAP updates with every code change.

If `GLOSSARY.md` doesn't exist, **proceed silently**. Don't flag its absence; don't suggest creating it upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates it lazily when terms actually get resolved.

## File structure

Single-context:

```
/
├── GLOSSARY.md
├── docs/ROADMAP.md        ← Decision log (instead of docs/adr/)
├── apps/  backend/  packages/
```

## Recording a decision

Where a skill says to write an ADR, add a row to the Decision log table in `docs/ROADMAP.md` instead (date, decision, why), in the style of the rows around it, and update "Last updated". Don't create `docs/adr/`.

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `GLOSSARY.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal: either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag decision conflicts

If your output contradicts an entry in the decision log, surface it explicitly rather than silently overriding:

> _Contradicts the decision log entry of 2026-10-24 (city packs are edited through GitHub pull requests), but worth reopening because…_
