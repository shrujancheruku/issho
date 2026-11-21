# serve

The lifecycle both commands share: log `started`, run until the signal-bound context ends, log `stopped`.

- `Run(ctx, log)` is for the worker, whose River client runs beside it.
- `HTTP(ctx, log, addr, h)` serves `h` on `addr` and logs the address it bound (`127.0.0.1:0` takes any free port). On shutdown it stops accepting and gives requests in flight 10 seconds to finish. Its `http.Server` sets `ErrorLog` to `logging.ServerErrorLog(log)`, so net/http's own errors, which can name a client's address, go through the helper with none of their text.

Tested through the commands (`cmd/cmd_test.go`) and through the API's panic test (`internal/rpc/panic_test.go`).
