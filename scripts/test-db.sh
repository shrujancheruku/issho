#!/usr/bin/env bash
# `make test-db`: scripts/db-checks.sh against a throwaway Supabase Postgres container of
# its own (one per run, named after the worktree, on a free port), removed on
# exit, failure or Ctrl-C. Never reads .env* (those point at the hosted
# issho-dev); it sets its own URL for the container only.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"

{ command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; } ||
  { echo "make test-db needs Docker running (see docs/onboarding.md)." >&2; exit 1; }

# Same image as CI's backend job, read from ci.yml so the two can't drift.
image="$(grep -o 'public.ecr.aws/supabase/postgres:[A-Za-z0-9.-]*' "$root/.github/workflows/ci.yml" | head -1)"
[ -n "$image" ] || { echo "no Supabase postgres image found in .github/workflows/ci.yml" >&2; exit 1; }

# Worktree name plus the pid, so two runs in one directory can't collide either.
name="issho-testdb-$(basename "$root" | tr -c 'A-Za-z0-9_.\n-' '-')-$$"
trap 'docker rm -f "$name" >/dev/null 2>&1 || true' EXIT
trap 'exit 130' INT TERM

# The image is multi-arch, so it runs natively on Apple silicon.
# -p 127.0.0.1::5432 lets Docker pick a free host port, reachable only locally.
docker run -d --name "$name" -p 127.0.0.1::5432 -e POSTGRES_PASSWORD=postgres \
  --health-cmd "pg_isready -h 127.0.0.1 -U postgres" --health-interval 2s --health-timeout 5s --health-retries 30 \
  "$image" >/dev/null

# Wait on the TCP health check: the image's init scripts (creating PostGIS) run on a
# temporary server that listens only on the Unix socket, so a socket check reports
# healthy before that server stops.
status=""
for _ in $(seq 120); do
  status="$(docker inspect -f '{{.State.Health.Status}}' "$name" 2>/dev/null || true)"
  [ "$status" = healthy ] && break
  # Fail fast if the container died (bad image, no memory) instead of waiting out the loop.
  [ "$(docker inspect -f '{{.State.Running}}' "$name" 2>/dev/null || echo false)" = true ] ||
    { echo "PostGIS container exited" >&2; docker logs "$name" >&2 || true; exit 1; }
  sleep 1
done
[ "$status" = healthy ] || { echo "PostGIS not healthy after 120s" >&2; docker logs "$name" >&2 || true; exit 1; }

port="$(docker port "$name" 5432/tcp | head -1 | sed 's/.*://')"
export DATABASE_URL="postgres://postgres:postgres@127.0.0.1:$port/postgres?sslmode=disable"
"$root/scripts/db-checks.sh"
