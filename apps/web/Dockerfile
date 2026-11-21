# The Expo server: the member app's web build (apps/app) and the small Express
# server that serves it (server.js). Build from the repository root:
#
#   docker build --file apps/web/Dockerfile --build-arg EXPO_PUBLIC_API_URL=https://api.example.com .
#
# EXPO_PUBLIC_API_URL is the Go API's address. Like every EXPO_PUBLIC_* value it
# is bundled into the app, so it is public and is a build argument, not a secret.
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
# CI=1 keeps the Expo CLI from prompting or watching.
RUN cd apps/app && CI=1 pnpm exec expo export --platform web --output-dir /out

# The server needs only express and expo-server: the export holds its own code.
FROM base AS runtime
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
