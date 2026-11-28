#!/usr/bin/env bash
# The end-to-end tests: Playwright against the real API binary, the web build and
# a throwaway local stack, all started here and removed at the end.
# Usage: scripts/e2e.sh   (from anywhere; needs Docker)
#
# The stack is four throwaway containers on a Docker network of their own, named
# issho-e2e-<pid>-*: Postgres (migrated with goose, the fixture loaded), Supabase
# Auth (GoTrue) with the repo's own sign-in email template, a mail catcher
# (Mailpit) where the sign-in code lands, and a gateway (nginx) that puts Auth
# under /auth/v1 with CORS, as Supabase's own gateway does. Every container
# publishes its port on 127.0.0.1 only, and Docker picks the host port. Nothing
# here starts, stops, resets or connects to `supabase start` (port 54322) or any
# hosted service, and nothing reads .env*: the database password, the JWT secret
# and the signing key are made fresh for each run and never leave this process
# and the containers.
#
# It also chooses three free ports for the API, the admin and the Expo server
# rather than the dev defaults (8080, 8081, 5173), so it can run beside a
# developer's servers; Playwright starts those and stops them when the run ends
# (e2e/playwright.config.ts).
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
chmod 700 "$work"

# The images, by digest (Renovate does not see a shell script: bump these by hand
# when the tags move). Postgres is what the Go tests use; Auth is what Supabase
# runs; Mailpit is a mail catcher with a JSON API.
postgres_image='postgres:17-alpine@sha256:f9cf3e1f0066f0eb0c48707d864f0ac5941e9ff454a072077d97a4d278d07d73'
auth_image='supabase/gotrue:v2.198.0@sha256:ba8c0c82c574756c835c696df2a2c698020cc408a42034013d3712e34de7964d'
mail_image='axllent/mailpit:v1.27@sha256:e22dce5b36f93c77082e204a3942fb6b283b7896e057458400a4c88344c3df68'
gateway_image='nginx:1.29-alpine@sha256:5616878291a2eed594aee8db4dade5878cf7edcb475e59193904b198d9b830de'

# Everything this run starts has this prefix and the label below, so cleanup
# removes exactly it, and a run that was killed leaves containers that the next
# one recognises as dead (the label holds the pid) and removes.
run="issho-e2e-$$"
label="issho.e2e.pid"
cleanup() {
  local ids
  ids=$(docker ps -aq --filter "label=$label=$$" 2>/dev/null || true)
  [ -z "$ids" ] || docker rm -f -v $ids >/dev/null 2>&1 || true
  docker network rm "$run" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

# Leftovers of runs whose process is gone (a kill -9, a closed terminal).
for pid in $(docker ps -a --filter "label=$label" --format '{{.Label "'"$label"'"}}' 2>/dev/null | sort -u); do
  if ! kill -0 "$pid" 2>/dev/null; then
    docker rm -f -v $(docker ps -aq --filter "label=$label=$pid") >/dev/null 2>&1 || true
    docker network rm "issho-e2e-$pid" >/dev/null 2>&1 || true
  fi
done

# A free port: bind 0 and ask which one we got. Another process could take it
# before the server starts; Playwright then fails to start, which is loud enough.
free_port() { node -e 'const s=require("net").createServer().listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})'; }
# host_port <container> <container port>: where Docker published it on 127.0.0.1.
host_port() { docker port "$run-$1" "$2/tcp" | head -n1 | sed 's/.*://'; }
# wait_for <what> <command...>: runs the command until it succeeds, for at most 60 s.
wait_for() {
  local what=$1; shift
  for _ in $(seq 1 120); do "$@" >/dev/null 2>&1 && return 0; sleep 0.5; done
  echo "e2e: $what did not come up" >&2
  docker ps -a --filter "label=$label=$$" --format '{{.Names}}: {{.Status}}' >&2
  return 1
}
start() { # start <name> <docker run args...>: a detached container of this run
  local name=$1; shift
  docker run --detach --name "$run-$name" --network "$run" --label "$label=$$" "$@" >/dev/null
}

secret() { node -e 'console.log(require("crypto").randomBytes(24).toString("hex"))'; }
db_password=$(secret)
jwt_secret=$(secret)

docker network create --label "$label=$$" "$run" >/dev/null

# --- Postgres: the app's database and Auth's, as Supabase's one database holds both.
start postgres --publish 127.0.0.1::5432 --env POSTGRES_PASSWORD="$db_password" "$postgres_image"
pg() { docker exec "$run-postgres" psql -U postgres -v ON_ERROR_STOP=1 "$@"; }
wait_for "Postgres" docker exec "$run-postgres" pg_isready -U postgres -h 127.0.0.1
# The official image restarts once after its init scripts; wait until a query works.
wait_for "Postgres (queries)" pg -c 'select 1'
# The role and schema Auth's own migrations expect (Supabase's image has them).
pg -q -c "create role supabase_auth_admin noinherit createrole login password '$db_password'; create schema auth authorization supabase_auth_admin; alter role supabase_auth_admin set search_path = auth; grant all on database postgres to supabase_auth_admin;"
export E2E_DATABASE_URL="postgres://postgres:$db_password@127.0.0.1:$(host_port postgres 5432)/postgres?sslmode=disable"

# --- The mail catcher: Auth sends through its SMTP; the test reads its JSON API.
start mail --publish 127.0.0.1::8025 "$mail_image"
export E2E_MAIL_URL="http://127.0.0.1:$(host_port mail 8025)"

# --- Auth. The signing key is made now: an ES256 key as a JWK, which is what the
# API's verifier reads from the key set (never the JWT secret, which Auth needs
# but the API never sees).
node -e '
const { generateKeyPairSync, randomUUID } = require("crypto");
const jwk = generateKeyPairSync("ec", { namedCurve: "P-256" }).privateKey.export({ format: "jwk" });
console.log(JSON.stringify([{ ...jwk, kid: randomUUID(), alg: "ES256", key_ops: ["sign", "verify"], ext: true }]));
' > "$work/jwt-keys.json"
# The token's `role` claim is the default group (the API requires "authenticated").
# The gateway's port is not known until it runs, and Auth's issuer and the app's
# Supabase URL both name it, so Docker's choice is made here instead.
gateway_port=$(free_port)
auth_url="http://127.0.0.1:$gateway_port/auth/v1"
# `confirmation` is the mail a new address gets and `magic_link` the one a known
# address gets: both show the code (supabase/config.toml does the same).
# GOTRUE_MAILER_TEMPLATES_* are URLs Auth fetches when it sends: the gateway
# serves the repo's template, so what the test reads is what a person would.
# Tokens last a year of real time: the API checks their expiry against the fake
# clock, which a journey moves days ahead.
cat > "$work/auth.env" <<ENV
GOTRUE_API_HOST=0.0.0.0
GOTRUE_API_PORT=9999
API_EXTERNAL_URL=$auth_url
GOTRUE_JWT_ISSUER=$auth_url
GOTRUE_SITE_URL=http://$run-gateway
GOTRUE_DB_DRIVER=postgres
GOTRUE_DB_DATABASE_URL=postgres://supabase_auth_admin:$db_password@$run-postgres:5432/postgres?search_path=auth
GOTRUE_JWT_SECRET=$jwt_secret
GOTRUE_JWT_KEYS=$(cat "$work/jwt-keys.json")
GOTRUE_JWT_AUD=authenticated
GOTRUE_JWT_DEFAULT_GROUP_NAME=authenticated
GOTRUE_JWT_EXP=31536000
GOTRUE_DISABLE_SIGNUP=false
GOTRUE_EXTERNAL_EMAIL_ENABLED=true
GOTRUE_MAILER_AUTOCONFIRM=false
GOTRUE_MAILER_OTP_LENGTH=6
GOTRUE_MAILER_OTP_EXP=600
GOTRUE_SMTP_HOST=$run-mail
GOTRUE_SMTP_PORT=1025
GOTRUE_SMTP_USER=e2e
GOTRUE_SMTP_PASS=e2e
GOTRUE_SMTP_ADMIN_EMAIL=noreply@e2e.invalid
GOTRUE_SMTP_MAX_FREQUENCY=1s
GOTRUE_RATE_LIMIT_EMAIL_SENT=1000
GOTRUE_MAILER_SUBJECTS_CONFIRMATION=Your Issho code
GOTRUE_MAILER_SUBJECTS_MAGIC_LINK=Your Issho code
GOTRUE_MAILER_TEMPLATES_CONFIRMATION=http://$run-gateway/templates/code.html
GOTRUE_MAILER_TEMPLATES_MAGIC_LINK=http://$run-gateway/templates/code.html
ENV
chmod 600 "$work/auth.env"
start auth --env-file "$work/auth.env" "$auth_image"

# --- The gateway: Auth under /auth/v1 as Supabase serves it, the CORS preflight
# the app's origin needs, and the email template.
cp "$repo/supabase/templates/code.html" "$work/code.html"
cat > "$work/nginx.conf" <<CONF
events {}
http {
  server {
    listen 80;
    location /auth/v1/ {
      # Auth answers a real request with its own CORS headers, but refuses the
      # preflight of the headers supabase-js sends (apikey, x-client-info), so the
      # gateway answers the preflight, as Supabase's does.
      if (\$request_method = OPTIONS) {
        add_header Access-Control-Allow-Origin * always;
        add_header Access-Control-Allow-Headers "authorization, apikey, content-type, x-client-info, x-supabase-api-version" always;
        add_header Access-Control-Allow-Methods "GET, POST, PUT, PATCH, DELETE, OPTIONS" always;
        return 204;
      }
      proxy_pass http://$run-auth:9999/;
    }
    location = /templates/code.html { alias /templates/code.html; default_type text/html; }
  }
}
CONF
start gateway --publish "127.0.0.1:$gateway_port:80" \
  --volume "$work/nginx.conf:/etc/nginx/nginx.conf:ro" --volume "$work/code.html:/templates/code.html:ro" "$gateway_image"
wait_for "Auth" curl -fsS "$auth_url/health"
export E2E_SUPABASE_URL="http://127.0.0.1:$gateway_port"
export E2E_AUTH_URL="$auth_url"

# --- The app's own binaries: the API built for end-to-end (a fake clock, the
# worker in-process, the test-only RPCs: backend/cmd/api/e2e_on.go), with a
# version of its own so the page can only show it by asking the API; goose; and
# the seed, which loads the fixture and admits the owner.
export E2E_API_VERSION="e2e-$$"
export E2E_API_BINARY="$work/api"
export E2E_MIGRATE_BINARY="$work/migrate"
export E2E_SEED_BINARY="$work/seed"
(cd "$repo/backend" \
  && go build -tags e2e -ldflags "-X main.version=$E2E_API_VERSION" -o "$E2E_API_BINARY" ./cmd/api \
  && go build -o "$E2E_MIGRATE_BINARY" ./cmd/migrate \
  && go build -o "$E2E_SEED_BINARY" ./cmd/seed)
# goose against the throwaway database only: the URL is the container's.
DATABASE_URL="$E2E_DATABASE_URL" "$E2E_MIGRATE_BINARY" up
DATABASE_URL="$E2E_DATABASE_URL" "$E2E_SEED_BINARY"

export E2E_API_PORT=$(free_port)
export E2E_WEB_PORT=$(free_port)
export E2E_APP_PORT=$(free_port)
# The Expo server's build (the member app's web export) and where Playwright finds it.
export E2E_APP_DIST="$work/app-dist"

pnpm --filter @issho/admin build
# The member app, built as the web image builds it: for its own origin (the Expo
# server forwards the API's paths, as in deployment), and 127.0.0.1 counts as a
# dev API, so the page shows the API's version. Its Supabase URL is this run's
# gateway, and its publishable key a stand-in: Auth does not check the key
# (Supabase's gateway does), so no real project's key appears anywhere.
(cd "$repo/apps/app" && CI=1 EXPO_PUBLIC_API_URL="http://127.0.0.1:$E2E_APP_PORT" \
  EXPO_PUBLIC_SUPABASE_URL="$E2E_SUPABASE_URL" EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY="e2e-stand-in-key" \
  pnpm exec expo export --platform web --output-dir "$E2E_APP_DIST")
pnpm --filter @issho/e2e exec playwright install chromium

pnpm --filter @issho/e2e test
