# @issho/admin

The admin: a Vite React single-page app for staff, on the same Go API as the member app ([architecture](../../docs/spec/architecture.md#what-v2-is-built-from)). It is served at its own address and calls the API only through [`@issho/api-client`](../../packages/api-client/README.md). Its screens come in the milestones that need them ([build order](../../docs/spec/build-order.md#the-milestones)); for now it has one page, which asks the public health check for the API's version.

## Layout

- `src/main.tsx` builds the client with `createApi` and mounts the page.
- `src/HealthPage.tsx` is the page. It takes the client as a prop, so tests pass a fake.
- `src/HealthPage.test.tsx` renders the page against a fake of the generated client (Vitest and Testing Library, in jsdom).
- `vite.config.ts` sets the dev port, the API proxy and the test environment.
- `worker/index.ts` is the deployed admin's Worker: it forwards the API's paths to the API (see Deploying). `worker/index.test.ts` tests it.
- `wrangler.jsonc` is the Cloudflare config; its `dev` environment is `admin-dev.joinissho.com`.

## Setup

From the repo root, run `make setup` once (it installs the workspace's dependencies, `apps/admin` included).

## Running

```sh
pnpm --filter @issho/admin dev    # http://localhost:5173
```

The dev server proxies the API's paths (`/issho.v1.*`) to `http://localhost:8080`, so start the API first: `cd backend && go run ./cmd/api` (it listens on `:8080`, or `API_ADDR`). For an API on another address, set `API_PROXY_TARGET` (e.g. `API_PROXY_TARGET=http://127.0.0.1:9090`); it is read by the dev server only and never bundled. Without a running API the page says the API could not be reached.

To point the build at another API, set `VITE_API_URL` at build time. An empty value (the default) means the API is on the page's own origin.

## Building

```sh
pnpm --filter @issho/admin build  # type check, then static files in dist/
```

`make lint` runs this build; the output is ignored by git.

## Deploying

CI deploys the admin to dev as a Cloudflare Worker, `issho-admin-dev` at `admin-dev.joinissho.com`, after the API ([docs/deploy.md](../../docs/deploy.md)): the Vite build in `dist/` is served as static assets and `worker/index.ts` answers only the API's paths. Nothing here is deployed from a laptop.

**How the deployed admin reaches the API.** It is on its own address and the API has no CORS, so the Worker forwards `/issho.v1.*` to `API_ORIGIN` (`https://api-dev.joinissho.com`, set in `wrangler.jsonc`) and the page calls relative paths, as it does through Vite's proxy in development (`VITE_API_URL` stays empty). The Worker forwards nothing else, so it is not an open proxy, and it drops the headers that carry the member's address (`cf-connecting-ip`, `x-forwarded-for` and the like), since Issho keeps no IP address. Why no CORS: [ADR 0006](../../docs/adr/0006-browsers-reach-the-api-on-their-own-origin.md). The API stays closed to every browser origin but the one that proxies it, and no origin list has to be kept. The cost is a second hop for the admin's calls and a Worker request for each; both are small for staff.

## Testing

```sh
pnpm --filter @issho/admin test
```

`make test` runs the same. The screen test asserts what a person sees: the version the API reports, and a message when the API cannot be reached. The Worker's test calls it with a stand-in `fetch`: an RPC is forwarded with its method, headers and body, the API's errors pass through, nothing outside the API's paths is forwarded, the member's address headers are not passed on, and an unreachable API answers 502. It needs no API and no database.

## Adding a screen

Add the page under `src/`, take the client as a prop as `HealthPage` does, and test it against a fake of the generated client. Call the RPCs through the client only; the admin never talks to the database.

## Screenshots

`docs/screenshots/` holds the page's states at phone width (390×844), light and dark: `version-*.png` (the API answered) and `unreachable-*.png` (it did not). They were taken with headless Chrome against the dev server: the API built from `backend/cmd/api` with `API_ADDR=127.0.0.1:0` and `API_PROXY_TARGET` pointing at it for the first, and a closed port for the second. Chrome's `--blink-settings=preferredColorScheme=1` forces light (`0` dark). The admin is web-only, so there are no simulator or emulator captures.
