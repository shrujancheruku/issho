package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/people"
)

type joinLinkService struct {
	people *people.Service
}

// CreateJoinLink makes a link for the caller, who must be admitted and have a
// link left. The account is the interceptor's verified identity and the
// request names none, so nobody makes a link as another person.
func (s joinLinkService) CreateJoinLink(ctx context.Context, _ *connect.Request[isshov1.CreateJoinLinkRequest]) (*connect.Response[isshov1.CreateJoinLinkResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	l, err := s.people.MakeJoinLink(ctx, id.Subject)
	switch {
	case errors.Is(err, people.ErrNotAdmitted):
		return nil, joinLinkProblem(connect.CodeFailedPrecondition, "you have not been admitted yet", isshov1.JoinLinkProblemReason_JOIN_LINK_PROBLEM_REASON_NOT_ADMITTED)
	case errors.Is(err, people.ErrNoJoinLinksLeft):
		return nil, joinLinkProblem(connect.CodeFailedPrecondition, "you have no join links left", isshov1.JoinLinkProblemReason_JOIN_LINK_PROBLEM_REASON_NONE_LEFT)
	case errors.Is(err, people.ErrNotFound):
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("make the account first"))
	case err != nil:
		// The error may quote the database; the caller gets none of it.
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not make the join link"))
	}
	return connect.NewResponse(&isshov1.CreateJoinLinkResponse{Link: joinLinkMessage(l)}), nil
}

// CancelJoinLink cancels one of the caller's own outstanding links. A link that
// isn't theirs, was used, has expired or doesn't exist all read the same.
func (s joinLinkService) CancelJoinLink(ctx context.Context, req *connect.Request[isshov1.CancelJoinLinkRequest]) (*connect.Response[isshov1.CancelJoinLinkResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	err := s.people.CancelJoinLink(ctx, id.Subject, req.Msg.GetId())
	switch {
	case errors.Is(err, people.ErrJoinLinkNotFound):
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such join link"))
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not cancel the join link"))
	}
	return connect.NewResponse(&isshov1.CancelJoinLinkResponse{}), nil
}

// ListMyJoinLinks lists the caller's own links, and nobody else's: the request
// has no account to name, and a link never says who used it.
func (s joinLinkService) ListMyJoinLinks(ctx context.Context, _ *connect.Request[isshov1.ListMyJoinLinksRequest]) (*connect.Response[isshov1.ListMyJoinLinksResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	got, err := s.people.MyJoinLinks(ctx, id.Subject)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the join links"))
	}
	out := &isshov1.ListMyJoinLinksResponse{Remaining: int32(got.Remaining)}
	for _, l := range got.Links {
		out.Links = append(out.Links, joinLinkMessage(l))
	}
	return connect.NewResponse(out), nil
}

// RedeemJoinLink admits the caller with the link's code. Every way a code can
// fail to admit anyone reads the same, so a code can't be probed.
func (s joinLinkService) RedeemJoinLink(ctx context.Context, req *connect.Request[isshov1.RedeemJoinLinkRequest]) (*connect.Response[isshov1.RedeemJoinLinkResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	err := s.people.RedeemJoinLink(ctx, id.Subject, req.Msg.GetCode())
	switch {
	case errors.Is(err, people.ErrJoinLinkUnusable):
		return nil, joinLinkProblem(connect.CodeFailedPrecondition, "this join link cannot be used", isshov1.JoinLinkProblemReason_JOIN_LINK_PROBLEM_REASON_LINK_UNUSABLE)
	case errors.Is(err, people.ErrAlreadyAdmitted):
		return nil, joinLinkProblem(connect.CodeFailedPrecondition, "you are already in", isshov1.JoinLinkProblemReason_JOIN_LINK_PROBLEM_REASON_ALREADY_ADMITTED)
	case errors.Is(err, people.ErrNotFound):
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("make the account first"))
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not use the join link"))
	}
	return connect.NewResponse(&isshov1.RedeemJoinLinkResponse{}), nil
}

// RequireAdmitted is the check a signup makes before anything else. For a
// caller who hasn't been admitted it returns FAILED_PRECONDITION carrying
// SignupProblem NOT_ADMITTED, which is what the app acts on by offering the
// waitlist. A nil result means go on.
func RequireAdmitted(ctx context.Context, svc *people.Service, accountID string) error {
	err := svc.RequireAdmitted(ctx, accountID)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, people.ErrNotAdmitted):
		return signupProblem(connect.CodeFailedPrecondition, "you have not been admitted yet", isshov1.SignupProblemReason_SIGNUP_PROBLEM_REASON_NOT_ADMITTED)
	case errors.Is(err, people.ErrNotFound):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("make the account first"))
	default:
		return connect.NewError(connect.CodeInternal, errors.New("could not check the admission"))
	}
}

func joinLinkMessage(l people.JoinLink) *isshov1.JoinLink {
	state := isshov1.JoinLinkState_JOIN_LINK_STATE_OUTSTANDING
	switch l.State {
	case people.JoinLinkUsed:
		state = isshov1.JoinLinkState_JOIN_LINK_STATE_USED
	case people.JoinLinkExpired:
		state = isshov1.JoinLinkState_JOIN_LINK_STATE_EXPIRED
	}
	return &isshov1.JoinLink{
		Id: l.ID, Code: l.Code, State: state,
		CreatedAt: timestamppb.New(l.CreatedAt), ExpiresAt: timestamppb.New(l.ExpiresAt),
	}
}

// joinLinkProblem is an error with a JoinLinkProblem detail for the app to act on.
func joinLinkProblem(code connect.Code, msg string, reason isshov1.JoinLinkProblemReason) error {
	err := connect.NewError(code, errors.New(msg))
	if d, derr := connect.NewErrorDetail(&isshov1.JoinLinkProblem{Reason: reason}); derr == nil {
		err.AddDetail(d)
	}
	return err
}
