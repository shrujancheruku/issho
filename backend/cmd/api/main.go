package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/shrujancheruku/issho/backend/internal/serve"
)

func main() {
	// SIGTERM is what the host sends on a deploy; SIGINT is Ctrl-C locally.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil)).With("cmd", "api")
	if err := serve.Run(ctx, log); err != nil {
		log.Error("exited", "err", err)
		os.Exit(1)
	}
}
