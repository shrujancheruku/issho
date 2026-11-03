package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

// hung blocks until its context ends, like a database that accepts the
// connection and never answers.
type hung struct{}

func (hung) Ping(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }

type pinger struct{ err error }

func (p pinger) Ping(context.Context) error { return p.err }

func TestCheckReturnsWhenThePingHangs(t *testing.T) {
	h := &Handler{Pool: hung{}, Version: "v"}
	start := time.Now()
	resp, err := h.Check(context.Background(), connect.NewRequest(&isshov1.CheckRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.DatabaseOk {
		t.Error("database_ok = true, want false")
	}
	if d := time.Since(start); d > PingTimeout+time.Second {
		t.Errorf("took %v, want about %v", d, PingTimeout)
	}
}

func TestCheckReportsAHealthyDatabase(t *testing.T) {
	h := &Handler{Pool: pinger{}, Version: "v"}
	resp, _ := h.Check(context.Background(), connect.NewRequest(&isshov1.CheckRequest{}))
	if !resp.Msg.DatabaseOk || resp.Msg.Version != "v" {
		t.Errorf("resp = %v", resp.Msg)
	}
}

func TestUp(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    Pinger
		want int
	}{{"ok", pinger{}, 200}, {"down", pinger{errors.New("x")}, 503}} {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			(&Handler{Pool: tt.p}).Up(rec, httptest.NewRequest(http.MethodGet, "/up", nil))
			if rec.Code != tt.want {
				t.Errorf("code = %d", rec.Code)
			}
		})
	}
}
