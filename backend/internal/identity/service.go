// Package identity manages app accounts for users authenticated by Supabase Auth.
package identity

import (
	"context"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

type Account struct {
	User      dbgen.AppUser
	Onboarded bool
}

type Service struct {
	Q *dbgen.Queries
}

// Me returns the caller's account, creating the app row on first sight of a new auth user.
func (s *Service) Me(ctx context.Context, c auth.Caller) (Account, error) {
	var email *string
	if c.Email != "" {
		email = &c.Email
	}
	u, err := s.Q.UpsertUser(ctx, dbgen.UpsertUserParams{ID: c.UserID, Email: email})
	if err != nil {
		return Account{}, err
	}
	onboarded, err := s.Q.HasProfile(ctx, u.ID)
	if err != nil {
		return Account{}, err
	}
	return Account{User: u, Onboarded: onboarded}, nil
}
