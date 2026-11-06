package moderation

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/connect/v2/connectproto"
	"github.com/google/uuid"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

// Installed builds match PermissionDenied + "account suspended"; the reason
// detail is additive. Run behind the real errs.Interceptor, as in cmd/api.
func TestSuspendedErrorOnTheWire(t *testing.T) {
	id := uuid.New()
	s := &Suspensions{Q: &fakeSuspended{ids: []uuid.UUID{id}}}
	setCaller := connect.ServerInterceptor(func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			return next(auth.WithCaller(ctx, auth.Caller{UserID: id}), spec, stream)
		}
	})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	server := connect.NewServer(errs.Interceptor(log), setCaller, Gate(s))
	isshov1connect.RegisterPushServiceHandler(server, isshov1connect.UnimplementedPushServiceHandler{})
	connecthttp.Mount(mux, server)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := isshov1connect.NewPushServiceClient(connect.NewClient(connecthttp.NewTransport(srv.Client(), srv.URL))).
		RegisterPushToken(context.Background(), &isshov1.RegisterPushTokenRequest{})
	var ce *connect.Error
	ok := errors.As(err, &ce)
	if !ok || ce.Code() != connect.CodePermissionDenied || ce.Message() != "account suspended" {
		t.Fatalf("got %v", err)
	}
	if len(ce.Details()) != 1 {
		t.Fatalf("details = %d, want 1", len(ce.Details()))
	}
	m, verr := connectproto.UnmarshalErrorDetail(ce.Details()[0])
	if rd, isRD := m.(*isshov1.ErrorReasonDetail); verr != nil || !isRD || rd.Reason != isshov1.ErrorReason_ERROR_REASON_SUSPENDED {
		t.Fatalf("detail = %v, %v", m, verr)
	}
}
