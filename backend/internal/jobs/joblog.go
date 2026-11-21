package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/shrujancheruku/issho/backend/internal/clock"
)

// logJobs logs one line per job worked: its kind, its outcome and how long it
// took. Never its arguments, which hold what a person gave us, and never the
// error's text, which may quote them; the account's ID and the route are all a
// log may say about a person (docs/spec/operations.md, "Errors and logs").
type logJobs struct {
	river.MiddlewareDefaults
	log *slog.Logger
	clk clock.Clock
}

func (m *logJobs) Work(ctx context.Context, job *rivertype.JobRow, doInner func(context.Context) error) error {
	start := m.clk.Now()
	err := doInner(ctx)
	outcome := "done"
	if err != nil {
		outcome = "failed"
	}
	m.log.Info("job",
		"kind", job.Kind,
		"outcome", outcome,
		"duration_ms", float64(m.clk.Now().Sub(start))/float64(time.Millisecond),
	)
	return err
}
