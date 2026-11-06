// Package deadline bounds how long a unary RPC may run.
package deadline

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect/v2"
)

// Default is the deadline of a unary RPC, unless the caller's is shorter.
const Default = 30 * time.Second

// Interceptor gives each unary RPC's context a deadline of def, or of the
// procedure's entry in overrides, where 0 means "no deadline of ours" (the
// caller's still applies, and a client that goes away still cancels the
// context). Run it inside errs.Interceptor: a handler cut off by the
// deadline is answered DeadlineExceeded here, which errs passes through
// untouched, rather than as an "internal error".
//
// Streaming RPCs and /ws/chat (a plain HTTP handler) are not unary and are
// not touched.
func Interceptor(def time.Duration, overrides map[string]time.Duration) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			// v2 interceptors see every RPC type; keep streams out, as the doc says.
			if spec.StreamType != connect.StreamTypeUnary {
				return next(ctx, spec, stream)
			}
			d := def
			if o, ok := overrides[spec.Procedure]; ok {
				d = o
			}
			if d <= 0 {
				return next(ctx, spec, stream)
			}
			// WithTimeout keeps the earlier of this and the caller's deadline.
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			err := next(ctx, spec, stream)
			if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
				// An *errs.Error (say Upstream wrapping a deadline) also answers
				// errors.As for *connect.Error and keeps its own kind on purpose.
				// A handler's own Connect error (say NotFound) stays; a bare
				// context error from a database or HTTP call becomes ours.
				var ce *connect.Error
				if !errors.As(err, &ce) {
					return connect.NewError(connect.CodeDeadlineExceeded, "request took too long")
				}
			}
			return err
		}
	}
}
