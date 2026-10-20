// Package health implements HealthService.
package health

import (
	"context"

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
