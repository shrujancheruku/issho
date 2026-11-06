// Package validate enforces the buf.validate rules written on the request
// messages in proto/. Those annotations are the one place a field limit is
// defined: the server enforces them here, and the app reads the same numbers
// from the generated descriptors (@issho/api-client's maxLen).
//
// A violation is returned as errs.Invalid ("field: what's allowed"), so it has
// the same shape as every other bad-input error and errs.Interceptor turns it
// into InvalidArgument.
package validate

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"

	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

// Interceptor validates every request message before its handler runs. Put it
// inside auth (after it in the chain) so a caller who is not signed in hears
// Unauthenticated rather than a hint about the request's rules. Rules are
// compiled once per message type, on first use.
func Interceptor() connect.ServerInterceptor {
	v, err := protovalidate.New()
	if err != nil {
		// Only bad options fail here, and there are none: a bug caught at startup.
		panic("validate: " + err.Error())
	}
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			return next(ctx, spec, checkedStream{ServerStream: stream, v: v})
		}
	}
}

// checkedStream validates each request message as it is read. A generated
// handler reads its request before calling ours, so a bad one never reaches it.
type checkedStream struct {
	connect.ServerStream
	v protovalidate.Validator
}

func (s checkedStream) Receive(msg any) error {
	if err := s.ServerStream.Receive(msg); err != nil {
		return err
	}
	if m, ok := msg.(proto.Message); ok {
		return Check(s.v, m)
	}
	return nil
}

// Check validates msg and returns the first violation as errs.Invalid, or nil.
func Check(v protovalidate.Validator, msg proto.Message) error {
	err := v.Validate(msg)
	if err == nil {
		return nil
	}
	var ve *protovalidate.ValidationError
	if errors.As(err, &ve) && len(ve.Violations) > 0 {
		p := ve.Violations[0].Proto
		return errs.Invalid("%s: %s", protovalidate.FieldPathString(p.GetField()), p.GetMessage())
	}
	// The rules themselves could not run: a bug on our side, not the caller's.
	return err
}
