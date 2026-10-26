// Package auth verifies Supabase access tokens and carries the caller's identity
// through request contexts.
package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	// Admin is set in Supabase (app_metadata.admin, by `tool set-admin`)
	// and carried in the access token, so a change takes effect when the
	// admin's token is next refreshed (within the token lifetime, 1 h).
	Admin bool
	// TranslatorLangs are the languages a translator may edit in the admin
	// SPA's translation editor (app_metadata.translator_langs, by `tool
	// set-translator`), carried in the token like Admin. Admins may edit
	// every language without it.
	TranslatorLangs []string
}

// Translates reports whether the caller may edit lang's translations.
func (c Caller) Translates(lang string) bool {
	return c.Admin || slices.Contains(c.TranslatorLangs, lang)
}

type claims struct {
	Email string `json:"email"`
	// Only app_metadata: Supabase lets nobody but the service role change
	// it. user_metadata is editable by the user and must never grant
	// anything.
	AppMetadata struct {
		Admin           bool     `json:"admin"`
		TranslatorLangs []string `json:"translator_langs"`
	} `json:"app_metadata"`
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
	// keyfunc caches the JWKS and refreshes it in the background (and on unknown
	// key IDs, rate-limited), so key rotation in Supabase needs no API restart.
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

// Verify validates signature, issuer, audience and expiry, then extracts the caller.
//
// Only asymmetric algorithms are accepted. Pinning the method list prevents
// algorithm-confusion attacks (e.g. a token signed with HS256 using the public
// key as the HMAC secret) and rejects legacy Supabase HS256 tokens, which are
// signed with a shared secret we deliberately never load into the API.
// Audience "authenticated" excludes anon/service_role tokens, which Supabase
// signs with the same keys.
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
	return Caller{UserID: id, Email: c.Email, Admin: c.AppMetadata.Admin, TranslatorLangs: c.AppMetadata.TranslatorLangs}, nil
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
// Auth is deny-by-default: a newly added RPC requires a valid token unless it is
// explicitly listed as public where the interceptor is constructed (cmd/api).
// Error messages are intentionally generic so callers can't probe why a token failed.
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

// AdminOnly rejects calls to the given services (e.g. "issho.v1.ModerationService")
// unless the caller is an admin. It runs after Interceptor; listing the
// services where cmd/api builds it keeps every admin RPC covered, new ones
// included, without each handler having to remember a check.
func AdminOnly(services ...string) connect.UnaryInterceptorFunc {
	return guard(services, "admins only", func(c Caller) bool { return c.Admin })
}

// TranslatorsOnly rejects calls to the given services unless the caller is
// an admin or a translator (has at least one language). Which languages a
// translator may edit is checked by the handler (Caller.Translates); this
// keeps everyone else out of the whole service, new RPCs included.
func TranslatorsOnly(services ...string) connect.UnaryInterceptorFunc {
	return guard(services, "translators only", func(c Caller) bool { return c.Admin || len(c.TranslatorLangs) > 0 })
}

// guard refuses calls to the services unless there's a caller that allowed
// accepts.
func guard(services []string, refusal string, allowed func(Caller) bool) connect.UnaryInterceptorFunc {
	prefixes := make([]string, len(services))
	for i, s := range services {
		prefixes[i] = "/" + s + "/"
	}
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			for _, p := range prefixes {
				if strings.HasPrefix(req.Spec().Procedure, p) {
					if c, ok := FromContext(ctx); !ok || !allowed(c) {
						return nil, connect.NewError(connect.CodePermissionDenied, errors.New(refusal))
					}
					break
				}
			}
			return next(ctx, req)
		}
	}
}

// Require returns the signed-in caller, for handlers of non-public
// procedures (Interceptor has already checked the token).
func Require(ctx context.Context) (Caller, error) {
	c, ok := FromContext(ctx)
	if !ok {
		return c, connect.NewError(connect.CodeUnauthenticated, errors.New("no caller"))
	}
	return c, nil
}

// RequireAdmin returns the caller if they're an admin. Admin handlers call
// it although AdminOnly already guards their service, so a wiring mistake
// in cmd/api fails closed.
func RequireAdmin(ctx context.Context) (Caller, error) {
	c, err := Require(ctx)
	if err != nil {
		return c, err
	}
	if !c.Admin {
		return c, connect.NewError(connect.CodePermissionDenied, errors.New("admins only"))
	}
	return c, nil
}
