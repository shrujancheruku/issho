// Package health implements HealthService.
package health

import (
	"context"
	"net/http"
	"time"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

// PingTimeout bounds the database ping, so a hung database reads as
// database_ok = false instead of a hung health check.
const PingTimeout = 2 * time.Second

// Pinger is the part of *pgxpool.Pool the checks use.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	Pool    Pinger
	Version string
	// PingTimeout overrides the package default (tests).
	PingTimeout time.Duration
}

func (h *Handler) ping(ctx context.Context) error {
	d := PingTimeout
	if h.PingTimeout > 0 {
		d = h.PingTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	return h.Pool.Ping(ctx)
}

func (h *Handler) Check(ctx context.Context, _ *isshov1.CheckRequest) (*isshov1.CheckResponse, error) {
	return &isshov1.CheckResponse{
		DatabaseOk: h.ping(ctx) == nil,
		Version:    h.Version,
	}, nil
}

// Up answers GET /up with 200 when the database answers, 503 otherwise.
// kamal-proxy polls it before sending traffic to a new container, so a
// deploy with a wrong DATABASE_URL never replaces a working one. Plain HTTP
// (not Check) because the proxy only does GETs.
func (h *Handler) Up(w http.ResponseWriter, r *http.Request) {
	if err := h.ping(r.Context()); err != nil {
		http.Error(w, "database unreachable", http.StatusServiceUnavailable)
		return
	}
	_, _ = w.Write([]byte("ok\n"))
}
