package rpc_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/logging"
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

// kamal-proxy polls a plain GET before it sends a container traffic; the RPC
// health check is a POST, which it can't make.
func TestUpAnswers200ToAPlainGET(t *testing.T) {
	srv := httptest.NewServer(rpc.NewHandler(rpc.Config{Version: "1.2.3"}))
	t.Cleanup(srv.Close)

	res, err := srv.Client().Get(srv.URL + "/up")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /up = %d, want 200", res.StatusCode)
	}
}

// The probe comes every few seconds for as long as the container lives; it must
// not fill the request log.
func TestUpIsNotLogged(t *testing.T) {
	var buf bytes.Buffer
	h := rpc.NewHandler(rpc.Config{Version: "1.2.3", Log: logging.New(&buf, "api"), Clock: clock.NewFake(time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC))})
	srv := httptest.NewServer(h)
	res, err := srv.Client().Get(srv.URL + "/up")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	srv.Close()
	if buf.Len() != 0 {
		t.Fatalf("GET /up logged %q", buf.String())
	}
}
