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
		if errors.Is(err, ErrAccountDeleted) {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}
		return nil, err
	}
	u := &isshov1.User{
		Id:                acct.User.ID.String(),
		Onboarded:         acct.Onboarded,
		PrimaryLanguage:   acct.PrimaryLanguage,
		SecondaryLanguage: acct.SecondaryLanguage,
		Suspended:         acct.Suspended,
		CityId:            acct.CityID,
	}
	if acct.User.Email != nil {
		u.Email = *acct.User.Email
	}
	return connect.NewResponse(&isshov1.GetMeResponse{User: u}), nil
}

func (h *Handler) DeleteAccount(ctx context.Context, _ *connect.Request[isshov1.DeleteAccountRequest]) (*connect.Response[isshov1.DeleteAccountResponse], error) {
	c, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no caller"))
	}
	if err := h.Svc.DeleteAccount(ctx, c); err != nil {
		if errors.Is(err, ErrDeletionUnavailable) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, err
	}
	return connect.NewResponse(&isshov1.DeleteAccountResponse{}), nil
}
