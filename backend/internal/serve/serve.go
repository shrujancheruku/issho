// Package serve is the lifecycle both commands share: log that the process is
// up, then block until the signal-bound context ends. The real work (the
// ConnectRPC server in #595, River in #594) hangs off Run later.
package serve

import (
	"context"
	"log/slog"
)

func Run(ctx context.Context, log *slog.Logger) error {
	log.Info("started")
	<-ctx.Done()
	log.Info("stopped")
	return nil
}
