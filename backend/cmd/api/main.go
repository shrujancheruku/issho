package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/logging"
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

	log := logging.New(os.Stdout, "api")
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if err := run(ctx, log, addr); err != nil {
		log.Error("exited", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger, addr string) error {
	// The real clock is built here and only here (tools/clockcheck enforces it).
	clk := clock.Real{}
	cfg := rpc.Config{Version: version, Log: log, Clock: clk}
	acc, err := newAccounts(ctx, log, os.Getenv, clk)
	if err != nil {
		return err
	}
	if acc != nil {
		defer acc.close()
		cfg.People, cfg.Auth = acc.people, acc.auth
		cfg.Cities, cfg.Plans = acc.cities, acc.plans
	} else {
		// Without them every token is refused and only the public RPCs work.
		log.Warn("accounts are not configured: DATABASE_URL and SUPABASE_AUTH_URL are unset")
	}
	return serve.HTTP(ctx, log, addr, rpc.NewHandler(cfg))
}
