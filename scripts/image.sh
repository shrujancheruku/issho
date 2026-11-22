#!/usr/bin/env bash
# Builds the two container images the deploy ships, checks each, and removes
# them. Usage: scripts/image.sh   (needs Docker)
#
#   backend/Dockerfile  holds both the api and the worker
#   apps/web/Dockerfile serves the member app's web build: a plain GET of a page
#                       gets its server-rendered HTML
#
# The images are not kept or pushed: this is the `image` check, "the containers
# build". The deploy workflow (.github/workflows/deploy-dev.yml) builds and ships
# them.
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
# A name of its own, so a build here never moves a tag someone else uses.
image="issho-backend:image-check-$$"
web_image="issho-web:image-check-$$"
work=$(mktemp -d)
container=""
web_container=""
cleanup() {
  [ -z "$container" ] || docker rm -f "$container" >/dev/null 2>&1 || true
  [ -z "$web_container" ] || docker rm -f "$web_container" >/dev/null 2>&1 || true
  docker rmi -f "$image" "$web_image" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

docker build --file "$repo/backend/Dockerfile" --tag "$image" "$repo/backend"

# The runtime image has no shell, so look inside it by copying the files out of
# a container that is created and never started.
container=$(docker create "$image")
for bin in api worker; do
  docker cp "$container:/usr/local/bin/$bin" "$work/$bin"
  [ -x "$work/$bin" ] || { echo "image: /usr/local/bin/$bin is not executable" >&2; exit 1; }
  # 7f 45 4c 46: an ELF binary, not a script or an empty file.
  [ "$(head -c 4 "$work/$bin" | od -An -tx1 | tr -d ' \n')" = "7f454c46" ] \
    || { echo "image: /usr/local/bin/$bin is not a binary" >&2; exit 1; }
done
echo "image: ok (backend holds api and worker)"

# The web image: built from the repository root, as the deploy builds it.
docker build --file "$repo/apps/web/Dockerfile" --build-arg EXPO_PUBLIC_API_URL=https://api.image-check.invalid --tag "$web_image" "$repo"
# Docker picks the host port, so nothing on a laptop (Metro's 8081 included) clashes.
web_container=$(docker run --detach --publish 127.0.0.1::8081 "$web_image")
port=$(docker port "$web_container" 8081/tcp | head -n1 | sed 's/.*://')
base="http://127.0.0.1:$port"

# The server needs a moment to listen.
up=""
for _ in $(seq 1 50); do
  if [ "$(curl -s -o /dev/null -w '%{http_code}' "$base/up" || true)" = 200 ]; then up=1; break; fi
  sleep 0.2
done
[ -n "$up" ] || { docker logs "$web_container" >&2; echo "image: the web container's /up never answered 200" >&2; exit 1; }

# Server-rendered: the page's own words are in the HTML a client with no
# JavaScript gets, and the bundle it points at is served.
html=$(curl -fsS "$base/plans")
grep -q 'No plans yet.' <<<"$html" || { echo "image: /plans is not server-rendered (no 'No plans yet.' in its HTML)" >&2; exit 1; }
script=$(grep -oE '/_expo/static/js/web/[^"]+\.js' <<<"$html" | head -n1)
[ -n "$script" ] || { echo "image: /plans names no script" >&2; exit 1; }
[ "$(curl -s -o /dev/null -w '%{http_code}' "$base$script")" = 200 ] || { echo "image: $script is not served" >&2; exit 1; }
echo "image: ok (the web serves server-rendered HTML)"
