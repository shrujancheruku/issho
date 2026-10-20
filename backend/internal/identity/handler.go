package identity

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

type Handler struct {
	Svc *Service
}

func (h *Handler) GetMe(ctx context.Context, _ *connect.Request[isshov1.GetMeRequest]) (*connect.Response[isshov1.GetMeResponse], error) {
	c, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no caller"))
	}
	acct, err := h.Svc.Me(ctx, c)
	if err != nil {
		return nil, err
	}
	u := &isshov1.User{Id: acct.User.ID.String(), Onboarded: acct.Onboarded}
	if acct.User.Email != nil {
		u.Email = *acct.User.Email
	}
	return connect.NewResponse(&isshov1.GetMeResponse{User: u}), nil
}
