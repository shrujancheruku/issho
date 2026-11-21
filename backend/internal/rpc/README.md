# rpc

The API's ConnectRPC surface. `NewHandler(Config)` serves every service behind the access interceptor (`access.go`), which enforces the `access` option each RPC declares in its proto. `Config` carries the build's version, the `Authenticator` and the logger.

**Request log (`requestlog.go`).** With `Config.Log` set, the handler logs one `request` line per request: `route` (the RPC's procedure, `/issho.v1.HealthService/Check`, or `unmatched` for a path no service owns), `status`, `duration_ms` (read from `Config.Clock`) and `account` (the caller's ID) when the caller sent a good token. The line is built from those fields only, never from the request, so no IP address, email, body, header or token reaches it. `Config.Clock` is required when `Log` is set; with `Log` nil nothing is logged.

Tests go through HTTP: `access_test.go` (the interceptor), `health_test.go`, `requestlog_test.go` (the request line, including that an email in the body and a forwarded-for header never appear). `rpcauth/` is the check behind `make rpc-auth`.
