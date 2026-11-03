package errs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
)

// serve runs a Health service whose Check returns handlerErr behind the real
// Interceptor, and returns a client: the wire is what clients see.
func serve(t *testing.T, handlerErr error) isshov1connect.HealthServiceClient {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	mux.Handle(isshov1connect.NewHealthServiceHandler(failingHealth{handlerErr}, connect.WithInterceptors(Interceptor(log))))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return isshov1connect.NewHealthServiceClient(srv.Client(), srv.URL)
}

type failingHealth struct{ err error }

func (f failingHealth) Check(context.Context, *connect.Request[isshov1.CheckRequest]) (*connect.Response[isshov1.CheckResponse], error) {
	return nil, f.err
}

func check(t *testing.T, err error) error {
	t.Helper()
	_, got := serve(t, err).Check(context.Background(), connect.NewRequest(&isshov1.CheckRequest{}))
	return got
}

// reasonOf is what a Go client reads from the detail.
func reasonOf(t *testing.T, err error) isshov1.ErrorReason {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("not a connect error: %v", err)
	}
	for _, d := range ce.Details() {
		m, verr := d.Value()
		if verr != nil {
			t.Fatal(verr)
		}
		if rd, ok := m.(*isshov1.ErrorReasonDetail); ok {
			return rd.Reason
		}
	}
	return isshov1.ErrorReason_ERROR_REASON_UNSPECIFIED
}

func TestKindsMapToCodes(t *testing.T) {
	const suspended = isshov1.ErrorReason_ERROR_REASON_SUSPENDED
	const tooLate = isshov1.ErrorReason_ERROR_REASON_TOO_LATE
	for _, tt := range []struct {
		name   string
		err    error
		code   connect.Code
		msg    string
		reason isshov1.ErrorReason
	}{
		{"invalid", Invalid("city_id: required"), connect.CodeInvalidArgument, "city_id: required", 0},
		{"not found", NotFound("no such plan"), connect.CodeNotFound, "no such plan", 0},
		{"precondition", Precondition(tooLate, "too late"), connect.CodeFailedPrecondition, "too late", tooLate},
		{"precondition, no reason", Precondition(0, "not now"), connect.CodeFailedPrecondition, "not now", 0},
		{"denied", Denied(suspended, "account suspended"), connect.CodePermissionDenied, "account suspended", suspended},
		{"forbidden", Forbidden("admins only"), connect.CodePermissionDenied, "admins only", 0},
		{"unauthenticated", Unauthenticated("invalid token"), connect.CodeUnauthenticated, "invalid token", 0},
		{"zero kind", &Error{msg: "bug"}, connect.CodeInternal, "bug", 0},
		{"wrapped", fmt.Errorf("ctx: %w", Invalid("x")), connect.CodeInvalidArgument, "x", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := check(t, tt.err)
			if connect.CodeOf(err) != tt.code {
				t.Fatalf("code = %v, want %v", connect.CodeOf(err), tt.code)
			}
			if got := err.(*connect.Error).Message(); got != tt.msg {
				t.Fatalf("message = %q, want %q", got, tt.msg)
			}
			if got := reasonOf(t, err); got != tt.reason {
				t.Fatalf("reason = %v, want %v", got, tt.reason)
			}
		})
	}
}

func TestUpstreamHidesCauseAndReports(t *testing.T) {
	var gotErr error
	var gotTags map[string]string
	old := report
	report = func(_ context.Context, err error, tags map[string]string) { gotErr, gotTags = err, tags }
	t.Cleanup(func() { report = old })

	raw := errors.New(`gemini 400: {"error":{"message":"API key not valid","status":"INVALID_ARGUMENT"}}`)
	err := check(t, Upstream("gemini", raw))
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("code = %v", connect.CodeOf(err))
	}
	msg := err.(*connect.Error).Message()
	if msg == "" || strings.Contains(msg, "API key") || strings.Contains(msg, "400") || strings.Contains(msg, "INVALID") {
		t.Fatalf("upstream text leaked or message empty: %q", msg)
	}
	if gotErr != raw || gotTags["service"] != "gemini" || gotTags["procedure"] != isshov1connect.HealthServiceCheckProcedure {
		t.Fatalf("sentry got err=%v tags=%v", gotErr, gotTags)
	}
}

func TestClientErrorsAreNotReported(t *testing.T) {
	old := report
	report = func(context.Context, error, map[string]string) { t.Error("client error reported") }
	t.Cleanup(func() { report = old })
	_ = check(t, Invalid("x"))
	_ = check(t, Denied(isshov1.ErrorReason_ERROR_REASON_SUSPENDED, "account suspended"))
}

func TestCodeOfSeesHandlerErrors(t *testing.T) {
	// Handler tests call handlers directly; they must see the wire code.
	if connect.CodeOf(NotFound("x")) != connect.CodeNotFound {
		t.Fatal("CodeOf(errs.NotFound) is not NotFound")
	}
	cause := errors.New("sentinel")
	if !errors.Is(WithCause(Invalid("x"), cause), cause) {
		t.Fatal("WithCause lost the sentinel")
	}
}
