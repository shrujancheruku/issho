# proto

The API contract. **These files are the source of truth** for the Go server, the TypeScript client and the [API reference](../docs/generated/api.md).

- Package `issho.v1`, one file per service (`identity.proto` is `IdentityService`, and so on).
- buf module config: `buf.yaml` (lint: `STANDARD`, breaking: `FILE`). Generation config: `../buf.gen.yaml`.

## Workflow

```sh
make gen                      # from root: Go (backend/gen), TS (packages/api-client/src/gen), docs (docs/generated/api.md)
go tool buf lint proto        # also part of `make lint`
```

## Rules

- **Comment every service, RPC and field.** The comments are published as the API docs. For each RPC, state its auth requirement and HTTP path.
- Field names use `snake_case` in proto and appear as `camelCase` in JSON and TypeScript.
- **No breaking changes** once a field ships in an app build. CI runs `buf breaking` against `main` on every PR. To retire a field, deprecate it and keep the number reserved (`reserved 5;`). Never reuse field numbers.
- **Lists that can grow page with [AIP-158](https://google.aip.dev/158):** the request has `int32 page_size` and `string page_token`, the response `string next_page_token` (empty on the last page). The server treats `page_size` 0 as 50 and clamps it to 200 (`platform/paging`). The token is opaque and means "the page after this row": a keyset cursor (sort key plus id), never an offset, tied to the list and its filters, so a token from another list or filter is `INVALID_ARGUMENT`. Keep a `total` only where a screen shows a count. A list that is bounded on purpose ("the latest 50", "the best 20") says so in its comment instead of paging. `ListMessages.after_id` in chat is a sync cursor, not paging.
- New RPCs are authenticated by default (see `backend/cmd/api/main.go`).
