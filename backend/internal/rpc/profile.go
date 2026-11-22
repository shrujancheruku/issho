package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/people"
)

type profileService struct {
	people *people.Service
}

// GetProfile returns the caller's own profile, and nobody's else: the account
// is the interceptor's verified identity, and the request has no id to name
// another. A caller with no profile gets an empty response.
func (s profileService) GetProfile(ctx context.Context, _ *connect.Request[isshov1.GetProfileRequest]) (*connect.Response[isshov1.GetProfileResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	p, err := s.people.Profile(ctx, id.Subject)
	switch {
	case errors.Is(err, people.ErrProfileRequired):
		return connect.NewResponse(&isshov1.GetProfileResponse{}), nil
	case errors.Is(err, people.ErrNotFound):
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("make the account first"))
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the profile"))
	}
	return connect.NewResponse(&isshov1.GetProfileResponse{Profile: profileMessage(p)}), nil
}

// SaveProfile makes or changes the caller's own profile.
func (s profileService) SaveProfile(ctx context.Context, req *connect.Request[isshov1.SaveProfileRequest]) (*connect.Response[isshov1.SaveProfileResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	m := req.Msg
	p, err := s.people.SaveProfile(ctx, id.Subject, people.ProfileInput{
		Name:                  m.GetName(),
		DateOfBirth:           m.GetDateOfBirth(),
		Gender:                people.Gender(m.GetGender()),
		Languages:             m.GetLanguages(),
		LivesInCity:           m.GetLivesInCity(),
		AcceptedCodeOfConduct: m.GetAcceptedCodeOfConduct(),
		OnlyWomenAndNonBinary: m.GetOnlyWomenAndNonBinary(),
	})
	switch {
	case errors.Is(err, people.ErrUnderage):
		return nil, problem(connect.CodeInvalidArgument, "you must be 18 or over", isshov1.ProfileProblemReason_PROFILE_PROBLEM_REASON_UNDER_AGE)
	case errors.Is(err, people.ErrWomenOnlyNotAvailable):
		return nil, problem(connect.CodeInvalidArgument, "only women and non-binary people is not available for this gender", isshov1.ProfileProblemReason_PROFILE_PROBLEM_REASON_WOMEN_ONLY_NOT_AVAILABLE)
	case errors.Is(err, people.ErrInvalid):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, people.ErrNotFound):
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("make the account first"))
	case err != nil:
		// The error may quote the database; the caller gets none of it.
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save the profile"))
	}
	return connect.NewResponse(&isshov1.SaveProfileResponse{Profile: profileMessage(p)}), nil
}

// RequireProfile is the check every RPC that needs a profile (a signup) makes
// first. For a caller with none it returns FAILED_PRECONDITION carrying
// ProfileProblem PROFILE_REQUIRED, which is what the app acts on by opening the
// profile's questions. A nil result means go on.
func RequireProfile(ctx context.Context, svc *people.Service, accountID string) error {
	err := svc.RequireProfile(ctx, accountID)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, people.ErrProfileRequired):
		return problem(connect.CodeFailedPrecondition, "a profile is required first", isshov1.ProfileProblemReason_PROFILE_PROBLEM_REASON_PROFILE_REQUIRED)
	case errors.Is(err, people.ErrNotFound):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("make the account first"))
	default:
		return connect.NewError(connect.CodeInternal, errors.New("could not read the profile"))
	}
}

// problem is an error with a ProfileProblem detail for the app to act on.
func problem(code connect.Code, msg string, reason isshov1.ProfileProblemReason) error {
	err := connect.NewError(code, errors.New(msg))
	if d, derr := connect.NewErrorDetail(&isshov1.ProfileProblem{Reason: reason}); derr == nil {
		err.AddDetail(d)
	}
	return err
}

func profileMessage(p people.Profile) *isshov1.Profile {
	return &isshov1.Profile{
		Name: p.Name, Gender: string(p.Gender), Languages: p.Languages, LivesInCity: p.LivesInCity,
		OnlyWomenAndNonBinary: p.OnlyWomenAndNonBinary, CodeOfConductVersion: int32(p.CodeOfConductVersion),
	}
}
