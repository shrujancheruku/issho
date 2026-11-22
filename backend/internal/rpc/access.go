package rpc

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

// Identity is who a token says the caller is, plus what the API knows of them.
type Identity struct {
	Subject string
	// Email is the address the token was issued for. It is the caller's own and
	// never goes in a log (the request log is built from route and Subject only).
	Email string
	// SecondStep is whether the person took the second sign-in step (the
	// token's `aal2`).
	SecondStep bool
	// Making is whether they are the owner or an insider, and Admin whether they
	// are staff. Neither is in the token: the verifier reads them from the
	// staff table on each request, so removing a role takes effect at once.
	Making bool
	Admin  bool
}

// Authenticator turns a bearer token into an Identity, or says why it can't.
// NewAuthenticator is the one that checks Supabase Auth's tokens; a process with
// none configured runs with one that accepts nothing.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (Identity, error)
}

// denyAll is the Authenticator when none is configured: every token is refused,
// so only public RPCs can be called.
type denyAll struct{}

func (denyAll) Authenticate(context.Context, string) (Identity, error) {
	return Identity{}, errors.New("no authenticator configured")
}

type identityKey struct{}

// IdentityFrom is the caller of a handler that passed the interceptor with a
// token: a member, Making or admin RPC always has one. A public RPC has one
// only when the caller sent a valid token.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)
	return id, ok
}

// AccessInterceptor enforces the `access` option each RPC declares in its
// proto. It denies by default: an RPC with no option, or one set to
// ACCESS_UNSPECIFIED, is refused for everyone, so a forgotten rule can't open
// an RPC (`make rpc-auth` catches it before it gets here).
type AccessInterceptor struct {
	auth  Authenticator
	files *protoregistry.Files
}

// NewAccessInterceptor reads rules from files, the registry that holds the
// services' descriptors: protoregistry.GlobalFiles in the API, a private one in
// a test that serves a service of its own.
func NewAccessInterceptor(auth Authenticator, files *protoregistry.Files) *AccessInterceptor {
	if auth == nil {
		auth = denyAll{}
	}
	return &AccessInterceptor{auth: auth, files: files}
}

func (a *AccessInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := a.check(ctx, req.Spec().Procedure, req.Header().Get("Authorization"))
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (a *AccessInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // a client sends its own token; there is nothing to enforce
}

func (a *AccessInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := a.check(ctx, conn.Spec().Procedure, conn.RequestHeader().Get("Authorization"))
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (a *AccessInterceptor) check(ctx context.Context, procedure, authorization string) (context.Context, error) {
	// A procedure is always one a registered service owns: Connect routes no
	// other, so it is safe to log.
	note(ctx, procedure, "")
	rule := a.rule(procedure)
	if rule == isshov1.Access_ACCESS_UNSPECIFIED {
		// A server bug, not the caller's doing, so it says nothing of why.
		return ctx, connect.NewError(connect.CodeInternal, errors.New("rpc has no access rule"))
	}
	if rule == isshov1.Access_ACCESS_PUBLIC {
		return a.withOptionalIdentity(ctx, authorization), nil
	}

	token, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok || token == "" {
		return ctx, connect.NewError(connect.CodeUnauthenticated, errors.New("sign in first"))
	}
	id, err := a.auth.Authenticate(ctx, token)
	if err != nil {
		return ctx, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token"))
	}

	switch rule {
	case isshov1.Access_ACCESS_MEMBER:
		// Any signed-in caller: nothing more to check.
	case isshov1.Access_ACCESS_MAKING:
		if !id.Making {
			return ctx, connect.NewError(connect.CodePermissionDenied, errors.New("not allowed"))
		}
	case isshov1.Access_ACCESS_ADMIN:
		if !id.Admin || !id.SecondStep {
			return ctx, connect.NewError(connect.CodePermissionDenied, errors.New("not allowed"))
		}
	default:
		// A rule this code has no case for must not let anyone in: deny by
		// default, and say nothing of why (a server bug, not the caller's).
		return ctx, connect.NewError(connect.CodeInternal, errors.New("rpc has an unknown access rule"))
	}
	note(ctx, "", id.Subject)
	return context.WithValue(ctx, identityKey{}, id), nil
}

// withOptionalIdentity lets a public RPC see who is calling when the caller
// sent a good token, and ignores a missing or bad one: public means anyone.
func (a *AccessInterceptor) withOptionalIdentity(ctx context.Context, authorization string) context.Context {
	token, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok || token == "" {
		return ctx
	}
	id, err := a.auth.Authenticate(ctx, token)
	if err != nil {
		return ctx
	}
	note(ctx, "", id.Subject)
	return context.WithValue(ctx, identityKey{}, id)
}

// rule is the access option of the RPC named by a Connect procedure
// ("/issho.v1.HealthService/Check"), or ACCESS_UNSPECIFIED if the procedure
// has none or isn't in the registry.
func (a *AccessInterceptor) rule(procedure string) isshov1.Access {
	service, method, ok := strings.Cut(strings.TrimPrefix(procedure, "/"), "/")
	if !ok {
		return isshov1.Access_ACCESS_UNSPECIFIED
	}
	desc, err := a.files.FindDescriptorByName(protoreflect.FullName(service + "." + method))
	if err != nil {
		return isshov1.Access_ACCESS_UNSPECIFIED
	}
	m, ok := desc.(protoreflect.MethodDescriptor)
	if !ok {
		return isshov1.Access_ACCESS_UNSPECIFIED
	}
	rule, _ := proto.GetExtension(m.Options(), isshov1.E_Access).(isshov1.Access)
	return rule
}
