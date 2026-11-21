// Package rpc is the API's ConnectRPC surface: the services, and the
// interceptor that enforces the access rule each RPC declares in its proto.
package rpc

import (
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/clock"
)

// Config is what the handler needs from the process that builds it.
type Config struct {
	// Version is the build's version, reported by the health check.
	Version string
	// Auth verifies bearer tokens. Nil refuses every token, which leaves only
	// the public RPCs callable (the verifier is #608's).
	Auth Authenticator
	// Log gets one line per request (see logRequests), timed by Clock. Nil logs
	// nothing; set, Clock is required.
	Log   *slog.Logger
	Clock clock.Clock
}

// NewHandler serves every service of the API.
func NewHandler(cfg Config) http.Handler {
	// Every service is served behind the access interceptor, so no RPC is
	// reachable without passing its declared rule.
	opts := connect.WithInterceptors(NewAccessInterceptor(cfg.Auth, protoregistry.GlobalFiles))
	mux := http.NewServeMux()
	mux.Handle(isshov1connect.NewHealthServiceHandler(healthService{version: cfg.Version}, opts))
	var rpcs http.Handler = mux
	if cfg.Log != nil {
		rpcs = logRequests(cfg.Log, cfg.Clock, mux)
	}

	// /up is the deploy's liveness probe: kamal-proxy polls a plain GET every few
	// seconds and sends a new container traffic only once it answers 200. It
	// sits outside the access interceptor (it is not an RPC) and outside the
	// request log, which it would fill with a line per poll.
	top := http.NewServeMux()
	top.HandleFunc("GET /up", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	top.Handle("/", rpcs)
	return top
}
