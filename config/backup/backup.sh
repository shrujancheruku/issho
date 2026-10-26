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
set -eu

: "${DATABASE_URL:?}" "${R2_ENDPOINT:?}" "${R2_BUCKET:?}" "${R2_ACCESS_KEY_ID:?}" "${R2_SECRET_ACCESS_KEY:?}"
command -v curl >/dev/null || apk add --no-cache curl >/dev/null

backup() {
  name="issho-$(date -u +%Y-%m-%dT%H%M%SZ).dump"
  file="/tmp/$name"
  # The app's schemas, plus auth: app.users points at auth.users, which
  # holds everyone's sign-in identity. Supabase's other schemas (storage,
  # realtime…) hold nothing of ours. Live sessions and one-time tokens
  # aren't kept: a leaked dump mustn't open anyone's account, and after a
  # restore people sign in again.
  pg_dump --format=custom --no-owner --no-privileges \
    --schema=app --schema=goose --schema=river --schema=auth \
    --exclude-table-data=auth.sessions --exclude-table-data=auth.refresh_tokens \
    --exclude-table-data=auth.one_time_tokens --exclude-table-data=auth.flow_state \
    --exclude-table-data=auth.mfa_challenges \
    --file="$file" "$DATABASE_URL"
  curl --fail --silent --show-error --aws-sigv4 "aws:amz:auto:s3" \
    --user "$R2_ACCESS_KEY_ID:$R2_SECRET_ACCESS_KEY" \
    --upload-file "$file" "$R2_ENDPOINT/$R2_BUCKET/$name"
  echo "backup: uploaded $name ($(wc -c <"$file") bytes)"
  rm -f "$file"
}

if [ "${BACKUP_ONCE:-}" = 1 ]; then
  backup
  exit 0
fi

at="${BACKUP_AT:-3}"
while true; do
  # Sleep until the next BACKUP_AT:00 UTC.
  now=$(date -u +%s)
  next=$(( now - now % 86400 + at * 3600 ))
  [ "$next" -gt "$now" ] || next=$(( next + 86400 ))
  sleep $(( next - now ))
  # A failed night is logged and retried the next night, not fatal.
  backup || echo "backup: failed" >&2
done
