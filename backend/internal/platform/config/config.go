// Package config loads process configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL string
	SupabaseURL string
	// SupabaseSecretKey (sb_secret_…) enables admin calls such as account
	// deletion. Optional: without it, DeleteAccount returns FAILED_PRECONDITION.
	SupabaseSecretKey string
	APIAddr           string
	// AllowedOrigins lists browser origins (the admin SPA) allowed to call the API.
	AllowedOrigins []string
	// GoogleTranslateAPIKey enables on-demand machine translation (Cloud
	// Translation). Optional: without it, text is shown in the languages
	// already stored.
	GoogleTranslateAPIKey string
	// GeminiAPIKey (a Google Cloud Agent Platform key) enables interest
	// suggestions (Gemini). Optional: without it,
	// CatalogService.SuggestActivities returns FAILED_PRECONDITION.
	GeminiAPIKey string
	// GoogleMapsAPIKey enables venue names and addresses on plans (Places
	// Details, fetched live). Optional: without it, plans show no venue
	// names.
	GoogleMapsAPIKey string
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		SupabaseURL:           strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		APIAddr:               getenv("API_ADDR", ":8080"),
		SupabaseSecretKey:     os.Getenv("SUPABASE_SECRET_KEY"),
		GoogleTranslateAPIKey: os.Getenv("GOOGLE_TRANSLATE_API_KEY"),
		GeminiAPIKey:          os.Getenv("GOOGLE_GEMINI_API_KEY"),
		GoogleMapsAPIKey:      os.Getenv("GOOGLE_MAPS_API_KEY"),
	}
	if v := os.Getenv("ALLOWED_ORIGINS"); v != "" {
		c.AllowedOrigins = strings.Split(v, ",")
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
	// DatabaseURL must allow LISTEN/NOTIFY (River): a direct or
	// session-pooler connection, never the transaction pooler.
	DatabaseURL string
	// ExpoAccessToken is needed to send push notifications only when the
	// Expo project enables enhanced push security. Optional.
	ExpoAccessToken string
	// GoogleTranslateAPIKey translates notification text and plan titles
	// into each person's language. Optional: without it they're English.
	GoogleTranslateAPIKey string
	// GoogleMapsAPIKey rebuilds travel-time matrices before the Maps
	// caching limit and checks venues are open before scheduling. Optional:
	// without it, stale travel times are deleted instead of rebuilt and
	// opening hours aren't checked.
	GoogleMapsAPIKey string
	// GeminiAPIKey (a Google Cloud Agent Platform key), with
	// GoogleMapsAPIKey, composes walking plans (internal/walks). Optional:
	// without both, no walks are opened.
	GeminiAPIKey string
}

// LoadWorker reads WORKER_DATABASE_URL, falling back to DATABASE_URL (fine
// locally, where both are the same direct connection).
func LoadWorker() (Worker, error) {
	w := Worker{
		DatabaseURL:           getenv("WORKER_DATABASE_URL", os.Getenv("DATABASE_URL")),
		ExpoAccessToken:       os.Getenv("EXPO_ACCESS_TOKEN"),
		GoogleTranslateAPIKey: os.Getenv("GOOGLE_TRANSLATE_API_KEY"),
		GoogleMapsAPIKey:      os.Getenv("GOOGLE_MAPS_API_KEY"),
		GeminiAPIKey:          os.Getenv("GOOGLE_GEMINI_API_KEY"),
	}
	if w.DatabaseURL == "" {
		return w, fmt.Errorf("WORKER_DATABASE_URL or DATABASE_URL is required")
	}
	return w, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
