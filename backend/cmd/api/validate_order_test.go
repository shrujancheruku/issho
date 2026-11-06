package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

// Validation runs after the caller's interceptors (auth), so someone who is
// not signed in hears Unauthenticated, not a hint about the request's rules.
func TestValidationRunsAfterAuth(t *testing.T) {
	deny := connect.ServerInterceptor(func(connect.ServerFunc) connect.ServerFunc {
		return func(context.Context, connect.Spec, connect.ServerStream) error {
			return errs.Unauthenticated("sign in")
		}
	})
	mux := http.NewServeMux()
	rpc := newRPCServer(slog.Default(), deny)
	isshov1connect.RegisterChatServiceHandler(rpc, isshov1connect.UnimplementedChatServiceHandler{})
	mountRPC(mux, rpc)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := isshov1connect.NewChatServiceClient(connect.NewClient(connecthttp.NewTransport(srv.Client(), srv.URL)))

	_, err := c.SendMessage(context.Background(), &isshov1.SendMessageRequest{
		Room: &isshov1.ChatRoom{Room: &isshov1.ChatRoom_EventId{EventId: "e"}}, Body: strings.Repeat("a", 1001),
	})
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("over-limit request while signed out: %v, want Unauthenticated", err)
	}
}
