package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/shrujancheruku/issho/backend/internal/platform/sentry"
)

// sentryTest sends one test error through the same reporting the API and
// worker use, to check SENTRY_DSN end to end (docs/deploy.md):
//
//	kamal app exec --primary "tool sentry-test"
func sentryTest() error {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return errors.New("SENTRY_DSN is not set")
	}
	flush, err := sentry.Init(dsn, version, os.Getenv("SENTRY_ENVIRONMENT"), "tool", 0)
	if err != nil {
		return err
	}
	sentry.Report(context.Background(), fmt.Errorf("sentry-test: a test error from tool (version %s)", version), map[string]string{"check": "sentry-test"})
	flush()
	slog.Info("sentry-test: sent one test error")
	return nil
}
