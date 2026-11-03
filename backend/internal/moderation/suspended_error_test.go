package moderation

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
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
	setCaller := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			return next(auth.WithCaller(ctx, auth.Caller{UserID: id}), req)
		}
	})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	mux.Handle(isshov1connect.NewPushServiceHandler(isshov1connect.UnimplementedPushServiceHandler{},
		connect.WithInterceptors(errs.Interceptor(log), setCaller, Gate(s))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := isshov1connect.NewPushServiceClient(srv.Client(), srv.URL).
		RegisterPushToken(context.Background(), connect.NewRequest(&isshov1.RegisterPushTokenRequest{}))
	ce, ok := err.(*connect.Error)
	if !ok || ce.Code() != connect.CodePermissionDenied || ce.Message() != "account suspended" {
		t.Fatalf("got %v", err)
	}
	if len(ce.Details()) != 1 {
		t.Fatalf("details = %d, want 1", len(ce.Details()))
	}
	m, verr := ce.Details()[0].Value()
	if rd, isRD := m.(*isshov1.ErrorReasonDetail); verr != nil || !isRD || rd.Reason != isshov1.ErrorReason_ERROR_REASON_SUSPENDED {
		t.Fatalf("detail = %v, %v", m, verr)
	}
}
