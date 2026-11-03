// Package config loads process configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Environment names what a deployment is. Production refuses to start
// without the keys it needs; the others keep the optional, warn-only
// behaviour so a laptop or a preview runs on a few keys.
const (
	EnvDevelopment = "development"
	EnvPreview     = "preview"
	EnvProduction  = "production"
)

// Common holds the settings the API, the worker and the tool all read, so
// each is parsed once.
type Common struct {
	// Env is APP_ENV: development (the default), preview or production.
	// Anything else is a startup error, so a typo never silently runs a
	// deployment with production's checks off.
	Env string
	// GoogleTranslateAPIKey enables machine translation by Cloud
	// Translation. Optional outside production.
	GoogleTranslateAPIKey string
	// Translator picks the machine translator (TRANSLATOR): "gemini", the
	// default, which uses GeminiAPIKey, or "google" (internal/translate/translators).
	Translator string
	// GeminiAPIKey (a Google Cloud Agent Platform key) enables interest
	// suggestions, walking plans and the default translator. Optional
	// outside production.
	GeminiAPIKey string
	// GoogleMapsAPIKey enables venue names, travel-time rebuilds, opening
	// hours and walks. Optional outside production.
	GoogleMapsAPIKey string
	// GitHubToken (a fine-grained token for GitHubRepo with Contents and
	// Pull requests write) enables the city-pack and translation editors,
	// and lets both processes follow merged UI translations.
	GitHubToken string
	// GitHubRepo is "owner/name" (default shrujancheruku/issho);
	// GitHubAPI is GitHub's API address (default https://api.github.com;
	// GitHub Enterprise, or a local fake for checks).
	GitHubRepo, GitHubAPI string
	// SentryDSN turns on error reporting (internal/platform/sentry);
	// SentryEnvironment names the deployment (SENTRY_ENVIRONMENT, default
	// APP_ENV; an explicit value wins).
	SentryDSN, SentryEnvironment string
}

func loadCommon() (Common, error) {
	env := getenv("APP_ENV", EnvDevelopment)
	switch env {
	case EnvDevelopment, EnvPreview, EnvProduction:
	default:
		return Common{}, fmt.Errorf("APP_ENV: %q is not development, preview or production", env)
	}
	return Common{
		Env:                   env,
		GoogleTranslateAPIKey: os.Getenv("GOOGLE_TRANSLATE_API_KEY"),
		Translator:            os.Getenv("TRANSLATOR"),
		GeminiAPIKey:          os.Getenv("GOOGLE_GEMINI_API_KEY"),
		GoogleMapsAPIKey:      os.Getenv("GOOGLE_MAPS_API_KEY"),
		GitHubToken:           os.Getenv("GITHUB_TOKEN"),
		GitHubRepo:            getenv("GITHUB_REPO", "shrujancheruku/issho"),
		GitHubAPI:             getenv("GITHUB_API_URL", "https://api.github.com"),
		SentryDSN:             os.Getenv("SENTRY_DSN"),
		SentryEnvironment:     getenv("SENTRY_ENVIRONMENT", env),
	}, nil
}

// Production reports whether this is the production deployment.
func (c Common) Production() bool { return c.Env == EnvProduction }

// translatorKey returns the key the selected translator reads and whether
// it is set. An unknown TRANSLATOR returns no name: translators.New reports
// it at startup.
func (c Common) translatorKey() (name string, set bool) {
	switch c.Translator {
	case "", "gemini":
		return "GOOGLE_GEMINI_API_KEY", c.GeminiAPIKey != ""
	case "google":
		return "GOOGLE_TRANSLATE_API_KEY", c.GoogleTranslateAPIKey != ""
	}
	return "", true
}

// required is one key a production process can't run without.
type required struct {
	name string
	set  bool
}

// missing returns one error naming every unset key, or nil. The names are
// deduplicated: the translator's key can also be a key listed on its own.
func missing(keys []required) error {
	var names []string
	for _, k := range keys {
		if k.set || k.name == "" {
			continue
		}
		dup := false
		for _, n := range names {
			dup = dup || n == k.name
		}
		if !dup {
			names = append(names, k.name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	return fmt.Errorf("APP_ENV=production requires %s", strings.Join(names, ", "))
}

// Config is cmd/api's configuration.
type Config struct {
	Common
	DatabaseURL string
	SupabaseURL string
	// SupabaseSecretKey (sb_secret_…) enables admin calls such as account
	// deletion. Optional outside production: without it, DeleteAccount
	// returns FAILED_PRECONDITION.
	SupabaseSecretKey string
	APIAddr           string
	// AllowedOrigins lists browser origins (the admin SPA) allowed to call the API.
	AllowedOrigins []string
	// SentryTracesRate is the share of API requests traced, 0 to 1
	// (SENTRY_TRACES_SAMPLE_RATE, default 1: every request while traffic is
	// small; lower it as traffic grows). Requests the app traced are always
	// traced, so both halves link up.
	SentryTracesRate float64
}

// Load reads the API's configuration. In production it fails naming every
// missing key at once; elsewhere only DATABASE_URL and SUPABASE_URL are
// required and the rest warn at startup.
func Load() (Config, error) {
	common, err := loadCommon()
	if err != nil {
		return Config{}, err
	}
	c := Config{
		Common:            common,
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		SupabaseURL:       strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		APIAddr:           getenv("API_ADDR", ":8080"),
		SupabaseSecretKey: os.Getenv("SUPABASE_SECRET_KEY"),
	}
	rate, err := strconv.ParseFloat(getenv("SENTRY_TRACES_SAMPLE_RATE", "1"), 64)
	if err != nil || rate < 0 || rate > 1 {
		return c, fmt.Errorf("SENTRY_TRACES_SAMPLE_RATE: want a number from 0 to 1")
	}
	c.SentryTracesRate = rate
	if v := os.Getenv("ALLOWED_ORIGINS"); v != "" {
		c.AllowedOrigins = strings.Split(v, ",")
	}
	if c.Production() {
		tn, tset := c.translatorKey()
		if err := missing([]required{
			{"DATABASE_URL", c.DatabaseURL != ""},
			{"SUPABASE_URL", c.SupabaseURL != ""}, // the JWKS
			{"SUPABASE_SECRET_KEY", c.SupabaseSecretKey != ""},
			{"SENTRY_DSN", c.SentryDSN != ""},
			{"GITHUB_TOKEN", c.GitHubToken != ""},             // propose-via-PR, uistrings.Follow
			{"GOOGLE_MAPS_API_KEY", c.GoogleMapsAPIKey != ""}, // venue names on plans
			{tn, tset},
		}); err != nil {
			return c, err
		}
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if c.SupabaseURL == "" {
		return c, fmt.Errorf("SUPABASE_URL is required")
	}
	return c, nil
}

// Worker is cmd/worker's configuration.
type Worker struct {
	Common
	// DatabaseURL must allow LISTEN/NOTIFY (River): a direct or
	// session-pooler connection, never the transaction pooler.
	DatabaseURL string
	// ExpoAccessToken is needed to send push notifications only when the
	// Expo project enables enhanced push security. Optional outside
	// production, where it is required so a misconfigured deploy fails at
	// startup instead of at the first push.
	ExpoAccessToken string
}

// LoadWorker reads WORKER_DATABASE_URL, falling back to DATABASE_URL (fine
// locally, where both are the same direct connection). Production needs
// every key below, or it fails naming them all.
func LoadWorker() (Worker, error) {
	common, err := loadCommon()
	if err != nil {
		return Worker{}, err
	}
	w := Worker{
		Common:          common,
		DatabaseURL:     getenv("WORKER_DATABASE_URL", os.Getenv("DATABASE_URL")),
		ExpoAccessToken: os.Getenv("EXPO_ACCESS_TOKEN"),
	}
	if w.Production() {
		tn, tset := w.translatorKey()
		if err := missing([]required{
			{"WORKER_DATABASE_URL or DATABASE_URL", w.DatabaseURL != ""},
			{"SENTRY_DSN", w.SentryDSN != ""},
			{"GOOGLE_MAPS_API_KEY", w.GoogleMapsAPIKey != ""},
			{"GOOGLE_GEMINI_API_KEY", w.GeminiAPIKey != ""}, // walks
			{"EXPO_ACCESS_TOKEN", w.ExpoAccessToken != ""},
			{"GITHUB_TOKEN", w.GitHubToken != ""},
			{tn, tset},
		}); err != nil {
			return w, err
		}
	}
	if w.DatabaseURL == "" {
		return w, fmt.Errorf("WORKER_DATABASE_URL or DATABASE_URL is required")
	}
	return w, nil
}

// Tool is cmd/tool's configuration. Nothing is required here: each
// subcommand checks the keys it uses, and `tool migrate` must run in
// production with whatever the container has.
type Tool struct {
	Common
	DatabaseURL string
	// SupabaseURL and SupabaseSecretKey are for set-admin and set-translator.
	SupabaseURL, SupabaseSecretKey string
	// APIURL is the API ui-bundle reads (API_URL).
	APIURL string
	// AnthropicAPIKey and GoogleMapsEmbedAPIKey are walk-bench's.
	AnthropicAPIKey, GoogleMapsEmbedAPIKey string
}

// LoadTool reads cmd/tool's configuration. It fails only on a bad APP_ENV.
func LoadTool() (Tool, error) {
	common, err := loadCommon()
	if err != nil {
		return Tool{}, err
	}
	return Tool{
		Common:                common,
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		SupabaseURL:           strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		SupabaseSecretKey:     os.Getenv("SUPABASE_SECRET_KEY"),
		APIURL:                os.Getenv("API_URL"),
		AnthropicAPIKey:       os.Getenv("ANTHROPIC_API_KEY"),
		GoogleMapsEmbedAPIKey: os.Getenv("GOOGLE_MAPS_EMBED_API_KEY"),
	}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
