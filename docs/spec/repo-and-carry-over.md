# Where v2 lives and what carries over

From [Where v2 lives and what carries over](https://github.com/shrujancheruku/issho/issues/570). Reference for it: [A digest of what v1 decided](https://github.com/shrujancheruku/issho/issues/534), on the unmerged [`research/v1-digest`](https://github.com/shrujancheruku/issho/blob/research/v1-digest/docs/research/v1-digest/README.md) branch.

| Question | Decision |
|---|---|
| The repo | v2 starts clean in this repo. The spec's tickets, research branches, ADRs and tracker setup are here and link to each other by issue number. |
| v1's code | The last v1 commit is tagged `v1-final`, then one PR deletes `apps/`, `backend/`, `proto/`, `packages/`, `site/`, `supabase/`, `data/`, `config/` and the v1-only docs (architecture, deploy, hosted Supabase, Google Maps, design system, onboarding). No legacy folder: it would still be linted, updated and copied from. That PR is the first ticket of the build. |
| v1's history and docs | History, tags, the ADRs and `docs/decisions-2026.md` stay. The research and prototype branches stay unmerged. |
| The spec's form | One document per product area under `docs/spec/`, each section linking its ticket. |
| Amendments | The documents are the spec. An amendment is a PR that edits the document ([README](README.md#amending-the-spec)). |
| The glossary | The v2 glossary is the root `GLOSSARY.md`. v1's stays readable at the tag. Until the deletion PR, main holds v2's glossary beside v1's code; v1 work is paused, so nothing reads the pair together. |
| Accounts and identifiers | The domain, the Cloudflare zone, Resend, the EAS project, the bundle ID and the store accounts carry over as they are. No data is migrated from any of them; if Supabase stays, the `issho-dev` project is wiped and reused. There are no waitlist emails to keep. |
| City data | None carries over. v2's knowledge comes only from vouching, and v1's catalogue was derived from Google Places in ways that break its terms. The twelve plans judged in the [prototype](plan-bar.md#what-the-prototype-showed) enter v2 as leads for the owner to vouch. |
| v1's running services | The API, the web build and the admin come down when the build starts. The site at `joinissho.com` stays up as deployed until v2's replaces it, and can be redeployed from the tag. |
| Agent tooling | Carried unchanged: the tracker conventions, `docs/agents/`, the flow setup, the git hooks, commitlint, gitleaks, the PR template and the tracker-check workflow. Rewritten by build tickets once the stack is chosen: the CI and deploy workflows, the Makefile, Renovate's config, the Supabase section of `CLAUDE.md` and `docs/secrets.md`. |
| Versions | v2's first release is `2.0.0`, in the same line of tags; `1.x` is skipped. The open `0.11.0` release PR is closed unmerged. |
| Reading and porting v1 | Anyone may read v1 at the tag. Copying anything needs a build ticket with the line "Ports `<path>` at `v1-final`". Candidates: the brand assets and colour tokens, the site's story copy, the legal pages' text, the secret-scanning setup. Everything else is written from the spec. |
