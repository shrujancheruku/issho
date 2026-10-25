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
	mapsgoogle "github.com/shrujancheruku/issho/backend/internal/maps/google"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/config"
	"github.com/shrujancheruku/issho/backend/internal/platform/db"
	"github.com/shrujancheruku/issho/backend/internal/push"
	"github.com/shrujancheruku/issho/backend/internal/translate"
	"github.com/shrujancheruku/issho/backend/internal/translate/translators"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
	"github.com/shrujancheruku/issho/backend/internal/walks"
	walksgemini "github.com/shrujancheruku/issho/backend/internal/walks/gemini"
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
	if translations.Translator, err = translators.New(cfg.Translator, cfg.GoogleTranslateAPIKey, cfg.GeminiAPIKey); err != nil {
		return err
	} else if translations.Translator == nil {
		slog.Warn("no key for the translator (GOOGLE_TRANSLATE_API_KEY, or GOOGLE_GEMINI_API_KEY with TRANSLATOR=gemini): notifications in English only")
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
	var m worker.Maps
	if cfg.GoogleMapsAPIKey != "" {
		g := mapsgoogle.New(cfg.GoogleMapsAPIKey)
		m = worker.Maps{Router: g, Hours: g, Routes: g, Places: g, WalkRoutes: g}
		if cfg.GeminiAPIKey != "" {
			m.Walks = &walks.Composer{Proposer: walksgemini.New(cfg.GeminiAPIKey), Places: g, Routes: g, Rules: walks.DefaultRules()}
		} else {
			slog.Warn("GOOGLE_GEMINI_API_KEY not set: no walking plans are opened")
		}
	} else {
		slog.Warn("GOOGLE_MAPS_API_KEY not set: travel times past 30 days are deleted, not rebuilt; venue hours not checked; no walking plans are opened")
	}
	client, err := worker.NewClient(pool, slog.Default(), plans.DefaultTiming(), notifier, m)
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
