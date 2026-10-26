// Package errs keeps internal error details on the server.
package errs

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/shrujancheruku/issho/backend/internal/platform/sentry"
)

// Interceptor turns any error that isn't already a *connect.Error into a
// generic CodeInternal error, and logs the original with the procedure name.
//
// Without it, connect-go sends err.Error() to the client as an "unknown" error.
// That leaks database messages (table and constraint names, SQLSTATEs) and
// upstream responses to anyone calling a public API. Handlers that want callers
// to see a message must return connect.NewError with an explicit code.
func Interceptor(log *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err == nil {
				return resp, nil
			}
			var ce *connect.Error
			if errors.As(err, &ce) {
				return resp, err
			}
			log.ErrorContext(ctx, "unhandled error", "procedure", req.Spec().Procedure, "err", err)
			sentry.Report(ctx, err, map[string]string{"procedure": req.Spec().Procedure})
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
	}
}
