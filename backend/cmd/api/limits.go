package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/platform/deadline"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/validate"
)

// maxRequestBytes caps every Connect request body. The largest real ones are
// a city pack in ProposeCityPack (the biggest shipped pack is ~15 kB) and
// translation saves, so 1 MiB is far above any legitimate call. A handler
// that needs more gets its own limit here, with the reason; none does yet.
const maxRequestBytes = 1 << 20

// Server timeouts. There is deliberately no WriteTimeout: /ws/chat is served
// by this same server and a write deadline would cut long-lived sockets
// (Hijack clears the read deadline, but nothing resets a write deadline for
// the frames the hub pushes later). Slow unary responses are bounded by the
// RPC deadline instead.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	idleTimeout       = 120 * time.Second
)

// rpcDeadlines are the unary RPCs that legitimately outlive deadline.Default
// (each makes many upstream calls). 0 means no deadline of ours: the admin's
// request is still cancelled when they close the page.
var rpcDeadlines = map[string]time.Duration{
	// ApplyCityPack and DiscoverAreaVenues used to be here (a pack apply
	// rebuilds travel times; a venue search makes hundreds of calls). They are
	// River jobs now (internal/adminjobs): the RPC only validates and queues,
	// so the default deadline fits.
	// Validates against the default branch, then opens a pull request.
	isshov1connect.CityPackServiceProposeCityPackProcedure: 2 * time.Minute,
	// A model call per draft, which gives up after its own 60 s budget
	// (gemini.InteractiveBudget); the rest is the lookups before it. Under
	// Cloudflare's 100 s cut-off.
	isshov1connect.TranslationServiceDraftAppStringProcedure: 90 * time.Second,
}

// serverTimeouts are the http.Server's; production uses prodTimeouts, tests
// shrink them. There is deliberately no write timeout field.
type serverTimeouts struct{ ReadHeader, Read, Idle time.Duration }

var prodTimeouts = serverTimeouts{ReadHeader: readHeaderTimeout, Read: readTimeout, Idle: idleTimeout}

// newRPCServer is the Connect server every service registers on. Its chain:
// errs outermost (so it also sanitises what the others return), the deadline
// inside it (so its DeadlineExceeded passes through as is), then the caller's
// interceptors (auth and so on), then request validation (so a caller who
// isn't signed in hears Unauthenticated, not the rules).
func newRPCServer(log *slog.Logger, inner ...connect.ServerInterceptor) *connect.Server {
	chain := append([]connect.ServerInterceptor{
		errs.Interceptor(log),
		deadline.Interceptor(deadline.Default, rpcDeadlines),
	}, inner...)
	chain = append(chain, validate.Interceptor())
	return connect.NewServer(chain...)
}

// mountRPC serves rpc's services on mux with the request size cap. Mount
// routes only the services registered so far, so it runs after the last one.
func mountRPC(mux *http.ServeMux, rpc *connect.Server) {
	connecthttp.Mount(mux, rpc, connecthttp.WithReadMaxBytes(maxRequestBytes))
}

func newServer(addr string, h http.Handler, protocols *http.Protocols, baseCtx context.Context, t serverTimeouts) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		Protocols:         protocols,
		ReadHeaderTimeout: t.ReadHeader,
		ReadTimeout:       t.Read,
		IdleTimeout:       t.Idle,
		// Not the signal context: that would cancel in-flight requests the
		// moment SIGTERM arrives, before Shutdown drains them. baseCancel
		// runs after Shutdown, to end the WebSockets it doesn't track
		// (hijacked connections).
		BaseContext: func(net.Listener) context.Context { return baseCtx },
	}
}
