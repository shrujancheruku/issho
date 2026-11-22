# The Expo server: the member app's web build (apps/app) and the small Express
# server that serves it (server.js). Build from the repository root:
#
#   EXPO_PUBLIC_SUPABASE_URL=https://example.supabase.co \
#   EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY=sb_publishable_example \
#   docker build --file apps/web/Dockerfile --build-arg EXPO_PUBLIC_API_URL=https://api.example.com \
#     --build-arg SUPABASE_VALUES_HASH=any-string-that-changes-with-the-values \
#     --secret id=EXPO_PUBLIC_SUPABASE_URL,env=EXPO_PUBLIC_SUPABASE_URL \
#     --secret id=EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY,env=EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY .
#
# EXPO_PUBLIC_API_URL is the Go API's address. Like every EXPO_PUBLIC_* value it
# is bundled into the app, so it is public and is a build argument, not a secret.
# The two Supabase sign-in values are public too, but the URL holds the project's
# ref, which stays out of this public repository (docs/secrets.md), so they come in
# as BuildKit build secrets (Kamal's builder secrets, read from Secret Manager). Once
# bundled they are in the app, but never in the repository, a build argument or the
# image's config. A build without them, or with an empty one, fails here rather than shipping an app that cannot sign in.
# The base image is pinned by digest; Renovate keeps it current.

FROM node:24-alpine@sha256:116b800c3754b440ee0aa3ed2f20f5bf3d99a554bfd9adeea09ea8017f6e85f5 AS base
# The pnpm version is the root package.json's packageManager.
RUN corepack enable
WORKDIR /repo

# The member app's export. Only what the app's install and build read, so a change
# to the backend or the docs doesn't invalidate it.
FROM base AS build
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY packages/api-client packages/api-client
COPY apps/app apps/app
COPY apps/web/package.json apps/web/package.json
RUN pnpm install --frozen-lockfile --filter @issho/app...
ARG EXPO_PUBLIC_API_URL
ENV EXPO_PUBLIC_API_URL=${EXPO_PUBLIC_API_URL}
# SUPABASE_VALUES_HASH is a digest of the two values, for the cache alone: BuildKit
# keys a layer on its build arguments but not on the contents of a secret mount, so
# without it a daemon that has built before would reuse the old export (and its old
# bundle) after a value changed. The digest is not a value and reveals none, so it
# may sit in the image's history. The deploy and scripts/image.sh give it; a plain
# docker build may give any string that changes with the values.
ARG SUPABASE_VALUES_HASH=
# CI=1 keeps the Expo CLI from prompting or watching. The secrets are mounted for
# this one command and exported into its environment, so neither value is left in a
# layer's metadata or the image's history; Expo inlines them into the export, as it
# does every EXPO_PUBLIC_* value, so they are in the bundle (public by design) but
# not in the repository, a build argument or the image's config. An empty value
# fails the build like a missing one: an app that cannot sign in is not shipped.
RUN --mount=type=secret,id=EXPO_PUBLIC_SUPABASE_URL \
    --mount=type=secret,id=EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY \
    : "${SUPABASE_VALUES_HASH}" \
    && export EXPO_PUBLIC_SUPABASE_URL="$(cat /run/secrets/EXPO_PUBLIC_SUPABASE_URL)" \
    && export EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY="$(cat /run/secrets/EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY)" \
    && [ -n "$EXPO_PUBLIC_SUPABASE_URL" ] && [ -n "$EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY" ] \
    && cd apps/app && CI=1 pnpm exec expo export --platform web --output-dir /out

# The server needs only express and expo-server: the export holds its own code.
FROM base AS runtime
# Links the package to this repository, so the deploy workflow's GITHUB_TOKEN may push it.
LABEL org.opencontainers.image.source="https://github.com/shrujancheruku/issho"
ENV NODE_ENV=production
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web apps/web
# The workspace file lists the other packages, but only @issho/web's are installed.
RUN pnpm install --frozen-lockfile --prod --filter @issho/web
COPY --from=build /out apps/web/dist
USER node
WORKDIR /repo/apps/web
EXPOSE 8081
CMD ["node", "server.js"]
