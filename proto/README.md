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
- **Lists that can grow page with [AIP-158](https://google.aip.dev/158):** the request has `int32 page_size` and `string page_token`, the response `string next_page_token` (empty on the last page). The server treats `page_size` 0 as 50 and clamps it to 200 (`platform/paging`). The token is opaque and means "the page after this row": a keyset cursor (sort key plus id), never an offset, tied to the list and its filters, so a token from another list or filter is `INVALID_ARGUMENT`. Keep a `total` only where a screen shows a count. A list that is bounded on purpose ("the latest 50", "the best 20") says so in its comment instead of paging. The app's own unpaged lists are bounded this way (`ListFeed` 200, `ListMyEvents` 200, `ListMyCrews` 50): the query has a hard `limit` (`platform/bound`), which logs a warning and reports to Sentry once per process when hit. If one ever gets paging, `page_size = 0` (what installed builds send) must keep returning the whole bounded list, so adding the fields isn't breaking. `ListMessages.after_id` in chat is a sync cursor, not paging.
- **Request-field limits are `buf.validate` rules** ([protovalidate](https://protovalidate.com), the `buf.build/bufbuild/protovalidate` dep in `buf.yaml`, locked in `buf.lock`): `string body = 2 [(buf.validate.field).string = {min_len: 1, max_len: 1000}];`. Add `import "buf/validate/validate.proto";`. The server enforces them (`backend/internal/platform/validate`, an `InvalidArgument` in the `errs.Invalid` shape) and the app reads them (`maxLen(schema, field)` in `@issho/api-client`), so a limit is written once, here, and the comment says it in words. `max_len` counts characters. Adding a rule doesn't change the wire format, so `buf breaking` passes. Tightening one is a behaviour change: older apps may send what the server now refuses.
- New RPCs are authenticated by default (see `backend/cmd/api/main.go`).
