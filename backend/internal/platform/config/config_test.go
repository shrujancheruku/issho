package config

import (
	"strings"
	"testing"
)

// allKeys is every variable the loaders read. Each test clears them all
// first (t.Setenv restores them after), so the developer's own environment
// never leaks in.
var allKeys = []string{
	"APP_ENV", "DATABASE_URL", "WORKER_DATABASE_URL", "SUPABASE_URL", "SUPABASE_SECRET_KEY",
	"API_ADDR", "ALLOWED_ORIGINS", "GOOGLE_TRANSLATE_API_KEY", "TRANSLATOR", "GOOGLE_GEMINI_API_KEY",
	"GOOGLE_MAPS_API_KEY", "GITHUB_TOKEN", "GITHUB_REPO", "GITHUB_API_URL", "SENTRY_DSN",
	"SENTRY_ENVIRONMENT", "SENTRY_TRACES_SAMPLE_RATE", "EXPO_ACCESS_TOKEN", "API_URL",
	"ANTHROPIC_API_KEY", "GOOGLE_MAPS_EMBED_API_KEY",
}

func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range allKeys {
		t.Setenv(k, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func apiEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://x", "SUPABASE_URL": "https://x.supabase.co/",
		"SUPABASE_SECRET_KEY": "sb_secret_x", "SENTRY_DSN": "https://k@s.io/1",
		"GITHUB_TOKEN": "gh", "GOOGLE_MAPS_API_KEY": "m", "GOOGLE_GEMINI_API_KEY": "g",
	}
}

func workerEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://x", "SENTRY_DSN": "https://k@s.io/1", "GOOGLE_MAPS_API_KEY": "m",
		"GOOGLE_GEMINI_API_KEY": "g", "EXPO_ACCESS_TOKEN": "e", "GITHUB_TOKEN": "gh",
	}
}

func without(m map[string]string, keys ...string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[k] = v
	}
	for _, k := range keys {
		delete(out, k)
	}
	return out
}

func with(m map[string]string, kv ...string) map[string]string {
	out := without(m)
	for i := 0; i < len(kv); i += 2 {
		out[kv[i]] = kv[i+1]
	}
	return out
}

func TestLoadProductionNamesEveryMissingKey(t *testing.T) {
	// Only DATABASE_URL and SUPABASE_URL are set, so everything else is named.
	setEnv(t, map[string]string{"APP_ENV": "production", "DATABASE_URL": "postgres://x", "SUPABASE_URL": "https://x"})
	_, err := Load()
	if err == nil {
		t.Fatal("production without its keys loaded")
	}
	for _, k := range []string{"SUPABASE_SECRET_KEY", "SENTRY_DSN", "GITHUB_TOKEN", "GOOGLE_MAPS_API_KEY", "GOOGLE_GEMINI_API_KEY"} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("error %q doesn't name %s", err, k)
		}
	}
	if strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error %q names a key that is set", err)
	}

	// The same environment loads in development, with the optional keys unset.
	setEnv(t, map[string]string{"DATABASE_URL": "postgres://x", "SUPABASE_URL": "https://x"})
	if _, err := Load(); err != nil {
		t.Errorf("development: %v", err)
	}
	setEnv(t, map[string]string{"APP_ENV": "preview", "DATABASE_URL": "postgres://x", "SUPABASE_URL": "https://x"})
	if _, err := Load(); err != nil {
		t.Errorf("preview: %v", err)
	}
}

func TestLoadProductionComplete(t *testing.T) {
	setEnv(t, with(apiEnv(), "APP_ENV", "production"))
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.SupabaseURL != "https://x.supabase.co" || !c.Production() {
		t.Errorf("got %+v", c)
	}
}

func TestLoadProductionTranslatorKey(t *testing.T) {
	env := with(without(apiEnv(), "GOOGLE_GEMINI_API_KEY"), "APP_ENV", "production")
	// The default translator is Gemini, so its key is named.
	setEnv(t, env)
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "GOOGLE_GEMINI_API_KEY") {
		t.Errorf("default translator: %v", err)
	}
	// "google" needs the Translation key instead, and no longer Gemini's.
	setEnv(t, with(env, "TRANSLATOR", "google"))
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "GOOGLE_TRANSLATE_API_KEY") ||
		strings.Contains(err.Error(), "GOOGLE_GEMINI_API_KEY") {
		t.Errorf("google translator: %v", err)
	}
	setEnv(t, with(env, "TRANSLATOR", "google", "GOOGLE_TRANSLATE_API_KEY", "k"))
	if _, err := Load(); err != nil {
		t.Errorf("google translator with its key: %v", err)
	}
}

func TestLoadWorkerProduction(t *testing.T) {
	setEnv(t, map[string]string{"APP_ENV": "production"})
	_, err := LoadWorker()
	if err == nil {
		t.Fatal("production without keys loaded")
	}
	for _, k := range []string{"DATABASE_URL", "SENTRY_DSN", "GOOGLE_MAPS_API_KEY", "GOOGLE_GEMINI_API_KEY", "EXPO_ACCESS_TOKEN", "GITHUB_TOKEN"} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("error %q doesn't name %s", err, k)
		}
	}
	// Gemini is both a key and the translator's: named once.
	if n := strings.Count(err.Error(), "GOOGLE_GEMINI_API_KEY"); n != 1 {
		t.Errorf("GOOGLE_GEMINI_API_KEY named %d times: %q", n, err)
	}

	setEnv(t, with(workerEnv(), "APP_ENV", "production"))
	if _, err := LoadWorker(); err != nil {
		t.Errorf("complete: %v", err)
	}
	// WORKER_DATABASE_URL stands in for DATABASE_URL.
	setEnv(t, with(without(workerEnv(), "DATABASE_URL"), "APP_ENV", "production", "WORKER_DATABASE_URL", "postgres://w"))
	if w, err := LoadWorker(); err != nil || w.DatabaseURL != "postgres://w" {
		t.Errorf("WORKER_DATABASE_URL: %v %q", err, w.DatabaseURL)
	}

	// Outside production the same bare environment loads.
	setEnv(t, map[string]string{"DATABASE_URL": "postgres://x"})
	if _, err := LoadWorker(); err != nil {
		t.Errorf("development: %v", err)
	}
	setEnv(t, nil)
	if _, err := LoadWorker(); err == nil {
		t.Error("no database URL loaded in development")
	}
}

func TestUnknownAppEnvFails(t *testing.T) {
	setEnv(t, with(apiEnv(), "APP_ENV", "prod"))
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "APP_ENV") {
		t.Errorf("Load: %v", err)
	}
	if _, err := LoadWorker(); err == nil {
		t.Error("LoadWorker accepted APP_ENV=prod")
	}
	if _, err := LoadTool(); err == nil {
		t.Error("LoadTool accepted APP_ENV=prod")
	}
}

func TestSentryEnvironmentFollowsAppEnv(t *testing.T) {
	for _, tc := range []struct{ appEnv, sentry, want string }{
		{"", "", "development"},
		{"preview", "", "preview"},
		{"production", "", "production"},
		{"preview", "staging", "staging"}, // an explicit value wins
	} {
		setEnv(t, with(apiEnv(), "APP_ENV", tc.appEnv, "SENTRY_ENVIRONMENT", tc.sentry))
		c, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if c.SentryEnvironment != tc.want {
			t.Errorf("%+v: got %q", tc, c.SentryEnvironment)
		}
	}
}

func TestLoadToolRequiresNothing(t *testing.T) {
	setEnv(t, map[string]string{"APP_ENV": "production"})
	if _, err := LoadTool(); err != nil {
		t.Errorf("tool in production without keys: %v", err)
	}
	setEnv(t, map[string]string{"SUPABASE_URL": "https://x/", "API_URL": "https://api"})
	c, err := LoadTool()
	if err != nil || c.SupabaseURL != "https://x" || c.APIURL != "https://api" || c.SentryEnvironment != "development" {
		t.Errorf("got %+v, %v", c, err)
	}
}
