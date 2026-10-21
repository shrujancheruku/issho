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
	// AnthropicAPIKey enables interest suggestions (Claude). Optional: without
	// it, CatalogService.SuggestActivities returns FAILED_PRECONDITION.
	AnthropicAPIKey string
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		SupabaseURL:           strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		APIAddr:               getenv("API_ADDR", ":8080"),
		SupabaseSecretKey:     os.Getenv("SUPABASE_SECRET_KEY"),
		GoogleTranslateAPIKey: os.Getenv("GOOGLE_TRANSLATE_API_KEY"),
		AnthropicAPIKey:       os.Getenv("ANTHROPIC_API_KEY"),
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

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
