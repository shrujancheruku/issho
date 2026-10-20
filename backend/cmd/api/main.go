// Command api serves the Connect API used by the mobile app and admin SPA.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/catalog"
	"github.com/shrujancheruku/issho/backend/internal/health"
	"github.com/shrujancheruku/issho/backend/internal/identity"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/config"
	"github.com/shrujancheruku/issho/backend/internal/platform/db"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/supabase"
	"github.com/shrujancheruku/issho/backend/internal/translate"
	gtranslate "github.com/shrujancheruku/issho/backend/internal/translate/google"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("api exited", "err", err)
		os.Exit(1)
	}
}

// withCORS allows browser calls only from the configured origins (the admin SPA).
// Native mobile clients don't send Origin and are unaffected.
//
// An explicit origin check is used instead of rs/cors' AllowedOrigins because an
// empty AllowedOrigins list there means "allow every origin"; here an empty list
// (ALLOWED_ORIGINS unset) allows none.
func withCORS(h http.Handler, origins []string) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[strings.TrimSpace(o)] = true
	}
	return cors.New(cors.Options{
		AllowOriginFunc: func(origin string) bool { return allowed[origin] },
		AllowedMethods:  connectcors.AllowedMethods(),
		AllowedHeaders:  append(connectcors.AllowedHeaders(), "Authorization"),
		ExposedHeaders:  connectcors.ExposedHeaders(),
	}).Handler(h)
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	verifier, err := auth.NewVerifier(ctx, cfg.SupabaseURL)
	if err != nil {
		return err
	}

	q := dbgen.New(pool)
	// errs runs outermost so it also sanitises errors returned by the auth interceptor.
	opts := connect.WithInterceptors(
		errs.Interceptor(slog.Default()),
		auth.Interceptor(verifier, isshov1connect.HealthServiceCheckProcedure),
	)

	mux := http.NewServeMux()
	mux.Handle(isshov1connect.NewHealthServiceHandler(&health.Handler{Pool: pool, Version: version}, opts))
	identitySvc := &identity.Service{Store: q}
	if cfg.SupabaseSecretKey != "" {
		identitySvc.Admin = supabase.NewAdmin(cfg.SupabaseURL, cfg.SupabaseSecretKey)
	} else {
		slog.Warn("SUPABASE_SECRET_KEY not set; account deletion disabled")
	}
	mux.Handle(isshov1connect.NewIdentityServiceHandler(&identity.Handler{Svc: identitySvc}, opts))
	translations := &translate.Service{Store: q}
	if cfg.GoogleTranslateAPIKey != "" {
		translations.Translator = gtranslate.New(cfg.GoogleTranslateAPIKey)
	} else {
		slog.Warn("GOOGLE_TRANSLATE_API_KEY not set: machine translation off, text shown in stored languages only")
	}
	mux.Handle(isshov1connect.NewCatalogServiceHandler(&catalog.Handler{Q: q, Translations: translations}, opts))

	handler := withCORS(mux, cfg.AllowedOrigins)

	// Serve HTTP/1.1 and cleartext HTTP/2; TLS terminates at Caddy/Cloudflare.
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	srv := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           handler,
		Protocols:         protocols,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", cfg.APIAddr, "version", version)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
