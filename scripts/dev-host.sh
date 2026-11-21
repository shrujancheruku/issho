#!/usr/bin/env bash
# Prints the dev VM's address, for the deploy to give Kamal as DEPLOY_HOST.
#
# The address is not written in the repository (it is public, and the VM would
# then be named to the world), and the deploy's service account may read secrets
# only, not describe the VM. So, in order:
#   1. DEV_HOST in the environment: a repository variable the founder sets;
#   2. Cloudflare, which holds it: the API's name is proxied and its A record's
#      content is the VM's static address. The stored Cloudflare token reads it,
#      if it may read DNS. The token never appears in an argument: curl reads its
#      header from a config it is given on a pipe.
set -euo pipefail

# An IPv4 address: anything else would be handed to ssh as a host name.
is_address() { [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; }

if [ -n "${DEV_HOST:-}" ]; then
  is_address "$DEV_HOST" || { echo "dev-host: DEV_HOST is not an IPv4 address" >&2; exit 1; }
  echo "$DEV_HOST"
  exit 0
fi

project=issho-dev-512213
zone_name=joinissho.com
record_name=api-dev.joinissho.com

token=$(gcloud secrets versions access latest --secret=dev-cloudflare-api-token --project="$project" | tr -d '\r\n')
[ -z "${GITHUB_ACTIONS:-}" ] || echo "::add-mask::$token"

# cf <path>: Cloudflare's JSON answer, or a failure that says what was asked.
cf() {
  curl -fsS --max-time 20 --config <(printf 'header = "Authorization: Bearer %s"\n' "$token") "https://api.cloudflare.com/client/v4/$1" \
    || { echo "dev-host: Cloudflare refused the request for ${1%%\?*}" >&2; return 1; }
}

zone=$(cf "zones?name=$zone_name" | jq -r '.result[0].id // empty')
[ -n "$zone" ] || { echo "dev-host: no zone $zone_name that the token can read" >&2; exit 1; }

address=$(cf "zones/$zone/dns_records?type=A&name=$record_name" | jq -r '.result[0].content // empty')
is_address "$address" || { echo "dev-host: no A record for $record_name" >&2; exit 1; }
[ -z "${GITHUB_ACTIONS:-}" ] || echo "::add-mask::$address" >&2
echo "$address"
