package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/coder/websocket"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
)

type echoUI struct {
	isshov1connect.UnimplementedUIServiceHandler
}

func (echoUI) GetStrings(context.Context, *isshov1.GetStringsRequest) (*isshov1.GetStringsResponse, error) {
	return &isshov1.GetStringsResponse{}, nil
}

func TestRequestSizeLimit(t *testing.T) {
	mux := http.NewServeMux()
	rpc := newRPCServer(slog.Default())
	isshov1connect.RegisterUIServiceHandler(rpc, echoUI{})
	mountRPC(mux, rpc)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := isshov1connect.NewUIServiceClient(connect.NewClient(connecthttp.NewTransport(srv.Client(), srv.URL)))

	if _, err := c.GetStrings(context.Background(), &isshov1.GetStringsRequest{Language: "en"}); err != nil {
		t.Fatalf("normal request: %v", err)
	}
	_, err := c.GetStrings(context.Background(), &isshov1.GetStringsRequest{Language: strings.Repeat("x", maxRequestBytes+1)})
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatalf("oversized request: %v, want ResourceExhausted", err)
	}
}

// The long admin operations are jobs: the RPCs that start them only validate
// and queue, so they take the default deadline, and the draft call (a model
// with its own 60 s budget) stays under Cloudflare's 100 s cut-off.
func TestAdminJobRPCsTakeTheDefaultDeadline(t *testing.T) {
	for _, p := range []string{isshov1connect.CityPackServiceApplyCityPackProcedure, isshov1connect.CityPackServiceDiscoverAreaVenuesProcedure} {
		if d, ok := rpcDeadlines[p]; ok {
			t.Errorf("%s has its own deadline %v; it only queues a job", p, d)
		}
	}
	if d := rpcDeadlines[isshov1connect.TranslationServiceDraftAppStringProcedure]; d <= 0 || d >= 100*time.Second {
		t.Errorf("DraftAppString deadline = %v, want under Cloudflare's 100 s", d)
	}
}

// A socket must outlive every server timeout. Same newServer as production,
// with the timeouts shrunk so the test takes about a second.
func TestWebSocketOutlivesServerTimeouts(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = ws.CloseNow() }()
		// Write after every timeout would have fired, as the hub does.
		time.Sleep(time.Second)
		_ = ws.Write(r.Context(), websocket.MessageText, []byte("late"))
		_ = ws.Close(websocket.StatusNormalClosure, "")
	})
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	srv := httptest.NewUnstartedServer(nil)
	srv.Config = newServer("", mux, &protocols, context.Background(),
		serverTimeouts{ReadHeader: 300 * time.Millisecond, Read: 300 * time.Millisecond, Idle: 300 * time.Millisecond})
	if srv.Config.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %v, want none", srv.Config.WriteTimeout)
	}
	srv.Start()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil) //nolint:bodyclose // coder/websocket Dial: callers never close the handshake resp.Body
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.CloseNow() }()
	_, msg, err := ws.Read(ctx)
	if err != nil || string(msg) != "late" {
		t.Fatalf("read after the timeouts: %q, %v", msg, err)
	}
}

func TestServerTimeouts(t *testing.T) {
	s := newServer(":0", http.NewServeMux(), nil, context.Background(), prodTimeouts)
	if s.ReadTimeout != 30*time.Second || s.IdleTimeout != 120*time.Second || s.ReadHeaderTimeout != 10*time.Second || s.WriteTimeout != 0 {
		t.Errorf("timeouts = %v %v %v %v", s.ReadTimeout, s.IdleTimeout, s.ReadHeaderTimeout, s.WriteTimeout)
	}
}
