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
	"github.com/shrujancheruku/issho/backend/internal/outings"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
)

// Mount makes one more service of the process, handed the access interceptor
// every service here is served behind: it is the shape of the generated
// `New<Name>ServiceHandler`, so `isshov1connect.NewHealthServiceHandler(h, opts...)`
// is a Mount once `h` is bound. Only a build with a service of its own sets any
// (the end-to-end build's clock, cmd/api/e2e_on.go); the production process sets
// none.
type Mount func(opts ...connect.HandlerOption) (path string, handler http.Handler)

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
	// Signups keeps who is signed up to which plan. With People it serves
	// SignupService, and with Cities and Plans it puts each plan's count of
	// signups on the plan RPCs; nil leaves both out.
	Signups *plans.Signups
	// Outings keeps the groups matching formed and serves OutingService; nil
	// leaves it unserved, as it is for a process with no database.
	Outings *outings.Service
	// Mounts are further services, served behind the same interceptor.
	Mounts []Mount
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
		mux.Handle(isshov1connect.NewProfileServiceHandler(profileService{people: cfg.People}, opts))
		mux.Handle(isshov1connect.NewJoinLinkServiceHandler(joinLinkService{people: cfg.People}, opts))
	}
	if cfg.Cities != nil && cfg.Plans != nil {
		mux.Handle(isshov1connect.NewPlanServiceHandler(planService{cities: cfg.Cities, plans: cfg.Plans, signups: cfg.Signups}, opts))
	}
	if cfg.People != nil && cfg.Signups != nil {
		mux.Handle(isshov1connect.NewSignupServiceHandler(signupService{people: cfg.People, signups: cfg.Signups}, opts))
	}
	if cfg.Outings != nil {
		mux.Handle(isshov1connect.NewOutingServiceHandler(outingService{outings: cfg.Outings}, opts))
	}
	for _, mount := range cfg.Mounts {
		mux.Handle(mount(opts))
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
