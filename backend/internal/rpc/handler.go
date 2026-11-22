// Package rpc is the API's ConnectRPC surface: the services, and the
// interceptor that enforces the access rule each RPC declares in its proto.
package rpc

import (
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
)

// Config is what the handler needs from the process that builds it.
type Config struct {
	// Version is the build's version, reported by the health check.
	Version string
	// Auth verifies bearer tokens. Nil refuses every token, which leaves only
	// the public RPCs callable.
	Auth Authenticator
	// People keeps the accounts. Nil leaves AccountService unserved, as it is
	// for a process with no database.
	People *people.Service
	// Cities and Plans keep the listed cities and their plans. Both must be set
	// to serve PlanService; either nil leaves it unserved, as it is for a
	// process with no database.
	Cities *cities.Service
	Plans  *plans.Service
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
	if cfg.People != nil {
		mux.Handle(isshov1connect.NewAccountServiceHandler(accountService{people: cfg.People}, opts))
	}
	if cfg.Cities != nil && cfg.Plans != nil {
		mux.Handle(isshov1connect.NewPlanServiceHandler(planService{cities: cfg.Cities, plans: cfg.Plans}, opts))
	}
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
