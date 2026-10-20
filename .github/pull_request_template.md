## What & why

<!-- One or two sentences. Link the roadmap item (docs/ROADMAP.md). -->

## Checklist

- [ ] PR title / commits follow Conventional Commits (`feat(scope): …`)
- [ ] `docs/ROADMAP.md` updated (status, decisions, "Last updated")
- [ ] Directory READMEs / docs updated if behaviour, setup or endpoints changed
- [ ] `make gen` run if `proto/` or `backend/db/queries/` changed
- [ ] `make lint test` pass locally
- [ ] Non-obvious logic has comments explaining *why*
- [ ] No secrets added (pre-commit hook enabled via `make setup`)
- [ ] New RPCs: auth requirement considered (deny-by-default)
- [ ] Migrations are new files (merged ones untouched) with a working `Down`

## How it was tested

<!-- Commands run, screenshots for UI changes. -->
