package rpc_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
)

// The health check is public: a probe with no token gets the version the API
// was built with.
func TestHealthCheckReportsTheBuildVersionWithoutAToken(t *testing.T) {
	srv := httptest.NewServer(rpc.NewHandler(rpc.Config{Version: "1.2.3"}))
	t.Cleanup(srv.Close)

	client := isshov1connect.NewHealthServiceClient(srv.Client(), srv.URL)
	res, err := client.Check(context.Background(), connect.NewRequest(&isshov1.CheckRequest{}))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got := res.Msg.GetVersion(); got != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3", got)
	}
}

// Public means anyone: a bad token is ignored, not refused.
func TestHealthCheckIgnoresATokenThatIsNotValid(t *testing.T) {
	srv := httptest.NewServer(rpc.NewHandler(rpc.Config{Version: "1.2.3"}))
	t.Cleanup(srv.Close)

	client := isshov1connect.NewHealthServiceClient(srv.Client(), srv.URL)
	req := connect.NewRequest(&isshov1.CheckRequest{})
	req.Header().Set("Authorization", "Bearer forged")
	if _, err := client.Check(context.Background(), req); err != nil {
		t.Fatalf("Check: %v", err)
	}
}
