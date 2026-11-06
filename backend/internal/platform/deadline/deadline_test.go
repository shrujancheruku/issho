package deadline

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

// slow waits for its context, like a handler stuck on a slow upstream.
type slow struct {
	isshov1connect.UnimplementedHealthServiceHandler
}

func (slow) Check(ctx context.Context, _ *isshov1.CheckRequest) (*isshov1.CheckResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err() // a bare context error, as a database call returns it
	case <-time.After(300 * time.Millisecond):
		return &isshov1.CheckResponse{DatabaseOk: true}, nil
	}
}

func call(t *testing.T, i connect.ServerInterceptor) error {
	t.Helper()
	mux := http.NewServeMux()
	server :=
		// errs outermost, as in main.
		connect.NewServer(errs.Interceptor(slog.Default()), i)
	isshov1connect.RegisterHealthServiceHandler(server, slow{})
	connecthttp.Mount(mux, server)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	_, err := isshov1connect.NewHealthServiceClient(connect.NewClient(connecthttp.NewTransport(srv.Client(), srv.URL))).Check(context.Background(), &isshov1.CheckRequest{})
	return err
}

func TestBlockedHandlerIsDeadlineExceeded(t *testing.T) {
	err := call(t, Interceptor(50*time.Millisecond, nil))
	if connect.CodeOf(err) != connect.CodeDeadlineExceeded {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
}

func TestOptedOutProcedureIsNotCutOff(t *testing.T) {
	i := Interceptor(50*time.Millisecond, map[string]time.Duration{isshov1connect.HealthServiceCheckProcedure: 0})
	if err := call(t, i); err != nil {
		t.Fatalf("opted-out RPC failed: %v", err)
	}
}

func TestLongerOverride(t *testing.T) {
	i := Interceptor(50*time.Millisecond, map[string]time.Duration{isshov1connect.HealthServiceCheckProcedure: 5 * time.Second})
	if err := call(t, i); err != nil {
		t.Fatalf("RPC with a longer deadline failed: %v", err)
	}
}

func TestShorterCallerDeadlineWins(t *testing.T) {
	var got time.Duration
	next := func(ctx context.Context, _ connect.Spec, _ connect.ServerStream) error {
		dl, _ := ctx.Deadline()
		got = time.Until(dl)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = Interceptor(time.Minute, nil)(next)(ctx, connect.Spec{}, nil)
	if got > time.Second {
		t.Fatalf("deadline %v, want the caller's 1s", got)
	}
}
