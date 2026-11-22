#!/usr/bin/env bash
# Prints the dev VM's address, for the deploy to give Kamal as DEPLOY_HOST.
#
# The address is not stored anywhere: not in the repository, not in a GitHub
# variable. The deploy service account has a custom role with only
# compute.addresses.get, and the run asks Google Cloud for the VM's reserved
# address, issho-dev-ip, each time. The caller masks what this prints
# (::add-mask::) before anything else runs: this repository's Actions logs are
# public, and the address names the VM.
set -euo pipefail

address=$(gcloud compute addresses describe issho-dev-ip \
  --region asia-southeast1 --project issho-dev-512213 --format='value(address)' | tr -d '\r\n')

# An IPv4 address: anything else would be handed to ssh as a host name.
[[ "$address" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] || { echo "dev-host: issho-dev-ip has no IPv4 address" >&2; exit 1; }
echo "$address"
