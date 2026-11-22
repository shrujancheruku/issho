package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/people"
)

type accountService struct {
	people *people.Service
}

// GetAccount returns the caller's own account and nobody's else: the account is
// the interceptor's verified identity, and the request has no id to name another.
func (s accountService) GetAccount(ctx context.Context, req *connect.Request[isshov1.GetAccountRequest]) (*connect.Response[isshov1.GetAccountResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		// A member RPC always has an identity; reaching here is a wiring bug.
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	acct, err := s.people.Join(ctx,
		people.Claims{Subject: id.Subject, Email: id.Email},
		people.Preferences{City: req.Msg.GetCity(), DisplayLanguage: req.Msg.GetDisplayLanguage()})
	switch {
	case errors.Is(err, people.ErrInvalid):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, people.ErrEmailTaken):
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("this email already has an account"))
	case err != nil:
		// The error may quote the database; the caller gets none of it.
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the account"))
	}
	return connect.NewResponse(&isshov1.GetAccountResponse{Account: &isshov1.Account{
		Id: acct.ID, Email: acct.Email, City: acct.City, DisplayLanguage: acct.DisplayLanguage,
	}}), nil
}
