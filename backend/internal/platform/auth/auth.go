// Package auth verifies Supabase access tokens and carries the caller's identity
// through request contexts.
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Caller is the authenticated user making a request.
type Caller struct {
	UserID uuid.UUID
	Email  string
}

type claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// Verifier checks Supabase-issued JWTs against the project's JWKS.
type Verifier struct {
	keyfunc jwt.Keyfunc
	issuer  string
}

// NewVerifier fetches and keeps refreshing the signing keys of a Supabase project.
// It requires asymmetric JWT signing keys to be enabled in the project.
func NewVerifier(ctx context.Context, supabaseURL string) (*Verifier, error) {
	k, err := keyfunc.NewDefaultCtx(ctx, []string{supabaseURL + "/auth/v1/.well-known/jwks.json"})
	if err != nil {
		return nil, fmt.Errorf("load jwks: %w", err)
	}
	return NewVerifierWithKeyfunc(k.Keyfunc, supabaseURL), nil
}

// NewVerifierWithKeyfunc builds a Verifier from any key source; tests use it with a local key.
func NewVerifierWithKeyfunc(kf jwt.Keyfunc, supabaseURL string) *Verifier {
	return &Verifier{keyfunc: kf, issuer: supabaseURL + "/auth/v1"}
}

func (v *Verifier) Verify(token string) (Caller, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, v.keyfunc,
		jwt.WithValidMethods([]string{"ES256", "RS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience("authenticated"),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Caller{}, err
	}
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return Caller{}, fmt.Errorf("invalid subject: %w", err)
	}
	return Caller{UserID: id, Email: c.Email}, nil
}

type ctxKey struct{}

// FromContext returns the caller set by the interceptor.
func FromContext(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(ctxKey{}).(Caller)
	return c, ok
}

// WithCaller attaches a caller to ctx. Exposed for tests of downstream handlers.
func WithCaller(ctx context.Context, c Caller) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// Interceptor rejects unauthenticated calls, except to procedures listed in public.
func Interceptor(v *Verifier, public ...string) connect.UnaryInterceptorFunc {
	open := make(map[string]bool, len(public))
	for _, p := range public {
		open[p] = true
	}
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if open[req.Spec().Procedure] {
				return next(ctx, req)
			}
			token, ok := strings.CutPrefix(req.Header().Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer token"))
			}
			caller, err := v.Verify(token)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token"))
			}
			return next(WithCaller(ctx, caller), req)
		}
	}
}
