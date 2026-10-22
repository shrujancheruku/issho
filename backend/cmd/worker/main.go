// Command worker runs background jobs (River): matching passes that open
// plans and schedule them.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/config"
	"github.com/shrujancheruku/issho/backend/internal/platform/db"
	"github.com/shrujancheruku/issho/backend/internal/push"
	"github.com/shrujancheruku/issho/backend/internal/translate"
	gtranslate "github.com/shrujancheruku/issho/backend/internal/translate/google"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
	"github.com/shrujancheruku/issho/backend/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadWorker()
	if err != nil {
		return err
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	translations := &translate.Service{Store: dbgen.New(pool)}
	if cfg.GoogleTranslateAPIKey != "" {
		translations.Translator = gtranslate.New(cfg.GoogleTranslateAPIKey)
	} else {
		slog.Warn("GOOGLE_TRANSLATE_API_KEY not set: notifications in English only")
	}
	catalog, err := uistrings.Load()
	if err != nil {
		return err
	}
	notifier := &plans.Notifier{
		Plans:   &plans.Service{DB: pool, Translations: translations},
		Sender:  push.NewExpo(cfg.ExpoAccessToken),
		Catalog: catalog,
		Log:     slog.Default(),
	}
	client, err := worker.NewClient(pool, slog.Default(), plans.DefaultTiming(), notifier)
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	slog.Info("worker started")
	<-ctx.Done()
	// Let running jobs finish, but not forever.
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return client.Stop(stopCtx)
}
