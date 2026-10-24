// Package health implements HealthService.
package health

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

type Handler struct {
	Pool    *pgxpool.Pool
	Version string
}

func (h *Handler) Check(ctx context.Context, _ *connect.Request[isshov1.CheckRequest]) (*connect.Response[isshov1.CheckResponse], error) {
	return connect.NewResponse(&isshov1.CheckResponse{
		DatabaseOk: h.Pool.Ping(ctx) == nil,
		Version:    h.Version,
	}), nil
}

// Up answers GET /up with 200 when the database answers, 503 otherwise.
// kamal-proxy polls it before sending traffic to a new container, so a
// deploy with a wrong DATABASE_URL never replaces a working one. Plain HTTP
// (not Check) because the proxy only does GETs.
func (h *Handler) Up(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.Pool.Ping(ctx); err != nil {
		http.Error(w, "database unreachable", http.StatusServiceUnavailable)
		return
	}
	_, _ = w.Write([]byte("ok\n"))
}
