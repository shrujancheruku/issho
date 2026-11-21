// Package rpc is the API's ConnectRPC surface: the services, and the
// interceptor that enforces the access rule each RPC declares in its proto.
package rpc

import (
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
)

// Config is what the handler needs from the process that builds it.
type Config struct {
	// Version is the build's version, reported by the health check.
	Version string
	// Auth verifies bearer tokens. Nil refuses every token, which leaves only
	// the public RPCs callable (the verifier is #608's).
	Auth Authenticator
}

// NewHandler serves every service of the API.
func NewHandler(cfg Config) http.Handler {
	// Every service is served behind the access interceptor, so no RPC is
	// reachable without passing its declared rule.
	opts := connect.WithInterceptors(NewAccessInterceptor(cfg.Auth, protoregistry.GlobalFiles))
	mux := http.NewServeMux()
	mux.Handle(isshov1connect.NewHealthServiceHandler(healthService{version: cfg.Version}, opts))
	return mux
}
