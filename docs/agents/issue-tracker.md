# Issue tracker: GitHub

Issues and specs for this repo live as GitHub issues. Use the `gh` CLI for all operations.

## Conventions

- **Create an issue**: `gh issue create --title "..." --body "..."`. Use a heredoc for multi-line bodies.
- **Read an issue**: `gh issue view <number> --json number,title,body,labels,comments`.
- **List issues**: `gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'` with appropriate `--label` and `--state` filters.
- **Make an issue a sub-issue of a parent**: `gh issue create --parent <parent> ...`, or `gh issue edit <parent> --add-sub-issue <child>` afterwards (`gh` 2.94+). Older `gh`: `gh api --method POST repos/<owner>/<repo>/issues/<parent>/sub_issues -F sub_issue_id=<child-db-id>` (database id, as in **Blocking** below). Without sub-issues, put `Part of #<parent>` at the top of the child body.
- **Comment on an issue**: `gh issue comment <number> --body "..."`
- **Apply / remove labels**: `gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- **Close**: `gh issue close <number> --comment "..."`

Infer the repo from `git remote -v`; `gh` does this automatically when run inside a clone.

The repository is public: an issue says what and why, never who, and never includes secrets, personal data or user content.

## Planning rules

How work is planned and tracked. The decision is in the resolution comment of #418.

- **Work parent.** One parent issue is active at a time, and every `ready-for-agent` or `ready-for-human` ticket is a sub-issue of it. Sub-issue order is run order; the owner reorders it on the parent. It is the only target for orchestrate runs. Tickets reach it only through triage. No orphan tickets, except the Pilot-bug exception below.
- **Ticket shape.** Use `.github/ISSUE_TEMPLATE/ticket.md`: what and why; acceptance criteria as behaviour you can observe; expected paths; a required `size:S|M|L` label; blockers wired as native dependencies (see **Blocking**); no references to retired docs. A ticket fits one worker session; an L that isn't on a risk path gets split. The PR-only labels `go` and `breaking-ok` never go on issues.
- **When a ticket is ready.** Label it `ready-for-agent` only when no decision is left open in it (open decisions go to a wayfinder ticket first), its criteria can be tested, and it has a size label, a parent and wired blockers.
- **Pilot-bug exception.** A bug found during the Pilot may stand alone, without a parent, and goes to the front of the run order.
- **Fog and deferred ideas.** In-scope fog stays in the active map's **Not yet specified** section. Ideas beyond any current destination go as bullets in the one pinned Ideas issue (#360), not as an issue each. An idea becomes an issue only when the owner makes it a destination or pulls it into a plan.
- **Reviews file findings for triage.** A review never files `ready-for-agent` tickets directly. It opens one parent issue of its own, with its findings as sub-issues labelled `needs-triage`. The owner moves each into the work parent (shaped and labelled ready), into the Ideas list, or closes it.
- **When to re-plan.** At each Pilot checkpoint, reorder open tickets and close stale ones. Before the Pilot, review the plan when each audit-driven milestone finishes.

## The tracker check

`make tracker-check` (`scripts/tracker-check.sh`, needs `gh` logged in) checks the rules above and the hygiene bar (#414). `.github/workflows/tracker-check.yml` runs it every Monday and on demand; a failing run lists the offenders in its log and job summary. It uses only the default `GITHUB_TOKEN`. It fails when:

- an open issue does not have exactly one triage label (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). Exempt: `wayfinder:*` issues, the Dependency Dashboard (#269) and the Ideas issue (#360);
- a closed issue still carries a triage label. `wontfix` may stay on a closed issue: it records why it was closed;
- an open issue links `docs/ROADMAP.md` or has an acceptance box mentioning the ROADMAP. A `Moved from ...` provenance line is allowed;
- a PR merged in the last 8 days closes no issue (`TRACKER_CHECK_DAYS` changes the window). Release-please, Renovate and Dependabot PRs are exempt;
- an open `ready-for-agent` or `ready-for-human` issue has no parent (a `bug` is exempt, the Pilot-bug exception), or a `ready-for-agent` issue has no `size:` label;
- an issue carries a PR-only label (`go`, `breaking-ok`).

The rules are in `scripts/tracker-check.sh`, and `scripts/tracker-check.test.sh` tests them against recorded `gh` JSON in `scripts/testdata/tracker-check/` (CI runs it too).

## Pull requests as a triage surface

**PRs as a request surface: no.** _(Set to `yes` if this repo treats external PRs as feature requests; `/triage` reads this flag.)_

When set to `yes`, PRs run through the same labels and states as issues, using the `gh pr` equivalents:

- **Read a PR**: `gh pr view <number> --comments` and `gh pr diff <number>` for the diff.
- **List external PRs for triage**: `gh api --paginate 'repos/{owner}/{repo}/pulls?state=open' --jq '.[] | select(.author_association | IN("OWNER","MEMBER","COLLABORATOR") | not) | {number, title, author: .user.login, author_association, labels: [.labels[].name]}'`.
- **Comment / label / close**: `gh pr comment`, `gh pr edit --add-label`/`--remove-label`, `gh pr close`.

GitHub shares one number space across issues and PRs, so a bare `#42` may be either: resolve with `gh pr view 42` and fall back to `gh issue view 42`.

## When a skill says "publish to the issue tracker"

Create a GitHub issue.

## When a skill says "fetch the relevant ticket"

Read it as in **Read an issue** above.

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single issue with **child** issues as tickets.

- **Map**: a single issue labelled `wayfinder:map`, holding the Notes / Decisions-so-far / Fog body. `gh issue create --label wayfinder:map`.
- **Child ticket**: an issue linked to the map as a GitHub sub-issue (see **Make an issue a sub-issue of a parent**). Where sub-issues aren't enabled, add the child to a task list in the map body and put `Part of #<map>` at the top of the child body. Labels: `wayfinder:<type>` (`research`/`prototype`/`grilling`/`task`). Once claimed, the ticket is assigned to the driving dev.
- **Blocking**: GitHub's **native issue dependencies**, the canonical, UI-visible representation. Add an edge with `gh api --method POST repos/<owner>/<repo>/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>`, where `<blocker-db-id>` is the blocker's numeric **database id** (`gh api repos/<owner>/<repo>/issues/<n> --jq .id`, _not_ the `#number` or `node_id`). GitHub reports `issue_dependencies_summary.blocked_by` (open blockers only, the live gate). Where dependencies aren't available, fall back to a `Blocked by: #<n>, #<n>` line at the top of the child body. A ticket is unblocked when every blocker is closed.
- **Frontier query**: list the map's open children (`gh issue list --state open`, scoped to the map's sub-issues / task list), drop any with an open blocker (`issue_dependencies_summary.blocked_by > 0`, or an open issue in the `Blocked by` line) or an assignee; first in map order wins.
- **Claim**: `gh issue edit <n> --add-assignee @me`, the session's first write.
- **Resolve**: `gh issue comment <n> --body "<answer>"`, then `gh issue close <n>`, then append a context pointer (gist + link) to the map's Decisions-so-far.
