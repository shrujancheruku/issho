package moderation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

type fakeSuspended struct {
	ids   []uuid.UUID
	loads int
}

func (f *fakeSuspended) ListSuspendedIDs(context.Context) ([]uuid.UUID, error) {
	f.loads++
	return f.ids, nil
}

func TestSuspensionsCache(t *testing.T) {
	who := uuid.New()
	q := &fakeSuspended{}
	now := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	s := &Suspensions{Q: q, Now: func() time.Time { return now }}
	check := func(want bool, loads int) {
		t.Helper()
		got, err := s.Suspended(context.Background(), who)
		if err != nil || got != want || q.loads != loads {
			t.Fatalf("Suspended = %v, %v after %d loads; want %v after %d", got, err, q.loads, want, loads)
		}
	}
	check(false, 1)
	q.ids = []uuid.UUID{who}
	check(false, 1) // cached
	now = now.Add(suspensionsTTL)
	check(true, 2) // reloaded when stale
	q.ids = nil
	s.Invalidate()
	check(false, 3) // reloaded at once after a change here
}

func TestGate(t *testing.T) {
	suspended, other := uuid.New(), uuid.New()
	s := &Suspensions{Q: &fakeSuspended{ids: []uuid.UUID{suspended}}}
	var caller *auth.Caller
	setCaller := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if caller != nil {
				ctx = auth.WithCaller(ctx, *caller)
			}
			return next(ctx, req)
		}
	})
	opts := connect.WithInterceptors(setCaller, Gate(s, isshov1connect.IdentityServiceGetMeProcedure))
	mux := http.NewServeMux()
	mux.Handle(isshov1connect.NewIdentityServiceHandler(isshov1connect.UnimplementedIdentityServiceHandler{}, opts))
	mux.Handle(isshov1connect.NewPushServiceHandler(isshov1connect.UnimplementedPushServiceHandler{}, opts))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	identity := isshov1connect.NewIdentityServiceClient(srv.Client(), srv.URL)
	push := isshov1connect.NewPushServiceClient(srv.Client(), srv.URL)
	getMe := func() error {
		_, err := identity.GetMe(context.Background(), connect.NewRequest(&isshov1.GetMeRequest{}))
		return err
	}
	register := func() error {
		_, err := push.RegisterPushToken(context.Background(), connect.NewRequest(&isshov1.RegisterPushTokenRequest{}))
		return err
	}
	// Unimplemented = the gate let the call through.
	for _, tt := range []struct {
		name   string
		caller *auth.Caller
		call   func() error
		want   connect.Code
	}{
		{"suspended, other calls", &auth.Caller{UserID: suspended}, register, connect.CodePermissionDenied},
		{"suspended, GetMe", &auth.Caller{UserID: suspended}, getMe, connect.CodeUnimplemented},
		{"not suspended", &auth.Caller{UserID: other}, register, connect.CodeUnimplemented},
		{"no caller (public)", nil, register, connect.CodeUnimplemented},
	} {
		t.Run(tt.name, func(t *testing.T) {
			caller = tt.caller
			if got := connect.CodeOf(tt.call()); got != tt.want {
				t.Fatalf("code = %v, want %v", got, tt.want)
			}
		})
	}
}
