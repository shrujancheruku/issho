#!/usr/bin/env bash
# Tests scripts/dev-ssh-pin.sh: with a stand-in for gcloud, that it writes the
# VM's address and key as the only known host and an ssh config that refuses any
# other key; then, against a real sshd in Docker and through the net-ssh that
# Kamal uses, that the pinned key is accepted and a different one refused.
# Usage: scripts/dev-ssh-pin.test.sh   (needs Docker)
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
script="$repo/scripts/dev-ssh-pin.sh"
kamal_image="ghcr.io/basecamp/kamal:v$(tr -d '[:space:]' < "$repo/config/kamal-version")"
fail=0
bad() { echo "FAIL $*"; fail=1; }

work=$(mktemp -d)
trap 'rm -rf "$work"; docker rm -f "${sshd:-none}" >/dev/null 2>&1 || true; docker network rm "${net:-none}" >/dev/null 2>&1 || true' EXIT
mkdir "$work/bin"

cat > "$work/bin/gcloud" <<'STUB'
#!/bin/sh
echo "gcloud $*" >> "$LOG"
[ -z "${FAIL:-}" ] || exit 1
printf '%s\r\n' "$KEY"
STUB
chmod +x "$work/bin/gcloud"

# pin <address> <directory>: the script with the stub, KEY being the secret's value.
pin() { : > "$work/log"; PATH="$work/bin:$PATH" LOG="$work/log" "$script" "$@" 2> "$work/err"; }

good='ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIK0wmN/Cr3JXqmLW7u+g9vTdLbWXAMPLEKEYAAAAAAAA'
config=$(KEY="$good root@issho-dev" pin 203.0.113.7 "$work/out") || bad "failed on a good key: $(cat "$work/err")"
[ "$config" = "$work/out/ssh_config" ] || bad "printed '$config', not the config's path"
grep -q -- '--secret=dev-ssh-host-key' "$work/log" || bad "did not read dev-ssh-host-key"
grep -q -- '--project=issho-dev-512213' "$work/log" || bad "did not name the project"
[ "$(cat "$work/out/known_hosts")" = "203.0.113.7 $good" ] || bad "known_hosts is '$(cat "$work/out/known_hosts")'"
grep -qx '  StrictHostKeyChecking yes' "$work/out/ssh_config" || bad "the config does not refuse unknown keys"
grep -qx "  UserKnownHostsFile $work/out/known_hosts" "$work/out/ssh_config" || bad "the config does not read the pinned file"
grep -qx '  GlobalKnownHostsFile /dev/null' "$work/out/ssh_config" || bad "the config reads the runner's known hosts"
if grep -qi 'accept-new\|StrictHostKeyChecking no' "$work/out/ssh_config"; then bad "the config accepts new keys"; fi

if KEY='' pin 203.0.113.7 "$work/x" >/dev/null; then bad "succeeded with no key"; fi
if KEY="203.0.113.7 $good" pin 203.0.113.7 "$work/x" >/dev/null; then bad "accepted a line with an address in it"; fi
if KEY=$'ssh-ed25519 AAAA\nssh-ed25519 BBBB' pin 203.0.113.7 "$work/x" >/dev/null; then bad "accepted two lines"; fi
if KEY="$good" FAIL=1 pin 203.0.113.7 "$work/x" >/dev/null; then bad "succeeded although gcloud failed"; fi
if KEY="$good" pin >/dev/null 2>&1; then bad "ran with no address"; fi

# A real sshd, and Kamal's net-ssh connecting with the written config. The
# client is refused a login either way (it offers no key the server knows), so
# what tells the cases apart is the error: a changed host key is refused before
# any login is tried.
net=$(docker network create "dev-ssh-pin-$$")
sshd=$(docker run -d --network "$net" --entrypoint sh "$kamal_image" -c \
  'apk add -q --no-cache openssh-server >/dev/null && ssh-keygen -q -A && exec /usr/sbin/sshd -D -e')
for _ in $(seq 1 60); do
  docker exec "$sshd" test -f /etc/ssh/ssh_host_ed25519_key.pub 2>/dev/null && docker exec "$sshd" pgrep sshd >/dev/null 2>&1 && break
  sleep 1
done
server_key=$(docker exec "$sshd" cut -d' ' -f1,2 /etc/ssh/ssh_host_ed25519_key.pub)
address=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$sshd")

# connect <key> [<pinned address>]: what net-ssh says when Kamal's options meet
# the server, with <key> pinned for the server's address (or another one).
connect() {
  rm -rf "$work/c"
  KEY="$1" pin "${2:-$address}" "$work/c" >/dev/null
  docker run --rm --network "$net" -v "$work/c:$work/c:ro" --entrypoint ruby "$kamal_image" -e '
require "net/ssh"
begin
  Net::SSH.start(ARGV[0], "kamal", config: [ARGV[1]], keys_only: true, key_data: [], non_interactive: true, timeout: 10) { }
  puts "connected"
rescue Net::SSH::HostKeyError => e
  puts "host key refused: #{e.class}"
rescue Net::SSH::AuthenticationFailed
  puts "host key accepted"
end' "$address" "$work/c/ssh_config"
}
got=$(connect "$server_key")
[ "$got" = "host key accepted" ] || bad "the pinned key: $got"
got=$(connect "$good")
[[ "$got" == "host key refused: "* ]] || bad "a changed key: $got"
got=$(connect "$server_key" 203.0.113.9)
[[ "$got" == "host key refused: "* ]] || bad "an unknown host: $got"

[ "$fail" -eq 0 ] && echo "dev-ssh-pin: ok"
exit "$fail"
