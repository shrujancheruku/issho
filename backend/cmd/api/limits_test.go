package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/coder/websocket"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
)

type echoUI struct{ isshov1connect.UnimplementedUIServiceHandler }

func (echoUI) GetStrings(context.Context, *connect.Request[isshov1.GetStringsRequest]) (*connect.Response[isshov1.GetStringsResponse], error) {
	return connect.NewResponse(&isshov1.GetStringsResponse{}), nil
}

func TestRequestSizeLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(isshov1connect.NewUIServiceHandler(echoUI{}, connect.WithReadMaxBytes(maxRequestBytes)))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := isshov1connect.NewUIServiceClient(srv.Client(), srv.URL)

	if _, err := c.GetStrings(context.Background(), connect.NewRequest(&isshov1.GetStringsRequest{Language: "en"})); err != nil {
		t.Fatalf("normal request: %v", err)
	}
	_, err := c.GetStrings(context.Background(), connect.NewRequest(&isshov1.GetStringsRequest{Language: strings.Repeat("x", maxRequestBytes+1)}))
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatalf("oversized request: %v, want ResourceExhausted", err)
	}
}

func TestCityPackAppliesAreNotCutAtThirtySeconds(t *testing.T) {
	if d, ok := rpcDeadlines[isshov1connect.CityPackServiceApplyCityPackProcedure]; !ok || d != 0 {
		t.Errorf("ApplyCityPack deadline = %v (set %v), want opted out", d, ok)
	}
}

// A chat socket must outlive the 30 s ReadTimeout and every other server
// timeout. Real time, so skipped with -short.
func TestWebSocketOutlivesServerTimeouts(t *testing.T) {
	if testing.Short() {
		t.Skip("holds a socket open for 35 s")
	}
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer ws.CloseNow()
		// Write after every timeout would have fired, as the hub does.
		time.Sleep(readTimeout + 5*time.Second)
		_ = ws.Write(r.Context(), websocket.MessageText, []byte("late"))
		_ = ws.Close(websocket.StatusNormalClosure, "")
	})
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	srv := httptest.NewUnstartedServer(nil)
	real := newServer("", mux, &protocols, context.Background())
	srv.Config.Handler, srv.Config.ReadTimeout, srv.Config.IdleTimeout, srv.Config.ReadHeaderTimeout =
		real.Handler, real.ReadTimeout, real.IdleTimeout, real.ReadHeaderTimeout
	if real.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %v, want none", real.WriteTimeout)
	}
	srv.Start()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), readTimeout+20*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	_, msg, err := ws.Read(ctx)
	if err != nil || string(msg) != "late" {
		t.Fatalf("read after %v: %q, %v", readTimeout, msg, err)
	}
}

func TestServerTimeouts(t *testing.T) {
	s := newServer(":0", http.NewServeMux(), nil, context.Background())
	if s.ReadTimeout != 30*time.Second || s.IdleTimeout != 120*time.Second || s.ReadHeaderTimeout != 10*time.Second || s.WriteTimeout != 0 {
		t.Errorf("timeouts = %v %v %v %v", s.ReadTimeout, s.IdleTimeout, s.ReadHeaderTimeout, s.WriteTimeout)
	}
}
