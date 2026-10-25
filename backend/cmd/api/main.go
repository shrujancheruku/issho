// Command api serves the Connect API used by the mobile app and admin SPA.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
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
	"github.com/shrujancheruku/issho/backend/internal/chatws"
	"github.com/shrujancheruku/issho/backend/internal/citypacks"
	"github.com/shrujancheruku/issho/backend/internal/curation"
	"github.com/shrujancheruku/issho/backend/internal/health"
	"github.com/shrujancheruku/issho/backend/internal/identity"
	mapsgoogle "github.com/shrujancheruku/issho/backend/internal/maps/google"
	"github.com/shrujancheruku/issho/backend/internal/metrics"
	"github.com/shrujancheruku/issho/backend/internal/moderation"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/config"
	"github.com/shrujancheruku/issho/backend/internal/platform/db"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/github"
	"github.com/shrujancheruku/issho/backend/internal/platform/supabase"
	"github.com/shrujancheruku/issho/backend/internal/profile"
	"github.com/shrujancheruku/issho/backend/internal/push"
	"github.com/shrujancheruku/issho/backend/internal/tagging"
	"github.com/shrujancheruku/issho/backend/internal/tagging/gemini"
	"github.com/shrujancheruku/issho/backend/internal/translate"
	translategemini "github.com/shrujancheruku/issho/backend/internal/translate/gemini"
	"github.com/shrujancheruku/issho/backend/internal/translate/translators"
	translationeditor "github.com/shrujancheruku/issho/backend/internal/translations"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
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

// originHosts turns ALLOWED_ORIGINS ("https://admin.example.com") into the
// host patterns the WebSocket library matches ("admin.example.com").
func originHosts(origins []string) []string {
	var out []string
	for _, o := range origins {
		if u, err := url.Parse(strings.TrimSpace(o)); err == nil && u.Host != "" {
			out = append(out, u.Host)
		}
	}
	return out
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
	suspensions := &moderation.Suspensions{Q: q}
	// errs runs outermost so it also sanitises errors returned by the auth interceptor.
	opts := connect.WithInterceptors(
		errs.Interceptor(slog.Default()),
		// UIService is public because the sign-in screens need their text
		// before anyone is signed in; it only translates the server's own
		// copy of the UI catalogue.
		auth.Interceptor(verifier, isshov1connect.HealthServiceCheckProcedure, isshov1connect.UIServiceGetStringsProcedure),
		// The admin SPA's services: only accounts with app_metadata.admin.
		auth.AdminOnly(isshov1connect.ModerationServiceName, isshov1connect.ReviewServiceName, isshov1connect.CurationServiceName,
			isshov1connect.MetricsServiceName, isshov1connect.CityPackServiceName),
		// The translation editor: admins, and translators for their languages.
		auth.TranslatorsOnly(isshov1connect.TranslationServiceName),
		// Suspended people may only see that they are, sign out (which
		// unregisters push) and delete their account.
		moderation.Gate(suspensions, isshov1connect.IdentityServiceGetMeProcedure,
			isshov1connect.IdentityServiceDeleteAccountProcedure, isshov1connect.PushServiceUnregisterPushTokenProcedure),
	)

	mux := http.NewServeMux()
	healthH := &health.Handler{Pool: pool, Version: version}
	mux.Handle(isshov1connect.NewHealthServiceHandler(healthH, opts))
	mux.HandleFunc("GET /up", healthH.Up)
	identitySvc := &identity.Service{Store: q}
	if cfg.SupabaseSecretKey != "" {
		identitySvc.Admin = supabase.NewAdmin(cfg.SupabaseURL, cfg.SupabaseSecretKey)
	} else {
		slog.Warn("SUPABASE_SECRET_KEY not set; account deletion disabled")
	}
	mux.Handle(isshov1connect.NewIdentityServiceHandler(&identity.Handler{Svc: identitySvc}, opts))
	translations := &translate.Service{Store: q}
	if translations.Translator, err = translators.New(cfg.Translator, cfg.GoogleTranslateAPIKey, cfg.GeminiAPIKey); err != nil {
		return err
	} else if translations.Translator == nil {
		slog.Warn("no key for the translator (GOOGLE_TRANSLATE_API_KEY, or GOOGLE_GEMINI_API_KEY with TRANSLATOR=gemini): machine translation off, text shown in stored languages only")
	}
	suggestions := &tagging.Service{Store: q}
	if cfg.GeminiAPIKey != "" {
		suggestions.Tagger = gemini.New(cfg.GeminiAPIKey)
	} else {
		slog.Warn("GOOGLE_GEMINI_API_KEY not set: interest suggestions off")
	}
	mux.Handle(isshov1connect.NewCatalogServiceHandler(&catalog.Handler{Q: q, Translations: translations, Suggestions: suggestions}, opts))
	mux.Handle(isshov1connect.NewProfileServiceHandler(&profile.Handler{Svc: &profile.Service{DB: pool}}, opts))
	planSvc := &plans.Service{DB: pool, Translations: translations}
	venues := &curation.Handler{DB: pool}
	packs := &citypacks.Handler{DB: pool, Base: "main"}
	var gh *github.Client
	if cfg.GitHubToken != "" {
		gh = github.New(cfg.GitHubToken, cfg.GitHubRepo)
		gh.API = cfg.GitHubAPI
		packs.GitHub, packs.Base = gh, gh.Base
	} else {
		slog.Warn("GITHUB_TOKEN not set: the city-pack editor can't read or propose packs, and the translation editor can't save")
	}
	// The worker's timing, so a plan an admin opens gets the same window.
	review := &plans.ReviewHandler{Svc: planSvc, Timing: plans.DefaultTiming()}
	if cfg.GoogleMapsAPIKey != "" {
		places := mapsgoogle.New(cfg.GoogleMapsAPIKey)
		planSvc.Places, venues.Places = places, places
		review.Places, review.Routes = places, places
		packs.Router, packs.Places = places, places
	} else {
		slog.Warn("GOOGLE_MAPS_API_KEY not set: plans show no venue names")
	}
	mux.Handle(isshov1connect.NewPlanServiceHandler(&plans.Handler{Svc: planSvc}, opts))
	mux.Handle(isshov1connect.NewEventServiceHandler(&plans.EventHandler{Svc: planSvc}, opts))
	mux.Handle(isshov1connect.NewCrewServiceHandler(&plans.CrewHandler{Svc: planSvc}, opts))
	mux.Handle(isshov1connect.NewPushServiceHandler(&push.Handler{Q: q}, opts))
	mux.Handle(isshov1connect.NewReviewServiceHandler(review, opts))
	mux.Handle(isshov1connect.NewCurationServiceHandler(venues, opts))
	mux.Handle(isshov1connect.NewMetricsServiceHandler(&metrics.Handler{Q: q}, opts))
	mux.Handle(isshov1connect.NewCityPackServiceHandler(packs, opts))
	mux.Handle(isshov1connect.NewModerationServiceHandler(&moderation.Handler{Q: q, Plans: planSvc, Suspensions: suspensions}, opts))
	uiCatalog, err := uistrings.Load()
	if err != nil {
		return err
	}
	mux.Handle(isshov1connect.NewUIServiceHandler(&uistrings.Handler{Catalog: uiCatalog, Translations: translations}, opts))
	translationEditor := &translationeditor.Handler{Q: q, Catalog: uiCatalog, Translations: translations, Base: "main"}
	if cfg.GeminiAPIKey != "" {
		translationEditor.Drafter = translategemini.New(cfg.GeminiAPIKey)
	}
	if gh != nil {
		translationEditor.GitHub, translationEditor.Base = gh, gh.Base
		// Merged human versions are served without a deploy.
		go uiCatalog.Follow(ctx, gh, gh.Base, 5*time.Minute, slog.Default())
	}
	mux.Handle(isshov1connect.NewTranslationServiceHandler(translationEditor, opts))

	// Group chat pushes new messages over WebSocket (internal/chatws); one
	// LISTEN connection per process wakes the sockets.
	chatHub := chatws.NewHub()
	go chatHub.Run(ctx, pool, slog.Default())
	mux.Handle("/ws/chat", &chatws.Handler{
		Hub: chatHub, Svc: planSvc, Verifier: verifier, Suspended: suspensions.Suspended, Log: slog.Default(),
		OriginPatterns: originHosts(cfg.AllowedOrigins),
	})

	handler := withCORS(mux, cfg.AllowedOrigins)

	// Serve HTTP/1.1 and cleartext HTTP/2; TLS terminates at kamal-proxy/Cloudflare.
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	srv := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           handler,
		Protocols:         protocols,
		ReadHeaderTimeout: 10 * time.Second,
		// Request contexts end when the process is told to stop. Shutdown
		// doesn't track WebSockets (hijacked connections); this ends them.
		BaseContext: func(net.Listener) context.Context { return ctx },
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
