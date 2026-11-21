#!/usr/bin/env bash
# Tests that the dev deploy's Kamal config (config/deploy*.yml, .kamal/secrets.dev)
# loads and says what docs/deploy.md says it does, using Kamal itself (the pinned
# release, in its Docker image) and a stand-in for gcloud, so it reads no real
# secret and touches no server. Usage: scripts/deploy-config.test.sh   (needs Docker)
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo"
kamal_image="ghcr.io/basecamp/kamal:v$(tr -d '[:space:]' < config/kamal-version)"
fail=0
bad() { echo "FAIL $*"; fail=1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# A repository of its own holding just the config, since Kamal asks git for the
# version and the worktree's .git is a pointer a container can't follow.
mkdir "$work/project"
cp -R config .kamal "$work/project/"
git -C "$work/project" init -q
git -C "$work/project" add -A
git -C "$work/project" -c user.email=test@example.test -c user.name=test commit -qm test

# Stands in for gcloud: signed in, and every secret's value is "value-of-<name>",
# except the certificate, which spans lines like a real PEM.
cat > "$work/gcloud" <<'STUB'
#!/bin/sh
case "$1 $2" in
  "auth list") echo '[{"account":"stub@example.test"}]' ;;
  "--version "*) echo "Google Cloud SDK stub" ;;
  "secrets versions")
    name=$(printf '%s\n' "$@" | sed -n 's/^--secret=//p')
    case "$name" in
      dev-origin-cert) v=$(printf -- '-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n') ;;
      *) v="value-of-$name" ;;
    esac
    printf '{"payload":{"data":"%s"}}\n' "$(printf '%s' "$v" | base64 | tr -d '\n')" ;;
  *) echo "gcloud stub: unexpected $*" >&2; exit 1 ;;
esac
STUB
chmod +x "$work/gcloud"

# kamal [args…]: Kamal in its image, over the project, with DEPLOY_HOST given as
# the deploy gives it.
kamal() {
  docker run --rm -v "$work/project:/workdir" -v "$work/gcloud:/usr/local/bin/gcloud:ro" \
    -e GIT_CONFIG_COUNT=1 -e GIT_CONFIG_KEY_0=safe.directory -e GIT_CONFIG_VALUE_0='*' \
    -e DEPLOY_HOST=203.0.113.7 -e KAMAL_REGISTRY_PASSWORD=stub \
    "$kamal_image" "$@"
}

# What each service's configuration comes to, from Kamal's own loader.
facts=$(docker run --rm -v "$work/project:/workdir" -v "$work/gcloud:/usr/local/bin/gcloud:ro" \
  -e GIT_CONFIG_COUNT=1 -e GIT_CONFIG_KEY_0=safe.directory -e GIT_CONFIG_VALUE_0='*' \
  -e DEPLOY_HOST=203.0.113.7 -e KAMAL_REGISTRY_PASSWORD=stub \
  --entrypoint ruby "$kamal_image" -e '
require "kamal"
require "json"
out = {}
{ "backend" => "config/deploy.yml", "web" => "config/deploy.web.yml" }.each do |name, file|
  c = Kamal::Configuration.create_from(config_file: Pathname.new(file), destination: "dev")
  out[name] = {
    hosts: c.all_hosts,
    image: c.repository,
    roles: c.roles.map { |r| { name: r.name, cmd: r.cmd, proxied: r.running_proxy? } },
    proxy: c.proxy.deploy_options.transform_keys(&:to_s),
    registry: c.registry.server,
    ssh_user: c.ssh.user,
    keys_only: c.ssh.keys_only,
    ssh_key_read: c.ssh.key_data,
    cert: c.proxy.certificate_pem_content,
    build_args: c.builder.args,
    worker_secrets: c.raw_config.servers["worker"]&.dig("env", "secret"),
  }
end
puts JSON.generate(out)
')

q() { jq -r "$1" <<<"$facts"; }
expect() { # <what> <jq expression> <expected>
  local got
  got=$(q "$2")
  [ "$got" = "$3" ] || bad "$1: got '$got', want '$3'"
}

# Both services deploy to the one dev VM, which DEPLOY_HOST names (no address is in the repository).
expect "backend host" '.backend.hosts | join(",")' 203.0.113.7
expect "web host" '.web.hosts | join(",")' 203.0.113.7
expect "backend image" '.backend.image' ghcr.io/shrujancheruku/issho-backend
expect "web image" '.web.image' ghcr.io/shrujancheruku/issho-web
expect "kamal logs in as the deploy user" '.backend.ssh_user' kamal
expect "only the given key is offered" '.backend.keys_only' true
expect "the key comes from Secret Manager" '.backend.ssh_key_read[0]' value-of-dev-ssh-private-key

# The backend: the API behind the proxy, the worker beside it with no proxy.
expect "roles" '.backend.roles | map(.name) | join(",")' web,worker
expect "the API is proxied" '.backend.roles[] | select(.name == "web") | .proxied' true
expect "the worker runs the worker binary" '.backend.roles[] | select(.name == "worker") | .cmd' /usr/local/bin/worker
expect "the worker has no proxy" '.backend.roles[] | select(.name == "worker") | .proxied' false
expect "the worker's secrets" '.backend.worker_secrets | join(",")' WORKER_DATABASE_URL,RESEND_API_KEY,EMAIL_FROM

# Routing: the API on its own name and on the web's, but only its RPC paths on
# the web's, with the prefix left on, so a browser calls it on the page's origin.
expect "the API's names" '.backend.proxy.host | join(",")' api-dev.joinissho.com,dev.joinissho.com
expect "the API's paths" '.backend.proxy["path-prefix"] | join(",")' /issho.v1.
expect "the prefix is not stripped" '.backend.proxy["strip-path-prefix"]' false
expect "the API's probe" '.backend.proxy["health-check-path"]' /up
expect "the web's name" '.web.proxy.host | join(",")' dev.joinissho.com
expect "the web takes every other path" '.web.proxy["path-prefix"] | length' 0
expect "the web's probe" '.web.proxy["health-check-path"]' /up
expect "the web's port is Expo's server" '.web.roles[0].proxied' true

# TLS is Cloudflare's origin certificate, read from Secret Manager, multi-line intact.
expect "TLS is on for the API" '.backend.proxy.tls' true
expect "TLS is on for the web" '.web.proxy.tls' true
expect "the certificate keeps its lines" '.backend.cert | split("\n") | length' 3

# The version is a string even when a short commit is all digits (YAML would
# read 1518948 as a number, and 123e456 as a float).
expect "the API is built with the commit's version, as a string" '.backend.build_args.VERSION | type' string

# The web is built for the page's own origin.
expect "the web is built against the page's origin" '.web.build_args.EXPO_PUBLIC_API_URL' https://dev.joinissho.com

# A call without a destination is refused, so nothing can deploy to the wrong place.
if kamal config >/dev/null 2>&1; then bad "kamal ran without a destination"; fi

# .kamal/secrets.dev holds names and reads, never a value: every line is a comment, a
# blank, a fetch from Secret Manager, or an extract of what it fetched.
bad_lines=$(grep -vE '^\s*(#|$)|^SECRETS=\$\(kamal secrets fetch --adapter gcp_secret_manager |^[A-Z_]+=\$\(kamal secrets extract dev-[a-z-]+ \$\{SECRETS\}\)$|^KAMAL_REGISTRY_PASSWORD=\$KAMAL_REGISTRY_PASSWORD$' .kamal/secrets.dev || true)
[ -z "$bad_lines" ] || bad ".kamal/secrets.dev holds something but names and reads: $bad_lines"

# Kamal refuses a build whose registry password is not listed in the secrets file
# ("Secret 'KAMAL_REGISTRY_PASSWORD' not found"), as the first CI run showed.
grep -qx 'KAMAL_REGISTRY_PASSWORD=$KAMAL_REGISTRY_PASSWORD' .kamal/secrets.dev || bad ".kamal/secrets.dev does not pass KAMAL_REGISTRY_PASSWORD through"

[ "$fail" -eq 0 ] && echo "deploy-config: ok"
exit "$fail"
