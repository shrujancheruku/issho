// Package logging builds the one logger a process logs through. Every package
// takes a *slog.Logger it is handed and never makes one of its own, so a test
// that builds a logger on a buffer captures a whole run's output.
package logging

import (
	"context"
	"io"
	"log/slog"
)

// New returns a logger that writes one JSON object per line to w, tagged with
// the command that owns the process ("api", "worker"). Only cmd/ calls it with
// os.Stdout; a test calls it with a buffer.
func New(w io.Writer, cmd string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, nil)).With("cmd", cmd)
}

// Discard is the logger for code that is given none: it keeps a package from
// reaching for slog's global default.
func Discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// WithoutErrors returns log with the "error" attribute dropped from every line.
// It is for a library that logs through a logger it is given (River, say) and
// reports a failure with the error's text, which can quote what a person gave
// us, an email in a failed send for one. The other attributes stay.
func WithoutErrors(log *slog.Logger) *slog.Logger {
	return slog.New(errorStripper{log.Handler()})
}

type errorStripper struct{ slog.Handler }

func (s errorStripper) Handle(ctx context.Context, r slog.Record) error {
	kept := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key != "error" {
			kept.AddAttrs(a)
		}
		return true
	})
	return s.Handler.Handle(ctx, kept)
}

func (s errorStripper) WithAttrs(attrs []slog.Attr) slog.Handler {
	return errorStripper{s.Handler.WithAttrs(attrs)}
}

func (s errorStripper) WithGroup(name string) slog.Handler {
	return errorStripper{s.Handler.WithGroup(name)}
}
