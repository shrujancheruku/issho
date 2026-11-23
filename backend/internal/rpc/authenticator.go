package rpc

import (
	"context"
	"errors"

	"github.com/shrujancheruku/issho/backend/internal/people"
)

// TokenVerifier checks a Supabase Auth access token. *people.Verifier is the
// one that does, against the project's JWKS.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (people.Claims, error)
}

type tokenAuthenticator struct{ verifier TokenVerifier }

// NewAuthenticator makes the Authenticator that trusts what v verifies. The
// token says who the caller is and whether they took the second sign-in step;
// whether they are the owner, an insider or staff is not in it, and is added
// when staff roles exist (until then nobody is Making or admin).
func NewAuthenticator(v TokenVerifier) Authenticator { return tokenAuthenticator{v} }

func (a tokenAuthenticator) Authenticate(ctx context.Context, token string) (Identity, error) {
	c, err := a.verifier.Verify(ctx, token)
	if err != nil {
		return Identity{}, err
	}
	if c.Subject == "" {
		return Identity{}, errors.New("token has no subject")
	}
	return Identity{Subject: c.Subject, Email: c.Email, SecondStep: c.SecondStep}, nil
}
