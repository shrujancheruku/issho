# proto

The API's contract. **These files are the source of truth** for the Go server stubs (`backend/gen/issho`) and the TypeScript client (`packages/api-client/src/gen`). Both are generated: never edit them.

- Package `issho.v1`, in `issho/v1/`. One file per service (`health.proto` holds `HealthService`).
- `access.proto` defines the `Access` enum and the `access` method option every RPC must set.
- Config: `../buf.yaml` (lint `STANDARD`, breaking `FILE`) and `../buf.gen.yaml` (the generators). `make gen` needs `make setup` first, for the TypeScript plugin.

## Every RPC declares who may call it

```proto
rpc Check(CheckRequest) returns (CheckResponse) {
  option (issho.v1.access) = ACCESS_PUBLIC;
}
```

| Rule | Who may call |
|---|---|
| `ACCESS_PUBLIC` | Anyone, signed in or not |
| `ACCESS_MEMBER` | Any signed-in person |
| `ACCESS_MAKING` | The owner and the insiders of the city (the Making tab) |
| `ACCESS_ADMIN` | Staff in the admin, who have also taken the second sign-in step |

The API enforces the rule (`backend/internal/rpc`, an interceptor that reads the option) and denies by default: an RPC with no rule, or `ACCESS_UNSPECIFIED`, is refused for everyone. `make rpc-auth` (part of `make lint`) fails and names any RPC that has none, so it never reaches that point.

## Adding an RPC

1. Add the `rpc` to a service in `issho/v1/<area>.proto`, or start a new `<area>.proto` with a `<Area>Service`. Name its messages `<Rpc>Request` and `<Rpc>Response` (buf lint requires it).
2. Set its `option (issho.v1.access) = ...;` from the table above, and `import "issho/v1/access.proto";` if the file doesn't yet.
3. Comment the service, the RPC and each field: the comments become the generated code's docs. For an RPC, say what it does and why that rule fits.
4. `make gen`, then commit what it wrote. CI fails if `make gen` leaves a diff.
5. Implement the service in `backend/internal/rpc` and mount it in `NewHandler`, which puts it behind the interceptor. Add it to `createApi` in `packages/api-client/src/index.ts`.
6. Test it through the Go client over HTTP (see `health_test.go`), including who is refused.

Field names are `snake_case` in proto and `camelCase` in JSON and TypeScript. Fields are never renumbered or reused: retire one with `reserved`.

## Checks

```sh
make lint            # buf lint, and rpc-auth
make rpc-auth        # just the access-rule check
make proto-breaking  # buf breaking against main
```

`make proto-breaking` is not in `make lint` yet: `main` still holds v1's `proto/`, so every v1 file reads as deleted. See #595's resolution comment.
