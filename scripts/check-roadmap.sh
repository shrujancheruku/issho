#!/usr/bin/env bash
# Fails when a change touches code/config but not docs/ROADMAP.md, so the
# build-plan status stays current with every push. Usage: check-roadmap.sh <base-ref>
#
# Exempt paths (documentation-only and release-please bookkeeping) don't require
# a roadmap update on their own.
set -euo pipefail

base="${1:?usage: check-roadmap.sh <base-ref>}"
# A brand-new branch push reports an all-zero "before" SHA; nothing to compare.
if [[ "$base" =~ ^0+$ ]] || ! git cat-file -e "$base^{commit}" 2>/dev/null; then
  echo "No base commit to compare against; skipping."
  exit 0
fi

changed=$(git diff --name-only "$base" HEAD)
[[ -z "$changed" ]] && exit 0

if grep -qx 'docs/ROADMAP.md' <<<"$changed"; then
  echo "docs/ROADMAP.md updated."
  exit 0
fi

exempt='^(docs/|.*\.md$|CHANGELOG\.md$|version\.txt$|\.release-please-manifest\.json$|package\.json$|apps/mobile/package\.json$|apps/mobile/app\.json$)'
needs=$(grep -Ev "$exempt" <<<"$changed" || true)
if [[ -n "$needs" ]]; then
  echo "These changes need a matching update to docs/ROADMAP.md (status, decisions or next steps):"
  echo "$needs" | sed 's/^/  /'
  exit 1
fi
echo "Only docs/release files changed; roadmap update not required."
