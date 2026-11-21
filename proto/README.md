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
make lint        # buf lint, buf breaking against main, and rpc-auth
make rpc-auth    # just the access-rule check
```

`buf breaking` (`scripts/proto-breaking.sh`) compares `proto/` with `origin/main`'s. It runs only once main holds v2's proto, which it tells by `proto/issho/v1/access.proto` (v1 had none): before that, every v2 file would read as a break of v1's, so `make lint` prints "buf breaking not run: ..." and exits 0. It decides from main's content alone, so it switches itself on at the merge of the Foundation milestone and nothing can turn it off; it is also skipped, with the same notice, when `origin/main` isn't fetched. `scripts/proto-breaking.test.sh` (part of `make test`) runs both branches against a throwaway git repo.
