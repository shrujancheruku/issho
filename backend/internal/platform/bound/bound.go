// Package bound caps the app's unpaged lists (ListFeed, ListMyEvents,
// ListMyCrews). These lists return everything a screen needs and have no
// paging fields before launch, because each is bounded by what it holds: a
// city's open plans, a person's recent events, their live crews. The cap is
// the backstop: generous enough never to cut a real list, and loud when one
// gets near it, so growth shows up as a warning before anyone sees a list
// cut short. The response shape never changes when a cap is hit, so builds
// already installed keep working.
package bound

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/shrujancheruku/issho/backend/internal/platform/sentry"
)

// report is sentry.Report; tests replace it.
var report = sentry.Report

// List is one capped list. Declare it once per list (a package variable):
// the Sentry report goes out once per process per List.
type List struct {
	// Name is the RPC, e.g. "ListFeed"; it tags the log line and report.
	Name string
	// Max is the most rows the query may return (its `limit`).
	Max int32

	once sync.Once
}

// Limit is Max as the *int32 the queries take for `limit sqlc.narg(max)`
// (nil there means no limit: the worker and single-row lookups read the same
// queries uncapped).
func (l *List) Limit() *int32 { return &l.Max }

// Check says a list of n rows was served. At the cap the list may have been
// cut short, so it logs a warning every time (the count and a scope such as
// the city, never user ids or content) and reports to Sentry once per
// process. A nil log uses slog.Default().
func (l *List) Check(ctx context.Context, log *slog.Logger, n int, scope ...slog.Attr) {
	if n < int(l.Max) {
		return
	}
	if log == nil {
		log = slog.Default()
	}
	attrs := append([]slog.Attr{slog.String("rpc", l.Name), slog.Int("rows", n), slog.Int("cap", int(l.Max))}, scope...)
	log.LogAttrs(ctx, slog.LevelWarn, "list hit its cap and may be cut short", attrs...)
	l.once.Do(func() {
		report(ctx, fmt.Errorf("%s hit its cap of %d rows", l.Name, l.Max), map[string]string{"rpc": l.Name})
	})
}
