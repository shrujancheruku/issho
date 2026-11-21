#!/usr/bin/env bash
# Checks the shape .github/workflows/ci.yml has to keep, which actionlint can't:
# the required checks exist under their exact names, every one can be required,
# the make targets they call exist, and nothing retries. Run from anywhere:
# scripts/ci-workflow.test.sh
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo"
workflow=.github/workflows/ci.yml
fail=0
bad() { echo "FAIL $*"; fail=1; }

# The required checks, as the founder sets them in branch protection. A check's
# name is its job id (no `name:` overrides, so the two can't drift).
required="secrets commits codegen backend boundaries policy rpc-auth retention proto-breaking typescript strings e2e native-bundle image"
# The jobs that skip themselves when their paths are untouched
# (scripts/ci-changes.sh names the same ones).
filtered="backend boundaries policy rpc-auth retention proto-breaking typescript strings e2e native-bundle image"

# The workflow without its full-line comments and blank lines, which the checks
# below read so a comment can't trip or satisfy them.
code=$(grep -vE '^\s*(#|$)' "$workflow")
# block <job>: the job's lines in the workflow.
block() { awk -v job="  $1:" '$0 == job {on=1; next} /^  [A-Za-z0-9_-]+:/ {on=0} on' <<<"$code"; }

for job in $required changes; do
  [ -n "$(block "$job")" ] || bad "job '$job' is missing"
done

# Job ids and checks are the same thing only while no job renames itself.
grep -nE '^    name:' "$workflow" && bad "a job has a name:, which would make its check differ from its id"

# A required check that skips must skip as a job (reported as passing), never
# through a workflow-level paths filter, which would leave the check pending.
awk '/^on:/ {on=1; next} /^[a-z]/ {on=0} on' <<<"$code" | grep -nE 'paths(-ignore)?:' && bad "the workflow filters on paths; use scripts/ci-changes.sh"

for job in $filtered; do
  b=$(block "$job")
  grep -q "needs.changes.outputs.$job == 'true'" <<<"$b" || bad "$job does not skip on needs.changes.outputs.$job"
  # If the changes job itself fails, the jobs run rather than all skip as
  # "passing".
  grep -q "needs.changes.result != 'success'" <<<"$b" || bad "$job skips when the changes job fails"
done

# `breaking-ok` skips proto-breaking and nothing else.
grep -q "breaking-ok" <<<"$(block proto-breaking)" || bad "proto-breaking does not look at the breaking-ok label"
[ "$(block proto-breaking | grep -c "breaking-ok")" -ge 1 ] || bad "proto-breaking lacks the label check"
[ "$(grep -c "contains(github.event.pull_request.labels" <<<"$code")" -eq 1 ] || bad "only proto-breaking may look at labels"
# make lint runs buf breaking and nothing can switch that off, so no job that
# honours the label may run it.
grep -nE 'make( [a-z-]+)* lint' <<<"$code" && bad "a job runs make lint, which includes buf breaking even on a breaking-ok PR"
# buf breaking needs origin/main: a full checkout has it.
grep -q 'fetch-depth: 0' <<<"$(block proto-breaking)" || bad "proto-breaking does not fetch main (fetch-depth: 0)"

# No retries anywhere in CI but Playwright's one, which is in its config.
grep -niE 'retry|retries|continue-on-error|rerun|attempt' - .github/actions/setup/action.yml <<<"$code" && bad "the workflow retries or ignores a failure"
grep -qE '^  retries: 1,' e2e/playwright.config.ts || bad "Playwright's config does not set retries: 1"
[ "$(grep -c 'retries' e2e/playwright.config.ts | tr -d ' ')" -ge 1 ] || bad "no retries line in Playwright's config"
# Nowhere else: the other test runners' retry settings.
if grep -rnE 'retryTimes|--retries|retry:' apps/app/package.json apps/app/jest.config.* apps/admin/vite.config.ts apps/admin/package.json Makefile 2>/dev/null; then
  bad "a test runner is set to retry"
fi

# Least privilege: read-only token, set once at the top.
awk '/^permissions:/ {on=1; print; next} /^[a-z]/ {on=0} on' <<<"$code" | tr -d '\n' | grep -qx 'permissions:  contents: read' || bad "the workflow's permissions are not exactly contents: read"
[ "$(grep -c '^ *permissions:' <<<"$code")" -eq 1 ] || bad "a job asks for permissions of its own"
grep -nE 'secrets\.[A-Za-z]' <<<"$code" | grep -v 'GITHUB_TOKEN' && bad "the workflow uses a secret"

# Every action is pinned to a commit.
if grep -rhE '^\s*(- )?uses:' .github/workflows/ci.yml .github/actions 2>/dev/null | grep -vE 'uses: \./' | grep -vE 'uses: [^@ ]+@[0-9a-f]{40}( |$)'; then
  bad "an action is not pinned to a commit SHA"
fi

# Every job times out.
for job in $required changes; do
  grep -q 'timeout-minutes:' <<<"$(block "$job")" || bad "$job has no timeout-minutes"
done

# Every `make` target a job calls exists.
for target in $(grep -hoE 'run: make [a-z0-9 -]+' <<<"$code" | sed 's/run: make //'); do
  make -n "$target" >/dev/null 2>&1 || bad "make target '$target' does not exist"
done

# Each job calls the target of its own name, so the check and the make target
# stay one idea (typescript's, native-bundle's and the rest are all thin calls).
for job in commits boundaries retention rpc-auth strings image e2e backend codegen typescript native-bundle proto-breaking; do
  grep -qE "run: make ([a-z-]+ )*$job( |$)" <<<"$(block "$job")" || bad "$job does not run make $job"
done
grep -q 'run: make policy-lint' <<<"$(block policy)" || bad "policy does not run make policy-lint"
grep -q 'run: make secrets-scan' <<<"$(block secrets)" || bad "secrets does not run make secrets-scan"

# The actions a job may use: checkout, the toolchains, and the e2e job's traces.
# Anything else would be a check that isn't a make target (a third-party
# commitlint action, say), so it couldn't run on a laptop.
allowed='actions/checkout|actions/setup-go|actions/upload-artifact|\./\.github/actions/setup'
if grep -nE '^\s*(- )?uses:' <<<"$code" | grep -vE "uses: ($allowed)(@| |$)"; then
  bad "a job uses an action other than checkout, the toolchains or upload-artifact; call a make target instead"
fi

# What CI itself runs on counts as touching every job's paths, or breaking it
# would skip the jobs it breaks and each would report as passing.
for path in "$workflow" Makefile $(grep -hoE 'uses: \./[^ ]+' <<<"$code" | sed 's|uses: \./||; s|$|/action.yml|' | sort -u); do
  grep -q '=false$' <<<"$(printf '%s\n' "$path" | scripts/ci-changes.sh)" && bad "a change to $path skips some jobs; add it to shared in scripts/ci-changes.sh"
done

# Nothing make lint test runs is left to the laptop: every command in it is run
# by a target some job calls, so a guard (this test, the path filter's test,
# proto-breaking.sh's test) can't exist only locally.
ci_targets=$(grep -hoE 'run: make [a-z0-9 -]+' <<<"$code" | sed 's/run: make //' | tr '\n' ' ')
# shellcheck disable=SC2086 # the targets are words
missing=$(comm -23 <(make -n lint test | sort -u) <(make -n $ci_targets | sort -u))
[ -z "$missing" ] || bad "make lint test runs these and no CI job does:"$'\n'"$missing"

[ "$fail" -eq 0 ] && echo "ci-workflow: ok"
exit "$fail"
