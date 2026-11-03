package main

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
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
	// Seeds a pack, then rebuilds the city's travel times with Google:
	// one routes call per pair of areas.
	isshov1connect.CityPackServiceApplyCityPackProcedure: 0,
	// One Nearby Search per venue category, for the area.
	isshov1connect.CityPackServiceDiscoverAreaVenuesProcedure: 2 * time.Minute,
	// Validates against the default branch, then opens a pull request.
	isshov1connect.CityPackServiceProposeCityPackProcedure: 2 * time.Minute,
	// A model call per draft.
	isshov1connect.TranslationServiceDraftAppStringProcedure: 2 * time.Minute,
}

func newServer(addr string, h http.Handler, protocols *http.Protocols, baseCtx context.Context) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		Protocols:         protocols,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
		// Not the signal context: that would cancel in-flight requests the
		// moment SIGTERM arrives, before Shutdown drains them. baseCancel
		// runs after Shutdown, to end the WebSockets it doesn't track
		// (hijacked connections).
		BaseContext: func(net.Listener) context.Context { return baseCtx },
	}
}
