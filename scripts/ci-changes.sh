#!/usr/bin/env bash
# Which of CI's path-filtered jobs have something to check. Usage:
#
#   git diff --name-only BASE...HEAD | scripts/ci-changes.sh   # the files a change touches, on stdin
#   scripts/ci-changes.sh --all < /dev/null                    # every job, whatever changed
#
# Prints one `<job>=true|false` line per job (the format $GITHUB_OUTPUT reads).
# The workflow's `changes` job runs it, and each filtered job skips itself when
# its line is false. A job skipped that way reports as passing, which is why
# every job can be a required check: a workflow-level `paths:` filter would
# leave the check "expected" forever on a PR that touches none of them.
#
# The patterns are deliberately generous: a job that runs for no reason costs
# minutes, a job that doesn't run when it should lets a break through. The jobs
# not listed (secrets, commits, codegen) always run.
set -euo pipefail

all=false
case "${1:-}" in
  --all) all=true ;;
  "") ;;
  *) echo "usage: ci-changes.sh [--all]  (changed paths on stdin)" >&2; exit 2 ;;
esac

# What every job runs on, so a change to it touches every job's paths: the
# workflow, its local actions, the Makefile, these scripts, the lockfiles and
# workspace files every install reads, and the tool pins. Left out, a break in
# one of them would skip the jobs it breaks, and each would report as passing.
shared='Makefile|\.github/workflows/ci\.yml|\.github/actions/.*|scripts/ci-.*|package\.json|pnpm-(lock|workspace)\.yaml|(.*/)?go\.(mod|sum)|go\.work(\.sum)?|tools/.*'
# The Go side. docs/spec is here because the policy lint reads the spec's documents.
go="(backend|proto|tools|docs/spec)/.*|go\.work(\.sum)?|buf(\.gen)?\.yaml"
# The TypeScript side. proto because the client is generated from it.
ts="(apps|packages|proto|e2e)/.*|package\.json|pnpm-.*"

# job|extended regex over a whole path. Output order is the order here.
patterns=$(cat <<PATTERNS
backend|^($shared|scripts/proto-breaking.*|$go)$
boundaries|^($shared|$go)$
e2e|^($shared|scripts/e2e.*|backend/.*|supabase/templates/.*|$ts)$
image|^($shared|scripts/image.*|backend/.*|apps/(app|web)/.*|packages/.*)$
native-bundle|^($shared|scripts/native-.*|apps/app/.*|packages/.*|proto/.*|package\.json|pnpm-.*)$
policy|^($shared|$go)$
proto-breaking|^($shared|scripts/proto-breaking.*|proto/.*|buf(\.gen)?\.yaml|tools/.*)$
retention|^($shared|$go)$
rpc-auth|^($shared|proto/.*|backend/.*|tools/.*|go\.work(\.sum)?|buf(\.gen)?\.yaml)$
strings|^($shared|apps/app/.*|package\.json|pnpm-.*)$
typescript|^($shared|$ts)$
PATTERNS
)

changed=$(cat)
while IFS='|' read -r job regex; do
  if $all || { [ -n "$changed" ] && grep -Eq "$regex" <<<"$changed"; }; then
    echo "$job=true"
  else
    echo "$job=false"
  fi
done <<<"$patterns"
