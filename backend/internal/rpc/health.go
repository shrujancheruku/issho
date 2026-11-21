package rpc

import (
	"context"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

type healthService struct {
	version string
}

func (h healthService) Check(context.Context, *connect.Request[isshov1.CheckRequest]) (*connect.Response[isshov1.CheckResponse], error) {
	return connect.NewResponse(&isshov1.CheckResponse{Version: h.version}), nil
}
