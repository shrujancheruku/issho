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
  docker rmi -f "$image" "$web_image" "$web_image-b" "$web_image-empty" >/dev/null 2>&1 || true
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
# The test-only RPCs (the end-to-end build's clock and forming trigger, built only
# under the `e2e` tag) are compiled out: neither binary holds their package name,
# which is in the binary's own descriptors and route names if they are linked in
# (#622). The same check on a binary built with the tag, in
# backend/cmd/cmd_test.go, proves this one can find them.
for bin in api worker; do
  ! grep -aqF -- "issho.e2e.v1" "$work/$bin" || { echo "image: /usr/local/bin/$bin holds the test-only RPCs (issho.e2e.v1)" >&2; exit 1; }
done
echo "image: ok (backend holds api and worker, and neither holds the test-only RPCs)"

# The web image: built from the repository root, as the deploy builds it.
# The sign-in values are build secrets the Dockerfile requires; these stand-ins are
# not a real project's. SUPABASE_VALUES_HASH is what the deploy gives too (see the
# Dockerfile): secret mounts are not part of BuildKit's cache key, so without it a
# daemon that has built before reuses an old export when the values change.
build_web() { # build_web <tag> <url> <key>
  EXPO_PUBLIC_SUPABASE_URL=$2 EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY=$3 \
  docker build --file "$repo/apps/web/Dockerfile" --build-arg EXPO_PUBLIC_API_URL=https://api.image-check.invalid \
    --build-arg SUPABASE_VALUES_HASH="$(printf '%s\n%s' "$2" "$3" | shasum -a 256 | cut -d' ' -f1)" \
    --secret id=EXPO_PUBLIC_SUPABASE_URL,env=EXPO_PUBLIC_SUPABASE_URL \
    --secret id=EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY,env=EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY \
    --tag "$1" "$repo"
}
# bundle_has <tag> <text>: whether the exported bundle in the image holds the text.
bundle_has() {
  docker run --rm --entrypoint grep "$1" -rqF -- "$2" /repo/apps/web/dist/client/_expo/static/js
}

url_a=https://image-check-a.invalid key_a=image-check-key-a
url_b=https://image-check-b.invalid key_b=image-check-key-b
build_web "$web_image" "$url_a" "$key_a"
# The sign-in values are in the bundle: without them the app cannot sign in.
bundle_has "$web_image" "$url_a" || { echo "image: the bundle does not hold the Supabase URL" >&2; exit 1; }
bundle_has "$web_image" "$key_a" || { echo "image: the bundle does not hold the publishable key" >&2; exit 1; }
# A rebuild with other values, on the same daemon and so the same cache, must
# bundle the new ones, not the first build's.
web_image_b="$web_image-b"
build_web "$web_image_b" "$url_b" "$key_b"
bundle_has "$web_image_b" "$url_b" && bundle_has "$web_image_b" "$key_b" \
  || { echo "image: a rebuild with new Supabase values kept the old bundle" >&2; exit 1; }
! bundle_has "$web_image_b" "$url_a" || { echo "image: the rebuilt bundle still holds the first URL" >&2; exit 1; }
# An empty value is refused, as a missing one is, and by the Dockerfile's own
# guard: any other failure (a pull, BuildKit) is not this check passing, so the
# build's output must name the cause.
refused_empty() { # refused_empty <what> <url> <key>
  local out
  if out=$(BUILDKIT_PROGRESS=plain build_web "$web_image-empty" "$2" "$3" 2>&1); then
    echo "image: the build accepted an empty $1" >&2; exit 1
  fi
  grep -qF "EXPO_PUBLIC_SUPABASE_URL or _PUBLISHABLE_KEY is empty" <<<"$out" \
    || { printf '%s\n' "$out" | tail -n 20 >&2; echo "image: the build with an empty $1 failed, but not at the guard" >&2; exit 1; }
}
refused_empty "Supabase URL" "" "$key_a"
refused_empty "publishable key" "$url_a" ""
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
# JavaScript gets, and the bundle it points at is served. The plans themselves
# come from the API, which this container has none of, so the check reads the
# words around them: the tab bar's names.
html=$(curl -fsS "$base/plans")
grep -q '>Together<' <<<"$html" && grep -q '>Plans<' <<<"$html" || { echo "image: /plans is not server-rendered (no tab bar in its HTML)" >&2; exit 1; }
script=$(grep -oE '/_expo/static/js/web/[^"]+\.js' <<<"$html" | head -n1)
[ -n "$script" ] || { echo "image: /plans names no script" >&2; exit 1; }
[ "$(curl -s -o /dev/null -w '%{http_code}' "$base$script")" = 200 ] || { echo "image: $script is not served" >&2; exit 1; }
echo "image: ok (the web serves server-rendered HTML)"
