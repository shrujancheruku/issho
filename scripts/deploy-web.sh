#!/usr/bin/env bash
# Builds and deploys the admin SPA (issho-admin) and the app's web build
# (issho-web) to Cloudflare Workers, pointed at the given API.
# Usage: scripts/deploy-web.sh <api-url> [admin|web]
#
# The API URL is baked into each build, so redeploy whenever it changes
# (a Cloudflare quick tunnel gets a new URL every time it starts). Supabase
# URL and publishable key come from apps/mobile/.env (issho-dev; both are
# public). Needs `wrangler login` (or CLOUDFLARE_API_TOKEN). See
# apps/admin/README.md.
set -euo pipefail

api="${1:?usage: deploy-web.sh <api-url> [admin|web]}"
only="${2:-}"
root="$(cd "$(dirname "$0")/.." && pwd)"
read_env() { sed -n "s/^$1=//p" "$root/apps/mobile/.env"; }
supabase_url="$(read_env EXPO_PUBLIC_SUPABASE_URL)"
publishable_key="$(read_env EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY)"
if [[ -z "$supabase_url" || -z "$publishable_key" ]]; then
  echo "apps/mobile/.env needs EXPO_PUBLIC_SUPABASE_URL and EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY" >&2
  exit 1
fi

if [[ "$only" != web ]]; then
  (cd "$root/apps/admin" && VITE_API_URL="$api" VITE_SUPABASE_URL="$supabase_url" \
    VITE_SUPABASE_PUBLISHABLE_KEY="$publishable_key" pnpm run deploy)
fi
if [[ "$only" != admin ]]; then
  # Expo doesn't override variables already set, so this wins over .env's API URL.
  (cd "$root/apps/mobile" && EXPO_PUBLIC_API_URL="$api" pnpm run deploy:web)
fi
cat <<MSG

Deployed against $api. The API needs these origins in ALLOWED_ORIGINS:
  https://issho-admin.shrujancheruku.workers.dev,https://issho-web.shrujancheruku.workers.dev
MSG
