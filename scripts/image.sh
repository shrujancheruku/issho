#!/usr/bin/env bash
# Builds backend/Dockerfile and checks the image holds both the api and the
# worker, then removes the image. Usage: scripts/image.sh   (needs Docker)
#
# The image is not kept or pushed: this is the `image` check, "the container
# builds". The deploy workflow (#605) builds and ships it.
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
# A name of its own, so a build here never moves a tag someone else uses.
image="issho-backend:image-check-$$"
work=$(mktemp -d)
container=""
cleanup() {
  [ -z "$container" ] || docker rm -f "$container" >/dev/null 2>&1 || true
  docker rmi -f "$image" >/dev/null 2>&1 || true
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
echo "image: ok (holds api and worker)"
