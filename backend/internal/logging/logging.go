// Package logging builds the one logger a process logs through. Every package
// takes a *slog.Logger it is handed and never makes one of its own, so a test
// that builds a logger on a buffer captures a whole run's output.
package logging

import (
	"context"
	"io"
	"log"
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

// Restricted is the logger for a library that logs through a logger it is
// given (River, say) and attaches what it likes: an error's text, a panic's
// value, which can quote what a person gave us. Each line keeps only the
// attributes named in keep, so one the library adds later is dropped rather
// than leaked. drop, when not nil, discards a whole line: it gets the line's
// level and reports whether the line, before filtering, has an attribute.
func Restricted(l *slog.Logger, keep []string, drop func(level slog.Level, has func(key string) bool) bool) *slog.Logger {
	kept := map[string]bool{}
	for _, k := range keep {
		kept[k] = true
	}
	return slog.New(restricted{Handler: l.Handler(), keep: kept, drop: drop})
}

type restricted struct {
	slog.Handler
	keep map[string]bool
	drop func(slog.Level, func(string) bool) bool
	// with is the keys the library attached with With, which drop sees too.
	with []string
}

func (h restricted) Handle(ctx context.Context, r slog.Record) error {
	if h.drop != nil {
		has := func(key string) bool {
			for _, k := range h.with {
				if k == key {
					return true
				}
			}
			found := false
			r.Attrs(func(a slog.Attr) bool {
				found = a.Key == key
				return !found
			})
			return found
		}
		if h.drop(r.Level, has) {
			return nil
		}
	}
	out := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if h.keep[a.Key] {
			out.AddAttrs(a)
		}
		return true
	})
	return h.Handler.Handle(ctx, out)
}

func (h restricted) WithAttrs(attrs []slog.Attr) slog.Handler {
	var kept []slog.Attr
	with := append([]string(nil), h.with...)
	for _, a := range attrs {
		with = append(with, a.Key)
		if h.keep[a.Key] {
			kept = append(kept, a)
		}
	}
	return restricted{Handler: h.Handler.WithAttrs(kept), keep: h.keep, drop: h.drop, with: with}
}

func (h restricted) WithGroup(name string) slog.Handler {
	return restricted{Handler: h.Handler.WithGroup(name), keep: h.keep, drop: h.drop, with: h.with}
}

// ServerErrorLog is the error log for an http.Server. Without one net/http
// writes to the standard logger, outside this helper, and its messages are free
// text that can name the client's address ("TLS handshake error from …"). Each
// message becomes a fixed "http server error" line that keeps none of the text.
func ServerErrorLog(l *slog.Logger) *log.Logger {
	return log.New(serverErrors{l}, "", 0)
}

type serverErrors struct{ log *slog.Logger }

func (s serverErrors) Write(p []byte) (int, error) {
	s.log.Warn("http server error")
	return len(p), nil
}
