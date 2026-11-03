package worker

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// A worker without a Maps key must delete nothing. The pool is nil, so any
// query (the city list, a delete) would panic:
// returning cleanly proves the pass touched no data.
func TestRefreshMapsWithoutRouterTouchesNoData(t *testing.T) {
	var buf bytes.Buffer
	w := &refreshMapsWorker{log: slog.New(slog.NewTextHandler(&buf, nil))}
	if err := w.Work(context.Background(), nil); err != nil {
		t.Fatalf("Work: %v", err)
	}
	if !strings.Contains(buf.String(), "GOOGLE_MAPS_API_KEY") {
		t.Errorf("no error logged about the missing key: %q", buf.String())
	}
}
