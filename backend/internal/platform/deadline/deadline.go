// Package deadline bounds how long a unary RPC may run.
package deadline

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
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
func Interceptor(def time.Duration, overrides map[string]time.Duration) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			d := def
			if o, ok := overrides[req.Spec().Procedure]; ok {
				d = o
			}
			if d <= 0 {
				return next(ctx, req)
			}
			// WithTimeout keeps the earlier of this and the caller's deadline.
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			resp, err := next(ctx, req)
			if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
				// An *errs.Error (say Upstream wrapping a deadline) also answers
				// errors.As for *connect.Error and keeps its own kind on purpose.
				// A handler's own Connect error (say NotFound) stays; a bare
				// context error from a database or HTTP call becomes ours.
				var ce *connect.Error
				if !errors.As(err, &ce) {
					return nil, connect.NewError(connect.CodeDeadlineExceeded, errors.New("request took too long"))
				}
			}
			return resp, err
		}
	}
}
