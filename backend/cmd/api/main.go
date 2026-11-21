package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/shrujancheruku/issho/backend/internal/rpc"
	"github.com/shrujancheruku/issho/backend/internal/serve"
)

// version is set at build time (`-ldflags "-X main.version=1.2.3"`); a plain
// `go build` reports "dev".
var version = "dev"

func main() {
	// SIGTERM is what the host sends on a deploy; SIGINT is Ctrl-C locally.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil)).With("cmd", "api")
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	// No Auth yet: the token verifier is #608's, and until it exists only the
	// public RPCs can be called.
	if err := serve.HTTP(ctx, log, addr, rpc.NewHandler(rpc.Config{Version: version})); err != nil {
		log.Error("exited", "err", err)
		os.Exit(1)
	}
}
