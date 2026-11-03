package bound

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestCheckWarnsAtTheCapAndReportsOnce(t *testing.T) {
	var reports int
	defer func(old func(context.Context, error, map[string]string)) { report = old }(report)
	report = func(context.Context, error, map[string]string) { reports++ }

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	l := &List{Name: "ListFeed", Max: 3}
	ctx := context.Background()

	l.Check(ctx, log, 2)
	if buf.Len() != 0 || reports != 0 {
		t.Fatalf("below the cap: log %q, %d reports; want none", buf.String(), reports)
	}
	l.Check(ctx, log, 3, slog.String("city", "tokyo"))
	l.Check(ctx, log, 3)
	if got := strings.Count(buf.String(), "level=WARN"); got != 2 {
		t.Errorf("%d warnings, want one per hit:\n%s", got, buf.String())
	}
	for _, want := range []string{"rpc=ListFeed", "rows=3", "cap=3", "city=tokyo"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, buf.String())
		}
	}
	if reports != 1 {
		t.Errorf("%d Sentry reports, want 1 per process", reports)
	}
}
