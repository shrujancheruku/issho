# @issho/admin

The admin: a Vite React single-page app for staff, on the same Go API as the member app ([architecture](../../docs/spec/architecture.md#what-v2-is-built-from)). It is served at its own address and calls the API only through [`@issho/api-client`](../../packages/api-client/README.md). Its screens come in the milestones that need them ([build order](../../docs/spec/build-order.md#the-milestones)); for now it has one page, which asks the public health check for the API's version.

## Layout

- `src/main.tsx` builds the client with `createApi` and mounts the page.
- `src/HealthPage.tsx` is the page. It takes the client as a prop, so tests pass a fake.
- `src/HealthPage.test.tsx` renders the page against a fake of the generated client (Vitest and Testing Library, in jsdom).
- `vite.config.ts` sets the dev port, the API proxy and the test environment.

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

## Testing

```sh
pnpm --filter @issho/admin test
```

`make test` runs the same. The screen test asserts what a person sees: the version the API reports, and a message when the API cannot be reached. It needs no API and no database.

## Adding a screen

Add the page under `src/`, take the client as a prop as `HealthPage` does, and test it against a fake of the generated client. Call the RPCs through the client only; the admin never talks to the database.

## Screenshots

`docs/screenshots/` holds the page's states at phone width (390×844), light and dark: `version-*.png` (the API answered) and `unreachable-*.png` (it did not). They were taken with headless Chrome against the dev server: the API built from `backend/cmd/api` with `API_ADDR=127.0.0.1:0` and `API_PROXY_TARGET` pointing at it for the first, and a closed port for the second. Chrome's `--blink-settings=preferredColorScheme=1` forces light (`0` dark). The admin is web-only, so there are no simulator or emulator captures.
