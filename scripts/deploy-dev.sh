#!/usr/bin/env bash
# Deploys the commit checked out here to dev: builds and pushes the two images,
# migrates the issho-dev database, then deploys the containers and the admin.
# Run by .github/workflows/deploy-dev.yml (the only place that deploys; nothing
# here is run from a laptop). docs/deploy.md has the picture.
#
# Needs, in the environment:
#   DEPLOY_HOST              the dev VM's address
#   KAMAL_REGISTRY_PASSWORD  a token that can push to ghcr.io/shrujancheruku
# and on PATH: kamal (config/kamal-version), gcloud signed in as the deploy
# service account, go, pnpm (installed, so wrangler is there). No secret is passed
# as an argument; the ones read here go into the environment of the one command
# that needs them.
#
# The order is the point. Images first, so a build that fails changes nothing.
# Then the migration, before any container is replaced: a migration that fails
# stops the script (set -e) with the old containers still serving, so they never
# run against a schema the new code expects, or a half-applied one. Then the
# containers, the API first, since the admin's Worker forwards to it.
set -euo pipefail

: "${DEPLOY_HOST:?DEPLOY_HOST is not set}"
: "${KAMAL_REGISTRY_PASSWORD:?KAMAL_REGISTRY_PASSWORD is not set}"

repo=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo"

project=issho-dev-512213
# Each Kamal call names its service's config and the dev destination.
backend=(-d dev)
web=(-c config/deploy.web.yml -d dev)

# secret <name> <variable>: sets the variable to a secret's value from Secret
# Manager. On GitHub Actions the value is masked in the log first, in case
# something prints it (the mask has to be printed here, not in a command
# substitution, or the runner would never see it). $(...) drops the trailing
# newline a stored value may carry, and so would a CR, which Cloudflare rejects
# in a token.
secret() {
  local value
  value=$(gcloud secrets versions access latest --secret="$1" --project="$project" | tr -d '\r')
  [ -z "${GITHUB_ACTIONS:-}" ] || echo "::add-mask::$value"
  printf -v "$2" '%s' "$value"
}

step() { echo "== $*"; }

step "Check that every secret the deploy names exists"
# The names are the ones .kamal/secrets.dev fetches. Kamal would fail on a missing
# one at its first call anyway; checking here fails first and says which, before
# the VM, a build or a container is touched. It reads each secret the way Kamal
# does (the accessor permission the deploy account has) and discards the value
# unseen: `secrets describe` would need secretmanager.secrets.get, which that
# account is not given (docs/deploy.md), and every deploy would stop here with a
# false "no secret named".
missing=()
for name in $(sed -n 's/^SECRETS=.*issho-dev-512213 //p' .kamal/secrets.dev | tr -d ')'); do
  gcloud secrets versions access latest --secret="$name" --project="$project" >/dev/null 2>&1 || missing+=("$name")
done
[ "${#missing[@]}" -eq 0 ] || { echo "Secret Manager has no secret named: ${missing[*]}" >&2; exit 1; }

# The web's two sign-in values must not be empty either: an empty one would build an
# app that cannot sign in. Their digest goes to the build as SUPABASE_VALUES_HASH
# (apps/web/Dockerfile): BuildKit's cache ignores secret mounts, so a build on a
# daemon that has built before would otherwise reuse the old bundle after a value
# changed. The digest is not a value; the values themselves are masked on Actions.
secret dev-supabase-url supabase_url
secret dev-supabase-publishable-key supabase_key
for pair in "dev-supabase-url:$supabase_url" "dev-supabase-publishable-key:$supabase_key"; do
  [ -n "${pair#*:}" ] || { echo "Secret Manager's ${pair%%:*} is empty" >&2; exit 1; }
done
SUPABASE_VALUES_HASH=$(printf '%s\n%s' "$supabase_url" "$supabase_key" | shasum -a 256 | cut -d' ' -f1)
export SUPABASE_VALUES_HASH

step "Docker on the VM (installs it on the first run)"
kamal server bootstrap "${backend[@]}"

step "Build and push the images"
kamal build push "${backend[@]}"
kamal build push "${web[@]}"

step "Migrate issho-dev"
# The migration role's own connection (a session pooler), never the API's.
secret dev-migrate-database-url database_url
(cd backend && DATABASE_URL="$database_url" go run ./cmd/migrate up)

step "Deploy the API and the worker"
# --skip-push: the hosts pull the images pushed above, so what runs is what was built.
kamal deploy --skip-push "${backend[@]}"

step "Deploy the Expo server"
kamal deploy --skip-push "${web[@]}"

step "Deploy the admin"
pnpm --filter @issho/admin build
# The stored token ends in a newline, which Cloudflare rejects; secret() drops it.
secret dev-cloudflare-api-token cloudflare_token
CLOUDFLARE_API_TOKEN="$cloudflare_token" WRANGLER_SEND_METRICS=false \
  pnpm --filter @issho/admin exec wrangler deploy --env dev
