// Package identity manages app accounts for users authenticated by Supabase Auth.
package identity

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

var (
	// ErrDeletionUnavailable is returned when no Supabase admin client is configured.
	ErrDeletionUnavailable = errors.New("account deletion is not configured on this server")
	// ErrAccountDeleted is returned when a still-valid token belongs to a deleted auth user.
	ErrAccountDeleted = errors.New("account deleted")
)

// Store is the subset of generated queries this service needs (dbgen.Queries satisfies it).
type Store interface {
	UpsertUser(ctx context.Context, arg dbgen.UpsertUserParams) (dbgen.AppUser, error)
	GetDisplayLanguages(ctx context.Context, userID uuid.UUID) (dbgen.GetDisplayLanguagesRow, error)
	IsSuspended(ctx context.Context, userID uuid.UUID) (bool, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
}

// AuthAdmin deletes users from the identity provider (Supabase Auth).
type AuthAdmin interface {
	DeleteUser(ctx context.Context, id uuid.UUID) error
}

type Account struct {
	User      dbgen.AppUser
	Onboarded bool
	// Display languages; empty until onboarded.
	PrimaryLanguage, SecondaryLanguage string
	// A moderator suspended the account (app.suspensions).
	Suspended bool
}

type Service struct {
	Store Store
	Admin AuthAdmin // nil when SUPABASE_SECRET_KEY is unset
}

// Me returns the caller's account, creating the app row on first sight of a new auth user.
func (s *Service) Me(ctx context.Context, c auth.Caller) (Account, error) {
	var email *string
	if c.Email != "" {
		email = &c.Email
	}
	u, err := s.Store.UpsertUser(ctx, dbgen.UpsertUserParams{ID: c.UserID, Email: email})
	if err != nil {
		// Access tokens outlive account deletion (until they expire). The
		// app.users → auth.users foreign key then rejects re-creating the row,
		// which is the signal that the account is gone.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation && pgErr.ConstraintName == "users_auth_fk" {
			return Account{}, ErrAccountDeleted
		}
		return Account{}, err
	}
	acct := Account{User: u}
	if acct.Suspended, err = s.Store.IsSuspended(ctx, u.ID); err != nil {
		return Account{}, err
	}
	langs, err := s.Store.GetDisplayLanguages(ctx, u.ID)
	switch {
	case errors.Is(err, pgx.ErrNoRows): // no profile yet
	case err != nil:
		return Account{}, err
	default:
		acct.Onboarded = true
		acct.PrimaryLanguage = langs.PrimaryLanguage
		if langs.SecondaryLanguage != nil {
			acct.SecondaryLanguage = *langs.SecondaryLanguage
		}
	}
	return acct, nil
}

// DeleteAccount deletes the auth user first, then the app row.
//
// The order matters. If the auth deletion fails, nothing is removed, and the
// user can still sign in and retry. Deleting the app row first would leave a
// live login with no data, and the next GetMe would silently recreate an empty
// account. On Supabase the auth deletion already cascades to app.users; the
// explicit delete covers databases without that foreign key.
func (s *Service) DeleteAccount(ctx context.Context, c auth.Caller) error {
	if s.Admin == nil {
		return ErrDeletionUnavailable
	}
	if err := s.Admin.DeleteUser(ctx, c.UserID); err != nil {
		return err
	}
	return s.Store.DeleteUser(ctx, c.UserID)
}
