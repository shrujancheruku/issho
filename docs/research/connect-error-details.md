# Connect error details and how installed builds degrade

Research for [#142](https://github.com/shrujancheruku/issho/issues/142) (parent [#140](https://github.com/shrujancheruku/issho/issues/140), blocks [#145](https://github.com/shrujancheruku/issho/issues/145)). Checked 2026-11-02.

**Question:** how should the API send machine-readable error reasons, so the app stops matching on codes and message text? And how do builds that are already installed behave once it does?

## Recommendation (short version)

1. Add one Issho-owned detail message with an **enum** of reasons, in `proto/issho/v1/errors.proto`: `ErrorReasonDetail { ErrorReason reason = 1; }`. Don't use `google.rpc.ErrorInfo` for now (reasoning below).
2. The server attaches it **in addition to** today's code and message. It **keeps every code and message that installed builds match on unchanged**, `PermissionDenied` + `"account suspended"` above all.
3. `@issho/api-client` exports one helper, `errorReason(err)`. It returns the enum value or `undefined`. Screens switch on the reason and fall back to today's code check when there is no detail.
4. Order: ship the server first (the change is additive and invisible to old builds), then the app.

## What the repo uses today

| Piece | Version | Source |
|---|---|---|
| connect-go | `connectrpc.com/connect` **v1.21.1** (direct). `connectrpc.com/connect/v2 v2.0.1` is only an indirect dependency, pulled in by the `buf` CLI tool (`go mod why -m connectrpc.com/connect/v2` → `github.com/bufbuild/buf/cmd/buf`). | `backend/go.mod` |
| connect-es | `@connectrpc/connect` and `@connectrpc/connect-web` **^2.2.0** (2.2.0 installed) | `packages/api-client/package.json`, `pnpm-lock.yaml` |
| protobuf-es | `@bufbuild/protobuf` / `protoc-gen-es` **^2.16.0** | same, plus `apps/admin/package.json` |
| Transport | Connect protocol, **JSON** (`createConnectTransport({ useBinaryFormat: false })`) | `packages/api-client/src/index.ts` |
| buf | `buf.yaml` v2, lint `STANDARD`, breaking `FILE`. No deps, so googleapis isn't available. | `proto/buf.yaml`, `buf.gen.yaml` |
| OTA | `expo-updates`, `runtimeVersion.policy: fingerprint` | `apps/mobile/app.json` |

**How errors are produced.** Handlers return `connect.NewError(code, err)`. `errs.Interceptor` (`backend/internal/platform/errs/errs.go`) passes any `*connect.Error` through untouched, details included, and turns everything else into `CodeInternal "internal error"`. Several sentinels share one code. For example, `plans/events_handler.go` `eventError` maps `ErrNoProfile`, `ErrEventOver`, `ErrTooLate` and `ErrNotNow` (plus `ErrCheckInClosed` and `ErrFeedbackClosed`) to `FailedPrecondition`. `moderation/suspensions.go:63` defines `ErrSuspended = connect.NewError(CodePermissionDenied, errors.New("account suspended"))`.

**How errors are consumed.** The mobile app matches only on `code`: `event/[id].tsx:153` and `crew/[id].tsx:166` read `FailedPrecondition` as "too late". `feedback/[id].tsx`, `report/[id].tsx`, `suggest-activities.tsx`, `save-error.ts`, `query-screen.tsx` and `chat/room.ts` do the same. There is one message match: `features/auth/auth-provider.tsx:119` checks `err.code === Code.PermissionDenied && err.rawMessage === 'account suspended'`. The admin SPA just shows `ConnectError.from(e).message`. Nothing calls `findDetails` today.

## 1. How details are encoded

### Connect protocol (what Issho uses)

- **Unary errors are always JSON**, whatever codec the RPC uses. The spec says the Unary-Error content type "must be application/json" ([Connect protocol: unary error](https://connectrpc.com/docs/protocol#unary-response)). connect-go v1 does this too: `// In unary Connect, errors always use application/json.` (`protocol_connect.go`, `connectUnaryHandlerConn.Close`). So for errors, "binary vs JSON Connect" makes no difference: the envelope is always JSON.
- Shape: `{"code": "failed_precondition", "message": "...", "details": [{"type": "issho.v1.ErrorReasonDetail", "value": "<unpadded base64 of the protobuf binary>", "debug": {...}}]}`. `type` is the fully qualified message name, with no `type.googleapis.com/` prefix. `value` is always **protobuf binary**, even on a JSON transport. `debug` is optional, and "Clients must not depend on data in the 'debug' key" ([protocol spec](https://connectrpc.com/docs/protocol#error-end-stream)).
- connect-go v1 writes `debug` (the detail as protojson) when it can, and drops it silently otherwise (`connectWireDetail.MarshalJSON`, [protocol_connect.go](https://github.com/connectrpc/connect-go/blob/v1.21.1/protocol_connect.go)). One edge case: if the whole error JSON is bigger than `sendMaxBytes`, connect-go **drops all details** and sends only code and message. Issho doesn't set `WithSendMaxBytes`, so this doesn't apply today.
- Streaming RPCs carry the same `Error` object in the end-stream message's `"error"` key, sent with HTTP 200 ([protocol spec, end-stream](https://connectrpc.com/docs/protocol#error-end-stream)).

### gRPC / gRPC-Web (not used by Issho's clients, for completeness)

The same `*connect.Error` is sent as a binary `google.rpc.Status` in the `grpc-status-details-bin` trailer, with each detail packed as a `google.protobuf.Any` (connect-go `protocol_grpc.go`). The handler code doesn't change; connect-go handles all three protocols from the same error value.

### connect-go API

- **v1.21.1 (current):** `connect.NewErrorDetail(msg proto.Message) (*ErrorDetail, error)` wraps the message in an `Any`, and `(*Error).AddDetail(d)` **mutates** the error. To read a detail: `ErrorDetail.Type()` gives the name and `.Value()` decodes it through the global registry ([error.go v1.21.1](https://github.com/connectrpc/connect-go/blob/v1.21.1/error.go), [pkg.go.dev](https://pkg.go.dev/connectrpc.com/connect@v1.21.1#NewErrorDetail)).
- **v2 (v2.0.1, for when the backend moves):** the API is different. `connect.NewError(code, message string)` takes a string message, not an `error`, and there's `connect.Errorf`. Details are plain structs `{Type, Value, Debug}` built with `connectproto.NewErrorDetail(msg)`. They're attached with `(*Error).WithDetail(d)`, which returns a **clone**, and decoded with `connectproto.UnmarshalErrorDetail` (`connect.go` and `connectproto/errordetail.go` in [connect-go v2](https://pkg.go.dev/connectrpc.com/connect/v2)). The wire format is the same, so this is a server-only refactor and clients don't notice.

## 2. How connect-es v2 reads them

- `errorFromJson` (`@connectrpc/connect` 2.2.0, `dist/esm/protocol-connect/error-json.js`) parses each detail into `{type, value: base64Decode(value), debug}`. It does **not** decode them; it keeps raw bytes on `err.details`. One strictness point: if any detail is missing a string `type` or `value`, the **whole error** is replaced by the fallback error, which is built from the HTTP status. connect-go always writes both, so this only matters for hand-written error bodies, such as a proxy or a test fixture.
- `ConnectError.findDetails(schemaOrRegistry)` (`connect-error.js`) goes through `err.details`. For each entry whose `type` the given schema or registry knows, it runs `fromBinary(desc, value)`. **Unknown types are skipped, and a detail that fails to parse is silently ignored** (the source comment calls this "the least worst behavior"). It returns an array, which is empty when nothing matches ([Connect for Web: errors](https://connectrpc.com/docs/web/errors)).
- "Registered" here only means the caller passes the generated `…Schema` (or a `createRegistry(...)`). There's no global registry. Because `ErrorReasonDetailSchema` would be generated into `packages/api-client/src/gen` like every other Issho message, the app always has it. With `google.rpc.ErrorInfo` the client would need googleapis generated as well (`buf generate buf.build/googleapis/googleapis`, per the connect-es docs).
- Errors built in tests with `new ConnectError(msg, code, undefined, [{ desc: ErrorReasonDetailSchema, value: { reason } }])` also work with `findDetails` (the `"desc" in data` branch), so the existing Jest mocks can carry reasons.
- Proto3 enums are **open**. When the client decodes binary carrying an enum number it doesn't know, the number is kept as is ([protobuf.dev: enum behavior](https://protobuf.dev/programming-guides/enum/)). Since detail values are always binary (see above), an old client that does read the detail but predates a new reason gets a plain number. A `switch` falls through to `default`, and nothing throws.

## 3. What older installed builds see

Every build installed today has no `findDetails` call, so:

- **Adding a detail changes nothing for them.** `code`, `rawMessage` and `message` are unchanged; `details` gets one more entry that nothing reads. The extra `debug` JSON is ignored as well.
- **What breaks old builds is changing the code or the message they match on.** These are the contracts to freeze until the oldest supported build has the reason-aware client:
  - `PermissionDenied` + `rawMessage === "account suspended"`: changing that text silently turns suspension into a generic error in the app (`auth-provider.tsx:119`).
  - `FailedPrecondition` from `RespondToEvent` / crew respond, which shows "too late". If a reason were moved to a different code, old builds would show the network/generic error instead.
  - `NotFound`, `PermissionDenied` and `InvalidArgument`, as used by `query-screen`, `chat/room`, `feedback` and `save-error`.
- AIP-193 says the same: once clients may parse `Status.message`, it "is implicitly part of the API contract, so must not be updated". Only an RPC that has always returned a machine-readable reason may change its message freely ([AIP-193](https://google.aip.dev/193)).
- **The new app against an older server.** expo-updates can ship new JS (including `@issho/api-client`) to installed binaries with the same fingerprint. There can also be a window where the app is newer than the API. So the new client must treat "no detail" as "fall back to the code check", never as "unknown error". `errorReason()` returning `undefined` covers this.
- **A newer server against a reason-aware older app.** The app gets an enum number it doesn't know (open enum), so the screen hits its `default` branch. That branch should be today's code-based behaviour, so adding reasons later never needs an app release first.

## 4. Naming reasons, and the alternatives

### `google.rpc.ErrorInfo` (`reason` + `domain` + `metadata`)

- `reason`: "a constant value that identifies the proximate cause of the error", UPPER_SNAKE_CASE, regex `[A-Z][A-Z0-9_]+[A-Z0-9]`, at most 63 characters. `domain`: a globally unique logical grouping, usually the service name (e.g. `pubsub.googleapis.com`). `metadata` keys are lowerCamelCase, at most 64 characters, with units in the key (`instanceLimitPerRequest`) ([error_details.proto](https://github.com/googleapis/googleapis/blob/master/google/rpc/error_details.proto)).
- Contract: "Services must use the same (reason, domain) pair for the same error" and must not reuse a pair for a logically different error. Metadata keys may be added but never removed ([AIP-193](https://google.aip.dev/193)). AIP-193 also says every error should carry an ErrorInfo. That's Google's API policy, not a Connect requirement.
- Good fit when third parties consume the API, or for interop with tools that understand `google.rpc`.
- Costs for Issho: reasons are **free-form strings**. That means no generated TS constants, no exhaustiveness checks and no `buf breaking` protection. It also needs a googleapis dependency in `proto/buf.yaml` plus generated TS for `google/rpc/error_details.proto` (Go already has `google.golang.org/genproto/googleapis/rpc` in `go.sum`, indirectly).

### A custom message holding an enum (recommended)

- Schema lives with the rest of the API. TS gets a real enum. Tests and screens import the values instead of copying strings.
- **buf lint (`STANDARD`)** already enforces the naming: the type is PascalCase (`ENUM_PASCAL_CASE`), values are UPPER_SNAKE_CASE (`ENUM_VALUE_UPPER_SNAKE_CASE`) prefixed with the enum name (`ENUM_VALUE_PREFIX`), and the zero value ends in `_UNSPECIFIED` (`ENUM_ZERO_VALUE_SUFFIX`) ([buf lint rules](https://buf.build/docs/lint/rules/)). So the values read `ERROR_REASON_ACCOUNT_SUSPENDED`, the same UPPER_SNAKE style as AIP-193 reasons.
- **buf breaking (`FILE`)** protects the contract: `ENUM_VALUE_NO_DELETE` (deprecate a value, don't delete it), `ENUM_VALUE_SAME_NAME` (a number can't be renamed) and `MESSAGE_NO_DELETE` ([buf breaking rules](https://buf.build/docs/breaking/rules/)). With ErrorInfo strings, buf can't see any of this; it has no rules about error payloads, and string constants in Go code are outside its scope.
- Neither buf lint nor buf breaking has any rule about error details or error messages as such.
- Name reasons after the **cause**, not the UI copy: `EVENT_OVER`, `RESPONSE_DEADLINE_PASSED`, not `SHOW_TOO_LATE`. This matches AIP-193's "proximate cause". It also lets one reason drive different copy on different screens.

**Why not both?** Attaching ErrorInfo next to the enum detail costs a googleapis dependency and buys nothing until something outside Issho reads the errors. If that day comes, the enum's value names can be used as the ErrorInfo `reason` (strip the `ERROR_REASON_` prefix) with domain `issho.app`, purely additively.

## 5. Proposed scheme for Issho

### Proto

```proto
// proto/issho/v1/errors.proto
syntax = "proto3";
package issho.v1;

// Why a call failed, for clients to branch on. It travels as a Connect error
// detail next to the code and message, which stay as they are for installed builds.
// Values name the cause, not the UI copy. Never delete or renumber: deprecate.
enum ErrorReason {
  ERROR_REASON_UNSPECIFIED = 0;
  ERROR_REASON_ACCOUNT_SUSPENDED = 1;    // PermissionDenied
  ERROR_REASON_PROFILE_REQUIRED = 2;     // plans.ErrNoProfile
  ERROR_REASON_EVENT_OVER = 3;           // plans.ErrEventOver
  ERROR_REASON_RESPONSE_DEADLINE_PASSED = 4; // plans.ErrTooLate
  ERROR_REASON_ANSWER_NOT_APPLICABLE = 5;    // plans.ErrNotNow
  ERROR_REASON_CHECK_IN_CLOSED = 6;
  ERROR_REASON_FEEDBACK_CLOSED = 7;
  // … crews.ErrOver, chat.ErrClosed, etc. as screens need them
}

message ErrorReasonDetail {
  ErrorReason reason = 1;
}
```

### Go (connect-go v1.21.1)

```go
// backend/internal/platform/errs
// WithReason is a *connect.Error that also carries an ErrorReasonDetail. code and
// msg must stay what installed builds already match on.
func WithReason(code connect.Code, reason isshov1.ErrorReason, err error) *connect.Error {
	ce := connect.NewError(code, err)
	if d, derr := connect.NewErrorDetail(&isshov1.ErrorReasonDetail{Reason: reason}); derr == nil {
		ce.AddDetail(d) // only fails if marshalling fails; the code still goes out
	}
	return ce
}

// plans/events_handler.go
case errors.Is(err, ErrTooLate):
	return errs.WithReason(connect.CodeFailedPrecondition, isshov1.ErrorReason_ERROR_REASON_RESPONSE_DEADLINE_PASSED, err)

// moderation/suspensions.go: same code, same message, plus the reason
var ErrSuspended = errs.WithReason(connect.CodePermissionDenied,
	isshov1.ErrorReason_ERROR_REASON_ACCOUNT_SUSPENDED, errors.New("account suspended"))
```

`ErrSuspended` is a shared package-level value. Build it once and never call `AddDetail` on it afterwards. On v2, `WithDetail` returns a clone, so this pitfall goes away.

### TypeScript (`@issho/api-client`)

```ts
import { ConnectError } from "@connectrpc/connect";
import { ErrorReason, ErrorReasonDetailSchema } from "./gen/issho/v1/errors_pb";

/** The server's reason for err, or undefined (not a ConnectError, an older server, or no reason sent). */
export function errorReason(err: unknown): ErrorReason | undefined {
  if (!(err instanceof ConnectError)) return undefined;
  const reason = err.findDetails(ErrorReasonDetailSchema)[0]?.reason;
  return reason === ErrorReason.UNSPECIFIED ? undefined : reason;
}
```

```ts
// auth-provider.tsx: the reason first, the old message match as the fallback for an older server
const suspended = errorReason(err) === ErrorReason.ACCOUNT_SUSPENDED
  || (err instanceof ConnectError && err.code === Code.PermissionDenied && err.rawMessage === "account suspended");
```

### Testing

- Go: a table test in `errs` that round-trips each reason through a real Connect handler (`httptest`), checking the code, the message and `findDetails`-equivalent decoding. A test in `moderation` that pins `ErrSuspended`'s message to `"account suspended"`, so the installed-build contract can't drift by accident.
- TS: Jest mocks build `new ConnectError(msg, code, undefined, [{ desc: ErrorReasonDetailSchema, value: { reason } }])`. Also test the no-detail fallback path.

## Decisions for the owner

1. **Custom enum vs `google.rpc.ErrorInfo`.** Recommended: the custom enum. Choose ErrorInfo only if you expect third-party API consumers.
2. **Oldest supported build.** Code and message matches have to stay frozen until every installed binary has the reason-aware JS. Is OTA enough, or is there a minimum-version gate? This decides when the `rawMessage === "account suspended"` fallback can go.
3. **Scope of the first reasons.** All sentinels now, or only the ones a screen branches on (suspension, the respond/check-in/feedback "too late" family)? Recommended: only the ones a screen branches on. Adding a reason is cheap and safe later.
4. **Message text.** Are server messages developer-facing English (as AIP-193 says), with all user copy from i18n keyed on reason? The admin SPA shows `message` directly today.
5. **connect-go v2 migration.** Do it separately. It changes the Go API (`NewError(code, string)`, `WithDetail` clone), not the wire.

## Sources

- Connect protocol spec: https://connectrpc.com/docs/protocol
- Connect for Web, errors and `findDetails`: https://connectrpc.com/docs/web/errors
- connect-go v1.21.1 `error.go`, `protocol_connect.go`: https://github.com/connectrpc/connect-go/tree/v1.21.1 (read from the local module cache)
- connect-go v2 `connect.go`, `connectproto/errordetail.go`: https://pkg.go.dev/connectrpc.com/connect/v2 (v2.0.1, read from the local module cache)
- connect-es 2.2.0 `connect-error.js`, `protocol-connect/error-json.js`: https://github.com/connectrpc/connect-es/tree/v2.2.0/packages/connect/src (read from the installed package)
- AIP-193 Errors: https://google.aip.dev/193
- `google/rpc/error_details.proto`: https://github.com/googleapis/googleapis/blob/master/google/rpc/error_details.proto
- Protobuf enum behavior: https://protobuf.dev/programming-guides/enum/
- buf lint rules: https://buf.build/docs/lint/rules/
- buf breaking rules: https://buf.build/docs/breaking/rules/
