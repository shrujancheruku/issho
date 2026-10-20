package errs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
)

func TestInterceptor(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	run := func(handlerErr error) error {
		next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) { return nil, handlerErr }
		_, err := Interceptor(log)(next)(context.Background(), connect.NewRequest(&struct{}{}))
		return err
	}

	err := run(errors.New(`insert violates foreign key constraint "users_auth_fk" (SQLSTATE 23503)`))
	if connect.CodeOf(err) != connect.CodeInternal || err.Error() != "internal: internal error" {
		t.Fatalf("raw error leaked: %v", err)
	}

	want := connect.NewError(connect.CodeInvalidArgument, errors.New("city_id is required"))
	if err := run(want); err != want {
		t.Fatalf("connect error changed: %v", err)
	}

	if err := run(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
