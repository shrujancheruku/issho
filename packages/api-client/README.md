# @issho/api-client

The generated TypeScript client for the Issho API, used by the app, the Expo server and the admin.

- `src/gen/` is **generated** from `proto/` by `protoc-gen-es` (`make gen` at the root, after `make setup`). Don't edit it.
- `src/index.ts` exports `createApi()` and re-exports the generated message types.

## Usage

```ts
import { createApi } from '@issho/api-client';

const api = createApi({
  baseUrl: 'http://localhost:8080',
  getAccessToken: async () => session?.access_token ?? null, // the Supabase session
});

const { version } = await api.health.check({});

// The signed-in person's own account; the first call makes it (members only).
const { account } = await api.account.getAccount({ displayLanguage: 'vi' });
```

- `getAccessToken` is called before every request. A token goes out as `Authorization: Bearer <token>`; with none, no header is sent, which only public RPCs accept.
- `onUnauthenticated` (optional) is called when the API answers that the caller is not signed in (the Connect code `unauthenticated`, HTTP 401); the error is still thrown. The app ends the session there, once, so no screen handles it.
- The transport is Connect over `fetch` with JSON encoding, which works with React Native's `fetch` and reads in a network inspector.
- Errors are thrown as `ConnectError`; branch on its `.code` (`Code.Unauthenticated`, `Code.PermissionDenied`).

## Adding a service

Add the `.proto` and run `make gen` (see `proto/README.md`), then import the generated service in `src/index.ts`, re-export its types and add it to the object `createApi` returns.

## Checks

`make lint` type-checks it (`pnpm --filter @issho/api-client typecheck`) and `make test` runs its tests (`src/index.test.ts`, against a fake `fetch`).
