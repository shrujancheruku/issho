#!/usr/bin/env bash
# Builds and deploys the admin SPA (issho-admin) and the app's web build
# (issho-web) to Cloudflare Workers, pointed at the given API.
# Usage: scripts/deploy-web.sh <api-url> [admin|web]
#
# The API URL is baked into each build, so redeploy whenever it changes
# (a Cloudflare quick tunnel gets a new URL every time it starts). The
# Supabase URL, publishable key and web Maps key (all public) come from the
# environment as EXPO_PUBLIC_* when set (CI: .github/workflows/deploy-web.yml),
# otherwise from apps/mobile/.env (issho-dev). Needs `wrangler login` (or
# CLOUDFLARE_API_TOKEN). See apps/admin/README.md.
set -euo pipefail

api="${1:?usage: deploy-web.sh <api-url> [admin|web]}"
api="${api%/}"
only="${2:-}"
root="$(cd "$(dirname "$0")/.." && pwd)"
# The environment wins over apps/mobile/.env, as Expo itself does.
read_env() {
  local v="${!1:-}"
  if [[ -z "$v" && -f "$root/apps/mobile/.env" ]]; then v="$(sed -n "s/^$1=//p" "$root/apps/mobile/.env")"; fi
  printf '%s' "$v"
}
supabase_url="$(read_env EXPO_PUBLIC_SUPABASE_URL)"
publishable_key="$(read_env EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY)"
if [[ -z "$supabase_url" || -z "$publishable_key" ]]; then
  echo "Set EXPO_PUBLIC_SUPABASE_URL and EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY (environment or apps/mobile/.env)" >&2
  exit 1
fi
if [[ ! "$api" =~ ^https://[A-Za-z0-9.-]+(:[0-9]+)?$ ]]; then
  echo "The API URL must be an https origin with no path, e.g. https://api.example.com" >&2
  exit 1
fi

if [[ "$only" != web ]]; then
  (
    cd "$root/apps/admin"
    VITE_API_URL="$api" VITE_SUPABASE_URL="$supabase_url" VITE_SUPABASE_PUBLISHABLE_KEY="$publishable_key" pnpm exec vite build
    # public/_headers lets the page call any quick tunnel; the deployed copy
    # allows only this API's origin.
    grep -q 'https://\*\.trycloudflare\.com' dist/_headers
    sed -i.bak "s#https://\*\.trycloudflare\.com#$api#" dist/_headers && rm dist/_headers.bak
    pnpm exec wrangler deploy
  )
fi
if [[ "$only" != admin ]]; then
  # Expo doesn't override variables already set, so these win over .env.
  (cd "$root/apps/mobile" && EXPO_PUBLIC_API_URL="$api" EXPO_PUBLIC_SUPABASE_URL="$supabase_url" \
    EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY="$publishable_key" pnpm run deploy:web)
fi
cat <<MSG

Deployed against $api. The API needs these origins in ALLOWED_ORIGINS:
  https://issho-admin.shrujancheruku.workers.dev,https://issho-web.shrujancheruku.workers.dev
MSG
