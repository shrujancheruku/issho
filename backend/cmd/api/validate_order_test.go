package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

// Validation runs after the caller's interceptors (auth), so someone who is
// not signed in hears Unauthenticated, not a hint about the request's rules.
func TestValidationRunsAfterAuth(t *testing.T) {
	deny := connect.UnaryInterceptorFunc(func(connect.UnaryFunc) connect.UnaryFunc {
		return func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
			return nil, errs.Unauthenticated("sign in")
		}
	})
	mux := http.NewServeMux()
	mux.Handle(isshov1connect.NewChatServiceHandler(isshov1connect.UnimplementedChatServiceHandler{}, handlerOptions(slog.Default(), deny)))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := isshov1connect.NewChatServiceClient(srv.Client(), srv.URL)

	_, err := c.SendMessage(context.Background(), connect.NewRequest(&isshov1.SendMessageRequest{
		Room: &isshov1.ChatRoom{Room: &isshov1.ChatRoom_EventId{EventId: "e"}}, Body: strings.Repeat("a", 1001),
	}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("over-limit request while signed out: %v, want Unauthenticated", err)
	}
}
