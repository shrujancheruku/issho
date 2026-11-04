## What & why

<!-- One or two sentences. `Closes #<n>` for the issue it resolves. -->

## Checklist

- [ ] PR title / commits follow Conventional Commits (`feat(scope): …`)
- [ ] Linked to its issue; an ADR in `docs/adr/` if it settles a hard-to-reverse decision
- [ ] Directory READMEs / docs updated if behaviour, setup or endpoints changed
- [ ] `make gen` run if `proto/` or `backend/db/queries/` changed
- [ ] `make lint test` pass locally
- [ ] Non-obvious logic has comments explaining *why*
- [ ] No secrets added (pre-commit hook enabled via `make setup`)
- [ ] New RPCs: auth requirement considered (deny-by-default)
- [ ] Migrations are new files (merged ones untouched) with a working `Down`

## How it was tested

<!-- Commands run, screenshots for UI changes. -->
