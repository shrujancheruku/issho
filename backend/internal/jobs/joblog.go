package jobs

import (
	"context"
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/shrujancheruku/issho/backend/internal/logging"
)

// logJobs logs exactly one line per job worked: its kind and its outcome (done,
// failed or panicked). Never its arguments, which hold what a person gave us,
// nor the error's text or a panic's value, which may quote them; the account's
// ID and the route are all a log may say about a person
// (docs/spec/operations.md, "Errors and logs"). River's own lines about the job
// are dropped (see riverLog), so this is the only one.
type logJobs struct {
	river.MiddlewareDefaults
	log *slog.Logger
}

func (m *logJobs) Work(ctx context.Context, job *rivertype.JobRow, doInner func(context.Context) error) error {
	// Until doInner returns, the job is panicking: the deferred line still gets
	// written as the panic passes through to River, which recovers it.
	outcome := "panicked"
	defer func() { m.log.Info("job", "kind", job.Kind, "outcome", outcome) }()
	err := doInner(ctx)
	outcome = "done"
	if err != nil {
		outcome = "failed"
	}
	return err
}

// riverKeeps is every attribute of River's own lines that survives: which job,
// which kind, which queue, which client. River attaches others (error, err,
// panic_val, metadata) that can quote a job's arguments or what a person gave
// us, so the list says what stays rather than what goes.
var riverKeeps = []string{"job_id", "job_kind", "kind", "queue", "client_id"}

// riverLog is the logger River itself logs through. A line about one job below
// a warning ("Job errored", "Job panicked", "Job snoozed") repeats what logJobs
// says, and River's panic recovery line exists only to carry the panic's value,
// so both are dropped: a job has one line. River's warnings and errors about a
// job that are not its outcome (stuck, could not be marked complete) and its
// lines about the client stay, with only riverKeeps.
func riverLog(log *slog.Logger) *slog.Logger {
	return logging.Restricted(log, riverKeeps, func(level slog.Level, has func(key string) bool) bool {
		return has("panic_val") || (has("job_id") && level < slog.LevelWarn)
	})
}
