// Package serve is the lifecycle both commands share: log that the process is
// up, then block until the signal-bound context ends.
package serve

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/logging"
)

// Run is for a process with no listener of its own (the worker, whose River
// client runs beside it).
func Run(ctx context.Context, log *slog.Logger) error {
	log.Info("started")
	<-ctx.Done()
	log.Info("stopped")
	return nil
}

// HTTP serves h on addr until ctx ends, then stops accepting and lets requests
// in flight finish. It logs the address it bound, which says the port when addr
// asks for any free one (":0").
func HTTP(ctx context.Context, log *slog.Logger, addr string, h http.Handler) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler: h,
		// Without it a client can hold a connection open by never finishing its
		// headers. The body and response get no timeout: RPCs may stream.
		ReadHeaderTimeout: 10 * time.Second,
		// net/http's own errors go through the helper, which keeps none of their
		// text: it can name the client's address.
		ErrorLog: logging.ServerErrorLog(log),
	}
	failed := make(chan error, 1)
	go func() { failed <- srv.Serve(ln) }()
	log.Info("started", "addr", ln.Addr().String())

	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}
	// ctx is already done, so the grace period needs a context of its own.
	grace, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(grace); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Info("stopped")
	return nil
}
