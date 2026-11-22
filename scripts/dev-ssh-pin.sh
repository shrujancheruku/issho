#!/usr/bin/env bash
# Pins the dev VM's SSH host key for Kamal: writes a known_hosts line from the
# VM's address and its public host key, and an ssh config that refuses any other
# key, then prints the config's path for the deploy to give Kamal as
# DEV_SSH_CONFIG (config/deploy.dev.yml).
#
# The runner is new each run, so without this ssh would accept whatever key it
# was shown first, and a machine in the path could pose as the VM and be handed
# the containers' secrets. The key is the secret dev-ssh-host-key (one line,
# `ssh-ed25519 AAAA…`, no address); the address comes from scripts/dev-host.sh,
# so neither is written in the repository.
# Usage: scripts/dev-ssh-pin.sh <address> <directory>
set -euo pipefail

host=${1:?usage: dev-ssh-pin.sh <address> <directory>}
dir=${2:?usage: dev-ssh-pin.sh <address> <directory>}

key=$(gcloud secrets versions access latest --secret=dev-ssh-host-key --project=issho-dev-512213 | tr -d '\r')
# One line of a type and a key; a comment after them is dropped.
[ "$(printf '%s\n' "$key" | grep -c .)" -eq 1 ] || { echo "dev-ssh-pin: dev-ssh-host-key is not one line" >&2; exit 1; }
read -r type body _ <<<"$key"
[[ "$type" =~ ^(ssh-ed25519|ecdsa-sha2-nistp(256|384|521)|ssh-rsa)$ && "$body" =~ ^[A-Za-z0-9+/]+=*$ ]] \
  || { echo "dev-ssh-pin: dev-ssh-host-key is not a public host key" >&2; exit 1; }

mkdir -p "$dir"
printf '%s %s %s\n' "$host" "$type" "$body" > "$dir/known_hosts"
# StrictHostKeyChecking yes: an unknown or changed key is refused, never added.
# Only this file is read, so nothing else the runner knows can vouch for a host.
cat > "$dir/ssh_config" <<CONFIG
Host *
  StrictHostKeyChecking yes
  UserKnownHostsFile $dir/known_hosts
  GlobalKnownHostsFile /dev/null
CONFIG
echo "$dir/ssh_config"
