# @issho/api-client

A typed TypeScript client for the Issho API, shared by the mobile app and the (planned) admin SPA.

- `src/gen/` is **generated** from `proto/` by `protoc-gen-es` (`make gen` at the root). Don't edit it.
- `src/index.ts` exports `createApi()` and re-exports all generated message types.

## Usage

```ts
import { createApi } from '@issho/api-client';

const api = createApi({
  baseUrl: 'http://localhost:8080',
  getAccessToken: async () => session?.access_token ?? null, // Supabase session
});

const { user } = await api.identity.getMe({});
```

- `getAccessToken` is called before **every** request. When it returns a token, the client sends `Authorization: Bearer <token>`.
- The transport is Connect over HTTP/1.1 with **JSON** encoding (`useBinaryFormat: false`). JSON works with React Native's `fetch` and is readable in network inspectors.
- Errors are thrown as `ConnectError`, with `.code` (e.g. `Code.Unauthenticated`).

## Adding a service

1. Add the `.proto` under `proto/issho/v1/` and run `make gen`.
2. Import the generated service in `src/index.ts`, re-export its types and add it to the object returned by `createApi`.

## Checks

`pnpm typecheck` (root) or `pnpm --filter @issho/api-client typecheck`.
