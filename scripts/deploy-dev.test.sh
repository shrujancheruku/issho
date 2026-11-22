#!/usr/bin/env bash
# Tests scripts/deploy-dev.sh's order, with stand-ins for kamal, gcloud, go and
# pnpm that record what they are asked and can be told to fail. The point is the
# rule "a failed migration stops the deploy before any container is replaced":
# no stand-in sees a deploy call after the migration failed, and no image is
# deployed before the images are built and pushed.
# Usage: scripts/deploy-dev.test.sh
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
script="$repo/scripts/deploy-dev.sh"
fail=0
bad() { echo "FAIL $*"; fail=1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir "$work/bin"

# Every stand-in appends one line per call to $LOG. FAIL_ON names a call (the
# stand-in's name and its first two arguments, e.g. "go run") that exits 1.
for tool in kamal gcloud go pnpm; do
  cat > "$work/bin/$tool" <<'STUB'
#!/usr/bin/env bash
name=$(basename "$0")
echo "$name $*" >> "$LOG"
# What the environment carried to the command that needs a secret.
echo "$name DATABASE_URL=${DATABASE_URL:-} CLOUDFLARE_API_TOKEN=${CLOUDFLARE_API_TOKEN:-}" >> "$LOG.env"
if [ -n "${FAIL_ON:-}" ] && [[ "$name $*" == "$FAIL_ON"* ]]; then echo "$name: told to fail" >&2; exit 1; fi
# gcloud's secret read is the only call whose output is used: the value, with the
# trailing newline the stored Cloudflare token has.
if [ "$name" = gcloud ]; then printf 'value-of-secret\n'; fi
STUB
  chmod +x "$work/bin/$tool"
done

# run <FAIL_ON> [<GITHUB_ACTIONS>]: the script with the stand-ins first on PATH;
# prints nothing, leaves the calls in $LOG and returns the script's status.
# GITHUB_ACTIONS is set only when asked for, since the script prints masks on
# Actions and this test runs both on a laptop and on the runner.
run() {
  : > "$work/log"; : > "$work/log.env"
  PATH="$work/bin:$PATH" LOG="$work/log" FAIL_ON="${1:-}" GITHUB_ACTIONS="${2:-}" DEPLOY_HOST=203.0.113.7 KAMAL_REGISTRY_PASSWORD=stub "$script" >"$work/out" 2>&1
}
# line_of <prefix>: the line number of the first call starting with the prefix, or 0.
line_of() { grep -n "^$1" "$work/log" | head -n1 | cut -d: -f1 | grep . || echo 0; }

# --- the happy path -------------------------------------------------------------
run "" || { cat "$work/out"; bad "the script failed with every tool succeeding"; }
build_web=$(line_of "kamal build push -c config/deploy.web.yml")
build_api=$(line_of "kamal build push -d dev")
migrate=$(line_of "go run ./cmd/migrate up")
deploy_api=$(line_of "kamal deploy --skip-push -d dev")
deploy_web=$(line_of "kamal deploy --skip-push -c config/deploy.web.yml")
admin=$(line_of "pnpm --filter @issho/admin exec wrangler deploy")
for n in "$build_web" "$build_api" "$migrate" "$deploy_api" "$deploy_web" "$admin"; do
  [ "$n" -gt 0 ] || { bad "a step never ran: $(cat "$work/log")"; break; }
done
[ "$build_api" -lt "$migrate" ] && [ "$build_web" -lt "$migrate" ] || bad "the images are built after the migration"
[ "$migrate" -lt "$deploy_api" ] && [ "$migrate" -lt "$deploy_web" ] || bad "a container is replaced before the migration"
# The admin's Worker forwards to the API, so it follows the API's deploy.
[ "$deploy_api" -lt "$admin" ] || bad "the admin is deployed before the API"
# The VM has Docker before anything is pulled or booted on it.
[ "$(line_of "kamal server bootstrap")" -lt "$deploy_api" ] || bad "the VM is not bootstrapped before the deploy"
# Deploys use the images just pushed, not a rebuild that could differ.
grep -q '^kamal deploy -d' "$work/log" && bad "a deploy builds its own image instead of using the pushed one"

# --- a failed migration ---------------------------------------------------------
if run "go run"; then bad "the script succeeded although the migration failed"; fi
grep -q '^kamal deploy' "$work/log" && bad "a container was deployed after the migration failed"
grep -q 'wrangler' "$work/log" && bad "the admin was deployed after the migration failed"
grep -q '^go run ./cmd/migrate up' "$work/log" || bad "the migration was not attempted"

# --- a failed build -------------------------------------------------------------
if run "kamal build push -c config/deploy.web.yml"; then bad "the script succeeded although the web image failed to build"; fi
grep -q '^go run' "$work/log" && bad "the database was migrated although an image failed to build"
grep -q '^kamal deploy' "$work/log" && bad "a container was deployed although an image failed to build"

# --- a failed deploy of the API stops the web and the admin ----------------------
if run "kamal deploy --skip-push -d dev"; then bad "the script succeeded although the API's deploy failed"; fi
grep -q 'config/deploy.web.yml' <(grep '^kamal deploy' "$work/log") && bad "the web was deployed after the API's deploy failed"
grep -q 'wrangler' "$work/log" && bad "the admin was deployed after the API's deploy failed"

# --- the database's address reaches the migration and nothing else prints it -----
run "" || true
if grep -q 'value-of-secret' "$work/out"; then bad "a secret's value is in the script's output"; fi
if grep -q 'value-of-secret' "$work/log"; then bad "a secret's value was passed as an argument, where ps and logs show it"; fi

# On GitHub Actions, each value read is masked: the runner takes an exact
# "::add-mask::<value>" line as a command and keeps it out of the log. Any other
# line holding the value would be logged as it is.
run "" true || bad "the script failed on Actions with every tool succeeding"
if grep 'value-of-secret' "$work/out" | grep -vqx '::add-mask::value-of-secret'; then bad "a secret's value is in the script's output on Actions, outside a mask"; fi
[ "$(grep -cx '::add-mask::value-of-secret' "$work/out")" -eq 2 ] || bad "on Actions, the two secrets read are not each masked: $(grep -c 'add-mask' "$work/out") masks"
if grep -q 'value-of-secret' "$work/log"; then bad "a secret's value was passed as an argument on Actions"; fi

grep -q '^go DATABASE_URL=value-of-secret ' "$work/log.env" || bad "the migration did not get the migration role's database URL in its environment"
grep -q 'CLOUDFLARE_API_TOKEN=value-of-secret$' "$work/log.env" || bad "wrangler did not get the token in its environment, without its newline"

# --- no address, no deploy --------------------------------------------------------
: > "$work/log"
if PATH="$work/bin:$PATH" LOG="$work/log" env -u DEPLOY_HOST "$script" >/dev/null 2>&1; then bad "the script ran without DEPLOY_HOST"; fi
[ ! -s "$work/log" ] || bad "the script called a tool before it knew the host"

[ "$fail" -eq 0 ] && echo "deploy-dev: ok"
exit "$fail"
