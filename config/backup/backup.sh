#!/bin/sh
# Nightly off-site copy of the database (Kamal accessory "backup", see
# config/deploy.yml and docs/deploy.md): pg_dump in the custom format,
# uploaded to a Cloudflare R2 bucket with curl's own S3 signing, so the
# Postgres image needs nothing but curl. Supabase keeps its own backups
# (Pro); this copy lives with another vendor.
#
#   DATABASE_URL         a direct or session-pooler connection (pg_dump needs one session)
#   R2_ENDPOINT          https://<account id>.r2.cloudflarestorage.com
#   R2_BUCKET            the bucket; an R2 lifecycle rule deletes old dumps
#   R2_ACCESS_KEY_ID     an R2 API token's key pair, limited to that bucket,
#   R2_SECRET_ACCESS_KEY   with object read and write
#   BACKUP_AT            UTC hour to run, 0-23 (default 3)
#   BACKUP_ONCE=1        run once and exit (to check the setup)
#   SENTRY_DSN           optional. Each run checks in to the Sentry cron monitor
#                        "issho-backup" (in_progress, then ok or error); a night
#                        with no check-in alerts as missed. Unset = no check-ins.
set -eu

: "${DATABASE_URL:?}" "${R2_ENDPOINT:?}" "${R2_BUCKET:?}" "${R2_ACCESS_KEY_ID:?}" "${R2_SECRET_ACCESS_KEY:?}"
# curl is baked into the image (config/backup/Dockerfile).
command -v curl >/dev/null || { echo "backup: curl missing from the image" >&2; exit 1; }

at="${BACKUP_AT:-3}"

# Sentry cron monitor check-ins, via the DSN-keyed HTTP endpoint
# (https://<host>/api/<project id>/cron/<slug>/<public key>/). The first
# check-in carries the monitor's config, so the monitor is created and kept
# up to date from here. Every failure is swallowed: Sentry being down or the
# DSN being wrong must never fail (or stall) a backup, hence the short timeouts.
monitor=issho-backup
env_name="${SENTRY_ENVIRONMENT:-production}"
sentry_base=
if [ -n "${SENTRY_DSN:-}" ]; then
  # https://<key>@<host>/<project id>
  scheme=${SENTRY_DSN%%://*}
  rest=${SENTRY_DSN#*://}
  key=${rest%%@*}
  hostpath=${rest#*@}
  host=${hostpath%%/*}
  project=${hostpath#*/}
  if [ "$rest" != "$SENTRY_DSN" ] && [ "$hostpath" != "$rest" ] && [ "$project" != "$hostpath" ] && [ -n "$key" ] && [ -n "$host" ] && [ -n "$project" ]; then
    sentry_base="$scheme://$host/api/$project/cron/$monitor/$key/"
  else
    echo "backup: SENTRY_DSN is malformed, skipping check-ins" >&2
  fi
fi
checkin_id=

# checkin <in_progress|ok|error>; always returns 0.
checkin() {
  [ -n "$sentry_base" ] || return 0
  if [ "$1" = in_progress ]; then
    # Daily at BACKUP_AT UTC. A check-in later than the margin counts as
    # missed; running longer than max_runtime counts as failed (minutes).
    body='{"status":"in_progress","environment":"'"$env_name"'","monitor_config":{"schedule":{"type":"crontab","value":"0 '"$at"' * * *"},"timezone":"UTC","checkin_margin":60,"max_runtime":60,"failure_issue_threshold":1,"recovery_threshold":1}}'
    resp=$(curl --silent --max-time 10 --connect-timeout 5 -X POST \
      -H "Content-Type: application/json" -d "$body" "$sentry_base" 2>/dev/null) || resp=
    # The reply is {"id":"<check-in id>",...}; later calls update that check-in.
    checkin_id=$(printf '%s' "$resp" | sed -n 's/.*"id" *: *"\([^"]*\)".*/\1/p' | head -n 1)
    [ -n "$checkin_id" ] || echo "backup: sentry check-in (in_progress) failed" >&2
  else
    curl --silent --fail --max-time 10 --connect-timeout 5 -X POST \
      -H "Content-Type: application/json" \
      -d '{"status":"'"$1"'","environment":"'"$env_name"'"}' \
      "$sentry_base?check_in_id=$checkin_id" >/dev/null 2>&1 || echo "backup: sentry check-in ($1) failed" >&2
  fi
  return 0
}

backup() {
  name="issho-$(date -u +%Y-%m-%dT%H%M%SZ).dump"
  file="/tmp/$name"
  # The app's schemas, plus auth: app.users points at auth.users, which
  # holds everyone's sign-in identity. Supabase's other schemas (storage,
  # realtime…) hold nothing of ours. Live sessions and one-time tokens
  # aren't kept: a leaked dump mustn't open anyone's account, and after a
  # restore people sign in again. mfa_amr_claims goes too: its rows belong to
  # sessions, and without them the restore fails their foreign key.
  pg_dump --format=custom --no-owner --no-privileges \
    --schema=app --schema=goose --schema=river --schema=auth \
    --exclude-table-data=auth.sessions --exclude-table-data=auth.refresh_tokens \
    --exclude-table-data=auth.one_time_tokens --exclude-table-data=auth.flow_state \
    --exclude-table-data=auth.mfa_challenges --exclude-table-data=auth.mfa_amr_claims \
    --file="$file" "$DATABASE_URL" || { rm -f "$file"; return 1; }
  # Each step returns on failure itself: set -e is off inside a function
  # called from an if, and run() needs the status for the check-in.
  curl --fail --silent --show-error --aws-sigv4 "aws:amz:auto:s3" \
    --user "$R2_ACCESS_KEY_ID:$R2_SECRET_ACCESS_KEY" \
    --upload-file "$file" "$R2_ENDPOINT/$R2_BUCKET/$name" || { rm -f "$file"; return 1; }
  echo "backup: uploaded $name ($(wc -c <"$file") bytes)"
  rm -f "$file"
}

# One run, bracketed by check-ins. Returns backup's status.
run() {
  checkin in_progress
  if backup; then
    checkin ok
  else
    checkin error
    return 1
  fi
}

if [ "${BACKUP_ONCE:-}" = 1 ]; then
  run
  exit $?
fi

while true; do
  # Sleep until the next BACKUP_AT:00 UTC.
  now=$(date -u +%s)
  next=$(( now - now % 86400 + at * 3600 ))
  [ "$next" -gt "$now" ] || next=$(( next + 86400 ))
  sleep $(( next - now ))
  # A failed night is logged and retried the next night, not fatal.
  run || echo "backup: failed" >&2
done
