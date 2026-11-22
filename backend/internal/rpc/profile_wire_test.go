package rpc_test

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
)

// rpcHandlerRequiringAProfile stands in for a signup RPC, which doesn't exist
// yet: a server whose one method does what a signup will, call
// rpc.RequireProfile first, for the account id it is given. It is the
// health service's shape only because that is a service the generated code
// already has.
func rpcHandlerRequiringAProfile(r *profileRig, id string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(isshov1connect.NewHealthServiceHandler(requiresProfile{r: r, id: id}))
	return mux
}

type requiresProfile struct {
	isshov1connect.UnimplementedHealthServiceHandler
	r  *profileRig
	id string
}

func (h requiresProfile) Check(ctx context.Context, _ *connect.Request[isshov1.CheckRequest]) (*connect.Response[isshov1.CheckResponse], error) {
	if err := rpc.RequireProfile(ctx, h.r.people, h.id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.CheckResponse{}), nil
}
