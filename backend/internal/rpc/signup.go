package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
)

type signupService struct {
	people  *people.Service
	signups *plans.Signups
}

// SignUp signs the caller up to a plan, or replaces the ticks of the signup
// they have. The account is the interceptor's verified identity and the
// request names no other, so nobody signs up another person.
func (s signupService) SignUp(ctx context.Context, req *connect.Request[isshov1.SignUpRequest]) (*connect.Response[isshov1.SignUpResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	// A signup needs a profile (docs/spec/people.md, "The profile"); a caller
	// with none is told to ask its questions first.
	if err := RequireProfile(ctx, s.people, id.Subject); err != nil {
		return nil, err
	}
	ticks := make([]plans.Tick, len(req.Msg.GetTicks()))
	for i, t := range req.Msg.GetTicks() {
		ticks[i] = plans.Tick{Date: t.GetDate(), Part: t.GetPart()}
	}
	got, err := s.signups.Set(ctx, id.Subject, req.Msg.GetPlanId(), ticks)
	switch {
	case errors.Is(err, plans.ErrNotFound):
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such plan"))
	case errors.Is(err, plans.ErrTickTooSoon):
		return nil, signupProblem("a tick is less than 3 days ahead", isshov1.SignupProblemReason_SIGNUP_PROBLEM_REASON_TICK_TOO_SOON)
	case errors.Is(err, plans.ErrTickPartNotAllowed):
		return nil, signupProblem("the plan doesn't run in that part of the day", isshov1.SignupProblemReason_SIGNUP_PROBLEM_REASON_TICK_PART_NOT_ALLOWED)
	case errors.Is(err, plans.ErrTickTooFar):
		return nil, signupProblem("a tick is too far ahead", isshov1.SignupProblemReason_SIGNUP_PROBLEM_REASON_TICK_TOO_FAR)
	case errors.Is(err, plans.ErrInvalid):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case err != nil:
		// The error may quote the database; the caller gets none of it.
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save the signup"))
	}
	return connect.NewResponse(&isshov1.SignUpResponse{Signup: signupMessage(got)}), nil
}

// WithdrawSignup withdraws the caller's signup to a plan.
func (s signupService) WithdrawSignup(ctx context.Context, req *connect.Request[isshov1.WithdrawSignupRequest]) (*connect.Response[isshov1.WithdrawSignupResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	err := s.signups.Withdraw(ctx, id.Subject, req.Msg.GetPlanId())
	switch {
	case errors.Is(err, plans.ErrNotFound):
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such plan"))
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not withdraw the signup"))
	}
	return connect.NewResponse(&isshov1.WithdrawSignupResponse{}), nil
}

// ListMySignups lists the caller's own signups, and nobody else's: the request
// has no account to name.
func (s signupService) ListMySignups(ctx context.Context, _ *connect.Request[isshov1.ListMySignupsRequest]) (*connect.Response[isshov1.ListMySignupsResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	mine, err := s.signups.Mine(ctx, id.Subject)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the signups"))
	}
	out := make([]*isshov1.Signup, len(mine))
	for i, m := range mine {
		out[i] = signupMessage(m)
	}
	return connect.NewResponse(&isshov1.ListMySignupsResponse{Signups: out}), nil
}

func signupMessage(s plans.Signup) *isshov1.Signup {
	m := &isshov1.Signup{PlanId: s.PlanID, HearBy: s.HearBy}
	for _, t := range s.Ticks {
		m.Ticks = append(m.Ticks, &isshov1.Tick{Date: t.Date, Part: t.Part})
	}
	return m
}

// signupProblem is an INVALID_ARGUMENT with a SignupProblem detail for the app
// to act on.
func signupProblem(msg string, reason isshov1.SignupProblemReason) error {
	err := connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
	if d, derr := connect.NewErrorDetail(&isshov1.SignupProblem{Reason: reason}); derr == nil {
		err.AddDetail(d)
	}
	return err
}
