#!/usr/bin/env bash
# `buf breaking` against main, for `make lint`. Usage: scripts/proto-breaking.sh [ref]
#
# Until the Foundation milestone merges, main holds v1's proto/, so every v2 file
# would read as a break. The check therefore runs only once main holds v2's proto,
# which it recognises by proto/issho/v1/access.proto (v1 had no such file). It
# decides from main's content alone: nothing turns it off. When main has no v2
# proto, or the ref isn't fetched, it says so and exits 0.
set -euo pipefail

ref=${1:-origin/main}
buf=${BUF:-"go tool buf"}

if ! git rev-parse --verify --quiet "$ref^{commit}" >/dev/null; then
  echo "buf breaking not run: $ref is not fetched"
  exit 0
fi
if ! git cat-file -e "$ref:proto/issho/v1/access.proto" 2>/dev/null; then
  echo "buf breaking not run: $ref does not hold v2's proto yet (no proto/issho/v1/access.proto)"
  exit 0
fi

# shellcheck disable=SC2086 # $buf is a command line ("go tool buf"), split on purpose
exec $buf breaking --against ".git#ref=$ref"
