#!/bin/sh
# Deploys one commit of main to this server (docs/deploy.md, "Automatic
# deploys"). The only thing CI's deploy key can run: authorized_keys of the
# deploy user forces `sudo -u kamal /opt/issho/deploy.sh`, and the workflow
# writes "<commit> <registry token>" on stdin. The token is the workflow's own
# GITHUB_TOKEN, valid only while that job runs.
#
# Production secrets stay on this server (/opt/issho, readable by kamal
# only); Kamal runs here from its image, against this server over SSH.
set -eu

read -r sha token
case "$sha" in
  *[!0-9a-f]* | "") echo "deploy: not a commit: $sha" >&2; exit 1 ;;
esac
[ "${#sha}" -eq 40 ] || { echo "deploy: want a full commit hash" >&2; exit 1; }

# Only commits on main ship, whatever CI sends.
cd /opt/issho/repo
git fetch --quiet origin main
git merge-base --is-ancestor "$sha" origin/main || { echo "deploy: $sha is not on main" >&2; exit 1; }
git checkout --quiet --detach "$sha"

kamal() {
  # The repo checkout as the workdir, with the gitignored files Kamal reads
  # (.kamal/secrets and the env file it points at) mounted from /opt/issho.
  # safe.directory: the checkout belongs to kamal, the container runs as root.
  docker run --rm -i \
    -v /opt/issho/repo:/workdir \
    -v /opt/issho/kamal-secrets:/workdir/.kamal/secrets:ro \
    -v /opt/issho/env:/workdir/.env.issho-prod:ro \
    -v /opt/issho/certs:/root/.config/issho:ro \
    -v /home/kamal/.ssh/id_self:/root/.ssh/id_ed25519:ro \
    -v /home/kamal/.ssh/known_hosts:/root/.ssh/known_hosts:ro \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -e KAMAL_REGISTRY_PASSWORD="$token" \
    -e GIT_CONFIG_COUNT=1 -e GIT_CONFIG_KEY_0=safe.directory -e GIT_CONFIG_VALUE_0='*' \
    ghcr.io/basecamp/kamal:v2.12.0 "$@"
}

# Log in for this deploy only; don't leave the token (short-lived as it is)
# in kamal's Docker config.
trap 'docker logout ghcr.io >/dev/null 2>&1 || true' EXIT
kamal registry login
# Migrations first, with the new image (they're additive; docs/deploy.md).
kamal migrate --version "$sha"
kamal deploy --skip-push --version "$sha"
