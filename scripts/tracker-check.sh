#!/usr/bin/env bash
# Tracker hygiene check (#435): the tracker rules from docs/agents/issue-tracker.md
# ("Planning rules") and the hygiene bar (#414), checked mechanically. Run weekly by
# .github/workflows/tracker-check.yml and on demand by `make tracker-check`.
#
#   scripts/tracker-check.sh                  fetch from GitHub with gh, then check
#   scripts/tracker-check.sh ISSUES PRS       check recorded JSON (the tests do this)
#
# ISSUES is a JSON array of REST issues (state all, no PRs): number, title, state,
# body, labels[].name, parent_issue_url. PRS is the output of
# `gh pr list --state merged --json number,title,mergedAt,headRefName,closingIssuesReferences`.
# Only the default GITHUB_TOKEN (issues: read, pull-requests: read) is needed.
#
# Prints one line per offending issue or PR and exits 1 if there are any.
set -euo pipefail

# Merged PRs are checked back this many days: a week plus a day of slack for a late
# run. Older PRs predate the rule and are not audited.
DAYS=${TRACKER_CHECK_DAYS:-8}
# "now" is injectable so the tests can pin the PR window.
now=${TRACKER_CHECK_NOW:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

if [ "$#" -eq 2 ]; then
  issues=$1
  prs=$2
elif [ "$#" -eq 0 ]; then
  issues=$tmp/issues.json
  prs=$tmp/prs.json
  # The issues endpoint returns PRs too (they carry a pull_request key): drop them.
  # Its parent_issue_url is the only place the parent shows up in one list call.
  gh api --paginate --slurp 'repos/{owner}/{repo}/issues?state=all&per_page=100' \
    | jq 'add | map(select(has("pull_request") | not))' >"$issues"
  gh pr list --state merged --limit 200 \
    --json number,title,mergedAt,headRefName,closingIssuesReferences >"$prs"
else
  echo "usage: $0 [ISSUES.json PRS.json]" >&2
  exit 2
fi

out=$tmp/findings.txt

jq -r '
  def names: [.labels[].name];
  def triage: ["needs-triage","needs-info","ready-for-agent","ready-for-human","wontfix"];
  def line(rule): "#\(.number) [\(rule)] \(.title)";

  # Exempt from the exactly-one-triage-label rule: wayfinder maps and tickets,
  # the Dependency Dashboard (#269) and the Ideas issue (#360).
  def exempt: .number == 269 or .number == 360 or (names | any(startswith("wayfinder:")));

  # A closed issue may keep wontfix: it records why it was closed.
  def stale_triage: names | map(select(IN(triage[]) and . != "wontfix"));

  ( .[] | select(.state == "open") | select(exempt | not)
    | (names | map(select(IN(triage[]))) | length) as $n
    | select($n != 1)
    | line("triage-label: \($n) triage labels, want exactly 1") ),

  ( .[] | select(.state == "closed") | select(stale_triage | length > 0)
    | line("closed-with-triage-label: \(stale_triage | join(","))") ),

  # ROADMAP.md is retired. Fails a link to it or an acceptance box that mentions
  # ROADMAP; a "Moved from ..." provenance line is allowed.
  ( .[] | select(.state == "open")
    | select((.body // "") | split("\n") | any(
        (test("^\\s*([-*] )?Moved from"; "i") | not)
        and (test("ROADMAP\\.md") or (test("^\\s*[-*] \\[[ xX]\\]") and test("ROADMAP"; "i")))))
    | line("retired-roadmap: references the retired ROADMAP") ),

  ( .[] | select(.state == "open")
    | select(names | any(. == "ready-for-agent" or . == "ready-for-human"))
    | select((.parent_issue_url // null) == null)
    | select(names | index("bug") | not)   # Pilot bugs may stand alone
    | line("no-parent: ready ticket has no parent") ),

  ( .[] | select(.state == "open")
    | select(names | index("ready-for-agent"))
    | select(names | any(startswith("size:")) | not)
    | line("no-size: ready-for-agent ticket has no size: label") ),

  ( .[] | select(names | any(. == "go" or . == "breaking-ok"))
    | line("pr-only-label: \(names | map(select(. == "go" or . == "breaking-ok")) | join(","))") )
' "$issues" >"$out"

# Merged PRs that close no issue. Release-please, Renovate and Dependabot PRs are
# exempt: they have no ticket by design.
cutoff=$(jq -rn --arg now "$now" --argjson d "$DAYS" '$now | fromdateiso8601 - ($d * 86400) | todate')
jq -r --arg cutoff "$cutoff" '
  .[] | select(.mergedAt >= $cutoff)
  | select(.headRefName | test("^(release-please--|renovate/|dependabot/)") | not)
  | select((.closingIssuesReferences | length) == 0)
  | "PR #\(.number) [merged-pr-closes-no-issue] \(.title)"
' "$prs" >>"$out"

if [ -s "$out" ]; then
  sort -V "$out" | tee "$tmp/sorted.txt"
  echo
  echo "$(wc -l <"$out" | tr -d ' ') tracker finding(s). Rules: docs/agents/issue-tracker.md"
  { echo "### Tracker check"; echo '```'; cat "$tmp/sorted.txt"; echo '```'; } >>"${GITHUB_STEP_SUMMARY:-/dev/null}"
  exit 1
fi
echo "tracker check: clean"
