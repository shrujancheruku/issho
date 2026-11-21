# setup

The local action CI's jobs call to install what they use: `go: "true"` installs Go (the version in `backend/go.mod`, with the module and build caches), `node: "true"` installs pnpm and Node and runs `pnpm install --frozen-lockfile`. A job with neither inputs installs nothing. Use it as `- uses: ./.github/actions/setup` after the checkout; the actions inside are pinned to commits like every other in the repo ([Workflows](../../workflows/README.md)).
