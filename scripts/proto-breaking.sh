#!/usr/bin/env bash
# `buf breaking` against origin/main, for `make lint`. Usage: scripts/proto-breaking.sh
#
# Until the Foundation milestone merges, main holds v1's proto/, so every v2 file
# would read as a break. The check therefore runs only once main holds v2's proto,
# which it recognises by proto/issho/v1/access.proto (v1 had no such file). It
# decides from main's content alone: it takes no arguments or environment, so
# nothing given to make lint can turn it off. An origin/main that isn't fetched is
# an error, not a skip, so a checkout without main can't pass by not checking.
set -euo pipefail

# proto_breaking <ref> checks the git work tree in the current directory against
# <ref>. Only the test file calls it with another ref, by sourcing this script.
proto_breaking() {
  local ref=$1 here tools
  if ! git rev-parse --verify --quiet "$ref^{commit}" >/dev/null; then
    echo "buf breaking: $ref is not here; fetch it (git fetch origin main)" >&2
    return 1
  fi
  if ! git cat-file -e "$ref:proto/issho/v1/access.proto" 2>/dev/null; then
    echo "buf breaking not run: $ref does not hold v2's proto yet (no proto/issho/v1/access.proto)"
    return 0
  fi
  here=$(git rev-parse --show-toplevel)
  # The buf pinned in tools/go.mod, run from this script's repo so `go tool`
  # finds it, on the inputs of the work tree being checked.
  tools=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
  (cd "$tools" && go tool buf breaking "$here" --against "$here/.git#ref=$ref")
}

# Run as a command, not sourced.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  if [[ $# -ne 0 ]]; then
    echo "usage: scripts/proto-breaking.sh (it takes no arguments)" >&2
    exit 2
  fi
  proto_breaking origin/main
fi
