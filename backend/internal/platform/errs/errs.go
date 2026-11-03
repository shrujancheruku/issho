// Package errs is the one error shape on the server. Services and handlers
// return plain Go errors built here (Invalid, NotFound, Precondition, Denied,
// Upstream), never *connect.Error; Interceptor turns each kind into its
// Connect code, attaches a machine-readable reason when there is one, and
// keeps internal details on the server.
package errs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/sentry"
)

// Kind decides the Connect code.
type Kind int

const (
	KindInvalid         Kind = iota + 1 // InvalidArgument
	KindNotFound                        // NotFound
	KindPrecondition                    // FailedPrecondition
	KindDenied                          // PermissionDenied
	KindUnauthenticated                 // Unauthenticated
	KindUpstream                        // Unavailable, reported to Sentry
)

var codes = map[Kind]connect.Code{
	KindInvalid:         connect.CodeInvalidArgument,
	KindNotFound:        connect.CodeNotFound,
	KindPrecondition:    connect.CodeFailedPrecondition,
	KindDenied:          connect.CodePermissionDenied,
	KindUnauthenticated: connect.CodeUnauthenticated,
	KindUpstream:        connect.CodeUnavailable,
}

// Error is an error with a kind, a message for the caller (developer-facing
// English; user copy comes from the app's i18n, keyed on Reason), an optional
// Reason and an optional cause that stays on the server.
type Error struct {
	Kind    Kind
	Reason  isshov1.ErrorReason
	Service string // Upstream only: google, gemini, github, translate
	msg     string
	cause   error
}

func (e *Error) Error() string { return e.msg }
func (e *Error) Unwrap() error { return e.cause }

// As lets connect.CodeOf and errors.As(&*connect.Error) see the wire form, so
// code that inspects a handler's error (tests, wrappers) gets the same code
// and message the client will.
func (e *Error) As(target any) bool {
	if t, ok := target.(**connect.Error); ok {
		*t = e.connectError()
		return true
	}
	return false
}

func (e *Error) connectError() *connect.Error {
	code, ok := codes[e.Kind]
	if !ok {
		code = connect.CodeInternal // a zero or unknown Kind is a bug, not Code(0)
	}
	ce := connect.NewError(code, errors.New(e.msg))
	if e.Reason != isshov1.ErrorReason_ERROR_REASON_UNSPECIFIED {
		if d, err := connect.NewErrorDetail(&isshov1.ErrorReasonDetail{Reason: e.Reason}); err == nil {
			ce.AddDetail(d)
		}
	}
	return ce
}

// report is swapped in tests.
var report = sentry.Report

// Interceptor is the one place an error becomes a Connect error. In order:
//
//  1. An *Error (built by Invalid, NotFound, Precondition, Denied, Forbidden,
//     Unauthenticated or Upstream) becomes its kind's code, with the reason as
//     a detail. Upstream errors are also logged and reported to Sentry, tagged
//     with service and procedure; client errors are not reported. An *Error
//     wrapped inside connect.NewError takes the inner kind, not the outer code.
//  2. Any other *connect.Error passes through unchanged.
//  3. Anything else is logged and reported, and becomes CodeInternal "internal
//     error".
//
// Without step 3, connect-go sends err.Error() to the client as an "unknown"
// error, leaking database messages (table and constraint names, SQLSTATEs) and
// upstream responses to anyone calling a public API. Handlers and services
// should return errs constructors, not connect.NewError.
func Interceptor(log *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err == nil {
				return resp, nil
			}
			// Ours first: *Error also answers errors.As for *connect.Error.
			var e *Error
			if errors.As(err, &e) {
				if e.Kind == KindUpstream {
					tags := map[string]string{"procedure": req.Spec().Procedure, "service": e.Service}
					log.ErrorContext(ctx, "upstream error", "procedure", tags["procedure"], "service", e.Service, "err", e.cause)
					report(ctx, e.cause, tags)
				}
				return nil, e.connectError()
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

// Invalid is the error for a request a handler refuses: InvalidArgument
// with a message for the caller, conventionally "field: what's allowed".
func Invalid(format string, a ...any) error {
	return &Error{Kind: KindInvalid, msg: fmt.Sprintf(format, a...)}
}

// NotFound is for something the caller asked about that doesn't exist (or
// that they may not know exists).
func NotFound(format string, a ...any) error {
	return &Error{Kind: KindNotFound, msg: fmt.Sprintf(format, a...)}
}

// Precondition is for a call that's right but not now: FailedPrecondition,
// with a reason when a screen words it differently.
func Precondition(reason isshov1.ErrorReason, format string, a ...any) error {
	return &Error{Kind: KindPrecondition, Reason: reason, msg: fmt.Sprintf(format, a...)}
}

// Denied is for a caller who may not do this: PermissionDenied.
func Denied(reason isshov1.ErrorReason, format string, a ...any) error {
	return &Error{Kind: KindDenied, Reason: reason, msg: fmt.Sprintf(format, a...)}
}

// Forbidden is Denied without a reason.
func Forbidden(format string, a ...any) error {
	return &Error{Kind: KindDenied, msg: fmt.Sprintf(format, a...)}
}

// Unauthenticated is for a missing or bad token.
func Unauthenticated(format string, a ...any) error {
	return &Error{Kind: KindUnauthenticated, msg: fmt.Sprintf(format, a...)}
}

// Upstream wraps a failure of an outside service (google, gemini, github,
// translate). The client sees Unavailable and a fixed message; err goes to
// logs and Sentry only.
func Upstream(service string, err error) error {
	return &Error{Kind: KindUpstream, Service: service, cause: err, msg: service + " is unavailable; try again shortly"}
}

// WithCause keeps a lower-level error reachable with errors.Is/As (sentinels)
// when a kind is built from one. The message is the kind's own.
func WithCause(err, cause error) error {
	if e, ok := err.(*Error); ok {
		c := *e
		c.cause = cause
		return &c
	}
	return err
}
